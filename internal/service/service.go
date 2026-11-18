// Package service provides a Wails-ready application service that manages
// device discovery, hotplug reconnection, passive notifications, gentle
// battery polling, full onboard settings read/write, Demo Mode simulation,
// and local preset management.
package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/nico/rapoo-vt9pro-4k/internal/profile"
	"github.com/nico/rapoo-vt9pro-4k/pkg/hid"
	"github.com/nico/rapoo-vt9pro-4k/pkg/rapoo"
)

// PollingMode controls how frequently outbound 0xBA battery queries are sent.
// Passive COL05/COL06 notifications remain active in all modes.
type PollingMode string

const (
	// ModeNormal polls every 60s in addition to instant passive updates.
	ModeNormal PollingMode = "normal"
	// ModeLowFrequency polls every 5m in addition to passive updates.
	ModeLowFrequency PollingMode = "low_freq"
	// ModePassiveOnly opens only the read-only report handle (zero TX packets; gaming lock).
	ModePassiveOnly PollingMode = "passive_only"
)

// LinuxUdevRuleSnippet is the udev rule required on Linux for unprivileged /dev/hidraw* access.
const LinuxUdevRuleSnippet = `SUBSYSTEM=="hidraw", ATTRS{idVendor}=="24ae", ATTRS{idProduct}=="1211|4411|1227|4427", MODE="0660", TAG+="uaccess"`

// LinuxUdevInstallCmd is a copyable one-liner for Linux users if EACCES occurs.
const LinuxUdevInstallCmd = `echo 'SUBSYSTEM=="hidraw", ATTRS{idVendor}=="24ae", MODE="0660", TAG+="uaccess"' | sudo tee /etc/udev/rules.d/70-rapoo-vt9pro.rules && sudo udevadm control --reload-rules && sudo udevadm trigger`

// State represents the observable UI state for system tray icons and the Wails frontend.
type State struct {
	Connected        bool                 `json:"connected"` // Dongle or USB-C cable present
	DemoMode         bool                 `json:"demoMode"`  // True when using virtual VT9 Pro 4K simulator
	DeviceName       string               `json:"deviceName,omitempty"`
	Connection       rapoo.ConnectionMode `json:"connection,omitempty"`
	ProductID        uint16               `json:"productId,omitempty"`
	MaxRateHz        int                  `json:"maxRateHz,omitempty"`
	HasBattery       bool                 `json:"hasBattery"`  // True once at least one valid % was read
	Percentage       int                  `json:"percentage"`  // 0..100
	Charging         bool                 `json:"charging"`    // True when charging
	MouseOnline      bool                 `json:"mouseOnline"` // False when wireless mouse is sleeping
	Sleeping         bool                 `json:"sleeping"`
	EstimatedHours   float64              `json:"estimatedHours"`
	ActiveDPI        uint16               `json:"activeDpi,omitempty"`
	ActiveStage      int                  `json:"activeStage,omitempty"`
	StageCount       int                  `json:"stageCount,omitempty"`
	PollingRateHz    int                  `json:"pollingRateHz,omitempty"`
	Mode             PollingMode          `json:"mode"`
	PermissionDenied bool                 `json:"permissionDenied"` // Linux udev /dev/hidraw* EACCES
	Platform         string               `json:"platform"`
	UpdatedAt        time.Time            `json:"updatedAt"`
	LastError        string               `json:"lastError,omitempty"`
}

// UICatalog bundles static hardware option metadata so the frontend stays synced with Go.
type UICatalog struct {
	Platform            string                 `json:"platform"`
	DebounceSteps       []rapoo.DebounceStep   `json:"debounceSteps"`
	Buttons             []rapoo.ButtonSlotSpec `json:"buttons"`
	SupportedKeys       []rapoo.HIDKeyInfo     `json:"supportedKeys"`
	PollingRates        []int                  `json:"pollingRates"`
	LinuxUdevRule       string                 `json:"linuxUdevRule"`
	LinuxUdevInstallCmd string                 `json:"linuxUdevInstallCmd"`
}

