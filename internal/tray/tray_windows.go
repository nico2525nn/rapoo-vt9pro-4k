//go:build windows

package tray

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"

	"github.com/nico/rapoo-vt9pro-4k/internal/icon"
	"github.com/nico/rapoo-vt9pro-4k/internal/service"
	"github.com/nico/rapoo-vt9pro-4k/pkg/rapoo"
)

// ErrAlreadyRunning indicates another tray instance with the same MutexName is already active.
var ErrAlreadyRunning = errors.New("vt9tray is already running")

var (
	modUser32   = syscall.NewLazyDLL("user32.dll")
	modShell32  = syscall.NewLazyDLL("shell32.dll")
	modKernel32 = syscall.NewLazyDLL("kernel32.dll")
	modAdvapi32 = syscall.NewLazyDLL("advapi32.dll")

	procSetProcessDpiAwarenessContext = modUser32.NewProc("SetProcessDpiAwarenessContext")
	procGetModuleHandleW              = modKernel32.NewProc("GetModuleHandleW")
	procCreateMutexW                  = modKernel32.NewProc("CreateMutexW")
	procRegisterClassExW              = modUser32.NewProc("RegisterClassExW")
	procCreateWindowExW               = modUser32.NewProc("CreateWindowExW")
	procDefWindowProcW                = modUser32.NewProc("DefWindowProcW")
	procDestroyWindow                 = modUser32.NewProc("DestroyWindow")
	procGetMessageW                   = modUser32.NewProc("GetMessageW")
	procTranslateMessage              = modUser32.NewProc("TranslateMessage")
	procDispatchMessageW              = modUser32.NewProc("DispatchMessageW")
	procPostMessageW                  = modUser32.NewProc("PostMessageW")
	procPostQuitMessage               = modUser32.NewProc("PostQuitMessage")
	procRegisterWindowMessageW        = modUser32.NewProc("RegisterWindowMessageW")
	procGetSystemMetrics              = modUser32.NewProc("GetSystemMetrics")
	procCreateIconFromResourceEx      = modUser32.NewProc("CreateIconFromResourceEx")
	procDestroyIcon                   = modUser32.NewProc("DestroyIcon")
	procCreatePopupMenu               = modUser32.NewProc("CreatePopupMenu")
	procAppendMenuW                   = modUser32.NewProc("AppendMenuW")
	procDestroyMenu                   = modUser32.NewProc("DestroyMenu")
	procSetForegroundWindow           = modUser32.NewProc("SetForegroundWindow")
	procGetCursorPos                  = modUser32.NewProc("GetCursorPos")
	procTrackPopupMenu                = modUser32.NewProc("TrackPopupMenu")
	procRegisterDeviceNotificationW   = modUser32.NewProc("RegisterDeviceNotificationW")
	procUnregisterDeviceNotification  = modUser32.NewProc("UnregisterDeviceNotification")
	procShellNotifyIconW              = modShell32.NewProc("Shell_NotifyIconW")

	procRegOpenKeyExW    = modAdvapi32.NewProc("RegOpenKeyExW")
	procRegQueryValueExW = modAdvapi32.NewProc("RegQueryValueExW")
	procRegSetValueExW   = modAdvapi32.NewProc("RegSetValueExW")
	procRegDeleteValueW  = modAdvapi32.NewProc("RegDeleteValueW")
	procRegCloseKey      = modAdvapi32.NewProc("RegCloseKey")
)

const (
	errorAlreadyExists syscall.Errno = 183

	wmDestroy       = 0x0002
	wmClose         = 0x0010
	wmCommand       = 0x0111
	wmDeviceChange  = 0x0219
	wmDpiChanged    = 0x02E0
	wmLButtonUp     = 0x0202
	wmLButtonDblClk = 0x0203
	wmRButtonUp     = 0x0205
	wmContextMenu   = 0x007B
	wmAppTray       = 0x8001
	wmAppStateUpd   = 0x8002

	dbtDeviceArrival        = 0x8000
	dbtDeviceRemoveComplete = 0x8004
	dbtDevtypDeviceInterf   = 0x00000005
	deviceNotifyWindowHdl   = 0x00000000

	nimAdd    = 0x00000000
	nimModify = 0x00000001
	nimDelete = 0x00000002

	nifMessage = 0x00000001
	nifIcon    = 0x00000002
	nifTip     = 0x00000004

	smCxSmIcon = 49

	mfString     = 0x00000000
	mfSeparator  = 0x00000800
	mfGrayed     = 0x00000001
	mfDisabled   = 0x00000002
	mfChecked    = 0x00000008
	tpmRightBtn  = 0x0002
	tpmBottomAln = 0x0020
	tpmReturnCmd = 0x0100

	cmdOpenWindow  = 1000
	cmdRefresh     = 1001
	cmdModeNormal  = 1002
	cmdModeLowFreq = 1003
	cmdModePassive = 1004
	cmdAutoStart   = 1005
	cmdQuit        = 1099

	hkeyCurrentUser = syscall.Handle(0x80000001)
	keyRead         = 0x20019
	keySetValue     = 0x0002
	regSz           = 1

	runRegistryPath         = `Software\Microsoft\Windows\CurrentVersion\Run`
	defaultRunRegistryValue = "RapooVT9Tray"
)

