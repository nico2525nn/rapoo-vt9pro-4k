using System.Net;
using System.Net.Sockets;
using System.Runtime.InteropServices;
using System.Text;

namespace RapooControl.Services;

/// <summary>
/// Cross-platform single-instance bridge. The first process holds an exclusive
/// lock file and listens on a loopback port. Later launches ask the owner to
/// show or ping, validate the owner PID in the acknowledgement, and exit only
/// after the owner has accepted the request.
/// </summary>
public sealed class SingleInstance : IDisposable
{
    private const int ActivationTimeoutMs = 15000;
    private const int ClientTimeoutMs = 1500;

    private readonly string _lockPath;
    private readonly string _portPath;
    private readonly object _showGate = new();
    private FileStream? _lock;
    private TcpListener? _listener;
    private CancellationTokenSource? _cts;
    private Func<bool>? _show;
    private bool _pendingShow;
    private int _shuttingDown;
    private string _ownerToken = string.Empty;

    private SingleInstance()
    {
        var dir = Path.Combine(
            Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
            "rapoo-vt9pro-4k");
        Directory.CreateDirectory(dir);
        _lockPath = Path.Combine(dir, "uno-instance.lock");
        _portPath = Path.Combine(dir, "uno-instance.port");
    }

    public static SingleInstance Current { get; } = new();

    /// <summary>
    /// Takes ownership. Returns false when an existing live instance accepted
    /// the activation request. Retries while a previous owner is shutting down.
    /// </summary>
    public bool TryAcquire(bool requestShow)
    {
        var deadline = Environment.TickCount64 + ActivationTimeoutMs;
        while (true)
        {
            try
            {
                _lock = new FileStream(
                    _lockPath,
                    FileMode.OpenOrCreate,
                    FileAccess.ReadWrite,
                    FileShare.None);
                break;
            }
            catch (IOException ex)
            {
                switch (TryNotifyExisting(requestShow))
                {
                    case ActivationResult.Accepted:
                        return false;
                    case ActivationResult.Retry:
                    case ActivationResult.Unreachable:
                        if (Environment.TickCount64 >= deadline)
                        {
                            throw new IOException(
                                "別の Rapoo Control が起動していますが、応答しません。タスクマネージャーで終了してから再試行してください。",
                                ex);
                        }
                        Thread.Sleep(150);
                        continue;
                }
            }
        }

        try
        {
            _listener = new TcpListener(IPAddress.Loopback, 0);
            _listener.Start();
            var port = ((IPEndPoint)_listener.LocalEndpoint).Port;
            _ownerToken = $"{Environment.ProcessId}:{port}";
            WritePortFile(_ownerToken);
            _cts = new CancellationTokenSource();
            _ = Task.Run(() => AcceptLoopAsync(_listener, _cts.Token), CancellationToken.None);
            return true;
        }
        catch
        {
            _listener?.Stop();
            _listener = null;
            _lock?.Dispose();
            _lock = null;
            throw;
        }
    }

    /// <summary>
    /// Stops accepting activation requests before the sidecar is torn down.
    /// A duplicate then retries the lock and becomes the next owner instead of
    /// receiving an ACK that this instance cannot honor.
    /// </summary>
    public void BeginShutdown()
    {
        if (Interlocked.Exchange(ref _shuttingDown, 1) != 0)
        {
            return;
        }
        try
        {
            _cts?.Cancel();
        }
        catch
        {
        }
        try
        {
            _listener?.Stop();
        }
        catch
        {
        }
    }

    /// <summary>
    /// Registers the UI activation callback. The callback returns true only
    /// after the activation was queued, so the caller can acknowledge the
    /// duplicate launch without silently dropping it.
    /// </summary>
    public void SetShowHandler(Func<bool> show)
    {
        ArgumentNullException.ThrowIfNull(show);
        var replay = false;
        lock (_showGate)
        {
            _show = show;
            if (_pendingShow)
            {
                _pendingShow = false;
                replay = true;
            }
        }

        if (replay && !show())
        {
            lock (_showGate)
            {
                _pendingShow = true;
            }
        }
    }

    private ActivationResult TryNotifyExisting(bool requestShow)
    {
        if (!TryReadPort(out var ownerPid, out var port))
        {
            return ActivationResult.Unreachable;
        }

        return TrySendActivation(port, requestShow, ownerPid);
    }

    private bool TryReadPort(out int ownerPid, out int port)
    {
        ownerPid = 0;
        port = 0;
        try
        {
            if (!File.Exists(_portPath))
            {
                return false;
            }

            var token = File.ReadAllText(_portPath).Trim();
            var separator = token.IndexOf(':');
            return separator > 0 &&
                int.TryParse(token[..separator], out ownerPid) &&
                int.TryParse(token[(separator + 1)..], out port) &&
                ownerPid > 0 &&
                port is > 0 and <= 65535;
        }
        catch
        {
            return false;
        }
    }

