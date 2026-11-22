using System.Collections.Concurrent;
using System.Runtime.InteropServices;
using System.Text.Json;
using System.Threading.Channels;

namespace RapooControl.Services;

/// <summary>
/// Stores macros, registers their global shortcuts, and replays the key
/// sequences with SendInput. Runs while the app is open; no driver support is
/// required because macros are played on the host, not stored on the mouse.
/// </summary>
public sealed class MacroService : IDisposable
{
    private const int WM_HOTKEY = 0x0312;
    private const int WM_QUIT = 0x0012;
    private const int WM_APP = 0x8000;
    private const int WM_RELOAD = WM_APP + 1;

    private const uint MOD_ALT = 0x0001;
    private const uint MOD_CONTROL = 0x0002;
    private const uint MOD_SHIFT = 0x0004;
    private const uint MOD_WIN = 0x0008;
    private const uint MOD_NOREPEAT = 0x4000;

    private const uint INPUT_KEYBOARD = 1;
    private const uint KEYEVENTF_KEYUP = 0x0002;
    private const uint KEYEVENTF_EXTENDEDKEY = 0x0001;

    private const ushort VK_LSHIFT = 0xA0;
    private const ushort VK_LCONTROL = 0xA2;
    private const ushort VK_LMENU = 0xA4;
    private const ushort VK_LWIN = 0x5B;

    private static readonly JsonSerializerOptions JsonOptions = new() { WriteIndented = true };

    private readonly object _gate = new();
    private readonly object _reloadGate = new();
    private readonly ManualResetEventSlim _hotkeyReady = new(false);
    private readonly ConcurrentDictionary<string, byte> _playing = new();
    private readonly Channel<string> _playQueue = Channel.CreateUnbounded<string>(new UnboundedChannelOptions
    {
        SingleReader = true,
        SingleWriter = false,
    });

    private readonly string _storePath;
    private List<MacroDefinition> _macros = [];
    private Dictionary<int, string> _hotkeyMap = [];
    private Dictionary<string, string> _registrationErrors = [];

    private Thread? _hotkeyThread;
    private Task? _playbackTask;
    private CancellationTokenSource? _playbackCts;
    private TaskCompletionSource<bool>? _reloadCompletion;
    private int _hotkeyThreadId;
    private volatile bool _running;

    public MacroService()
    {
        var dir = Path.Combine(
            Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
            "rapoo-vt9pro-4k");
        _storePath = Path.Combine(dir, "macros.json");
    }

    public event EventHandler? MacrosChanged;

    public event EventHandler? RegistrationErrorsChanged;

    public event EventHandler<string>? PlaybackFailed;

    public string? LoadError { get; private set; }

    public bool IsRunning => _running;

    public IReadOnlyList<MacroDefinition> Macros
    {
        get
        {
            lock (_gate)
            {
                return _macros.Select(Clone).ToArray();
            }
        }
    }

    public async Task StartAsync(CancellationToken cancellationToken = default)
    {
        if (_running)
        {
            return;
        }

        Load();
        _playbackCts = new CancellationTokenSource();
        _playbackTask = Task.Run(() => PlaybackLoopAsync(_playbackCts.Token), CancellationToken.None);
        _running = true;
        _hotkeyReady.Reset();
        _hotkeyThread = new Thread(HotkeyLoop)
        {
            IsBackground = true,
            Name = "RapooMacroHotkeys",
        };
        _hotkeyThread.SetApartmentState(ApartmentState.STA);
        _hotkeyThread.Start();

        var ready = await Task.Run(() => _hotkeyReady.Wait(TimeSpan.FromSeconds(5)), cancellationToken)
            .ConfigureAwait(false);
        if (!ready)
        {
            throw new TimeoutException("マクロのグローバルホットキーを初期化できませんでした。");
        }
    }