type winGUID struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

var hidClassGUID = winGUID{
	Data1: 0x4D1E55B2,
	Data2: 0xF16F,
	Data3: 0x11CF,
	Data4: [8]byte{0x88, 0xCB, 0x00, 0x11, 0x11, 0x00, 0x00, 0x30},
}

type wndClassExW struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     syscall.Handle
	HIcon         syscall.Handle
	HCursor       syscall.Handle
	HbrBackground syscall.Handle
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       syscall.Handle
}

type notifyIconDataW struct {
	CbSize           uint32
	HWnd             syscall.Handle
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	HIcon            syscall.Handle
	SzTip            [128]uint16
	DwState          uint32
	DwStateMask      uint32
	SzInfo           [256]uint16
	UVersion         uint32
	SzInfoTitle      [64]uint16
	DwInfoFlags      uint32
	GuidItem         winGUID
	HBalloonIcon     syscall.Handle
}

type devBroadcastDeviceInterfaceW struct {
	DbccSize       uint32
	DbccDeviceType uint32
	DbccReserved   uint32
	DbccClassGuid  winGUID
	DbccName       [1]uint16
}

type winPoint struct {
	X int32
	Y int32
}

type winMsg struct {
	HWnd    syscall.Handle
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      winPoint
}

var activeApp *trayApp

type trayApp struct {
	svc               *service.MouseService
	opts              Options
	hwnd              syscall.Handle
	devNotify         uintptr
	currentIcon       syscall.Handle
	trayAdded         bool
	msgTaskbarCreated uint32
	userQuit          bool

	mu          sync.Mutex
	latestState service.State
}

// Run starts the standalone Win32 system tray icon with default options.
func Run(ctx context.Context, svc *service.MouseService) error {
	return RunWithOptions(ctx, svc, Options{})
}

