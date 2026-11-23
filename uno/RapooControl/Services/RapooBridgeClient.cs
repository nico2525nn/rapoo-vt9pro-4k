using System.Collections.Concurrent;
using System.Diagnostics;
using System.Text;
using System.Text.Json;
using System.Text.Json.Serialization;
using System.Threading.Channels;

namespace RapooControl.Services;

/// <summary>
/// Owns the Go sidecar process and speaks newline-delimited JSON over its
/// stdin/stdout. All HID access stays inside the sidecar.
///
/// Each process start is a <see cref="BridgeSession"/>. A session owns its
/// pending requests and readiness gate, so a late reply from an old process can
/// never satisfy a request belonging to a newer one. Teardown is shared and
/// confirms process exit before the session is cleared.
/// </summary>
public sealed class RapooBridgeClient : IAsyncDisposable
{
    private static readonly JsonSerializerOptions JsonOptions = new()
    {
        PropertyNameCaseInsensitive = true,
    };

    // The Go JSON decoder rejects a UTF-8 BOM, so stdin must not emit one.
    private static readonly Encoding Utf8NoBom = new UTF8Encoding(encoderShouldEmitUTF8Identifier: false);
    private const string BridgeProtocolVersion = "1";
    private static readonly TimeSpan ReadyTimeout = TimeSpan.FromSeconds(10);
    private static readonly TimeSpan RequestTimeout = TimeSpan.FromSeconds(15);

    private readonly SemaphoreSlim _lifecycle = new(1, 1);
    private readonly SemaphoreSlim _writeLock = new(1, 1);
    private readonly object _gate = new();
    private readonly object _eventGate = new();
    private readonly Channel<BridgeNotification> _notifications =
        Channel.CreateUnbounded<BridgeNotification>(new UnboundedChannelOptions
        {
            SingleReader = true,
            SingleWriter = false,
        });
    private readonly CancellationTokenSource _notificationCts = new();
    private long _nextSessionId;
    private long _wireSequence;
    private volatile BridgeSession? _session;
    private Task? _startTask;
    private Task? _disposeTask;
    private int _nextId;
    private int _disposed;

    public RapooBridgeClient()
    {
        _ = Task.Run(ProcessNotificationsAsync, CancellationToken.None);
    }

    public event EventHandler<BridgeStateEvent>? StateChanged;

    public event EventHandler<BridgeFault>? Faulted;

    public long CurrentSessionId => _session?.Id ?? 0;

    public bool IsSessionCurrent(long sessionId)
    {
        var session = _session;
        return session is { CanAcceptRequests: true } && session.Id == sessionId;
    }

    public long CurrentWireSequence => Interlocked.Read(ref _wireSequence);

    public bool IsRunning
    {
        get
        {
            // Lock-free: a StateChanged subscriber may call this while the
            // event is being delivered under the session gate.
            var session = _session;
            return session is { CanAcceptRequests: true };
        }
    }

    public Task StartAsync(CancellationToken cancellationToken = default)
    {
        ThrowIfDisposed();
        lock (_gate)
        {
            if (_startTask is { IsCompleted: false })
            {
                return _startTask;
            }
            if (_session is { CanAcceptRequests: true } session &&
                session.Ready.Task.IsCompletedSuccessfully)
            {
                return Task.CompletedTask;
            }
            _startTask = StartSerializedAsync(cancellationToken);
            return _startTask;
        }
    }

    private async Task StartSerializedAsync(CancellationToken cancellationToken)
    {
        await _lifecycle.WaitAsync(cancellationToken).ConfigureAwait(false);
        try
        {
            ThrowIfDisposed();
            var previous = GetSession();
            if (previous is not null)
            {
                await StopSessionAsync(previous).ConfigureAwait(false);
            }

            ThrowIfDisposed();
            await StartSessionAsync(cancellationToken).ConfigureAwait(false);
        }
        finally
        {
            _lifecycle.Release();
        }
    }

