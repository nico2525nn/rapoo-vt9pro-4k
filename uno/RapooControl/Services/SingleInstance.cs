using System.Net;
using System.Net.Sockets;
using System.Text;

namespace RapooControl.Services;

/// <summary>
/// Cross-platform single-instance bridge. The first process holds an exclusive
/// lock file and listens on a loopback port. Later launches send "show", wait
/// for an acknowledgement, and exit. The port file carries the owner token so
/// a shutting-down instance cannot delete its successor's endpoint.
/// </summary>
public sealed class SingleInstance : IDisposable
{
    private const int ActivationTimeoutMs = 5000;
    private const int ClientTimeoutMs = 3000;

    private readonly string _lockPath;
    private readonly string _portPath;
    private readonly object _showGate = new();
    private FileStream? _lock;
    private TcpListener? _listener;
    private CancellationTokenSource? _cts;
    private Action? _show;
    private bool _pendingShow;
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
    /// Takes ownership. Returns false when another live instance accepted the
    /// activation request. Throws <see cref="IOException"/> when the lock is
    /// held but the endpoint cannot be reached, which must not be confused
    /// with a filesystem failure.
    /// </summary>
    public bool TryAcquire(bool requestShow)
    {
        try
        {
            _lock = new FileStream(
                _lockPath,
                FileMode.OpenOrCreate,
                FileAccess.ReadWrite,
                FileShare.None);
        }
        catch (IOException ex)
        {
            if (NotifyExisting(requestShow))
            {
                return false;
            }
            throw new IOException(
                "別の Rapoo Control が起動していますが、応答しません。タスクマネージャーで終了してから再試行してください。",
                ex);
        }

        _listener = new TcpListener(IPAddress.Loopback, 0);
        _listener.Start();
        var port = ((IPEndPoint)_listener.LocalEndpoint).Port;
        _ownerToken = $"{Environment.ProcessId}:{port}";
        WritePortFile(_ownerToken);
        _cts = new CancellationTokenSource();
        _ = Task.Run(() => AcceptLoopAsync(_listener, _cts.Token));
        return true;
    }

    /// <summary>
    /// Registers the UI activation callback. A request that arrived before the
    /// window existed is replayed exactly once, under the same lock.
    /// </summary>
    public void SetShowHandler(Action show)
    {
        ArgumentNullException.ThrowIfNull(show);
        var invoke = false;
        lock (_showGate)
        {
            _show = show;
            if (_pendingShow)
            {
                _pendingShow = false;
                invoke = true;
            }
        }

        if (invoke)
        {
            show();
        }
    }

    /// <summary>True when a second launch asked to show this instance.</summary>
    public bool ConsumePendingShow()
    {
        lock (_showGate)
        {
            var pending = _pendingShow;
            _pendingShow = false;
            return pending;
        }
    }

    private bool NotifyExisting(bool requestShow)
    {
        var deadline = Environment.TickCount64 + ActivationTimeoutMs;
        while (Environment.TickCount64 < deadline)
        {
            try
            {
                if (TryReadPort(out var ownerToken))
                {
                    var separator = ownerToken.IndexOf(':');
                    if (separator > 0 &&
                        int.TryParse(ownerToken[(separator + 1)..], out var port) &&
                        TrySendActivation(port, requestShow))
                    {
                        return true;
                    }
                }
            }
            catch
            {
                // The first instance is still starting.
            }
            Thread.Sleep(100);
        }

        return false;
    }

    private bool TryReadPort(out string ownerToken)
    {
        ownerToken = string.Empty;
        try
        {
            if (!File.Exists(_portPath))
            {
                return false;
            }
            ownerToken = File.ReadAllText(_portPath).Trim();
            return ownerToken.Length > 0;
        }
        catch
        {
            return false;
        }
    }

    private static bool TrySendActivation(int port, bool requestShow)
    {
        if (!requestShow)
        {
            // A duplicate --hidden launch must not pop an existing window.
            return true;
        }

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
            return false;
        }

        using var stream = client.GetStream();
        stream.ReadTimeout = ClientTimeoutMs;
        stream.WriteTimeout = ClientTimeoutMs;
        using var writer = new StreamWriter(stream, new UTF8Encoding(false)) { AutoFlush = true };
        using var reader = new StreamReader(stream, Encoding.UTF8);
        writer.WriteLine("show");
        var response = reader.ReadLine();
        return response is not null && response.StartsWith("ok", StringComparison.Ordinal);
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
                if (!string.Equals(line, "show", StringComparison.Ordinal))
                {
                    await writer.WriteLineAsync("no").ConfigureAwait(false);
                    return;
                }

                Action? action;
                lock (_showGate)
                {
                    action = _show;
                    if (action is null)
                    {
                        _pendingShow = true;
                    }
                }

                await writer.WriteLineAsync($"ok {Environment.ProcessId}").ConfigureAwait(false);
                action?.Invoke();
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
            if (TryReadPort(out var ownerToken) &&
                string.Equals(ownerToken, _ownerToken, StringComparison.Ordinal))
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
}
