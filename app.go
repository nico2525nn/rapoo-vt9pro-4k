package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/nico/rapoo-vt9pro-4k/internal/profile"
	"github.com/nico/rapoo-vt9pro-4k/internal/service"
	"github.com/nico/rapoo-vt9pro-4k/internal/tray"
	"github.com/nico/rapoo-vt9pro-4k/pkg/rapoo"
)

const wailsRegistryAutoStartKey = "RapooControl"

// AppPreferences holds desktop shell preferences (tray behavior, startup).
type AppPreferences struct {
	CloseToTray bool   `json:"closeToTray"`
	AutoStart   bool   `json:"autoStart"`
	Platform    string `json:"platform"`
	Mode        string `json:"mode"`
}

// App binds MouseService and desktop shell controls to the Wails v2 frontend.
type App struct {
	ctx        context.Context
	svc        *service.MouseService
	trayCancel context.CancelFunc
	quitting   atomic.Bool

	prefsMu   sync.RWMutex
	prefsPath string
	prefs     AppPreferences
}

// NewApp constructs the Wails application controller.
func NewApp() *App {
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		base = os.TempDir()
	}
	cfgDir := filepath.Join(base, "rapoo-vt9pro-4k")
	_ = os.MkdirAll(cfgDir, 0o755)

	a := &App{
		prefsPath: filepath.Join(cfgDir, "app-prefs.json"),
		prefs: AppPreferences{
			CloseToTray: runtime.GOOS == "windows",
			Platform:    runtime.GOOS,
			Mode:        string(service.ModeNormal),
		},
	}
	a.loadPrefs()

	initialMode := service.ModeNormal
	switch service.PollingMode(a.prefs.Mode) {
	case service.ModeNormal, service.ModeLowFrequency, service.ModePassiveOnly:
		initialMode = service.PollingMode(a.prefs.Mode)
	}
	a.svc = service.NewMouseService(nil, service.Config{
		InitialMode: initialMode,
		PresetDir:   cfgDir,
		OnEventEmit: func(eventName string, payload any) {
			if a.ctx != nil {
				wailsRuntime.EventsEmit(a.ctx, eventName, payload)
			}
		},
	})
	return a
}

func (a *App) loadPrefs() {
	raw, err := os.ReadFile(a.prefsPath)
	if err != nil {
		return
	}
	var disk struct {
		CloseToTray *bool   `json:"closeToTray"`
		Mode        *string `json:"mode"`
	}
	if json.Unmarshal(raw, &disk) == nil {
		if disk.CloseToTray != nil {
			a.prefs.CloseToTray = *disk.CloseToTray
		}
		if disk.Mode != nil {
			switch service.PollingMode(*disk.Mode) {
			case service.ModeNormal, service.ModeLowFrequency, service.ModePassiveOnly:
				a.prefs.Mode = *disk.Mode
			}
		}
	}
}