    private async Task StartSessionAsync(CancellationToken cancellationToken)
    {
        ThrowIfDisposed();
        var sidecar = FindSidecar(out var isOverride);
        if (sidecar is null)
        {
            throw new FileNotFoundException(
                LocalizationService.Instance.T(
                    "rapoo-bridge が見つかりません。アプリと同じフォルダに配置してください。"));
        }

        if (!isOverride)
        {
            MakeExecutable(sidecar);
        }
        Log($"starting {sidecar}");
        var cts = new CancellationTokenSource();
        var process = new Process
        {
            StartInfo = new ProcessStartInfo
            {
                FileName = sidecar,
                WorkingDirectory = Path.GetDirectoryName(sidecar) ?? AppContext.BaseDirectory,
                RedirectStandardInput = true,
                RedirectStandardOutput = true,
                RedirectStandardError = true,
                StandardInputEncoding = Utf8NoBom,
                StandardOutputEncoding = Encoding.UTF8,
                StandardErrorEncoding = Encoding.UTF8,
                UseShellExecute = false,
                CreateNoWindow = true,
            },
            EnableRaisingEvents = true,
        };
        var session = new BridgeSession(
            process,
            cts,
            Interlocked.Increment(ref _nextSessionId));
        process.Exited += (_, _) => HandleProcessExited(session);
        try
        {
            process.Start();
        }
        catch
        {
            process.Dispose();
            cts.Dispose();
            throw;
        }

        lock (_gate)
        {
            _session = session;
        }
        if (Volatile.Read(ref _disposed) != 0)
        {
            await StopAfterStartFailureAsync(session).ConfigureAwait(false);
            throw new ObjectDisposedException(nameof(RapooBridgeClient));
        }

        _ = Task.Run(() => ReadLoopAsync(session), CancellationToken.None);
        _ = Task.Run(() => ReadStderrAsync(session), CancellationToken.None);

        try
        {
            using var readyCts = CancellationTokenSource.CreateLinkedTokenSource(
                cancellationToken,
                session.Cts.Token);
            await session.Ready.Task
                .WaitAsync(ReadyTimeout, readyCts.Token)
                .ConfigureAwait(false);
            ThrowIfDisposed();
        }
        catch (TimeoutException)
        {
            await StopAfterStartFailureAsync(session).ConfigureAwait(false);
            throw new TimeoutException(
                LocalizationService.Instance.T("rapoo-bridge が応答しませんでした。"));
        }
        catch
        {
            await StopAfterStartFailureAsync(session).ConfigureAwait(false);
            throw;
        }
    }

    private async Task StopAfterStartFailureAsync(BridgeSession session)
    {
        try
        {
            await StopSessionAsync(session).ConfigureAwait(false);
        }
        catch (Exception ex)
        {
            // Preserve the startup failure; the stop failure is diagnostic.
            Log($"stop after start failure: {ex.Message}");
        }
    }

    public Task StopAsync()
    {
        if (Volatile.Read(ref _disposed) != 0)
        {
            return Task.CompletedTask;
        }
        GetSession()?.Cancel();
        return StopSerializedAsync();
    }

    private async Task StopSerializedAsync()
    {
        await _lifecycle.WaitAsync().ConfigureAwait(false);
        try
        {
            var session = GetSession();
            if (session is not null)
            {
                await StopSessionAsync(session).ConfigureAwait(false);
            }
        }
        finally
        {
            _lifecycle.Release();
        }
    }

    private async Task StopSessionAsync(BridgeSession session)
    {
        Task stopTask;
        lock (session.Gate)
        {
            // Run the teardown outside the session gate. StopSessionCore
            // takes the client gate; creating it while holding a gate that a
            // concurrent Dispose also waits on can deadlock.
            stopTask = session.StopTask ??= Task.Run(
                () => StopSessionCoreAsync(session),
                CancellationToken.None);
        }
        await stopTask.ConfigureAwait(false);
    }