// RunWithOptions starts the native Win32 system tray icon and blocks until Quit or ctx cancellation.
func RunWithOptions(ctx context.Context, svc *service.MouseService, opts Options) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if opts.MutexName == "" {
		opts.MutexName = `Local\RapooVT9Pro4KTrayMutex`
	}
	if opts.WindowClass == "" {
		opts.WindowClass = "RapooVT9Pro4KTrayWndClass"
	}
	if opts.RegistryValueName == "" {
		opts.RegistryValueName = defaultRunRegistryValue
	}
	opts.Language = normalizeLanguage(opts.Language)
	if opts.Language == "" {
		opts.Language = normalizeLanguage(os.Getenv("RAPOO_LANG"))
	}

	if procSetProcessDpiAwarenessContext.Find() == nil {
		procSetProcessDpiAwarenessContext.Call(^uintptr(3))
	}

	mutexName, _ := syscall.UTF16PtrFromString(opts.MutexName)
	hMutex, _, lastErr := procCreateMutexW.Call(0, 0, uintptr(unsafe.Pointer(mutexName)))
	if hMutex == 0 {
		return fmt.Errorf("CreateMutexW: %w", lastErr)
	}
	defer syscall.CloseHandle(syscall.Handle(hMutex))
	if errors.Is(lastErr, errorAlreadyExists) {
		return ErrAlreadyRunning
	}

	app := &trayApp{
		svc:         svc,
		opts:        opts,
		latestState: svc.GetState(),
	}
	activeApp = app
	defer func() { activeApp = nil }()

	tbStr, _ := syscall.UTF16PtrFromString("TaskbarCreated")
	tbMsg, _, _ := procRegisterWindowMessageW.Call(uintptr(unsafe.Pointer(tbStr)))
	app.msgTaskbarCreated = uint32(tbMsg)

	hInstRaw, _, _ := procGetModuleHandleW.Call(0)
	hInst := syscall.Handle(hInstRaw)

	className, _ := syscall.UTF16PtrFromString(opts.WindowClass)
	wcx := wndClassExW{
		CbSize:        uint32(unsafe.Sizeof(wndClassExW{})),
		LpfnWndProc:   syscall.NewCallback(trayWndProc),
		HInstance:     hInst,
		LpszClassName: className,
	}
	_, _, _ = procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wcx)))

	wndTitle, _ := syscall.UTF16PtrFromString("Rapoo VT9 Pro 4K Battery Tray")
	hwndRaw, _, err := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(wndTitle)),
		0,
		0, 0, 0, 0,
		0,
		0,
		uintptr(hInst),
		0,
	)
	if hwndRaw == 0 {
		return fmt.Errorf("CreateWindowExW: %w", err)
	}
	app.hwnd = syscall.Handle(hwndRaw)

	var dbi devBroadcastDeviceInterfaceW
	dbi.DbccSize = uint32(unsafe.Sizeof(dbi))
	dbi.DbccDeviceType = dbtDevtypDeviceInterf
	dbi.DbccClassGuid = hidClassGUID
	notifyH, _, _ := procRegisterDeviceNotificationW.Call(
		uintptr(app.hwnd),
		uintptr(unsafe.Pointer(&dbi)),
		deviceNotifyWindowHdl,
	)
	app.devNotify = notifyH

	if err := app.syncTrayIcon(true); err != nil {
		app.cleanup()
		return err
	}

	unsub := svc.Subscribe(func(st service.State) {
		app.mu.Lock()
		app.latestState = st
		app.mu.Unlock()
		procPostMessageW.Call(uintptr(app.hwnd), wmAppStateUpd, 0, 0)
	})
	defer unsub()

	doneCh := make(chan struct{})
	defer close(doneCh)
	go func() {
		select {
		case <-ctx.Done():
			procPostMessageW.Call(uintptr(app.hwnd), wmClose, 0, 0)
		case <-doneCh:
		}
	}()

	var msg winMsg
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(ret) <= 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}

	app.cleanup()
	if app.userQuit && opts.OnQuit != nil {
		opts.OnQuit()
	}
	return nil
}

func trayWndProc(hwnd syscall.Handle, msg uint32, wParam, lParam uintptr) uintptr {
	app := activeApp
	if app != nil && app.msgTaskbarCreated != 0 && msg == app.msgTaskbarCreated {
		app.trayAdded = false
		_ = app.syncTrayIcon(true)
		return 0
	}

	switch msg {
	case wmAppStateUpd, wmDpiChanged:
		if app != nil {
			_ = app.syncTrayIcon(false)
		}
		return 0

	case wmDeviceChange:
		if app != nil && (wParam == dbtDeviceArrival || wParam == dbtDeviceRemoveComplete) {
			app.svc.NotifyDeviceChange()
		}
		return 0

	case wmAppTray:
		if app != nil {
			switch uint32(lParam) & 0xFFFF {
			case wmLButtonUp, wmLButtonDblClk:
				if app.opts.OnOpenWindow != nil {
					app.opts.OnOpenWindow()
				} else {
					app.svc.RefreshNow()
				}
			case wmRButtonUp, wmContextMenu:
				app.showPopupMenu()
			}
		}
		return 0

	case wmClose:
		procDestroyWindow.Call(uintptr(hwnd))
		return 0

	case wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	}

	ret, _, _ := procDefWindowProcW.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	return ret
}