    private static ActivationResult TrySendActivation(int port, bool requestShow, int expectedPid)
    {
        try
        {
            using var client = new TcpClient
            {
                ReceiveTimeout = ClientTimeoutMs,
                SendTimeout = ClientTimeoutMs,
            };
#if WINDOWS
            var connectTask = client.ConnectAsync(IPAddress.Loopback, port);
#else
            var connectTask = client.ConnectAsync(IPAddress.Loopback, port).AsTask();
#endif
            if (!connectTask.Wait(ClientTimeoutMs))
            {
                return ActivationResult.Unreachable;
            }

#if WINDOWS
            if (requestShow)
            {
                _ = AllowSetForegroundWindow(expectedPid);
            }
#endif

            using var stream = client.GetStream();
            stream.ReadTimeout = ClientTimeoutMs;
            stream.WriteTimeout = ClientTimeoutMs;
            using var writer = new StreamWriter(stream, new UTF8Encoding(false)) { AutoFlush = true };
            using var reader = new StreamReader(stream, Encoding.UTF8);
            writer.WriteLine(requestShow ? "show" : "ping");
            var response = reader.ReadLine();
            if (response is null)
            {
                return ActivationResult.Unreachable;
            }
            if (response == "retry")
            {
                return ActivationResult.Retry;
            }

            return response == $"ok {expectedPid}"
                ? ActivationResult.Accepted
                : ActivationResult.Retry;
        }
        catch
        {
            return ActivationResult.Unreachable;
        }
    }

    private async Task AcceptLoopAsync(TcpListener listener, CancellationToken cancellationToken)
    {
        try
        {
            while (!cancellationToken.IsCancellationRequested)
            {
                var client = await listener.AcceptTcpClientAsync(cancellationToken).ConfigureAwait(false);
                _ = Task.Run(() => HandleClientAsync(client, cancellationToken), CancellationToken.None);
            }
        }
        catch (OperationCanceledException)
        {
        }
        catch (ObjectDisposedException)
        {
        }
        catch (Exception ex)
        {
            System.Diagnostics.Debug.WriteLine($"single-instance listener: {ex}");
        }
    }

    private async Task HandleClientAsync(TcpClient client, CancellationToken cancellationToken)
    {
        using (client)
        {
            client.ReceiveTimeout = ClientTimeoutMs;
            client.SendTimeout = ClientTimeoutMs;
            try
            {
                using var timeout = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);
                timeout.CancelAfter(ClientTimeoutMs);
                var stream = client.GetStream();
                using var reader = new StreamReader(stream, Encoding.UTF8, leaveOpen: true);
                using var writer = new StreamWriter(stream, new UTF8Encoding(false), leaveOpen: true)
                {
                    AutoFlush = true,
                };
                var line = await reader.ReadLineAsync(timeout.Token).ConfigureAwait(false);
                if (Volatile.Read(ref _shuttingDown) != 0)
                {
                    await writer.WriteLineAsync("retry").ConfigureAwait(false);
                    return;
                }
                if (string.Equals(line, "ping", StringComparison.Ordinal))
                {
                    await writer.WriteLineAsync($"ok {Environment.ProcessId}").ConfigureAwait(false);
                    return;
                }
                if (!string.Equals(line, "show", StringComparison.Ordinal))
                {
                    await writer.WriteLineAsync("no").ConfigureAwait(false);
                    return;
                }

                Func<bool>? handler;
                var accepted = false;
                lock (_showGate)
                {
                    handler = _show;
                    if (handler is null)
                    {
                        _pendingShow = true;
                        accepted = true;
                    }
                }

                if (handler is not null)
                {
                    try
                    {
                        accepted = handler();
                    }
                    catch (Exception ex)
                    {
                        System.Diagnostics.Debug.WriteLine($"single-instance activation: {ex}");
                    }
                }

                if (!accepted)
                {
                    await writer.WriteLineAsync("retry").ConfigureAwait(false);
                    return;
                }

                await writer.WriteLineAsync($"ok {Environment.ProcessId}").ConfigureAwait(false);
            }
            catch (OperationCanceledException)
            {
            }
            catch (Exception ex)
            {
                System.Diagnostics.Debug.WriteLine($"single-instance client: {ex}");
            }
        }
    }

    private void WritePortFile(string ownerToken)
    {
        var temporary = _portPath + ".tmp";
        File.WriteAllText(temporary, ownerToken);
        File.Move(temporary, _portPath, overwrite: true);
    }

    public void Dispose()
    {
        BeginShutdown();
        try
        {
            _cts?.Cancel();
        }
        catch
        {
        }
        try
        {
            _listener?.Stop();
        }
        catch
        {
        }
        _cts?.Dispose();
        _cts = null;

        // Delete the endpoint while the ownership lock is still held so a
        // successor cannot have its port file removed by this process.
        try
        {
            if (TryReadPort(out _, out _) &&
                string.Equals(File.ReadAllText(_portPath).Trim(), _ownerToken, StringComparison.Ordinal))
            {
                File.Delete(_portPath);
            }
        }
        catch
        {
        }

        _lock?.Dispose();
        _lock = null;
    }

#if WINDOWS
    [DllImport("user32.dll")]
    private static extern bool AllowSetForegroundWindow(int processId);
#endif

    private enum ActivationResult
    {
        Accepted,
        Retry,
        Unreachable,
    }
}
