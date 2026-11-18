package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
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
		},
	}
	a.loadPrefs()

	a.svc = service.NewMouseService(nil, service.Config{
		InitialMode: service.ModeNormal,
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
		CloseToTray *bool `json:"closeToTray"`
	}
	if json.Unmarshal(raw, &disk) == nil && disk.CloseToTray != nil {
		a.prefs.CloseToTray = *disk.CloseToTray
	}
}

func (a *App) savePrefsLocked() {
	raw, err := json.MarshalIndent(map[string]any{
		"closeToTray": a.prefs.CloseToTray,
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
			_ = tray.RunWithOptions(trayCtx, a.svc, tray.Options{
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
			})
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
