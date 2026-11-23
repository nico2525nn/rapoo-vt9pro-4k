using System.Collections.Concurrent;
using System.Diagnostics;
using System.Security.Cryptography;
using System.Text;
using System.Text.Json;
using System.Text.Json.Serialization;

namespace RapooControl.Services;

/// <summary>
/// Owns the Go sidecar process and speaks newline-delimited JSON over its
/// stdin/stdout. All HID access stays inside the sidecar.
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

    private readonly ConcurrentDictionary<int, TaskCompletionSource<JsonElement>> _pending = new();
    private readonly SemaphoreSlim _writeLock = new(1, 1);
    private TaskCompletionSource<bool> _ready = new(TaskCreationOptions.RunContinuationsAsynchronously);
    private CancellationTokenSource? _cts;

    private readonly object _startGate = new();
    private Process? _process;
    private Task? _readTask;
    private Task? _stderrTask;
    private Task? _startTask;
    private int _nextId;
    private volatile bool _started;

    public event EventHandler<BridgeState>? StateChanged;

    public event EventHandler<string>? Faulted;

    public bool IsRunning => _started && _process is { HasExited: false };

    public Task StartAsync(CancellationToken cancellationToken = default)
    {
        lock (_startGate)
        {
            if (_startTask is { IsCompleted: false })
            {
                return _startTask;
            }
            if (_process is { HasExited: false } && _ready.Task.IsCompletedSuccessfully)
            {
                return Task.CompletedTask;
            }
            _startTask = StartCoreAsync(cancellationToken);
            return _startTask;
        }
    }

    private async Task StartCoreAsync(CancellationToken cancellationToken)
    {
        if (_process is not null)
        {
            _process.Dispose();
            _process = null;
        }

        _cts?.Dispose();
        _cts = new CancellationTokenSource();
        _ready = new TaskCompletionSource<bool>(TaskCreationOptions.RunContinuationsAsynchronously);

        var exe = FindSidecar() ?? throw new FileNotFoundException(
            "rapoo-bridge.exe が見つかりません。RapooControl.exe と同じフォルダに配置してください。");
        Log($"starting {exe}");
        if (!OperatingSystem.IsWindows())
        {
            try
            {
                File.SetUnixFileMode(exe,
                    UnixFileMode.UserRead | UnixFileMode.UserWrite | UnixFileMode.UserExecute |
                    UnixFileMode.GroupRead | UnixFileMode.GroupExecute |
                    UnixFileMode.OtherRead | UnixFileMode.OtherExecute);
            }
            catch
            {
                // The file may already be executable or on a read-only mount.
            }
        }

        var startInfo = new ProcessStartInfo
        {
            FileName = exe,
            WorkingDirectory = Path.GetDirectoryName(exe) ?? AppContext.BaseDirectory,
            RedirectStandardInput = true,
            RedirectStandardOutput = true,
            RedirectStandardError = true,
            StandardInputEncoding = Utf8NoBom,
            StandardOutputEncoding = Encoding.UTF8,
            StandardErrorEncoding = Encoding.UTF8,
            UseShellExecute = false,
            CreateNoWindow = true,
        };

        var process = new Process { StartInfo = startInfo, EnableRaisingEvents = true };
        process.Exited += (_, _) => OnProcessExited(process);
        var cts = _cts;
        try
        {
            process.Start();
        }
        catch
        {
            process.Dispose();
            throw;
        }
        _process = process;
        _started = true;
        _readTask = Task.Run(() => ReadLoopAsync(process, cts.Token));
        _stderrTask = Task.Run(() => ReadStderrAsync(process, cts.Token));

        var readyTask = _ready.Task;
        var ready = await Task.WhenAny(
            readyTask,
            Task.Delay(TimeSpan.FromSeconds(10), cancellationToken)).ConfigureAwait(false);
        if (ready != readyTask)
        {
            await StopAsync().ConfigureAwait(false);
            throw new TimeoutException("rapoo-bridge.exe が応答しませんでした。");
        }
        try
        {
            // Observe a faulted ready task (for example a protocol mismatch)
            // before the first RPC is allowed to run.
            await readyTask.ConfigureAwait(false);
        }
        catch
        {
            await StopAsync().ConfigureAwait(false);
            throw;
        }
    }

    public async Task StopAsync()
    {
        var process = _process;
        var cts = _cts;
        _cts = null;

        if (process is null)
        {
            cts?.Cancel();
            cts?.Dispose();
            return;
        }

        try
        {
            process.StandardInput.Close();
        }
        catch
        {
            // The process may already have exited.
        }

        cts?.Cancel();
        try
        {
            if (!process.WaitForExit(2500))
            {
                // The official firmware updater is a child of the bridge.
                // Kill only the bridge so an in-flight firmware write can
                // finish after the UI exits.
                process.Kill(entireProcessTree: false);
            }
        }
        catch
        {
            // Ignore teardown races.
        }

        process.Dispose();
        if (ReferenceEquals(_process, process))
        {
            _process = null;
        }
        _started = false;
        cts?.Dispose();

        foreach (var pending in _pending.Values)
        {
            pending.TrySetException(new InvalidOperationException("rapoo-bridge.exe が終了しました。"));
        }
        _pending.Clear();
        await Task.CompletedTask;
    }

    public Task<BridgeState> GetStateAsync(CancellationToken cancellationToken = default) =>
        RequestAsync<BridgeState>("state", null, cancellationToken);

    public Task<BridgeSnapshot> GetSnapshotAsync(CancellationToken cancellationToken = default) =>
        RequestAsync<BridgeSnapshot>("snapshot", null, cancellationToken);

    public Task<BridgeSnapshot> SetDemoModeAsync(bool enabled, CancellationToken cancellationToken = default) =>
        RequestAsync<BridgeSnapshot>("demo", new { enabled }, cancellationToken);

    public Task<BridgeState> SetLanguageAsync(string language, CancellationToken cancellationToken = default) =>
        RequestAsync<BridgeState>("setLanguage", new { language }, cancellationToken);

    public Task<BridgeState> SetPollingModeAsync(string mode, CancellationToken cancellationToken = default) =>
        RequestAsync<BridgeState>("pollingMode", new { mode }, cancellationToken);

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
        await StopAsync().ConfigureAwait(false);
        _cts?.Dispose();
        _writeLock.Dispose();
    }

    private async Task<T> RequestAsync<T>(string method, object? parameters, CancellationToken cancellationToken)
    {
        var element = await RequestElementAsync(method, parameters, cancellationToken).ConfigureAwait(false);
        try
        {
            var result = element.Deserialize<T>(JsonOptions);
            if (result is null)
            {
                throw new InvalidOperationException($"rapoo-bridge: {method} returned an empty result.");
            }
            return result;
        }
        catch (Exception ex)
        {
            Log($"decode {method} failed: {ex}\nraw={element.GetRawText()}");
            throw new InvalidOperationException(LocalizationService.Instance.Format("rapoo-bridge: {0} の応答を解釈できません: {1}", method, ex.Message), ex);
        }
    }

    private async Task RequestVoidAsync(string method, object? parameters, CancellationToken cancellationToken)
    {
        _ = await RequestElementAsync(method, parameters, cancellationToken).ConfigureAwait(false);
    }

    private async Task<JsonElement> RequestElementAsync(
        string method,
        object? parameters,
        CancellationToken cancellationToken)
    {
        var process = _process;
        if (process is null || process.HasExited)
        {
            throw new InvalidOperationException("rapoo-bridge.exe が起動していません。");
        }

        using var timeout = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);
        timeout.CancelAfter(TimeSpan.FromSeconds(15));
        var token = timeout.Token;

        if (!_ready.Task.IsCompletedSuccessfully)
        {
            await _ready.Task.WaitAsync(TimeSpan.FromSeconds(10), token).ConfigureAwait(false);
        }

        var id = Interlocked.Increment(ref _nextId);
        var completion = new TaskCompletionSource<JsonElement>(TaskCreationOptions.RunContinuationsAsynchronously);
        _pending[id] = completion;

        var payload = JsonSerializer.Serialize(new BridgeRequest
        {
            Id = id,
            Method = method,
            Params = parameters,
        }, JsonOptions);

        try
        {
            await _writeLock.WaitAsync(token).ConfigureAwait(false);
            try
            {
                await process.StandardInput.WriteAsync(payload.AsMemory(), token).ConfigureAwait(false);
                await process.StandardInput.WriteLineAsync().ConfigureAwait(false);
                await process.StandardInput.FlushAsync(token).ConfigureAwait(false);
            }
            finally
            {
                _writeLock.Release();
            }

            using var registration = token.Register(() =>
            {
                if (_pending.TryRemove(id, out var pending))
                {
                    pending.TrySetCanceled(token);
                }
            });

            return await completion.Task.ConfigureAwait(false);
        }
        catch (OperationCanceledException) when (!cancellationToken.IsCancellationRequested)
        {
            _pending.TryRemove(id, out _);
            throw new TimeoutException($"rapoo-bridge: {method} が15秒以内に応答しませんでした。");
        }
        catch
        {
            _pending.TryRemove(id, out _);
            throw;
        }
    }

    private async Task ReadLoopAsync(Process process, CancellationToken cancellationToken)
    {
        try
        {
            while (!cancellationToken.IsCancellationRequested)
            {
                var line = await process.StandardOutput.ReadLineAsync(cancellationToken).ConfigureAwait(false);
                if (line is null)
                {
                    break;
                }
                if (string.IsNullOrWhiteSpace(line))
                {
                    continue;
                }

                using var document = JsonDocument.Parse(line);
                var root = document.RootElement;

                if (root.TryGetProperty("event", out var eventName))
                {
                    HandleEvent(process, eventName.GetString(), root);
                    continue;
                }

                if (!root.TryGetProperty("id", out var idElement) ||
                    !idElement.TryGetInt32(out var id) ||
                    !_pending.TryRemove(id, out var completion))
                {
                    continue;
                }

                if (root.TryGetProperty("error", out var errorElement))
                {
                    completion.TrySetException(new RapooBridgeException(errorElement.GetString() ?? "unknown error"));
                }
                else if (root.TryGetProperty("result", out var resultElement))
                {
                    completion.TrySetResult(resultElement.Clone());
                }
                else
                {
                    completion.TrySetResult(default);
                }
            }
        }
        catch (OperationCanceledException)
        {
            // Normal shutdown.
        }
        catch (Exception ex)
        {
            Log($"read loop failed: {ex}");
            if (ReferenceEquals(_process, process))
            {
                Faulted?.Invoke(this, ex.Message);
                foreach (var pending in _pending.Values)
                {
                    pending.TrySetException(ex);
                }
                _pending.Clear();
            }
        }
    }

    private void HandleEvent(Process process, string? name, JsonElement root)
    {
        if (!ReferenceEquals(_process, process))
        {
            return;
        }
        if (name == "ready")
        {
            if (root.TryGetProperty("data", out var readyData) &&
                readyData.TryGetProperty("version", out var version) &&
                string.Equals(version.GetString(), BridgeProtocolVersion, StringComparison.Ordinal))
            {
                _ready.TrySetResult(true);
            }
            else
            {
                _ready.TrySetException(new InvalidOperationException(
                    "rapoo-bridge.exe のプロトコルバージョンが一致しません。アプリを再インストールしてください。"));
            }
            return;
        }
        if (name != "state" || !root.TryGetProperty("data", out var data))
        {
            return;
        }

        try
        {
            var state = data.Deserialize<BridgeState>(JsonOptions);
            if (state is not null)
            {
                StateChanged?.Invoke(this, state);
            }
        }
        catch (Exception ex)
        {
            Log($"state event decode failed: {ex}");
        }
    }

    private async Task ReadStderrAsync(Process process, CancellationToken cancellationToken)
    {
        try
        {
            while (!cancellationToken.IsCancellationRequested)
            {
                var line = await process.StandardError.ReadLineAsync(cancellationToken).ConfigureAwait(false);
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

    private void OnProcessExited(Process process)
    {
        if (!ReferenceEquals(_process, process) || !_started)
        {
            return;
        }
        _started = false;
        var message = "rapoo-bridge.exe が終了しました。";
        try
        {
            message = LocalizationService.Instance.Format("rapoo-bridge.exe が終了しました (code {0}).", _process?.ExitCode ?? -1);
        }
        catch
        {
        }
        Log(message);
        Faulted?.Invoke(this, message);
        foreach (var pending in _pending.Values)
        {
            pending.TrySetException(new InvalidOperationException(message));
        }
        _pending.Clear();
    }

    private static string? FindSidecar()
    {
        var overridePath = Environment.GetEnvironmentVariable("RAPOO_BRIDGE_PATH");
        if (!string.IsNullOrWhiteSpace(overridePath) && File.Exists(overridePath))
        {
            return overridePath;
        }

        var sidecarName = OperatingSystem.IsWindows() ? "rapoo-bridge.exe" : "rapoo-bridge";
        var local = Path.Combine(AppContext.BaseDirectory, sidecarName);
        if (File.Exists(local))
        {
            return local;
        }

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

        return null;
    }

    /// <summary>
    /// Single-file distribution embeds the sidecar. Extract it to a stable
    /// per-hash path so an upgrade never reuses a running, older binary.
    /// </summary>
    private static bool HasEmbeddedSidecar()
    {
        try
        {
            return typeof(RapooBridgeClient).Assembly
                .GetManifestResourceInfo("RapooControl.rapoo-bridge.exe") is not null;
        }
        catch
        {
            return false;
        }
    }

    private static string? ExtractEmbeddedSidecar()
    {
        try
        {
            var assembly = typeof(RapooBridgeClient).Assembly;
            using var stream = assembly.GetManifestResourceStream("RapooControl.rapoo-bridge.exe");
            if (stream is null)
            {
                return null;
            }

            var hash = Convert.ToHexString(SHA256.HashData(stream)).ToLowerInvariant()[..12];
            var directory = Path.Combine(
                Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
                "rapoo-vt9pro-4k",
                "bin");
            Directory.CreateDirectory(directory);
            var path = Path.Combine(directory, $"rapoo-bridge-{hash}.exe");
            if (File.Exists(path))
            {
                string existingHash;
                using (var existing = File.OpenRead(path))
                {
                    existingHash = Convert.ToHexString(SHA256.HashData(existing)).ToLowerInvariant()[..12];
                }
                if (!string.Equals(existingHash, hash, StringComparison.Ordinal))
                {
                    try
                    {
                        File.Delete(path);
                    }
                    catch (Exception ex)
                    {
                        Log($"stale embedded sidecar is locked: {ex.Message}");
                        return null;
                    }
                }
            }
            if (!File.Exists(path))
            {
                stream.Position = 0;
                var temporary = path + "." + Guid.NewGuid().ToString("N") + ".tmp";
                using (var file = File.Create(temporary))
                {
                    stream.CopyTo(file);
                }
                using (var extracted = File.OpenRead(temporary))
                {
                    var extractedHash = Convert.ToHexString(SHA256.HashData(extracted)).ToLowerInvariant()[..12];
                    if (!string.Equals(extractedHash, hash, StringComparison.Ordinal))
                    {
                        File.Delete(temporary);
                        return null;
                    }
                }
                File.Move(temporary, path, overwrite: true);
            }

            foreach (var old in Directory.EnumerateFiles(directory, "rapoo-bridge-*.exe"))
            {
                if (!string.Equals(old, path, StringComparison.OrdinalIgnoreCase))
                {
                    try
                    {
                        File.Delete(old);
                    }
                    catch
                    {
                        // The old sidecar may still be running.
                    }
                }
            }

            return path;
        }
        catch (Exception ex)
        {
            Log($"embedded sidecar extraction failed: {ex}");
            return null;
        }
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

    private sealed class BridgeRequest
    {
        [JsonPropertyName("id")] public int Id { get; set; }
        [JsonPropertyName("method")] public string Method { get; set; } = string.Empty;
        [JsonPropertyName("params")] public object? Params { get; set; }
    }
}

public sealed class RapooBridgeException(string message) : Exception(message);