    private async Task StopSessionCoreAsync(BridgeSession session)
    {
        Volatile.Write(ref session.Stopping, 1);
        session.Cancel();
        CompleteSession(
            session,
            new InvalidOperationException(
                LocalizationService.Instance.T("rapoo-bridge が終了しました。")));

        try
        {
            session.Process.StandardInput.Close();
        }
        catch
        {
            // The process may already have exited.
        }

        var exited = HasExited(session.Process);
        if (!exited)
        {
            exited = WaitForExit(session.Process, TimeSpan.FromMilliseconds(2500));
        }
        if (!exited)
        {
            try
            {
                // The official firmware updater is a child of the bridge.
                // Kill only the bridge so an in-flight firmware write can
                // finish after the UI exits.
                session.Process.Kill(entireProcessTree: false);
            }
            catch (Exception ex)
            {
                Log($"kill failed: {ex.Message}");
            }
            exited = WaitForExit(session.Process, TimeSpan.FromSeconds(3));
        }
        if (!exited)
        {
            lock (session.Gate)
            {
                session.StopTask = null;
            }
            throw new InvalidOperationException(
                LocalizationService.Instance.T("rapoo-bridge のプロセスを終了できませんでした。"));
        }

        // Drain requests admitted while the process was shutting down.
        CompleteSession(
            session,
            new InvalidOperationException(
                LocalizationService.Instance.T("rapoo-bridge が終了しました。")));

        lock (_gate)
        {
            if (ReferenceEquals(_session, session))
            {
                _session = null;
            }
        }
        session.Process.Dispose();
        session.Cts.Dispose();
    }

    public async Task<BridgeState> GetStateAsync(CancellationToken cancellationToken = default) =>
        (await GetStateWithSequenceAsync(cancellationToken).ConfigureAwait(false)).State;

    public async Task<BridgeStateEvent> GetStateWithSequenceAsync(CancellationToken cancellationToken = default)
    {
        var reply = await RequestElementAsync("state", null, cancellationToken).ConfigureAwait(false);
        return new BridgeStateEvent(
            reply.SessionId,
            DeserializeReply<BridgeState>(reply, "state"),
            reply.Sequence);
    }

    public Task<BridgeSnapshot> GetSnapshotAsync(CancellationToken cancellationToken = default) =>
        RequestAsync<BridgeSnapshot>("snapshot", null, cancellationToken);

    public Task<BridgeSnapshot> SetDemoModeAsync(bool enabled, CancellationToken cancellationToken = default) =>
        RequestAsync<BridgeSnapshot>("demo", new { enabled }, cancellationToken);

    public Task<BridgeState> SetLanguageAsync(string language, CancellationToken cancellationToken = default) =>
        RequestAsync<BridgeState>("setLanguage", new { language }, cancellationToken);

    public async Task<BridgeState> SetPollingModeAsync(
        string mode,
        CancellationToken cancellationToken = default) =>
        (await SetPollingModeWithSequenceAsync(mode, cancellationToken).ConfigureAwait(false)).State;

    public async Task<BridgeStateEvent> SetPollingModeWithSequenceAsync(
        string mode,
        CancellationToken cancellationToken = default)
    {
        var reply = await RequestElementAsync("pollingMode", new { mode }, cancellationToken).ConfigureAwait(false);
        return new BridgeStateEvent(
            reply.SessionId,
            DeserializeReply<BridgeState>(reply, "pollingMode"),
            reply.Sequence);
    }

    public Task<BridgeSnapshot> SetPollingRateAsync(int hz, CancellationToken cancellationToken = default) =>
        RequestAsync<BridgeSnapshot>("pollingRate", new { hz }, cancellationToken);