func (a *App) savePrefsLocked() {
	raw, err := json.MarshalIndent(map[string]any{
		"closeToTray": a.prefs.CloseToTray,
		"mode":        a.prefs.Mode,
	}, "", "  ")
	if err == nil {
		_ = os.WriteFile(a.prefsPath, raw, 0o644)
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.svc.Startup(ctx)

	if runtime.GOOS == "windows" {
		trayCtx, cancel := context.WithCancel(ctx)
		a.trayCancel = cancel
		go func() {
			if err := tray.RunWithOptions(trayCtx, a.svc, tray.Options{
				MutexName:         `Local\RapooControlWailsTrayMutex`,
				WindowClass:       "RapooControlWailsTrayWndClass",
				RegistryValueName: wailsRegistryAutoStartKey,
				OpenMenuLabel:     "Rapoo Control を開く(&O)",
				OnOpenWindow: func() {
					a.ShowWindow()
				},
				OnQuit: func() {
					a.QuitApp()
				},
			}); err != nil {
				log.Printf("rapoo: tray unavailable: %v", err)
				if a.ctx != nil {
					wailsRuntime.EventsEmit(a.ctx, "rapoo:error", "タスクトレイを初期化できませんでした")
				}
			}
		}()
	}
}

func (a *App) beforeClose(ctx context.Context) (prevent bool) {
	if a.quitting.Load() {
		return false
	}
	a.prefsMu.RLock()
	closeToTray := a.prefs.CloseToTray
	a.prefsMu.RUnlock()

	if runtime.GOOS == "windows" && closeToTray {
		wailsRuntime.WindowHide(ctx)
		wailsRuntime.EventsEmit(ctx, "rapoo:minimized-to-tray", true)
		return true
	}
	return false
}

func (a *App) shutdown(_ context.Context) {
	if a.trayCancel != nil {
		a.trayCancel()
	}
	a.svc.Shutdown()
}

// ShowWindow restores and focuses the main Wails window.
func (a *App) ShowWindow() {
	if a.ctx == nil {
		return
	}
	wailsRuntime.WindowShow(a.ctx)
	wailsRuntime.WindowUnminimise(a.ctx)
}

// QuitApp terminates the entire application (bypassing close-to-tray).
func (a *App) QuitApp() {
	a.quitting.Store(true)
	if a.ctx != nil {
		wailsRuntime.Quit(a.ctx)
	}
}

// GetAppPreferences returns desktop shell preferences.
func (a *App) GetAppPreferences() AppPreferences {
	a.prefsMu.RLock()
	p := a.prefs
	a.prefsMu.RUnlock()
	p.Platform = runtime.GOOS
	p.AutoStart = tray.IsAutoStartEnabled(wailsRegistryAutoStartKey)
	return p
}

// SetCloseToTray configures whether closing the window hides to the Windows system tray.
func (a *App) SetCloseToTray(enabled bool) AppPreferences {
	a.prefsMu.Lock()
	a.prefs.CloseToTray = enabled
	a.savePrefsLocked()
	a.prefsMu.Unlock()
	return a.GetAppPreferences()
}

// SetAutoStart toggles Windows login auto-start in HKCU\...\Run.
func (a *App) SetAutoStart(enabled bool) (AppPreferences, error) {
	err := tray.SetAutoStartEnabled(wailsRegistryAutoStartKey, enabled)
	return a.GetAppPreferences(), err
}

// SetWindowTheme syncs the native OS window titlebar appearance with the UI dark/light mode.
func (a *App) SetWindowTheme(dark bool) {
	if a.ctx == nil || runtime.GOOS != "windows" {
		return
	}
	if dark {
		wailsRuntime.WindowSetDarkTheme(a.ctx)
	} else {
		wailsRuntime.WindowSetLightTheme(a.ctx)
	}
}

// CloseWindow closes the custom title bar window while respecting close-to-tray.
func (a *App) CloseWindow() {
	if a.ctx == nil {
		return
	}
	a.prefsMu.RLock()
	closeToTray := a.prefs.CloseToTray
	a.prefsMu.RUnlock()

	if runtime.GOOS == "windows" && closeToTray {
		wailsRuntime.WindowHide(a.ctx)
		wailsRuntime.EventsEmit(a.ctx, "rapoo:minimized-to-tray", true)
		return
	}
	a.quitting.Store(true)
	wailsRuntime.Quit(a.ctx)
}

// OpenFirmwareUpdater launches the official Rapoo software, which performs the
// actual firmware write. This app only reads firmware versions and never flashes.
func (a *App) OpenFirmwareUpdater() (string, error) {
	if runtime.GOOS != "windows" {
		return "", errors.New("ファームウェア更新はWindows版の公式ソフトでのみ利用できます")
	}

	roots := []string{
		os.Getenv("ProgramFiles(x86)"),
		os.Getenv("ProgramFiles"),
		filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs"),
		os.Getenv("LOCALAPPDATA"),
		os.Getenv("APPDATA"),
	}
	seen := make(map[string]bool)
	var candidates []string
	for _, exe := range []string{"RapooGameDev.exe", "FirmwareExe.exe"} {
		for _, root := range roots {
			if root == "" {
				continue
			}
			path := filepath.Join(root, "Rapoo", "RapooGameDev", exe)
			if !seen[path] {
				seen[path] = true
				candidates = append(candidates, path)
			}
		}
	}

	for _, path := range candidates {
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			continue
		}
		return a.launchOfficialUpdater(path)
	}

	if a.ctx == nil {
		return "", errors.New("公式ソフトが見つかりません")
	}
	selected, err := wailsRuntime.OpenFileDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "Rapoo公式ソフトを選択",
		Filters: []wailsRuntime.FileFilter{
			{DisplayName: "RapooGameDev.exe / FirmwareExe.exe", Pattern: "*.exe"},
		},
	})
	if err != nil {
		return "", err
	}
	if selected == "" {
		return "", nil
	}
	if !isOfficialUpdaterName(selected) {
		return "", errors.New("Rapoo公式ソフト（RapooGameDev.exe または FirmwareExe.exe）を選択してください")
	}
	if !filepath.IsAbs(selected) {
		return "", errors.New("公式ソフトの絶対パスを選択してください")
	}
	info, err := os.Stat(selected)
	if err != nil || info.IsDir() {
		return "", errors.New("公式ソフトが見つかりません")
	}
	return a.launchOfficialUpdater(selected)
}

func (a *App) launchOfficialUpdater(path string) (string, error) {
	previousMode := a.svc.GetState().Mode
	// Release the vendor HID handle before the official updater takes over.
	a.svc.SetPollingMode(service.ModePassiveOnly)

	cmd := exec.Command(path)
	cmd.Dir = filepath.Dir(path)
	if err := cmd.Start(); err != nil {
		a.svc.SetPollingMode(previousMode)
		return "", err
	}
	go func() {
		_ = cmd.Wait()
		a.svc.SetPollingMode(previousMode)
	}()
	return path, nil
}