    public async Task StopAsync()
    {
        if (!_running)
        {
            return;
        }

        _running = false;
        var threadId = Volatile.Read(ref _hotkeyThreadId);
        if (threadId != 0)
        {
            PostThreadMessage((uint)threadId, WM_QUIT, IntPtr.Zero, IntPtr.Zero);
        }

        _playbackCts?.Cancel();
        if (_playbackTask is not null)
        {
            try
            {
                await _playbackTask.ConfigureAwait(false);
            }
            catch (OperationCanceledException)
            {
            }
        }

        _hotkeyThread?.Join(TimeSpan.FromSeconds(2));
        _hotkeyThread = null;
        Volatile.Write(ref _hotkeyThreadId, 0);
        _playbackTask = null;
        _playbackCts?.Dispose();
        _playbackCts = null;
    }

    public async Task<MacroDefinition> SaveAsync(MacroDefinition macro)
    {
        var copy = Clone(macro);
        copy.Name = copy.Name.Trim();
        if (string.IsNullOrWhiteSpace(copy.Name))
        {
            throw new InvalidOperationException("マクロ名を入力してください。");
        }
        if (copy.Id.Length == 0)
        {
            copy.Id = Guid.NewGuid().ToString("N");
        }

        string? error;
        lock (_gate)
        {
            error = Validate(copy, _macros);
        }
        if (error is not null)
        {
            throw new InvalidOperationException(error);
        }

        copy.UpdatedAt = DateTimeOffset.Now;
        lock (_gate)
        {
            var index = _macros.FindIndex(existing => existing.Id == copy.Id);
            if (index >= 0)
            {
                copy.CreatedAt = _macros[index].CreatedAt;
                _macros[index] = copy;
            }
            else
            {
                copy.CreatedAt = DateTimeOffset.Now;
                _macros.Add(copy);
            }
            Save();
        }

        await ReloadHotkeysAsync().ConfigureAwait(false);
        MacrosChanged?.Invoke(this, EventArgs.Empty);
        return Clone(copy);
    }

    public async Task DeleteAsync(string id)
    {
        lock (_gate)
        {
            _macros.RemoveAll(macro => macro.Id == id);
            Save();
        }

        await ReloadHotkeysAsync().ConfigureAwait(false);
        MacrosChanged?.Invoke(this, EventArgs.Empty);
    }

    public void Run(string id)
    {
        if (!_running)
        {
            throw new InvalidOperationException("マクロサービスが起動していません。");
        }
        if (!_playQueue.Writer.TryWrite(id))
        {
            throw new InvalidOperationException("マクロを実行できませんでした。");
        }
    }

    /// <summary>Plays an unsaved macro for the editor's test button.</summary>
    public void Run(MacroDefinition macro)
    {
        if (!_running)
        {
            throw new InvalidOperationException("マクロサービスが起動していません。");
        }

        var copy = Clone(macro);
        _ = Task.Run(async () =>
        {
            try
            {
                await PlayAsync(copy, _playbackCts?.Token ?? CancellationToken.None).ConfigureAwait(false);
            }
            catch (OperationCanceledException)
            {
            }
            catch (Exception ex)
            {
                PlaybackFailed?.Invoke(this, ex.Message);
            }
        });
    }

    public string? GetRegistrationError(string id)
    {
        lock (_gate)
        {
            return _registrationErrors.TryGetValue(id, out var message) ? message : null;
        }
    }

    public void Dispose()
    {
        try
        {
            StopAsync().GetAwaiter().GetResult();
        }
        catch
        {
            // Shutdown best effort.
        }
        _hotkeyReady.Dispose();
    }

    private void Load()
    {
        LoadError = null;
        lock (_gate)
        {
            _macros = [];
            if (!File.Exists(_storePath))
            {
                return;
            }

            try
            {
                var file = JsonSerializer.Deserialize<MacroFile>(File.ReadAllText(_storePath), JsonOptions);
                _macros = file?.Macros ?? [];
                foreach (var macro in _macros)
                {
                    macro.Trigger ??= new MacroTrigger();
                    macro.Steps ??= [];
                }
            }
            catch (Exception ex)
            {
                LoadError = $"マクロ設定を読み込めませんでした: {ex.Message}";
            }
        }
    }

    private void Save()
    {
        var directory = Path.GetDirectoryName(_storePath);
        if (!string.IsNullOrEmpty(directory))
        {
            Directory.CreateDirectory(directory);
        }

        var json = JsonSerializer.Serialize(new MacroFile { Macros = _macros }, JsonOptions);
        var temp = _storePath + ".tmp";
        File.WriteAllText(temp, json);
        File.Move(temp, _storePath, overwrite: true);
    }