// Config configures a MouseService instance.
type Config struct {
	InitialMode       PollingMode
	NormalInterval    time.Duration
	LowFreqInterval   time.Duration
	ReconnectInterval time.Duration
	PresetDir         string
	OnEventEmit       func(eventName string, payload any) // Optional Wails runtime.EventsEmit hook
}

func (c *Config) applyDefaults() {
	if c.InitialMode == "" {
		c.InitialMode = ModeNormal
	}
	if c.NormalInterval <= 0 {
		c.NormalInterval = 60 * time.Second
	}
	if c.LowFreqInterval <= 0 {
		c.LowFreqInterval = 5 * time.Minute
	}
	if c.ReconnectInterval <= 0 {
		c.ReconnectInterval = 5 * time.Second
	}
}

// Listener is invoked whenever the observable State changes.
type Listener func(State)

// MouseService manages lifecycle, battery telemetry, device configuration, and presets.
type MouseService struct {
	realProvider hid.Provider
	demoDevice   *rapoo.VirtualDevice
	presetStore  *profile.Store
	cfg          Config

	mu          sync.RWMutex
	demoMode    bool
	client      *rapoo.Client
	state       State
	cachedSnap  *rapoo.DeviceSnapshot
	lastWakeAt  time.Time
	nextSubID   int
	subscribers map[int]Listener

	refreshCh chan struct{}
	reopenCh  chan struct{}
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

// NewMouseService constructs a MouseService backed by provider (defaults to native OS HID).
func NewMouseService(provider hid.Provider, cfg Config) *MouseService {
	if provider == nil {
		provider = hid.NewProvider()
	}
	cfg.applyDefaults()
	store, _ := profile.NewStore(cfg.PresetDir)

	return &MouseService{
		realProvider: provider,
		demoDevice:   rapoo.NewVirtualDevice(),
		presetStore:  store,
		cfg:          cfg,
		state: State{
			Mode:     cfg.InitialMode,
			Platform: runtime.GOOS,
		},
		subscribers: make(map[int]Listener),
		refreshCh:   make(chan struct{}, 1),
		reopenCh:    make(chan struct{}, 1),
	}
}

func (s *MouseService) activeProvider() hid.Provider {
	s.mu.RLock()
	demo := s.demoMode
	s.mu.RUnlock()
	if demo {
		return s.demoDevice.Provider
	}
	return s.realProvider
}

// Startup starts background discovery, passive telemetry listening, and polling.
func (s *MouseService) Startup(ctx context.Context) {
	runCtx, cancel := context.WithCancel(ctx)
	s.mu.Lock()
	s.cancel = cancel
	s.mu.Unlock()

	s.wg.Add(1)
	go s.loop(runCtx)
}

// Shutdown cleanly closes open HID handles and stops background workers.
func (s *MouseService) Shutdown() {
	s.mu.Lock()
	cancel := s.cancel
	s.cancel = nil
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	s.wg.Wait()
	s.closeClient()
}

// Subscribe registers a state change callback and returns an unsubscribe function.
func (s *MouseService) Subscribe(fn Listener) func() {
	s.mu.Lock()
	id := s.nextSubID
	s.nextSubID++
	s.subscribers[id] = fn
	cur := s.state
	s.mu.Unlock()

	fn(cur)

	return func() {
		s.mu.Lock()
		delete(s.subscribers, id)
		s.mu.Unlock()
	}
}

// GetCatalog returns static UI metadata (debounce steps, button layout, USB HID keys, udev instructions).
func (s *MouseService) GetCatalog() UICatalog {
	return UICatalog{
		Platform:            runtime.GOOS,
		DebounceSteps:       rapoo.GenADebounceSteps,
		Buttons:             rapoo.VT9Pro4KButtons,
		SupportedKeys:       rapoo.SupportedHIDKeys,
		PollingRates:        []int{125, 250, 500, 1000, 2000, 4000},
		LinuxUdevRule:       LinuxUdevRuleSnippet,
		LinuxUdevInstallCmd: LinuxUdevInstallCmd,
	}
}

// GetState returns the latest cached device and battery state.
func (s *MouseService) GetState() State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// RefreshNow requests an immediate battery poll (unless ModePassiveOnly is active).
func (s *MouseService) RefreshNow() {
	select {
	case s.refreshCh <- struct{}{}:
	default:
	}
}

// NotifyDeviceChange triggers immediate re-enumeration.
func (s *MouseService) NotifyDeviceChange() {
	select {
	case s.reopenCh <- struct{}{}:
	default:
	}
}

// SetDemoMode toggles between real USB/4K hardware and the built-in VT9 Pro 4K simulator.
func (s *MouseService) SetDemoMode(enabled bool) (rapoo.DeviceSnapshot, error) {
	s.mu.Lock()
	changed := s.demoMode != enabled
	s.demoMode = enabled
	s.state.DemoMode = enabled
	if enabled && s.state.Mode == ModePassiveOnly {
		s.state.Mode = ModeNormal
	}
	s.mu.Unlock()

	if changed {
		s.reconnectAndPoll()
	}
	if !enabled {
		s.mu.RLock()
		connected := s.client != nil
		s.mu.RUnlock()
		if !connected {
			return rapoo.DeviceSnapshot{}, errors.New(friendlyError(rapoo.ErrDeviceNotFound))
		}
	}
	return s.ReadSnapshot()
}

// SetPollingMode switches between Normal (60s), LowFrequency (5m), and PassiveOnly (0-TX).
func (s *MouseService) SetPollingMode(mode PollingMode) {
	s.mu.Lock()
	changed := s.state.Mode != mode
	wasPassive := s.state.Mode == ModePassiveOnly
	isPassive := mode == ModePassiveOnly
	s.state.Mode = mode
	snap := s.state
	s.mu.Unlock()

	if !changed {
		return
	}
	s.publish(snap)

	if wasPassive != isPassive {
		s.NotifyDeviceChange()
	} else if !isPassive {
		s.RefreshNow()
	}
}

// ReadSnapshot queries full device details (FW, DPI, Sensor, 10 Buttons) and updates cached summary fields.
func (s *MouseService) ReadSnapshot() (rapoo.DeviceSnapshot, error) {
	s.mu.RLock()
	c := s.client
	demo := s.demoMode
	mode := s.state.Mode
	cached := s.cachedSnap
	s.mu.RUnlock()

	if c == nil {
		return rapoo.DeviceSnapshot{}, errors.New(friendlyError(rapoo.ErrDeviceNotFound))
	}
	if mode == ModePassiveOnly {
		if cached != nil {
			cp := *cached
			cp.DemoMode = demo
			return cp, nil
		}
		return rapoo.DeviceSnapshot{}, errors.New("ゲーム優先 (完全受動) モード中はマウスへの読み書き通信を停止しています。設定を開くにはモードを「通常」に切り替えてください。")
	}

	snap, err := c.ReadSnapshot()
	snap.DemoMode = demo
	if err != nil {
		if errors.Is(err, rapoo.ErrMouseAsleep) {
			s.markSleeping()
		}
		return snap, errors.New(friendlyError(err))
	}

	s.updateFromSnapshot(snap)
	return snap, nil
}

// ============================================================================
// Wails-Bindable Mutating Device Operations
// ============================================================================

func (s *MouseService) writableClient() (*rapoo.Client, error) {
	s.mu.RLock()
	c := s.client
	mode := s.state.Mode
	s.mu.RUnlock()

	if mode == ModePassiveOnly {
		return nil, errors.New("ゲーム中ロック (完全受動モード) が有効なため設定を変更できません。上部のロックを解除してください。")
	}
	if c == nil {
		return nil, errors.New(friendlyError(rapoo.ErrDeviceNotFound))
	}
	return c, nil
}

// SetPollingRate changes the mouse polling rate (125..4000 Hz) and returns the refreshed snapshot.
func (s *MouseService) SetPollingRate(hz int) (rapoo.DeviceSnapshot, error) {
	c, err := s.writableClient()
	if err != nil {
		return rapoo.DeviceSnapshot{}, err
	}
	if err := c.SetPollingRate(hz); err != nil {
		return rapoo.DeviceSnapshot{}, errors.New(friendlyError(err))
	}
	return s.ReadSnapshot()
}

// SetCurrentDPIGear switches the active DPI stage (1..7) and returns the refreshed snapshot.
func (s *MouseService) SetCurrentDPIGear(stage int) (rapoo.DeviceSnapshot, error) {
	c, err := s.writableClient()
	if err != nil {
		return rapoo.DeviceSnapshot{}, err
	}
	if err := c.SetCurrentDPIGear(stage); err != nil {
		return rapoo.DeviceSnapshot{}, errors.New(friendlyError(err))
	}
	return s.ReadSnapshot()
}

// SetDPIConfig updates X/Y DPI values, stage count (1..7), and active stage, returning the refreshed snapshot.
func (s *MouseService) SetDPIConfig(cfg rapoo.DPIConfig) (rapoo.DeviceSnapshot, error) {
	c, err := s.writableClient()
	if err != nil {
		return rapoo.DeviceSnapshot{}, err
	}
	if err := c.SetDPIConfig(cfg); err != nil {
		return rapoo.DeviceSnapshot{}, errors.New(friendlyError(err))
	}
	return s.ReadSnapshot()
}

// UpdateSensorConfig applies a SensorPatch (LOD, Motion Sync, Debounce, Sleep, Ripple, Linear, Angle, Glass).
func (s *MouseService) UpdateSensorConfig(patch rapoo.SensorPatch) (rapoo.DeviceSnapshot, error) {
	c, err := s.writableClient()
	if err != nil {
		return rapoo.DeviceSnapshot{}, err
	}
	if err := c.UpdateSensorConfig(patch); err != nil {
		return rapoo.DeviceSnapshot{}, errors.New(friendlyError(err))
	}
	return s.ReadSnapshot()
}

// SetButtonMapping updates one physical button's binding (with Left-Click safety validation).
func (s *MouseService) SetButtonMapping(buttonID int, action rapoo.ButtonAction) (rapoo.DeviceSnapshot, error) {
	c, err := s.writableClient()
	if err != nil {
		return rapoo.DeviceSnapshot{}, err
	}
	if err := c.SetButtonMapping(rapoo.ButtonID(buttonID), action); err != nil {
		return rapoo.DeviceSnapshot{}, errors.New(friendlyError(err))
	}
	return s.ReadSnapshot()
}

// ResetButtonsToDefault restores all 10 physical buttons to their factory bindings.
func (s *MouseService) ResetButtonsToDefault() (rapoo.DeviceSnapshot, error) {
	c, err := s.writableClient()
	if err != nil {
		return rapoo.DeviceSnapshot{}, err
	}
	if err := c.SetAllButtonMappings(rapoo.DefaultButtonMappings()); err != nil {
		return rapoo.DeviceSnapshot{}, errors.New(friendlyError(err))
	}
	return s.ReadSnapshot()
}

// FactoryReset restores all onboard settings on the mouse to factory defaults.
func (s *MouseService) FactoryReset() (rapoo.DeviceSnapshot, error) {
	c, err := s.writableClient()
	if err != nil {
		return rapoo.DeviceSnapshot{}, err
	}
	if err := c.FactoryReset(); err != nil {
		return rapoo.DeviceSnapshot{}, errors.New(friendlyError(err))
	}
	// Give mouse MCU a moment to reinitialize flash defaults when connected to real hardware.
	s.mu.RLock()
	demo := s.demoMode
	s.mu.RUnlock()
	if !demo {
		time.Sleep(180 * time.Millisecond)
	}
	return s.ReadSnapshot()
}

// ============================================================================
// Profile Preset Library Operations
// ============================================================================

// ListPresets returns all built-in and user-saved presets.
func (s *MouseService) ListPresets() []profile.Preset {
	if s.presetStore == nil {
		return profile.BuiltInPresets()
	}
	return s.presetStore.List()
}

// SavePreset snapshots the current mouse configuration as a named preset.
func (s *MouseService) SavePreset(id, name, description string) (profile.Preset, error) {
	if s.presetStore == nil {
		return profile.Preset{}, errors.New("プロファイル保存領域を初期化できませんでした")
	}
	snap, err := s.ReadSnapshot()
	if err != nil {
		return profile.Preset{}, err
	}
	return s.presetStore.SaveSnapshot(id, name, description, snap)
}

// ApplyPreset writes all settings from a preset (DPI, Polling Rate, Sensor, Buttons) to the mouse.
func (s *MouseService) ApplyPreset(id string) (rapoo.DeviceSnapshot, error) {
	if s.presetStore == nil {
		return rapoo.DeviceSnapshot{}, errors.New("プロファイル保存領域が利用できません")
	}
	preset, ok := s.presetStore.Get(id)
	if !ok {
		return rapoo.DeviceSnapshot{}, errors.New("指定されたプロファイルが見つかりません")
	}
	c, err := s.writableClient()
	if err != nil {
		return rapoo.DeviceSnapshot{}, err
	}

	// Clamp polling rate to connection capability (e.g. 1000Hz max when wired USB-C).
	rate := preset.PollingHz
	if rate > 0 {
		maxRate := c.Model().MaxRateHz
		if maxRate > 0 && rate > maxRate {
			rate = maxRate
		}
		if err := c.SetPollingRate(rate); err != nil {
			return rapoo.DeviceSnapshot{}, errors.New(friendlyError(err))
		}
	}
	if preset.DPI.StageCount >= 1 {
		if err := c.SetDPIConfig(preset.DPI); err != nil {
			return rapoo.DeviceSnapshot{}, errors.New(friendlyError(err))
		}
	}
	if err := c.UpdateSensorConfig(preset.Sensor); err != nil {
		return rapoo.DeviceSnapshot{}, errors.New(friendlyError(err))
	}
	if len(preset.Buttons) > 0 {
		if err := c.SetAllButtonMappings(preset.Buttons); err != nil {
			return rapoo.DeviceSnapshot{}, errors.New(friendlyError(err))
		}
	}
	return s.ReadSnapshot()
}

// DeletePreset deletes a user-saved preset.
func (s *MouseService) DeletePreset(id string) error {
	if s.presetStore == nil {
		return errors.New("プロファイル保存領域が利用できません")
	}
	return s.presetStore.Delete(id)
}

// ExportPresetJSON returns the formatted JSON string for a preset.
func (s *MouseService) ExportPresetJSON(id string) (string, error) {
	if s.presetStore == nil {
		return "", errors.New("プロファイル保存領域が利用できません")
	}
	return s.presetStore.ExportJSON(id)
}

// ImportPresetJSON imports and saves a preset from JSON text.
func (s *MouseService) ImportPresetJSON(rawJSON string) (profile.Preset, error) {
	if s.presetStore == nil {
		return profile.Preset{}, errors.New("プロファイル保存領域が利用できません")
	}
	return s.presetStore.ImportJSON(rawJSON)
}

// ============================================================================
// Background Loop & State Synchronization
// ============================================================================

func (s *MouseService) loop(ctx context.Context) {
	defer s.wg.Done()

	s.ensureConnectedAndPoll()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	lastPoll := time.Now()
	lastEnum := time.Now()

	for {
		select {
		case <-ctx.Done():
			return

		case <-s.reopenCh:
			select {
			case <-ctx.Done():
				return
			case <-time.After(150 * time.Millisecond):
			}
			s.reconnectAndPoll()
			lastPoll = time.Now()
			lastEnum = time.Now()

		case <-s.refreshCh:
			s.ensureConnectedAndPoll()
			lastPoll = time.Now()

		case now := <-ticker.C:
			s.mu.RLock()
			connected := s.client != nil
			mode := s.state.Mode
			s.mu.RUnlock()

			if !connected {
				if now.Sub(lastEnum) >= s.cfg.ReconnectInterval {
					s.ensureConnectedAndPoll()
					lastEnum = now
					lastPoll = now
				}
				continue
			}

			if now.Sub(lastEnum) >= s.cfg.ReconnectInterval {
				lastEnum = now
				if s.higherPriorityEndpointAvailable() {
					s.reconnectAndPoll()
					lastPoll = now
					continue
				}
			}

			interval := s.intervalForMode(mode)
			if interval > 0 && now.Sub(lastPoll) >= interval {
				s.pollOnce()
				lastPoll = now
			}
		}
	}
}

func (s *MouseService) intervalForMode(mode PollingMode) time.Duration {
	switch mode {
	case ModeNormal:
		return s.cfg.NormalInterval
	case ModeLowFrequency:
		return s.cfg.LowFreqInterval
	case ModePassiveOnly:
		return 0
	default:
		return s.cfg.NormalInterval
	}
}

func (s *MouseService) higherPriorityEndpointAvailable() bool {
	s.mu.RLock()
	curClient := s.client
	demo := s.demoMode
	s.mu.RUnlock()
	if curClient == nil || demo {
		return false
	}

	infos, err := s.realProvider.Enumerate(rapoo.DefaultHIDFilter())
	if err != nil {
		return false
	}
	pairs := rapoo.MatchEndpoints(infos)
	if len(pairs) == 0 {
		return true
	}
	return pairs[0].Model.ProductID != curClient.Model().ProductID
}

func (s *MouseService) reconnectAndPoll() {
	s.closeClient()
	s.ensureConnectedAndPoll()
}

func (s *MouseService) closeClient() {
	s.mu.Lock()
	c := s.client
	s.client = nil
	s.mu.Unlock()
	if c != nil {
		_ = c.Close()
	}
}

func (s *MouseService) ensureConnectedAndPoll() {
	s.mu.RLock()
	hasClient := s.client != nil
	mode := s.state.Mode
	demo := s.demoMode
	s.mu.RUnlock()

	if !hasClient {
		opts := rapoo.ClientOptions{
			PassiveOnly:  mode == ModePassiveOnly,
			OnAsyncEvent: s.handleAsyncEvent,
		}
		if demo {
			opts.BusyPollInterval = time.Millisecond
		}
		client, err := rapoo.OpenFirst(s.activeProvider(), opts)
		if err != nil {
			s.markDisconnected(err)
			return
		}

		model := client.Model()
		if demo {
			model.Name = "Rapoo VT9 Pro 4K (デモ)"
		}
		s.mu.Lock()
		s.client = client
		s.state.Connected = true
		s.state.DemoMode = demo
		s.state.DeviceName = model.Name
		s.state.Connection = model.Connection
		s.state.ProductID = model.ProductID
		s.state.MaxRateHz = model.MaxRateHz
		s.state.PermissionDenied = false
		s.state.LastError = ""
		snap := s.state
		s.mu.Unlock()
		s.publish(snap)
	}

	if mode != ModePassiveOnly {
		s.pollOnce()
	}
}

func (s *MouseService) pollOnce() {
	s.mu.RLock()
	c := s.client
	mode := s.state.Mode
	s.mu.RUnlock()

	if c == nil || mode == ModePassiveOnly {
		return
	}

	bat, err := c.ReadBattery()
	if err != nil {
		if errors.Is(err, rapoo.ErrMouseAsleep) {
			s.markSleeping()
			return
		}
		s.closeClient()
		s.markDisconnected(err)
		return
	}

	s.applyBattery(bat)
}

func (s *MouseService) markSleeping() {
	s.mu.Lock()
	s.state.MouseOnline = false
	s.state.Sleeping = true
	s.state.Charging = false
	s.state.LastError = ""
	snap := s.state
	s.mu.Unlock()
	s.publish(snap)
}

func (s *MouseService) handleAsyncEvent(ev rapoo.AsyncEvent) {
	switch ev.Kind {
	case rapoo.EventBattery:
		if ev.Battery == nil {
			return
		}
		if !ev.Battery.Online {
			s.markSleeping()
			return
		}
		s.applyBattery(*ev.Battery)

	case rapoo.EventWake:
		s.mu.Lock()
		mode := s.state.Mode
		shouldRefresh := mode != ModePassiveOnly && time.Since(s.lastWakeAt) >= 2*time.Second
		if shouldRefresh {
			s.lastWakeAt = time.Now()
		}
		s.mu.Unlock()

		if shouldRefresh {
			s.RefreshNow()
		}
	}
}

func (s *MouseService) applyBattery(bat rapoo.BatteryState) {
	s.mu.Lock()
	rate := s.state.PollingRateHz
	if rate <= 0 {
		rate = 1000
	}
	s.state.Connected = true
	s.state.HasBattery = true
	s.state.Percentage = bat.Percentage
	s.state.Charging = bat.Charging
	s.state.MouseOnline = bat.Online
	s.state.Sleeping = bat.Sleeping
	s.state.EstimatedHours = rapoo.EstimateBatteryHours(bat.Percentage, rate)
	s.state.UpdatedAt = bat.UpdatedAt
	s.state.PermissionDenied = false
	s.state.LastError = ""
	snap := s.state
	s.mu.Unlock()

	s.publish(snap)
}

func (s *MouseService) updateFromSnapshot(ds rapoo.DeviceSnapshot) {
	s.mu.Lock()
	cp := ds
	s.cachedSnap = &cp
	s.state.Connected = true
	s.state.DemoMode = ds.DemoMode
	s.state.HasBattery = true
	s.state.Percentage = ds.Battery.Percentage
	s.state.Charging = ds.Battery.Charging
	s.state.MouseOnline = ds.Battery.Online
	s.state.Sleeping = ds.Battery.Sleeping
	s.state.EstimatedHours = ds.EstimatedHours
	s.state.MaxRateHz = ds.Model.MaxRateHz
	if ds.Sensor != nil {
		s.state.PollingRateHz = ds.Sensor.PollingRateHz
	}
	if ds.DPI != nil {
		s.state.ActiveStage = ds.DPI.CurrentStage
		s.state.StageCount = ds.DPI.StageCount
		idx := ds.DPI.CurrentStage - 1
		if idx >= 0 && idx < len(ds.DPI.Stages) {
			s.state.ActiveDPI = ds.DPI.Stages[idx].X
		}
	}
	if !ds.Battery.UpdatedAt.IsZero() {
		s.state.UpdatedAt = ds.Battery.UpdatedAt
	}
	s.state.PermissionDenied = false
	s.state.LastError = ""
	st := s.state
	emit := s.cfg.OnEventEmit
	s.mu.Unlock()

	s.publish(st)
	if emit != nil {
		emit("rapoo:snapshot", ds)
	}
}

func isPermissionError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, os.ErrPermission) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "permission denied") || strings.Contains(msg, "access is denied")
}