func (a *trayApp) syncTrayIcon(forceAdd bool) error {
	a.mu.Lock()
	st := a.latestState
	a.mu.Unlock()

	metric, _, _ := procGetSystemMetrics.Call(smCxSmIcon)
	iconPx := int(metric)
	if iconPx < 16 {
		iconPx = 16
	}

	dib := icon.RenderDIB(iconPx, icon.VisualSpec{
		Connected:  st.Connected,
		HasBattery: st.HasBattery,
		Percentage: st.Percentage,
		Charging:   st.Charging,
		Sleeping:   st.Sleeping,
	})

	hIconRaw, _, err := procCreateIconFromResourceEx.Call(
		uintptr(unsafe.Pointer(&dib[0])),
		uintptr(len(dib)),
		1,
		0x00030000,
		uintptr(iconPx),
		uintptr(iconPx),
		0,
	)
	if hIconRaw == 0 {
		return fmt.Errorf("CreateIconFromResourceEx: %w", err)
	}
	newIcon := syscall.Handle(hIconRaw)

	var nid notifyIconDataW
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	nid.HWnd = a.hwnd
	nid.UID = 1
	nid.UFlags = nifMessage | nifIcon | nifTip
	nid.UCallbackMessage = wmAppTray
	nid.HIcon = newIcon
	copyUTF16Tip(nid.SzTip[:], formatTooltip(st, a.opts.Language))

	action := uintptr(nimModify)
	if forceAdd || !a.trayAdded {
		action = nimAdd
	}
	ok, _, nerr := procShellNotifyIconW.Call(action, uintptr(unsafe.Pointer(&nid)))
	if ok == 0 && action == nimModify {
		ok, _, nerr = procShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&nid)))
	}
	if ok == 0 {
		procDestroyIcon.Call(uintptr(newIcon))
		return fmt.Errorf("Shell_NotifyIconW: %w", nerr)
	}

	a.trayAdded = true
	if a.currentIcon != 0 {
		procDestroyIcon.Call(uintptr(a.currentIcon))
	}
	a.currentIcon = newIcon
	return nil
}

func (a *trayApp) showPopupMenu() {
	a.mu.Lock()
	st := a.latestState
	a.mu.Unlock()

	hMenu, _, _ := procCreatePopupMenu.Call()
	if hMenu == 0 {
		return
	}
	defer procDestroyMenu.Call(hMenu)

	appendMenuItem(hMenu, mfString|mfGrayed|mfDisabled, 0, formatMenuHeader(st, a.opts.Language))
	if !st.UpdatedAt.IsZero() {
		appendMenuItem(hMenu, mfString|mfGrayed|mfDisabled, 0,
			fmt.Sprintf(a.text("Last updated: %s", "最終更新: %s"), st.UpdatedAt.Local().Format("15:04:05")))
	}
	appendMenuItem(hMenu, mfSeparator, 0, "")

	if a.opts.OnOpenWindow != nil {
		lbl := a.opts.OpenMenuLabel
		if lbl == "" {
			lbl = a.text("Open Rapoo Control (&O)", "Rapoo Control を開く(&O)")
		}
		appendMenuItem(hMenu, mfString, cmdOpenWindow, lbl)
		appendMenuItem(hMenu, mfSeparator, 0, "")
	}

	refreshFlags := uintptr(mfString)
	if st.Mode == service.ModePassiveOnly {
		refreshFlags |= mfGrayed | mfDisabled
	}
	appendMenuItem(hMenu, refreshFlags, cmdRefresh, a.text("Refresh battery now (&R)", "今すぐバッテリー取得(&R)"))
	appendMenuItem(hMenu, mfSeparator, 0, "")

	appendMenuItem(hMenu, modeFlag(st.Mode == service.ModeNormal), cmdModeNormal,
		a.text("Normal mode: 60s refresh + notifications", "通常モード: 60秒更新 + 自動通知"))
	appendMenuItem(hMenu, modeFlag(st.Mode == service.ModeLowFrequency), cmdModeLowFreq,
		a.text("Low-frequency mode: 5m refresh + notifications", "低頻度モード: 5分更新 + 自動通知"))
	appendMenuItem(hMenu, modeFlag(st.Mode == service.ModePassiveOnly), cmdModePassive,
		a.text("Gaming passive (0-TX): no writes (notifications only)", "ゲーム優先・完全受動: 送信ゼロ (通知のみ)"))
	appendMenuItem(hMenu, mfSeparator, 0, "")

	autoFlags := uintptr(mfString)
	if IsAutoStartEnabled(a.opts.RegistryValueName) {
		autoFlags |= mfChecked
	}
	appendMenuItem(hMenu, autoFlags, cmdAutoStart, a.text("Start with Windows (&S)", "Windows起動時に自動起動(&S)"))
	appendMenuItem(hMenu, mfSeparator, 0, "")
	appendMenuItem(hMenu, mfString, cmdQuit, a.text("Quit (&Q)", "終了(&Q)"))

	var pt winPoint
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	procSetForegroundWindow.Call(uintptr(a.hwnd))

	cmd, _, _ := procTrackPopupMenu.Call(
		hMenu,
		tpmRightBtn|tpmBottomAln|tpmReturnCmd,
		uintptr(pt.X),
		uintptr(pt.Y),
		0,
		uintptr(a.hwnd),
		0,
	)
	procPostMessageW.Call(uintptr(a.hwnd), 0, 0, 0)

	switch int(cmd) {
	case cmdOpenWindow:
		if a.opts.OnOpenWindow != nil {
			a.opts.OnOpenWindow()
		}
	case cmdRefresh:
		a.svc.RefreshNow()
	case cmdModeNormal:
		a.svc.SetPollingMode(service.ModeNormal)
	case cmdModeLowFreq:
		a.svc.SetPollingMode(service.ModeLowFrequency)
	case cmdModePassive:
		a.svc.SetPollingMode(service.ModePassiveOnly)
	case cmdAutoStart:
		_ = SetAutoStartEnabled(a.opts.RegistryValueName, !IsAutoStartEnabled(a.opts.RegistryValueName))
	case cmdQuit:
		a.userQuit = true
		procPostMessageW.Call(uintptr(a.hwnd), wmClose, 0, 0)
	}
}