    private async Task ReloadHotkeysAsync()
    {
        var threadId = Volatile.Read(ref _hotkeyThreadId);
        if (!_running || threadId == 0)
        {
            return;
        }

        TaskCompletionSource<bool> completion;
        lock (_reloadGate)
        {
            completion = new TaskCompletionSource<bool>(TaskCreationOptions.RunContinuationsAsynchronously);
            _reloadCompletion = completion;
        }

        if (!PostThreadMessage((uint)threadId, WM_RELOAD, IntPtr.Zero, IntPtr.Zero))
        {
            lock (_reloadGate)
            {
                _reloadCompletion = null;
            }
            completion.TrySetResult(false);
        }

        await completion.Task.ConfigureAwait(false);
        RegistrationErrorsChanged?.Invoke(this, EventArgs.Empty);
    }

    private void HotkeyLoop()
    {
        Volatile.Write(ref _hotkeyThreadId, (int)GetCurrentThreadId());
        try
        {
            UnregisterAll();
            RegisterAll();
            _hotkeyReady.Set();

            while (_running)
            {
                var result = GetMessage(out var message, IntPtr.Zero, 0, 0);
                if (result <= 0)
                {
                    break;
                }

                switch (message.message)
                {
                    case WM_HOTKEY:
                        var hotkeyId = message.wParam.ToInt32();
                        string? macroId;
                        lock (_gate)
                        {
                            _hotkeyMap.TryGetValue(hotkeyId, out macroId);
                        }
                        if (macroId is not null)
                        {
                            _playQueue.Writer.TryWrite(macroId);
                        }
                        break;

                    case WM_RELOAD:
                        UnregisterAll();
                        RegisterAll();
                        TaskCompletionSource<bool>? completion;
                        lock (_reloadGate)
                        {
                            completion = _reloadCompletion;
                            _reloadCompletion = null;
                        }
                        completion?.TrySetResult(true);
                        break;
                }
            }
        }
        finally
        {
            UnregisterAll();
            _hotkeyReady.Set();
            Volatile.Write(ref _hotkeyThreadId, 0);
        }
    }

    private void RegisterAll()
    {
        var snapshot = Macros;
        var map = new Dictionary<int, string>();
        var errors = new Dictionary<string, string>();
        var nextId = 1;

        foreach (var macro in snapshot)
        {
            if (!macro.Enabled)
            {
                continue;
            }

            var error = ValidateTrigger(macro.Trigger);
            if (error is not null)
            {
                errors[macro.Id] = error;
                continue;
            }

            var modifiers = BuildModifiers(macro.Trigger) | MOD_NOREPEAT;
            if (!RegisterHotKey(IntPtr.Zero, nextId, modifiers, (uint)macro.Trigger.VirtualKey))
            {
                errors[macro.Id] = $"他のアプリがこのショートカットを使用しています (Win32 {Marshal.GetLastWin32Error()})";
                continue;
            }

            map[nextId] = macro.Id;
            nextId++;
        }

        lock (_gate)
        {
            _hotkeyMap = map;
            _registrationErrors = errors;
        }
    }

    private void UnregisterAll()
    {
        Dictionary<int, string> map;
        lock (_gate)
        {
            map = _hotkeyMap;
            _hotkeyMap = [];
        }

        foreach (var id in map.Keys)
        {
            UnregisterHotKey(IntPtr.Zero, id);
        }
    }

    private async Task PlaybackLoopAsync(CancellationToken cancellationToken)
    {
        try
        {
            await foreach (var id in _playQueue.Reader.ReadAllAsync(cancellationToken).ConfigureAwait(false))
            {
                MacroDefinition? macro;
                lock (_gate)
                {
                    macro = _macros.FirstOrDefault(item => item.Id == id);
                    macro = macro is null ? null : Clone(macro);
                }

                if (macro is null || !macro.Enabled)
                {
                    continue;
                }
                if (!_playing.TryAdd(id, 0))
                {
                    continue;
                }

                try
                {
                    await PlayAsync(macro, cancellationToken).ConfigureAwait(false);
                }
                catch (OperationCanceledException)
                {
                    throw;
                }
                catch (Exception ex)
                {
                    // A failed playback must not stop the queue for other macros.
                    PlaybackFailed?.Invoke(this, ex.Message);
                }
                finally
                {
                    _playing.TryRemove(id, out _);
                }
            }
        }
        catch (OperationCanceledException)
        {
        }
    }