    public Task<BridgeSnapshot> SetDpiConfigAsync(
        int stageCount,
        int currentStage,
        IReadOnlyList<BridgeDpiStage> stages,
        CancellationToken cancellationToken = default) =>
        RequestAsync<BridgeSnapshot>(
            "dpiConfig",
            new
            {
                config = new
                {
                    currentStage,
                    stageCount,
                    independentXY = stages.Any(stage => stage.X != stage.Y),
                    stages,
                },
            },
            cancellationToken);

    public Task<BridgeSnapshot> ApplySensorAsync(object patch, CancellationToken cancellationToken = default) =>
        RequestAsync<BridgeSnapshot>("sensor", new { patch }, cancellationToken);

    public Task<BridgeSnapshot> SetButtonAsync(
        int buttonId,
        object action,
        CancellationToken cancellationToken = default) =>
        RequestAsync<BridgeSnapshot>("button", new { buttonId, action }, cancellationToken);

    public Task<BridgeSnapshot> ResetButtonsAsync(CancellationToken cancellationToken = default) =>
        RequestAsync<BridgeSnapshot>("resetButtons", null, cancellationToken);

    public Task<BridgeSnapshot> FactoryResetAsync(CancellationToken cancellationToken = default) =>
        RequestAsync<BridgeSnapshot>("factoryReset", null, cancellationToken);

    public Task<BridgeCatalog> GetCatalogAsync(CancellationToken cancellationToken = default) =>
        RequestAsync<BridgeCatalog>("catalog", null, cancellationToken);

    public Task<List<BridgePreset>> ListPresetsAsync(CancellationToken cancellationToken = default) =>
        RequestAsync<List<BridgePreset>>("presets", null, cancellationToken);

    public Task<BridgePreset> SavePresetAsync(
        string id,
        string name,
        string description,
        CancellationToken cancellationToken = default) =>
        RequestAsync<BridgePreset>("savePreset", new { id, name, description }, cancellationToken);

    public Task<BridgeSnapshot> ApplyPresetAsync(string id, CancellationToken cancellationToken = default) =>
        RequestAsync<BridgeSnapshot>("applyPreset", new { id }, cancellationToken);

    public Task DeletePresetAsync(string id, CancellationToken cancellationToken = default) =>
        RequestVoidAsync("deletePreset", new { id }, cancellationToken);

    public Task<string> OpenFirmwareUpdaterAsync(string? path = null, CancellationToken cancellationToken = default) =>
        RequestAsync<string>("openUpdater", new { path = path ?? string.Empty }, cancellationToken);

    public async ValueTask DisposeAsync()
    {
        Task disposeTask;
        lock (_gate)
        {
            if (_disposeTask is null)
            {
                Volatile.Write(ref _disposed, 1);
                // DisposeCore acquires _lifecycle and may need _gate to clear
                // the session. Starting it under _gate would deadlock with a
                // start that is already waiting for the lifecycle semaphore.
                _disposeTask = Task.Run(DisposeCoreAsync, CancellationToken.None);
            }
            disposeTask = _disposeTask;
        }
        try
        {
            await disposeTask.ConfigureAwait(false);
        }
        finally
        {
            _notificationCts.Cancel();
            _notifications.Writer.TryComplete();
        }
    }

    private async Task DisposeCoreAsync()
    {
        // Interrupt a startup that is waiting for the ready event. StartSerialized
        // also rechecks disposal after it stops a previous session.
        GetSession()?.Cancel();
        await _lifecycle.WaitAsync().ConfigureAwait(false);
        try
        {
            var session = GetSession();
            if (session is not null)
            {
                await StopSessionAsync(session).ConfigureAwait(false);
            }
        }
        finally
        {
            _lifecycle.Release();
        }
    }

    private async Task<T> RequestAsync<T>(string method, object? parameters, CancellationToken cancellationToken)
    {
        var reply = await RequestElementAsync(method, parameters, cancellationToken).ConfigureAwait(false);
        return DeserializeReply<T>(reply, method);
    }