func isOfficialUpdaterName(path string) bool {
	switch strings.ToLower(filepath.Base(path)) {
	case "rapoogamedev.exe", "firmwareexe.exe":
		return true
	default:
		return false
	}
}

// SavePresetFile opens a native OS Save File dialog to export a preset JSON file reliably on Windows & Linux.
func (a *App) SavePresetFile(id, suggestedName string) (string, error) {
	raw, err := a.svc.ExportPresetJSON(id)
	if err != nil {
		return "", err
	}
	if a.ctx == nil {
		return "", nil
	}
	clean := strings.Map(func(r rune) rune {
		switch r {
		case '\\', '/', ':', '*', '?', '"', '<', '>', '|':
			return '_'
		default:
			return r
		}
	}, strings.TrimSpace(suggestedName))
	if clean == "" {
		clean = "rapoo-preset"
	}
	if !strings.HasSuffix(strings.ToLower(clean), ".json") {
		clean += ".json"
	}
	dest, err := wailsRuntime.SaveFileDialog(a.ctx, wailsRuntime.SaveDialogOptions{
		Title:           "プロファイル JSON を保存",
		DefaultFilename: clean,
		Filters: []wailsRuntime.FileFilter{
			{DisplayName: "JSON Files (*.json)", Pattern: "*.json"},
		},
	})
	if err != nil {
		return "", err
	}
	if dest == "" {
		return "", nil
	}
	if err := os.WriteFile(dest, []byte(raw), 0o644); err != nil {
		return "", err
	}
	return dest, nil
}

// ============================================================================
// Delegated MouseService Bindings for Wails Frontend
// ============================================================================

func (a *App) GetCatalog() service.UICatalog {
	return a.svc.GetCatalog()
}

func (a *App) GetState() service.State {
	return a.svc.GetState()
}

func (a *App) RefreshNow() {
	a.svc.RefreshNow()
}

func (a *App) ReadSnapshot() (rapoo.DeviceSnapshot, error) {
	return a.svc.ReadSnapshot()
}

func (a *App) SetDemoMode(enabled bool) (rapoo.DeviceSnapshot, error) {
	return a.svc.SetDemoMode(enabled)
}

func (a *App) SetPollingMode(mode string) service.State {
	switch service.PollingMode(mode) {
	case service.ModeNormal, service.ModeLowFrequency, service.ModePassiveOnly:
		a.svc.SetPollingMode(service.PollingMode(mode))
		a.prefsMu.Lock()
		a.prefs.Mode = mode
		a.savePrefsLocked()
		a.prefsMu.Unlock()
	}
	return a.svc.GetState()
}

func (a *App) SetPollingRate(hz int) (rapoo.DeviceSnapshot, error) {
	return a.svc.SetPollingRate(hz)
}

func (a *App) SetCurrentDPIGear(stage int) (rapoo.DeviceSnapshot, error) {
	return a.svc.SetCurrentDPIGear(stage)
}

func (a *App) SetDPIConfig(cfg rapoo.DPIConfig) (rapoo.DeviceSnapshot, error) {
	return a.svc.SetDPIConfig(cfg)
}

func (a *App) UpdateSensorConfig(patch rapoo.SensorPatch) (rapoo.DeviceSnapshot, error) {
	return a.svc.UpdateSensorConfig(patch)
}

func (a *App) SetButtonMapping(buttonID int, action rapoo.ButtonAction) (rapoo.DeviceSnapshot, error) {
	return a.svc.SetButtonMapping(buttonID, action)
}

func (a *App) ResetButtonsToDefault() (rapoo.DeviceSnapshot, error) {
	return a.svc.ResetButtonsToDefault()
}

func (a *App) FactoryReset() (rapoo.DeviceSnapshot, error) {
	return a.svc.FactoryReset()
}

func (a *App) ListPresets() []profile.Preset {
	return a.svc.ListPresets()
}

func (a *App) SavePreset(id, name, description string) (profile.Preset, error) {
	return a.svc.SavePreset(id, name, description)
}

func (a *App) ApplyPreset(id string) (rapoo.DeviceSnapshot, error) {
	return a.svc.ApplyPreset(id)
}

func (a *App) DeletePreset(id string) error {
	return a.svc.DeletePreset(id)
}

func (a *App) ExportPresetJSON(id string) (string, error) {
	return a.svc.ExportPresetJSON(id)
}

func (a *App) ImportPresetJSON(rawJSON string) (profile.Preset, error) {
	return a.svc.ImportPresetJSON(rawJSON)
}