    private static async Task PlayAsync(MacroDefinition macro, CancellationToken cancellationToken)
    {
        foreach (var step in macro.Steps)
        {
            cancellationToken.ThrowIfCancellationRequested();
            if (step.Kind == "delay")
            {
                await Task.Delay(Math.Clamp(step.DelayMs, 0, 10_000), cancellationToken).ConfigureAwait(false);
                continue;
            }

            SendKeyChord(step);
            await Task.Delay(Math.Clamp(step.DelayMs, 0, 10_000), cancellationToken).ConfigureAwait(false);
        }
    }

    private static void SendKeyChord(MacroStep step)
    {
        var inputs = new List<INPUT>(12);
        var modifiers = new List<ushort>(4);
        if (step.Ctrl) modifiers.Add(VK_LCONTROL);
        if (step.Shift) modifiers.Add(VK_LSHIFT);
        if (step.Alt) modifiers.Add(VK_LMENU);
        if (step.Win) modifiers.Add(VK_LWIN);

        foreach (var modifier in modifiers)
        {
            AddKey(inputs, modifier, up: false);
        }
        AddKey(inputs, (ushort)step.VirtualKey, up: false);
        AddKey(inputs, (ushort)step.VirtualKey, up: true);
        for (var i = modifiers.Count - 1; i >= 0; i--)
        {
            AddKey(inputs, modifiers[i], up: true);
        }

        var array = inputs.ToArray();
        var sent = SendInput((uint)array.Length, array, Marshal.SizeOf<INPUT>());
        if (sent != array.Length)
        {
            throw new InvalidOperationException($"キー送出に失敗しました (Win32 {Marshal.GetLastWin32Error()})");
        }
    }

    private static void AddKey(List<INPUT> inputs, ushort virtualKey, bool up)
    {
        var flags = up ? KEYEVENTF_KEYUP : 0;
        if (IsExtendedKey(virtualKey))
        {
            flags |= KEYEVENTF_EXTENDEDKEY;
        }
        inputs.Add(new INPUT
        {
            type = INPUT_KEYBOARD,
            union = new InputUnion
            {
                keyboard = new KEYBDINPUT
                {
                    virtualKey = virtualKey,
                    flags = flags,
                },
            },
        });
    }

    private static bool IsExtendedKey(ushort virtualKey) => virtualKey is
        0x21 or 0x22 or 0x23 or 0x24 or 0x25 or 0x26 or 0x27 or 0x28 or
        0x2C or 0x2D or 0x2E or 0x5B or 0x5C or 0x5D or 0x6F or 0x90 or
        0xA3 or 0xA5;

    private static uint BuildModifiers(MacroTrigger trigger)
    {
        var modifiers = 0u;
        if (trigger.Ctrl) modifiers |= MOD_CONTROL;
        if (trigger.Shift) modifiers |= MOD_SHIFT;
        if (trigger.Alt) modifiers |= MOD_ALT;
        if (trigger.Win) modifiers |= MOD_WIN;
        return modifiers;
    }

    private static string? Validate(MacroDefinition macro, IReadOnlyList<MacroDefinition> existing)
    {
        if (macro.Name.Length > 80)
        {
            return "マクロ名は80文字以内で入力してください。";
        }
        var triggerError = ValidateTrigger(macro.Trigger);
        if (triggerError is not null)
        {
            return triggerError;
        }
        if (macro.Steps.Count == 0)
        {
            return "キー操作を1つ以上追加してください。";
        }
        if (macro.Steps.Count > 50)
        {
            return "1つのマクロに追加できる操作は50個までです。";
        }

        foreach (var step in macro.Steps)
        {
            if (step.Kind is not ("key" or "delay"))
            {
                return "不明な操作が含まれています。";
            }
            if (step.DelayMs is < 0 or > 10_000)
            {
                return "待機時間は0〜10000 msの範囲で指定してください。";
            }
            if (step.Kind == "key" && (step.VirtualKey is < 1 or > 254))
            {
                return "キーが指定されていない操作があります。";
            }
        }

        if (macro.Enabled)
        {
            foreach (var other in existing)
            {
                if (other.Id == macro.Id || !other.Enabled)
                {
                    continue;
                }
                if (other.Trigger.VirtualKey == macro.Trigger.VirtualKey &&
                    other.Trigger.Ctrl == macro.Trigger.Ctrl &&
                    other.Trigger.Shift == macro.Trigger.Shift &&
                    other.Trigger.Alt == macro.Trigger.Alt &&
                    other.Trigger.Win == macro.Trigger.Win)
                {
                    return $"「{other.Name}」と同じショートカットです。";
                }
            }
        }

        return null;
    }