func (a *trayApp) cleanup() {
	if a.devNotify != 0 {
		procUnregisterDeviceNotification.Call(a.devNotify)
		a.devNotify = 0
	}
	if a.trayAdded && a.hwnd != 0 {
		var nid notifyIconDataW
		nid.CbSize = uint32(unsafe.Sizeof(nid))
		nid.HWnd = a.hwnd
		nid.UID = 1
		procShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&nid)))
		a.trayAdded = false
	}
	if a.currentIcon != 0 {
		procDestroyIcon.Call(uintptr(a.currentIcon))
		a.currentIcon = 0
	}
}

func appendMenuItem(hMenu, flags, id uintptr, label string) {
	if flags&mfSeparator != 0 {
		procAppendMenuW.Call(hMenu, flags, 0, 0)
		return
	}
	ptr, _ := syscall.UTF16PtrFromString(label)
	procAppendMenuW.Call(hMenu, flags, id, uintptr(unsafe.Pointer(ptr)))
}

func modeFlag(checked bool) uintptr {
	if checked {
		return mfString | mfChecked
	}
	return mfString
}

func formatConnLabel(conn rapoo.ConnectionMode, lang string) string {
	switch conn {
	case rapoo.ConnectionWired:
		if lang == "ja" {
			return "USB有線"
		}
		return "USB wired"
	case rapoo.ConnectionWireless4K:
		if lang == "ja" {
			return "4K無線"
		}
		return "4K wireless"
	default:
		return ""
	}
}

func formatMenuHeader(st service.State, lang string) string {
	if !st.Connected {
		if lang == "ja" {
			return "Rapoo VT9 Pro 4K — 未接続"
		}
		return "Rapoo VT9 Pro 4K — Not connected"
	}
	conn := formatConnLabel(st.Connection, lang)
	if !st.HasBattery {
		if st.Sleeping {
			if lang == "ja" {
				return fmt.Sprintf("%s (%s) — スリープ中", st.DeviceName, conn)
			}
			return fmt.Sprintf("%s (%s) — Sleeping", st.DeviceName, conn)
		}
		if lang == "ja" {
			return fmt.Sprintf("%s (%s) — 取得待ち", st.DeviceName, conn)
		}
		return fmt.Sprintf("%s (%s) — Waiting for data", st.DeviceName, conn)
	}
	switch {
	case st.Charging:
		if lang == "ja" {
			return fmt.Sprintf("%s (%s) — %d%% 充電中", st.DeviceName, conn, st.Percentage)
		}
		return fmt.Sprintf("%s (%s) — %d%% charging", st.DeviceName, conn, st.Percentage)
	case st.Sleeping:
		if lang == "ja" {
			return fmt.Sprintf("%s (%s) — %d%% (スリープ中)", st.DeviceName, conn, st.Percentage)
		}
		return fmt.Sprintf("%s (%s) — %d%% (sleeping)", st.DeviceName, conn, st.Percentage)
	default:
		return fmt.Sprintf("%s (%s) — %d%%", st.DeviceName, conn, st.Percentage)
	}
}