func friendlyError(err error) string {
	if err == nil {
		return ""
	}
	switch {
	case errors.Is(err, rapoo.ErrDeviceNotFound):
		return "Rapoo VT9 Pro 4K (4Kレシーバー または USBケーブル) が見つかりません。接続を確認するか、デモモードでお試しください。"
	case errors.Is(err, rapoo.ErrMouseAsleep):
		return "ワイヤレスマウスがスリープ中または電源OFFです。マウスを軽く動かして復帰させてください。"
	case errors.Is(err, rapoo.ErrMissingLeftClick):
		return "操作不能を防ぐため、少なくとも1つの物理ボタンに「左クリック」を残す必要があります。先に別のボタンへ左クリックを割り当ててください。"
	case isPermissionError(err):
		if runtime.GOOS == "linux" {
			return "Linux の /dev/hidraw デバイスへのアクセス権限がありません。設定タブの udev ルールを適用してください。"
		}
		return "HID デバイスへのアクセス権限がありません。"
	default:
		return fmt.Sprintf("デバイス通信エラー: %v", err)
	}
}

func (s *MouseService) markDisconnected(err error) {
	permDenied := isPermissionError(err)
	s.mu.Lock()
	changed := s.state.Connected || s.state.MouseOnline || s.state.PermissionDenied != permDenied
	s.state.Connected = false
	s.state.MouseOnline = false
	s.state.Sleeping = false
	s.state.Charging = false
	s.state.DeviceName = ""
	s.state.Connection = ""
	s.state.ProductID = 0
	s.state.PermissionDenied = permDenied
	if err != nil && !errors.Is(err, rapoo.ErrDeviceNotFound) {
		s.state.LastError = friendlyError(err)
	} else {
		s.state.LastError = ""
	}
	snap := s.state
	s.mu.Unlock()

	if changed {
		s.publish(snap)
	}
}

func (s *MouseService) publish(snap State) {
	s.mu.RLock()
	listeners := make([]Listener, 0, len(s.subscribers))
	for _, fn := range s.subscribers {
		listeners = append(listeners, fn)
	}
	emit := s.cfg.OnEventEmit
	s.mu.RUnlock()

	for _, fn := range listeners {
		fn(snap)
	}
	if emit != nil {
		emit("rapoo:state", snap)
	}
}