    private static string? ValidateTrigger(MacroTrigger trigger)
    {
        if (trigger.VirtualKey is < 1 or > 254)
        {
            return "ショートカットのキーを入力してください。";
        }
        if (!trigger.Ctrl && !trigger.Shift && !trigger.Alt && !trigger.Win &&
            trigger.VirtualKey is not (>= 0x70 and <= 0x87))
        {
            return "通常キーには Ctrl / Shift / Alt / Win のいずれかを付けてください（F1〜F24は単独可）。";
        }
        return null;
    }

    private static MacroDefinition Clone(MacroDefinition macro) => new()
    {
        Id = macro.Id,
        Name = macro.Name,
        Enabled = macro.Enabled,
        CreatedAt = macro.CreatedAt,
        UpdatedAt = macro.UpdatedAt,
        Trigger = new MacroTrigger
        {
            Ctrl = macro.Trigger?.Ctrl ?? false,
            Shift = macro.Trigger?.Shift ?? false,
            Alt = macro.Trigger?.Alt ?? false,
            Win = macro.Trigger?.Win ?? false,
            VirtualKey = macro.Trigger?.VirtualKey ?? 0,
            KeyName = macro.Trigger?.KeyName ?? string.Empty,
        },
        Steps = (macro.Steps ?? []).Select(step => new MacroStep
        {
            Kind = step.Kind,
            Ctrl = step.Ctrl,
            Shift = step.Shift,
            Alt = step.Alt,
            Win = step.Win,
            VirtualKey = step.VirtualKey,
            KeyName = step.KeyName,
            DelayMs = step.DelayMs,
        }).ToList(),
    };

    [StructLayout(LayoutKind.Sequential)]
    private struct INPUT
    {
        public uint type;
        public InputUnion union;
    }

    [StructLayout(LayoutKind.Explicit)]
    private struct InputUnion
    {
        [FieldOffset(0)] public KEYBDINPUT keyboard;
    }

    [StructLayout(LayoutKind.Sequential)]
    private struct KEYBDINPUT
    {
        public ushort virtualKey;
        public ushort scanCode;
        public uint flags;
        public uint time;
        public IntPtr extraInfo;
    }

    [StructLayout(LayoutKind.Sequential)]
    private struct MSG
    {
        public IntPtr hwnd;
        public uint message;
        public IntPtr wParam;
        public IntPtr lParam;
        public uint time;
        public POINT point;
    }

    [StructLayout(LayoutKind.Sequential)]
    private struct POINT
    {
        public int x;
        public int y;
    }

    [DllImport("user32.dll", SetLastError = true)]
    private static extern bool RegisterHotKey(IntPtr hWnd, int id, uint modifiers, uint virtualKey);

    [DllImport("user32.dll", SetLastError = true)]
    private static extern bool UnregisterHotKey(IntPtr hWnd, int id);

    [DllImport("user32.dll")]
    private static extern int GetMessage(out MSG message, IntPtr hWnd, uint filterMin, uint filterMax);

    [DllImport("user32.dll", SetLastError = true)]
    private static extern bool PostThreadMessage(uint threadId, uint message, IntPtr wParam, IntPtr lParam);

    [DllImport("kernel32.dll")]
    private static extern uint GetCurrentThreadId();

    [DllImport("user32.dll", SetLastError = true)]
    private static extern uint SendInput(uint inputCount, INPUT[] inputs, int size);
}