func formatTooltip(st service.State, lang string) string {
	header := formatMenuHeader(st, lang)
	var modeStr string
	switch st.Mode {
	case service.ModePassiveOnly:
		if lang == "ja" {
			modeStr = "モード: 完全受動 (送信ゼロ)"
		} else {
			modeStr = "Mode: passive (0-TX)"
		}
	case service.ModeLowFrequency:
		if lang == "ja" {
			modeStr = "モード: 低頻度 (5分)"
		} else {
			modeStr = "Mode: low frequency (5m)"
		}
	default:
		if lang == "ja" {
			modeStr = "モード: 通常 (60秒)"
		} else {
			modeStr = "Mode: normal (60s)"
		}
	}
	if !st.UpdatedAt.IsZero() {
		if lang == "ja" {
			return fmt.Sprintf("%s\n%s | 更新: %s", header, modeStr, st.UpdatedAt.Local().Format(time.TimeOnly))
		}
		return fmt.Sprintf("%s\n%s | Updated: %s", header, modeStr, st.UpdatedAt.Local().Format(time.TimeOnly))
	}
	return fmt.Sprintf("%s\n%s", header, modeStr)
}

func normalizeLanguage(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "ja" || value == "jp" || strings.HasPrefix(value, "ja-") {
		return "ja"
	}
	return "en"
}

func (a *trayApp) text(en, ja string) string {
	if normalizeLanguage(a.opts.Language) == "ja" {
		return ja
	}
	return en
}

func copyUTF16Tip(dst []uint16, s string) {
	enc := utf16.Encode([]rune(s))
	if len(enc) >= len(dst) {
		enc = enc[:len(dst)-1]
	}
	copy(dst, enc)
	dst[len(enc)] = 0
}

// IsAutoStartEnabled reports whether HKCU\...\Run contains valueName.
func IsAutoStartEnabled(valueName string) bool {
	if valueName == "" {
		valueName = defaultRunRegistryValue
	}
	keyPath, _ := syscall.UTF16PtrFromString(runRegistryPath)
	valName, _ := syscall.UTF16PtrFromString(valueName)
	var hKey syscall.Handle
	r, _, _ := procRegOpenKeyExW.Call(
		uintptr(hkeyCurrentUser),
		uintptr(unsafe.Pointer(keyPath)),
		0,
		keyRead,
		uintptr(unsafe.Pointer(&hKey)),
	)
	if r != 0 {
		return false
	}
	defer procRegCloseKey.Call(uintptr(hKey))

	var dataLen uint32
	r2, _, _ := procRegQueryValueExW.Call(
		uintptr(hKey),
		uintptr(unsafe.Pointer(valName)),
		0,
		0,
		0,
		uintptr(unsafe.Pointer(&dataLen)),
	)
	return r2 == 0 && dataLen > 0
}

// SetAutoStartEnabled writes or deletes the current executable path in HKCU\...\Run under valueName.
func SetAutoStartEnabled(valueName string, enable bool) error {
	if valueName == "" {
		valueName = defaultRunRegistryValue
	}
	keyPath, _ := syscall.UTF16PtrFromString(runRegistryPath)
	valName, _ := syscall.UTF16PtrFromString(valueName)
	var hKey syscall.Handle
	r, _, _ := procRegOpenKeyExW.Call(
		uintptr(hkeyCurrentUser),
		uintptr(unsafe.Pointer(keyPath)),
		0,
		keySetValue,
		uintptr(unsafe.Pointer(&hKey)),
	)
	if r != 0 {
		return fmt.Errorf("RegOpenKeyExW error %d", r)
	}
	defer procRegCloseKey.Call(uintptr(hKey))

	if !enable {
		procRegDeleteValueW.Call(uintptr(hKey), uintptr(unsafe.Pointer(valName)))
		return nil
	}

	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	cmdLine := fmt.Sprintf(`"%s"`, exePath)
	u16, err := syscall.UTF16FromString(cmdLine)
	if err != nil {
		return err
	}
	r2, _, _ := procRegSetValueExW.Call(
		uintptr(hKey),
		uintptr(unsafe.Pointer(valName)),
		0,
		regSz,
		uintptr(unsafe.Pointer(&u16[0])),
		uintptr(len(u16)*2),
	)
	if r2 != 0 {
		return fmt.Errorf("RegSetValueExW error %d", r2)
	}
	return nil
}