    private T DeserializeReply<T>(BridgeReply reply, string method)
    {
        try
        {
            var result = reply.Element.Deserialize<T>(JsonOptions);
            if (result is null)
            {
                throw new InvalidOperationException($"rapoo-bridge: {method} returned an empty result.");
            }
            return result;
        }
        catch (Exception ex)
        {
            Log($"decode {method} failed: {ex}\nraw={reply.Element.GetRawText()}");
            throw new InvalidOperationException(
                LocalizationService.Instance.Format(
                    "rapoo-bridge: {0} の応答を解釈できません: {1}",
                    method,
                    ex.Message),
                ex);
        }
    }

    private async Task RequestVoidAsync(string method, object? parameters, CancellationToken cancellationToken)
    {
        _ = await RequestElementAsync(method, parameters, cancellationToken).ConfigureAwait(false);
    }

    private async Task<BridgeReply> RequestElementAsync(
        string method,
        object? parameters,
        CancellationToken cancellationToken)
    {
        ThrowIfDisposed();
        var session = GetSession() ??
            throw new InvalidOperationException(
                LocalizationService.Instance.T("rapoo-bridge が起動していません。"));
        if (!session.CanAcceptRequests)
        {
            throw new InvalidOperationException(
                LocalizationService.Instance.T("rapoo-bridge が起動していません。"));
        }

        using var timeout = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);
        timeout.CancelAfter(RequestTimeout);
        var token = timeout.Token;

        if (!session.Ready.Task.IsCompletedSuccessfully)
        {
            using var readyCts = CancellationTokenSource.CreateLinkedTokenSource(
                token,
                session.Cts.Token);
            try
            {
                await session.Ready.Task
                    .WaitAsync(ReadyTimeout, readyCts.Token)
                    .ConfigureAwait(false);
            }
            catch (OperationCanceledException) when (!cancellationToken.IsCancellationRequested)
            {
                throw new InvalidOperationException(
                    LocalizationService.Instance.T("rapoo-bridge が起動していません。"));
            }
        }

        lock (_gate)
        {
            if (!ReferenceEquals(_session, session) || !session.CanAcceptRequests)
            {
                throw new InvalidOperationException(
                    LocalizationService.Instance.T("rapoo-bridge が起動していません。"));
            }
        }

        var id = Interlocked.Increment(ref _nextId);
        var completion = new TaskCompletionSource<BridgeReply>(TaskCreationOptions.RunContinuationsAsynchronously);
        var payload = JsonSerializer.Serialize(new BridgeRequest
        {
            Id = id,
            Method = method,
            Params = parameters,
        }, JsonOptions);

        lock (session.Gate)
        {
            if (!session.CanAcceptRequests)
            {
                throw new InvalidOperationException(
                    LocalizationService.Instance.T("rapoo-bridge が起動していません。"));
            }
            session.Pending[id] = completion;
        }

