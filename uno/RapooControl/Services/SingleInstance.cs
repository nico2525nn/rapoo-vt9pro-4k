using System.Net;
using System.Net.Sockets;
using System.Text;

namespace RapooControl.Services;

/// <summary>
/// Cross-platform single-instance bridge. The first process holds a lock file
/// and listens on a loopback port; later launches send "show" and exit.
/// </summary>
public sealed class SingleInstance : IDisposable
{
    private readonly string _lockPath;
    private readonly string _portPath;
    private FileStream? _lock;
    private TcpListener? _listener;
    private CancellationTokenSource? _cts;
    private volatile bool _pendingShow;

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

    public event EventHandler? ShowRequested;

    public bool TryAcquire()
    {
        try
        {
            _lock = new FileStream(
                _lockPath,
                FileMode.OpenOrCreate,
                FileAccess.ReadWrite,
                FileShare.None);
        }
        catch (IOException)
        {
            NotifyExisting();
            return false;
        }

        _listener = new TcpListener(IPAddress.Loopback, 0);
        _listener.Start();
        var port = ((IPEndPoint)_listener.LocalEndpoint).Port;
        File.WriteAllText(_portPath, port.ToString());
        _cts = new CancellationTokenSource();
        _ = Task.Run(() => AcceptLoopAsync(_cts.Token));
        return true;
    }

    public bool ConsumePendingShow()
    {
        if (!_pendingShow)
        {
            return false;
        }
        _pendingShow = false;
        return true;
    }

    private void NotifyExisting()
    {
        for (var attempt = 0; attempt < 50; attempt++)
        {
            try
            {
                if (File.Exists(_portPath) &&
                    int.TryParse(File.ReadAllText(_portPath), out var port))
                {
                    using var client = new TcpClient();
                    client.Connect(IPAddress.Loopback, port);
                    using var stream = client.GetStream();
                    using var writer = new StreamWriter(stream, Encoding.UTF8);
                    writer.WriteLine("show");
                    writer.Flush();
                    return;
                }
            }
            catch
            {
                // The first instance is still starting.
            }
            Thread.Sleep(100);
        }
    }

    private async Task AcceptLoopAsync(CancellationToken cancellationToken)
    {
        var listener = _listener;
        if (listener is null)
        {
            return;
        }

        try
        {
            while (!cancellationToken.IsCancellationRequested)
            {
                using var client = await listener.AcceptTcpClientAsync(cancellationToken).ConfigureAwait(false);
                using var stream = client.GetStream();
                using var reader = new StreamReader(stream, Encoding.UTF8);
                var line = await reader.ReadLineAsync(cancellationToken).ConfigureAwait(false);
                if (!string.Equals(line, "show", StringComparison.Ordinal))
                {
                    continue;
                }

                var handler = ShowRequested;
                if (handler is null)
                {
                    _pendingShow = true;
                }
                else
                {
                    handler.Invoke(this, EventArgs.Empty);
                }
            }
        }
        catch (OperationCanceledException)
        {
        }
        catch (Exception ex)
        {
            System.Diagnostics.Debug.WriteLine($"single-instance listener: {ex}");
        }
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
        _lock?.Dispose();
        _lock = null;
        try
        {
            File.Delete(_portPath);
        }
        catch
        {
        }
    }
}