        try
        {
            await _writeLock.WaitAsync(token).ConfigureAwait(false);
            try
            {
                await session.Process.StandardInput.WriteAsync(payload.AsMemory(), token).ConfigureAwait(false);
                await session.Process.StandardInput.WriteLineAsync().ConfigureAwait(false);
                await session.Process.StandardInput.FlushAsync(token).ConfigureAwait(false);
            }
            finally
            {
                _writeLock.Release();
            }

            using var registration = token.Register(() =>
            {
                if (session.Pending.TryRemove(id, out var pending))
                {
                    pending.TrySetCanceled(token);
                }
            });

            var reply = await completion.Task.ConfigureAwait(false);
            if (Volatile.Read(ref session.Faulted) != 0 ||
                Volatile.Read(ref session.Stopping) != 0 ||
                Volatile.Read(ref session.Exited) != 0 ||
                !ReferenceEquals(_session, session))
            {
                throw new InvalidOperationException(
                    LocalizationService.Instance.T("rapoo-bridge が起動していません。"));
            }
            return reply;
        }
        catch (OperationCanceledException) when (!cancellationToken.IsCancellationRequested)
        {
            session.Pending.TryRemove(id, out _);
            throw new TimeoutException(
                LocalizationService.Instance.Format(
                    "rapoo-bridge: {0} が15秒以内に応答しませんでした。",
                    method));
        }
        catch
        {
            session.Pending.TryRemove(id, out _);
            throw;
        }
    }

    private async Task ReadLoopAsync(BridgeSession session)
    {
        try
        {
            while (!session.Cts.IsCancellationRequested)
            {
                var line = await session.Process.StandardOutput
                    .ReadLineAsync(session.Cts.Token)
                    .ConfigureAwait(false);
                if (line is null)
                {
                    FaultSession(
                        session,
                        LocalizationService.Instance.T("rapoo-bridge が出力を閉じました。"));
                    return;
                }
                if (string.IsNullOrWhiteSpace(line))
                {
                    continue;
                }

                var sequence = Interlocked.Increment(ref _wireSequence);
                try
                {
                    using var document = JsonDocument.Parse(line);
                    var root = document.RootElement;
                    if (root.TryGetProperty("event", out var eventName))
                    {
                        HandleEvent(session, eventName.GetString(), root, sequence);
                        continue;
                    }

                    if (!root.TryGetProperty("id", out var idElement) ||
                        !idElement.TryGetInt32(out var id))
                    {
                        Log("ignoring message without an id");
                        continue;
                    }

                    string? error = null;
                    if (root.TryGetProperty("error", out var errorElement))
                    {
                        error = errorElement.ValueKind == JsonValueKind.String
                            ? errorElement.GetString()
                            : errorElement.GetRawText();
                    }
                    var hasResult = root.TryGetProperty("result", out var resultElement);
                    if (!session.Pending.TryRemove(id, out var completion))
                    {
                        continue;
                    }

                    if (error is not null)
                    {
                        completion.TrySetException(new RapooBridgeException(error));
                    }
                    else if (hasResult)
                    {
                        completion.TrySetResult(
                            new BridgeReply(resultElement.Clone(), session.Id, sequence));
                    }
                    else
                    {
                        // The Go bridge omits a nil result for void calls such
                        // as deletePreset; the id is a valid acknowledgement.
                        completion.TrySetResult(new BridgeReply(default, session.Id, sequence));
                    }
                }
                catch (JsonException ex)
                {
                    Log($"ignoring malformed bridge message: {ex.Message}");
                }
            }
        }
        catch (OperationCanceledException)
        {
            // Normal shutdown.
        }
        catch (Exception ex)
        {
            FaultSession(session, ex.Message);
        }
    }

    private void HandleEvent(BridgeSession session, string? name, JsonElement root, long sequence)
    {
        lock (_gate)
        {
            if (!ReferenceEquals(_session, session))
            {
                return;
            }
        }

        if (name == "ready")
        {
            if (root.TryGetProperty("data", out var readyData) &&
                readyData.TryGetProperty("version", out var version) &&
                string.Equals(version.GetString(), BridgeProtocolVersion, StringComparison.Ordinal))
            {
                session.Ready.TrySetResult(true);
            }
            else
            {
                FaultSession(
                    session,
                    LocalizationService.Instance.T(
                        "rapoo-bridge のプロトコルバージョンが一致しません。アプリを再インストールしてください。"));
            }
            return;
        }

        if (name != "state" ||
            !root.TryGetProperty("data", out var data))
        {
            return;
        }

        BridgeState? state;
        try
        {
            state = data.Deserialize<BridgeState>(JsonOptions);
        }
        catch (Exception ex)
        {
            Log($"state event decode failed: {ex}");
            return;
        }
        if (state is null)
        {
            return;
        }

        // Queue the event; delivery happens outside all session locks so a
        // subscriber can safely call GetStateAsync, StopAsync, or IsRunning.
        lock (_eventGate)
        {
            if (Volatile.Read(ref session.Stopping) != 0 ||
                Volatile.Read(ref session.Faulted) != 0 ||
                _session?.Id != session.Id)
            {
                return;
            }
            _notifications.Writer.TryWrite(
                new BridgeNotification(
                    session.Id,
                    new BridgeStateEvent(session.Id, state, sequence),
                    null,
                    sequence));
        }
    }

    private void HandleProcessExited(BridgeSession session)
    {
        if (Volatile.Read(ref session.Stopping) != 0)
        {
            return;
        }

        Volatile.Write(ref session.Exited, 1);
        int exitCode;
        try
        {
            exitCode = session.Process.ExitCode;
        }
        catch
        {
            exitCode = -1;
        }
        var message = LocalizationService.Instance.Format(
            "rapoo-bridge が終了しました (code {0}).",
            exitCode);
        Log(message);
        FaultSession(session, message);
    }

    private void FaultSession(BridgeSession session, string message)
    {
        var sequence = CurrentWireSequence;
        lock (_eventGate)
        {
            if (Volatile.Read(ref session.Stopping) != 0 ||
                Interlocked.Exchange(ref session.Faulted, 1) != 0)
            {
                return;
            }

            _notifications.Writer.TryWrite(
                new BridgeNotification(session.Id, null, message, sequence));
        }

        // Complete outside the event gate: CompleteSession takes the session
        // gate, and request admission may hold that gate while Process.Exited
        // synchronously re-enters here.
        CompleteSession(session, new InvalidOperationException(message));
    }

    private void CompleteSession(BridgeSession session, Exception error)
    {
        lock (session.Gate)
        {
            session.Ready.TrySetException(error);
            foreach (var pending in session.Pending.Values)
            {
                pending.TrySetException(error);
            }
            session.Pending.Clear();
        }
    }

    private async Task ProcessNotificationsAsync()
    {
        try
        {
            await foreach (var notification in _notifications.Reader
                .ReadAllAsync(_notificationCts.Token)
                .ConfigureAwait(false))
            {
                var session = _session;
                if (session is null || session.Id != notification.SessionId)
                {
                    continue;
                }

                try
                {
                    if (notification.State is { } state)
                    {
                        StateChanged?.Invoke(this, state);
                    }
                    else if (notification.Fault is { } fault)
                    {
                        Faulted?.Invoke(this, new BridgeFault(
                            notification.SessionId,
                            fault,
                            notification.Sequence));
                    }
                }
                catch (Exception ex)
                {
                    // One faulty subscriber must not stop the client from
                    // delivering subsequent state/fault notifications.
                    Log($"notification subscriber failed: {ex}");
                }
            }
        }
        catch (OperationCanceledException)
        {
        }
        catch (Exception ex)
        {
            Log($"notification loop failed: {ex}");
        }
    }

    private BridgeSession? GetSession()
    {
        lock (_gate)
        {
            return _session;
        }
    }

    private void ThrowIfDisposed()
    {
        ObjectDisposedException.ThrowIf(Volatile.Read(ref _disposed) != 0, this);
    }

    private static bool HasExited(Process process)
    {
        try
        {
            return process.HasExited;
        }
        catch
        {
            // An exception is not proof of exit; keep trying to stop it.
            return false;
        }
    }

    private static bool WaitForExit(Process process, TimeSpan timeout)
    {
        try
        {
            return process.WaitForExit((int)timeout.TotalMilliseconds);
        }
        catch
        {
            return false;
        }
    }

    private async Task ReadStderrAsync(BridgeSession session)
    {
        try
        {
            while (!session.Cts.IsCancellationRequested)
            {
                var line = await session.Process.StandardError
                    .ReadLineAsync(session.Cts.Token)
                    .ConfigureAwait(false);
                if (line is null)
                {
                    break;
                }
                Log($"sidecar: {line}");
            }
        }
        catch (OperationCanceledException)
        {
        }
        catch (Exception ex)
        {
            Log($"stderr loop failed: {ex.Message}");
        }
    }

    private static void MakeExecutable(string path)
    {
        if (OperatingSystem.IsWindows())
        {
            return;
        }

        try
        {
            File.SetUnixFileMode(path,
                UnixFileMode.UserRead | UnixFileMode.UserWrite | UnixFileMode.UserExecute |
                UnixFileMode.GroupRead | UnixFileMode.GroupExecute |
                UnixFileMode.OtherRead | UnixFileMode.OtherExecute);
        }
        catch
        {
            // The file may already be executable or on a read-only mount.
        }
    }

    private static string? FindSidecar(out bool isOverride)
    {
        isOverride = false;
        var overridePath = Environment.GetEnvironmentVariable("RAPOO_BRIDGE_PATH");
        if (!string.IsNullOrWhiteSpace(overridePath) && File.Exists(overridePath))
        {
            isOverride = true;
            return overridePath;
        }

        var sidecarName = OperatingSystem.IsWindows() ? "rapoo-bridge.exe" : "rapoo-bridge";
        var local = Path.Combine(AppContext.BaseDirectory, sidecarName);
        if (File.Exists(local))
        {
            return local;
        }

#if DEBUG
        var directory = new DirectoryInfo(AppContext.BaseDirectory);
        for (var i = 0; i < 9 && directory is not null; i++)
        {
            var candidate = Path.Combine(directory.FullName, "build", "bin", sidecarName);
            if (File.Exists(candidate))
            {
                return candidate;
            }
            directory = directory.Parent;
        }
#endif

        return null;
    }

    private static readonly object LogGate = new();

    private static void Log(string message)
    {
        try
        {
            var dir = Path.Combine(
                Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
                "rapoo-vt9pro-4k");
            Directory.CreateDirectory(dir);
            lock (LogGate)
            {
                File.AppendAllText(
                    Path.Combine(dir, "winui-bridge.log"),
                    $"{DateTimeOffset.Now:O} {message}{Environment.NewLine}");
            }
        }
        catch
        {
            // Diagnostics must never break the app.
        }
    }

    private sealed class BridgeSession(
        Process process,
        CancellationTokenSource cts,
        long id)
    {
        public Process Process { get; } = process;
        public CancellationTokenSource Cts { get; } = cts;
        public long Id { get; } = id;
        public TaskCompletionSource<bool> Ready { get; } =
            new(TaskCreationOptions.RunContinuationsAsynchronously);
        public ConcurrentDictionary<int, TaskCompletionSource<BridgeReply>> Pending { get; } = new();
        public object Gate { get; } = new();
        public Task? StopTask { get; set; }
        public int Stopping;
        public int Faulted;
        public int Exited;

        public void Cancel()
        {
            try
            {
                Cts.Cancel();
            }
            catch (ObjectDisposedException)
            {
                // A concurrent teardown already disposed the source.
            }
        }

        public bool CanAcceptRequests =>
            Volatile.Read(ref Stopping) == 0 &&
            Volatile.Read(ref Faulted) == 0 &&
            Volatile.Read(ref Exited) == 0;
    }

    private readonly record struct BridgeNotification(
        long SessionId,
        BridgeStateEvent? State,
        string? Fault,
        long Sequence);

    private readonly record struct BridgeReply(JsonElement Element, long SessionId, long Sequence);

    private sealed class BridgeRequest
    {
        [JsonPropertyName("id")] public int Id { get; set; }
        [JsonPropertyName("method")] public string Method { get; set; } = string.Empty;
        [JsonPropertyName("params")] public object? Params { get; set; }
    }
}

public sealed record BridgeStateEvent(long SessionId, BridgeState State, long Sequence);

public sealed record BridgeFault(long SessionId, string Message, long Sequence);

public sealed class RapooBridgeException(string message) : Exception(message);
