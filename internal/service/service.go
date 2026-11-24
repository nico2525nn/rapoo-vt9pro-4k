// Package service provides a Wails-ready application service that manages
// device discovery, hotplug reconnection, passive notifications, gentle
// battery polling, full onboard settings read/write, Demo Mode simulation,
// and local preset management.
package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nico/rapoo-vt9pro-4k/internal/profile"
	"github.com/nico/rapoo-vt9pro-4k/internal/updaterguard"
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
const LinuxUdevInstallCmd = `echo 'SUBSYSTEM=="hidraw", ATTRS{idVendor}=="24ae", ATTRS{idProduct}=="1211|4411|1227|4427", MODE="0660", TAG+="uaccess"' | sudo tee /etc/udev/rules.d/70-rapoo-vt9pro.rules && sudo udevadm control --reload-rules && sudo udevadm trigger`

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
	Revision         uint64               `json:"revision"`
	Generation       uint64               `json:"generation"`
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
	Language          string
	NormalInterval    time.Duration
	LowFreqInterval   time.Duration
	ReconnectInterval time.Duration
	SleepGrace        time.Duration // suppress transient 0x02 replies for this long after a good read
	PresetDir         string
	StartInDemo       bool                                // Skip real HID until demo mode is turned off
	OnEventEmit       func(eventName string, payload any) // Optional Wails runtime.EventsEmit hook
}

func (c *Config) applyDefaults() {
	if c.Language == "" {
		c.Language = normalizeLanguage(os.Getenv("RAPOO_LANG"))
	}
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
	if c.SleepGrace == 0 {
		c.SleepGrace = 10 * time.Second
	}
}

// Listener is invoked whenever the observable State changes.
type Listener func(State)

// MouseService manages lifecycle, battery telemetry, device configuration, and presets.
type MouseService struct {
	realProvider hid.Provider
	demoDevice   *rapoo.VirtualDevice
	presetStore  *profile.Store
	presetErr    error
	cfg          Config

	mu            sync.RWMutex
	demoMode      bool
	client        *rapoo.Client
	state         State
	cachedSnap    *rapoo.DeviceSnapshot
	lastBatteryOK time.Time
	lastWakeAt    time.Time
	updaterActive bool
	nextSubID     int
	subscribers   map[int]Listener
	language      atomic.Value // string: "en" or "ja"

	connectMu sync.Mutex
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
	store, storeErr := profile.NewStore(cfg.PresetDir)

	svc := &MouseService{
		realProvider: provider,
		demoDevice:   rapoo.NewVirtualDevice(),
		presetStore:  store,
		presetErr:    storeErr,
		cfg:          cfg,
		demoMode:     cfg.StartInDemo,
		state: State{
			Mode:     cfg.InitialMode,
			Platform: runtime.GOOS,
			DemoMode: cfg.StartInDemo,
		},
		subscribers: make(map[int]Listener),
		refreshCh:   make(chan struct{}, 1),
		reopenCh:    make(chan struct{}, 1),
	}
	svc.language.Store(normalizeLanguage(cfg.Language))
	return svc
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

// EnsureDemoReady opens the virtual provider before the bridge advertises
// readiness in demo mode. It never waits for real HID; real startup stays async.
func (s *MouseService) EnsureDemoReady() {
	s.mu.RLock()
	demo := s.demoMode
	s.mu.RUnlock()
	if demo {
		s.ensureConnectedAndPoll()
	}
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
	s.connectMu.Lock()
	s.closeClient()
	s.connectMu.Unlock()
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
// SetLanguage updates backend error messages at runtime.
func (s *MouseService) SetLanguage(language string) {
	s.language.Store(normalizeLanguage(language))
}

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
	if s.updaterActive || updaterguard.Held() {
		s.mu.Unlock()
		return rapoo.DeviceSnapshot{}, errors.New("ファームウェア更新中はデモモードを切り替えられません")
	}
	changed := s.demoMode != enabled
	s.demoMode = enabled
	s.state.DemoMode = enabled
	if enabled && s.state.Mode == ModePassiveOnly {
		s.state.Mode = ModeNormal
	}
	s.mu.Unlock()

	if changed {
		s.reconnectAndPoll()
	} else {
		// A ready event may arrive before the background loop has opened the
		// selected provider. Do not return a snapshot with a nil client.
		s.ensureConnectedAndPoll()
	}
	if !enabled {
		s.mu.RLock()
		connected := s.client != nil
		s.mu.RUnlock()
		if !connected {
			return rapoo.DeviceSnapshot{}, errors.New(s.friendlyError(rapoo.ErrDeviceNotFound))
		}
	}
	return s.readSnapshotRetry()
}

// readSnapshotFresh waits for the HID channel and always reads the current
// device. Preset saving must not use the display cache from an old generation.
func (s *MouseService) readSnapshotFresh() (rapoo.DeviceSnapshot, error) {
	if !s.connectMu.TryLock() {
		return rapoo.DeviceSnapshot{}, ErrBusy
	}
	defer s.connectMu.Unlock()
	return s.readSnapshotNow()
}

// readSnapshotRetry waits briefly for an in-flight poll to release the HID
// channel. Public ReadSnapshot stays non-blocking so a slow initial HID query
// cannot hold up the bridge RPC loop.
func (s *MouseService) readSnapshotRetry() (rapoo.DeviceSnapshot, error) {
	var lastErr error
	for attempt := 0; attempt < 20; attempt++ {
		snap, err := s.readSnapshotFresh()
		if !errors.Is(err, ErrBusy) {
			return snap, err
		}
		lastErr = err
		time.Sleep(25 * time.Millisecond)
	}
	return rapoo.DeviceSnapshot{}, lastErr
}

// SetPollingMode switches between Normal (60s), LowFrequency (5m), and PassiveOnly (0-TX).
func (s *MouseService) SetPollingMode(mode PollingMode) {
	s.connectMu.Lock()
	defer s.connectMu.Unlock()

	s.mu.Lock()
	if (s.updaterActive || updaterguard.Held()) && mode != ModePassiveOnly {
		s.mu.Unlock()
		return
	}
	changed := s.state.Mode != mode
	wasPassive := s.state.Mode == ModePassiveOnly
	isPassive := mode == ModePassiveOnly
	if !changed {
		s.mu.Unlock()
		return
	}

	var oldClient *rapoo.Client
	if wasPassive != isPassive {
		// Keep the old client visible to the loop while it is being closed so
		// another poll cannot open a replacement before this transition ends.
		oldClient = s.client
	}
	s.state.Mode = mode
	snap := s.snapshotLocked()
	s.mu.Unlock()

	if oldClient != nil {
		// Client.Close waits for an in-flight command exchange before releasing
		// the command handle, so 0-TX becomes effective before we publish it.
		_ = oldClient.Close()
		s.mu.Lock()
		if s.client == oldClient {
			s.client = nil
		}
		s.mu.Unlock()
	}

	s.publish(snap)

	if wasPassive != isPassive {
		s.NotifyDeviceChange()
	} else if !isPassive {
		s.RefreshNow()
	}
}

// BeginUpdater claims the exclusive firmware-update state. It fails if
// another updater is already running.
func (s *MouseService) BeginUpdater() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.updaterActive {
		return errors.New("ファームウェア更新はすでに実行中です")
	}
	s.updaterActive = true
	return nil
}

// EndUpdater releases the exclusive firmware-update state.
func (s *MouseService) EndUpdater() {
	s.mu.Lock()
	s.updaterActive = false
	s.mu.Unlock()
}

// UpdaterActive reports whether the official firmware updater is running.
func (s *MouseService) UpdaterActive() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.updaterActive
}

// RestorePollingMode restores a mode without waiting for the updater guard.
// The background loop still refuses to open HID until the guard is gone.
func (s *MouseService) RestorePollingMode(mode PollingMode) {
	s.connectMu.Lock()
	s.mu.Lock()
	if s.state.Mode == mode {
		s.mu.Unlock()
		s.connectMu.Unlock()
		return
	}
	s.state.Mode = mode
	snap := s.snapshotLocked()
	s.mu.Unlock()
	s.connectMu.Unlock()
	s.publish(snap)
}

// ReadSnapshot queries full device details (FW, DPI, Sensor, 10 Buttons) and updates cached summary fields.
func (s *MouseService) ReadSnapshot() (snap rapoo.DeviceSnapshot, err error) {
	if !s.connectMu.TryLock() {
		// A poll or write owns the HID command channel. Return the last good
		// snapshot only when it belongs to the current generation; otherwise
		// let the caller retry instead of saving another device's settings.
		s.mu.RLock()
		cached := s.cachedSnap
		demo := s.demoMode
		generation := s.state.Generation
		s.mu.RUnlock()
		if cached != nil && cached.Generation != 0 && cached.Generation == generation {
			cp := *cached
			cp.DemoMode = demo
			return cp, nil
		}
		return rapoo.DeviceSnapshot{}, ErrBusy
	}
	defer s.connectMu.Unlock()
	return s.readSnapshotNow()
}

func (s *MouseService) readSnapshotNow() (snap rapoo.DeviceSnapshot, err error) {
	s.mu.RLock()
	c := s.client
	demo := s.demoMode
	mode := s.state.Mode
	cached := s.cachedSnap
	readRevision := s.state.Revision
	readGeneration := s.state.Generation
	s.mu.RUnlock()

	defer func() {
		// The snapshot describes the state as of the read start. Never stamp it
		// with a revision that a later device/state transition produced.
		snap.StateRevision = readRevision
		if snap.Generation == 0 {
			snap.Generation = readGeneration
		}
	}()

	if c == nil {
		return rapoo.DeviceSnapshot{}, errors.New(s.friendlyError(rapoo.ErrDeviceNotFound))
	}
	if mode == ModePassiveOnly {
		if cached != nil {
			cp := *cached
			cp.DemoMode = demo
			return cp, nil
		}
		return rapoo.DeviceSnapshot{}, errors.New("ゲーム優先 (完全受動) モード中はマウスへの読み書き通信を停止しています。設定を開くにはモードを「通常」に切り替えてください。")
	}

	snap, err = c.ReadSnapshot()
	snap.DemoMode = demo
	if err != nil {
		if errors.Is(err, rapoo.ErrMouseAsleep) {
			// The snapshot starts with a battery query. A busy dongle can
			// reject that first query even though the mouse is online, so
			// retry the cheap battery command before showing "sleeping".
			if bat, batErr := s.readBatteryWithRetry(c, 3); batErr == nil {
				if !s.applyBatteryIfCurrent(bat, c, readGeneration) {
					return snap, fmt.Errorf("%w: 接続先が変わりました。設定を再読み込みしてください。", ErrDeviceChanged)
				}
				if cached != nil {
					cp := *cached
					cp.DemoMode = demo
					cp.Battery = bat
					return cp, nil
				}
				snap.Battery = bat
				return snap, nil
			}

			sleepApplied, sleepCurrent := s.markSleepingIfCurrent(true, c, readGeneration)
			if !sleepCurrent {
				return snap, fmt.Errorf("%w: 接続先が変わりました。設定を再読み込みしてください。", ErrDeviceChanged)
			}
			if !sleepApplied {
				if cached != nil {
					cp := *cached
					cp.DemoMode = demo
					return cp, nil
				}
				return snap, nil
			}
			if cached != nil {
				cp := *cached
				cp.DemoMode = demo
				cp.Battery.Online = false
				cp.Battery.Sleeping = true
				cp.Battery.Charging = false
				cp.Battery.Source = rapoo.BatterySourceCached
				if snap.Firmware != nil && snap.Firmware.DongleRaw != 0 && cp.Firmware != nil {
					fw := *cp.Firmware
					fw.DongleRaw = snap.Firmware.DongleRaw
					fw.DongleStr = snap.Firmware.DongleStr
					cp.Firmware = &fw
				}
				return cp, nil
			}
		}
		return snap, errors.New(s.friendlyError(err))
	}

	s.mu.RLock()
	sameClient := s.client == c
	s.mu.RUnlock()
	if !sameClient {
		return snap, errors.New("接続先が変わりました。設定を再読み込みしてください。")
	}
	// Stamp before updateFromSnapshot caches the snapshot, otherwise the
	// cached copy would carry generation 0 and later be mistaken for current.
	snap.StateRevision = readRevision
	if snap.Generation == 0 {
		snap.Generation = readGeneration
	}
	if !s.updateFromSnapshot(snap, c, readGeneration) {
		return snap, fmt.Errorf("%w: 接続先が変わりました。設定を再読み込みしてください。", ErrDeviceChanged)
	}
	return snap, nil
}

// ============================================================================
// Wails-Bindable Mutating Device Operations
// ============================================================================

func (s *MouseService) currentGeneration() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state.Generation
}

func deadlineExpired(deadline time.Time) bool {
	return !deadline.IsZero() && time.Now().After(deadline)
}

// withWritableClient serializes a mutating operation with reconnect/poll
// transitions and verifies the device generation immediately before the write.
// It holds connectMu for the whole callback so the HID target cannot be swapped
// underneath a multi-step operation.
func (s *MouseService) withWritableClient(
	expectedGeneration uint64,
	deadline time.Time,
	fn func(*rapoo.Client) error,
) (*rapoo.Client, uint64, error) {
	if deadlineExpired(deadline) {
		return nil, 0, ErrDeadlineExceeded
	}
	s.connectMu.Lock()
	defer s.connectMu.Unlock()
	if deadlineExpired(deadline) {
		return nil, 0, ErrDeadlineExceeded
	}

	s.mu.RLock()
	c := s.client
	mode := s.state.Mode
	generation := s.state.Generation
	s.mu.RUnlock()

	if mode == ModePassiveOnly {
		return nil, 0, errors.New("ゲーム中ロック (完全受動モード) が有効なため設定を変更できません。上部のロックを解除してください。")
	}
	if c == nil {
		return nil, 0, errors.New(s.friendlyError(rapoo.ErrDeviceNotFound))
	}
	if expectedGeneration != 0 && generation != expectedGeneration {
		return nil, 0, fmt.Errorf("%w (expected %d, got %d)", ErrDeviceChanged, expectedGeneration, generation)
	}
	return c, generation, fn(c)
}

func (s *MouseService) wrapFriendly(err error) error {
	if err == nil {
		return nil
	}
	var partial *PartialWriteError
	if errors.As(err, &partial) {
		return err
	}
	if errors.Is(err, rapoo.ErrResultUnknown) {
		return fmt.Errorf("%w: %s", rapoo.ErrResultUnknown, s.friendlyError(err))
	}
	if errors.Is(err, ErrDeviceChanged) ||
		errors.Is(err, ErrDeadlineExceeded) ||
		errors.Is(err, rapoo.ErrDeadlineExceeded) {
		return err
	}
	return errors.New(s.friendlyError(err))
}

func (s *MouseService) readAfterWrite(
	client *rapoo.Client,
	generation uint64,
	applied []string,
	verify func(rapoo.DeviceSnapshot) error,
) (rapoo.DeviceSnapshot, error) {
	if client == nil {
		return rapoo.DeviceSnapshot{}, &PartialWriteError{
			Applied: applied,
			Err:     errors.New("書き込み先のクライアントを特定できません"),
		}
	}
	s.mu.RLock()
	sameTarget := s.client == client && s.state.Generation == generation
	demo := s.demoMode
	revision := s.state.Revision
	s.mu.RUnlock()
	if !sameTarget {
		return rapoo.DeviceSnapshot{}, &PartialWriteError{
			Applied: applied,
			Err:     fmt.Errorf("%w: 書き込み後に接続先が変わりました", ErrDeviceChanged),
		}
	}

	// Read directly from the client that performed the write. s.ReadSnapshot()
	// may fall back to a cached snapshot (sleep/passive mode), which must not be
	// accepted as verification of a new write.
	snap, err := client.ReadSnapshot()
	if err != nil {
		return snap, &PartialWriteError{
			Applied: applied,
			Err:     fmt.Errorf("設定は書き込まれましたが、読み戻しに失敗しました: %s", s.friendlyError(err)),
		}
	}
	s.mu.RLock()
	sameTarget = s.client == client && s.state.Generation == generation
	s.mu.RUnlock()
	if !sameTarget {
		return snap, &PartialWriteError{
			Applied: applied,
			Err:     fmt.Errorf("%w: 読み戻し中に接続先が変わりました", ErrDeviceChanged),
		}
	}
	snap.DemoMode = demo
	snap.Generation = generation
	snap.StateRevision = revision
	if !s.updateFromSnapshot(snap, client, generation) {
		return snap, &PartialWriteError{
			Applied: applied,
			Err:     fmt.Errorf("%w: 読み戻しの反映中に接続先が変わりました", ErrDeviceChanged),
		}
	}
	if err := verifyAppliedSnapshot(snap, applied); err != nil {
		return snap, &PartialWriteError{Applied: applied, Err: err}
	}
	if verify != nil {
		if err := verify(snap); err != nil {
			return snap, &PartialWriteError{Applied: applied, Err: err}
		}
	}
	return snap, nil
}

func verifyPollingRate(snap rapoo.DeviceSnapshot, hz int) error {
	if snap.Sensor == nil {
		return errors.New("ポーリングレートを読み戻せませんでした")
	}
	if snap.Sensor.PollingRateHz != hz {
		return fmt.Errorf("ポーリングレートの反映を確認できませんでした (要求 %d Hz, 実測 %d Hz)", hz, snap.Sensor.PollingRateHz)
	}
	return nil
}

func verifyDPIConfig(snap rapoo.DeviceSnapshot, cfg rapoo.DPIConfig) error {
	if snap.DPI == nil {
		return errors.New("DPI設定を読み戻せませんでした")
	}
	stages := snap.DPI.AllStages
	if len(stages) == 0 {
		stages = snap.DPI.Stages
	}
	if snap.DPI.StageCount != cfg.StageCount {
		return fmt.Errorf("DPIステージ数の反映を確認できませんでした (要求 %d, 実測 %d)", cfg.StageCount, snap.DPI.StageCount)
	}
	expectedStage := cfg.CurrentStage
	if expectedStage < 1 || expectedStage > cfg.StageCount {
		expectedStage = 1
	}
	if snap.DPI.CurrentStage != expectedStage {
		return fmt.Errorf("使用DPIステージの反映を確認できませんでした (要求 %d, 実測 %d)", expectedStage, snap.DPI.CurrentStage)
	}
	src := cfg.AllStages
	if len(src) < cfg.StageCount {
		src = cfg.Stages
	}
	limit := cfg.StageCount
	if len(src) > limit {
		limit = len(src)
	}
	if limit > rapoo.MaxDPIStage {
		limit = rapoo.MaxDPIStage
	}
	if len(src) < limit || len(stages) < limit {
		return errors.New("DPIステージの読み戻しデータが不足しています")
	}
	for i := 0; i < limit; i++ {
		want := src[i]
		wantX := rapoo.NormalizeDPI(want.X)
		wantY := wantX
		if want.Y > 0 {
			wantY = rapoo.NormalizeDPI(want.Y)
		}
		got := stages[i]
		if got.X != wantX || got.Y != wantY {
			return fmt.Errorf("DPIステージ %d の反映を確認できませんでした (要求 %d/%d, 実測 %d/%d)",
				i+1, wantX, wantY, got.X, got.Y)
		}
	}
	return nil
}

func verifySensorPatch(snap rapoo.DeviceSnapshot, patch rapoo.SensorPatch) error {
	if snap.Sensor == nil {
		return errors.New("センサー設定を読み戻せませんでした")
	}
	sensor := snap.Sensor
	if patch.LiftOffLevel != nil && sensor.LiftOffLevel != *patch.LiftOffLevel {
		return fmt.Errorf("リフトオフの反映を確認できませんでした (要求 %d, 実測 %d)", *patch.LiftOffLevel, sensor.LiftOffLevel)
	}
	if patch.MotionSync != nil && sensor.MotionSync != *patch.MotionSync {
		return errors.New("Motion Sync の反映を確認できませんでした")
	}
	if patch.PressDebounceStep != nil {
		actual := sensor.PressDebounceStep
		if snap.Model.ProtocolGen != 1 {
			actual = sensor.PressDebounceMs
		}
		if actual != *patch.PressDebounceStep {
			return fmt.Errorf("押下デバウンスの反映を確認できませんでした (要求 %d, 実測 %d)", *patch.PressDebounceStep, actual)
		}
	}
	if patch.ReleaseDebounceStep != nil {
		actual := sensor.ReleaseDebounceStep
		if snap.Model.ProtocolGen != 1 {
			actual = sensor.ReleaseDebounceMs
		}
		if actual != *patch.ReleaseDebounceStep {
			return fmt.Errorf("解放デバウンスの反映を確認できませんでした (要求 %d, 実測 %d)", *patch.ReleaseDebounceStep, actual)
		}
	}
	if patch.SleepMinutes != nil && sensor.SleepMinutes != *patch.SleepMinutes {
		return fmt.Errorf("スリープ時間の反映を確認できませんでした (要求 %d, 実測 %d)", *patch.SleepMinutes, sensor.SleepMinutes)
	}
	if patch.RippleControl != nil && sensor.RippleControl != *patch.RippleControl {
		return errors.New("リップル補正の反映を確認できませんでした")
	}
	if patch.LinearCorrection != nil && sensor.LinearCorrection != *patch.LinearCorrection {
		return errors.New("直線補正の反映を確認できませんでした")
	}
	if patch.SensorAngleDeg != nil && int(sensor.SensorAngleDeg) != *patch.SensorAngleDeg {
		return fmt.Errorf("センサー角度の反映を確認できませんでした (要求 %d, 実測 %d)", *patch.SensorAngleDeg, sensor.SensorAngleDeg)
	}
	if patch.GlassMode != nil && sensor.GlassMode != *patch.GlassMode {
		return errors.New("ガラス面モードの反映を確認できませんでした")
	}
	if patch.LEDLowPowerMode != nil && sensor.LEDLowPowerMode != *patch.LEDLowPowerMode {
		return fmt.Errorf("LED 省電力の反映を確認できませんでした (要求 %d, 実測 %d)", *patch.LEDLowPowerMode, sensor.LEDLowPowerMode)
	}
	return nil
}

func verifyButtonMapping(snap rapoo.DeviceSnapshot, buttonID int, action rapoo.ButtonAction) error {
	for _, mapping := range snap.Buttons {
		if int(mapping.Slot.ID) != buttonID {
			continue
		}
		if !rapoo.ButtonActionsEqual(mapping.Action, action) {
			return fmt.Errorf("ボタン %d の反映を確認できませんでした", buttonID)
		}
		return nil
	}
	return fmt.Errorf("ボタン %d の設定を読み戻せませんでした", buttonID)
}

func verifyButtonMappings(snap rapoo.DeviceSnapshot, expected []rapoo.ButtonMapping) error {
	if len(snap.Buttons) == 0 {
		return errors.New("ボタン設定を読み戻せませんでした")
	}
	byID := make(map[rapoo.ButtonID]rapoo.ButtonMapping, len(snap.Buttons))
	for _, mapping := range snap.Buttons {
		byID[mapping.Slot.ID] = mapping
	}
	for _, want := range expected {
		got, ok := byID[want.Slot.ID]
		if !ok || !rapoo.ButtonActionsEqual(got.Action, want.Action) {
			return fmt.Errorf("ボタン %d の反映を確認できませんでした", want.Slot.ID)
		}
	}
	return nil
}

func verifyPresetSnapshot(snap rapoo.DeviceSnapshot, preset profile.Preset, pollingHz int) error {
	if pollingHz > 0 {
		if err := verifyPollingRate(snap, pollingHz); err != nil {
			return err
		}
	}
	if preset.DPI.StageCount >= 1 {
		if err := verifyDPIConfig(snap, preset.DPI); err != nil {
			return err
		}
	}
	if err := verifySensorPatch(snap, preset.Sensor); err != nil {
		return err
	}
	if len(preset.Buttons) > 0 {
		if err := verifyButtonMappings(snap, preset.Buttons); err != nil {
			return err
		}
	}
	return nil
}

func verifyAppliedSnapshot(snap rapoo.DeviceSnapshot, applied []string) error {
	for _, step := range applied {
		switch step {
		case "dpi", "dpiX", "dpiY", "dpiStage":
			if snap.DPI == nil {
				return errors.New("DPI設定は書き込まれましたが、読み戻せませんでした")
			}
		case "sensor", "sensor884", "sensor8C0", "pollingRate":
			if snap.Sensor == nil {
				return errors.New("センサー設定は書き込まれましたが、読み戻せませんでした")
			}
		case "button", "buttons":
			if len(snap.Buttons) == 0 {
				return errors.New("ボタン設定は書き込まれましたが、読み戻せませんでした")
			}
		case "factoryReset":
			if snap.DPI == nil || snap.Sensor == nil || len(snap.Buttons) == 0 {
				return errors.New("初期化は実行されましたが、設定を完全に読み戻せませんでした")
			}
		}
	}
	return nil
}

func wrapStepError(applied []string, err error) error {
	var partial *rapoo.PartialError
	if errors.As(err, &partial) {
		all := append(append([]string(nil), applied...), partial.Applied...)
		return &PartialWriteError{Applied: all, Err: partial.Err}
	}
	if errors.Is(err, rapoo.ErrDeadlineExceeded) {
		if len(applied) == 0 {
			return err
		}
		return &PartialWriteError{Applied: append([]string(nil), applied...), Err: err}
	}
	if errors.Is(err, rapoo.ErrResultUnknown) {
		if len(applied) == 0 {
			return err
		}
		return &PartialWriteError{Applied: append([]string(nil), applied...), Err: err}
	}
	if len(applied) > 0 {
		// A later step failed with an ordinary error. The earlier steps are
		// already applied, so the caller must still receive a partial result.
		return &PartialWriteError{Applied: append([]string(nil), applied...), Err: err}
	}
	return err
}

// SetPollingRate changes the mouse polling rate (125..4000 Hz) and returns the refreshed snapshot.
func (s *MouseService) SetPollingRate(hz int) (rapoo.DeviceSnapshot, error) {
	return s.SetPollingRateForGeneration(hz, s.currentGeneration(), time.Time{})
}

// SetPollingRateForGeneration binds the write to the generation the UI saw.
func (s *MouseService) SetPollingRateForGeneration(hz int, generation uint64, deadline time.Time) (rapoo.DeviceSnapshot, error) {
	client, actualGeneration, err := s.withWritableClient(generation, deadline, func(c *rapoo.Client) error {
		return c.SetPollingRateDeadline(hz, deadline)
	})
	if err != nil {
		return rapoo.DeviceSnapshot{}, s.wrapFriendly(err)
	}
	return s.readAfterWrite(client, actualGeneration, []string{"pollingRate"}, func(snap rapoo.DeviceSnapshot) error {
		return verifyPollingRate(snap, hz)
	})
}

// SetCurrentDPIGear switches the active DPI stage (1..7) and returns the refreshed snapshot.
func (s *MouseService) SetCurrentDPIGear(stage int) (rapoo.DeviceSnapshot, error) {
	return s.SetCurrentDPIGearForGeneration(stage, s.currentGeneration(), time.Time{})
}

// SetCurrentDPIGearForGeneration binds the write to the generation the UI saw.
func (s *MouseService) SetCurrentDPIGearForGeneration(stage int, generation uint64, deadline time.Time) (rapoo.DeviceSnapshot, error) {
	client, actualGeneration, err := s.withWritableClient(generation, deadline, func(c *rapoo.Client) error {
		return c.SetCurrentDPIGearDeadline(stage, deadline)
	})
	if err != nil {
		return rapoo.DeviceSnapshot{}, s.wrapFriendly(err)
	}
	return s.readAfterWrite(client, actualGeneration, []string{"dpiStage"}, func(snap rapoo.DeviceSnapshot) error {
		if snap.DPI == nil {
			return errors.New("DPI設定を読み戻せませんでした")
		}
		if snap.DPI.CurrentStage != stage {
			return fmt.Errorf("使用DPIステージの反映を確認できませんでした (要求 %d, 実測 %d)", stage, snap.DPI.CurrentStage)
		}
		return nil
	})
}

// SetDPIConfig updates X/Y DPI values, stage count (1..7), and active stage, returning the refreshed snapshot.
func (s *MouseService) SetDPIConfig(cfg rapoo.DPIConfig) (rapoo.DeviceSnapshot, error) {
	return s.SetDPIConfigForGeneration(cfg, s.currentGeneration(), time.Time{})
}

// SetDPIConfigForGeneration binds the write to the generation the UI saw.
func (s *MouseService) SetDPIConfigForGeneration(cfg rapoo.DPIConfig, generation uint64, deadline time.Time) (rapoo.DeviceSnapshot, error) {
	client, actualGeneration, err := s.withWritableClient(generation, deadline, func(c *rapoo.Client) error {
		return c.SetDPIConfigDeadline(cfg, deadline)
	})
	if err != nil {
		return rapoo.DeviceSnapshot{}, s.wrapFriendly(wrapStepError(nil, err))
	}
	return s.readAfterWrite(client, actualGeneration, []string{"dpi"}, func(snap rapoo.DeviceSnapshot) error {
		return verifyDPIConfig(snap, cfg)
	})
}

// UpdateSensorConfig applies a SensorPatch (LOD, Motion Sync, Debounce, Sleep, Ripple, Linear, Angle, Glass).
func (s *MouseService) UpdateSensorConfig(patch rapoo.SensorPatch) (rapoo.DeviceSnapshot, error) {
	return s.UpdateSensorConfigForGeneration(patch, s.currentGeneration(), time.Time{})
}

// UpdateSensorConfigForGeneration binds the write to the generation the UI saw.
func (s *MouseService) UpdateSensorConfigForGeneration(patch rapoo.SensorPatch, generation uint64, deadline time.Time) (rapoo.DeviceSnapshot, error) {
	client, actualGeneration, err := s.withWritableClient(generation, deadline, func(c *rapoo.Client) error {
		return c.UpdateSensorConfigDeadline(patch, deadline)
	})
	if err != nil {
		return rapoo.DeviceSnapshot{}, s.wrapFriendly(wrapStepError(nil, err))
	}
	return s.readAfterWrite(client, actualGeneration, []string{"sensor"}, func(snap rapoo.DeviceSnapshot) error {
		return verifySensorPatch(snap, patch)
	})
}

// SetButtonMapping updates one physical button's binding (with Left-Click safety validation).
func (s *MouseService) SetButtonMapping(buttonID int, action rapoo.ButtonAction) (rapoo.DeviceSnapshot, error) {
	return s.SetButtonMappingForGeneration(buttonID, action, s.currentGeneration(), time.Time{})
}

// SetButtonMappingForGeneration binds the write to the generation the UI saw.
func (s *MouseService) SetButtonMappingForGeneration(buttonID int, action rapoo.ButtonAction, generation uint64, deadline time.Time) (rapoo.DeviceSnapshot, error) {
	client, actualGeneration, err := s.withWritableClient(generation, deadline, func(c *rapoo.Client) error {
		return c.SetButtonMappingDeadline(rapoo.ButtonID(buttonID), action, deadline)
	})
	if err != nil {
		return rapoo.DeviceSnapshot{}, s.wrapFriendly(wrapStepError(nil, err))
	}
	return s.readAfterWrite(client, actualGeneration, []string{"button"}, func(snap rapoo.DeviceSnapshot) error {
		return verifyButtonMapping(snap, buttonID, action)
	})
}

// ResetButtonsToDefault restores all 10 physical buttons to their factory bindings.
func (s *MouseService) ResetButtonsToDefault() (rapoo.DeviceSnapshot, error) {
	return s.ResetButtonsToDefaultForGeneration(s.currentGeneration(), time.Time{})
}

// ResetButtonsToDefaultForGeneration binds the write to the generation the UI saw.
func (s *MouseService) ResetButtonsToDefaultForGeneration(generation uint64, deadline time.Time) (rapoo.DeviceSnapshot, error) {
	client, actualGeneration, err := s.withWritableClient(generation, deadline, func(c *rapoo.Client) error {
		return c.SetAllButtonMappingsDeadline(rapoo.DefaultButtonMappings(), deadline)
	})
	if err != nil {
		return rapoo.DeviceSnapshot{}, s.wrapFriendly(wrapStepError(nil, err))
	}
	return s.readAfterWrite(client, actualGeneration, []string{"buttons"}, func(snap rapoo.DeviceSnapshot) error {
		return verifyButtonMappings(snap, rapoo.DefaultButtonMappings())
	})
}

// FactoryReset restores all onboard settings on the mouse to factory defaults.
func (s *MouseService) FactoryReset() (rapoo.DeviceSnapshot, error) {
	return s.FactoryResetForGeneration(s.currentGeneration(), time.Time{})
}

// FactoryResetForGeneration binds the reset to the generation the dialog was
// opened against. A device swap while the dialog is open aborts the write.
func (s *MouseService) FactoryResetForGeneration(generation uint64, deadline time.Time) (rapoo.DeviceSnapshot, error) {
	client, actualGeneration, err := s.withWritableClient(generation, deadline, func(c *rapoo.Client) error {
		if err := c.FactoryResetDeadline(deadline); err != nil {
			return err
		}
		// Give mouse MCU a moment to reinitialize flash defaults when connected to real hardware.
		s.mu.RLock()
		demo := s.demoMode
		s.mu.RUnlock()
		if !demo {
			time.Sleep(180 * time.Millisecond)
		}
		return nil
	})
	if err != nil {
		return rapoo.DeviceSnapshot{}, s.wrapFriendly(err)
	}
	return s.readAfterWrite(client, actualGeneration, []string{"factoryReset"}, nil)
}

// ============================================================================
// Profile Preset Library Operations
// ============================================================================

func (s *MouseService) presetStoreError() error {
	if s.presetErr != nil {
		return s.presetErr
	}
	return errors.New("プロファイル保存領域を初期化できませんでした")
}

// ListPresets returns user-saved presets and a storage error, if any.
func (s *MouseService) ListPresets() ([]profile.Preset, error) {
	if s.presetStore == nil {
		return nil, s.presetStoreError()
	}
	return s.presetStore.List()
}

// SavePreset snapshots the current mouse configuration as a named preset.
func (s *MouseService) SavePreset(id, name, description string) (profile.Preset, error) {
	if s.presetStore == nil {
		return profile.Preset{}, s.presetStoreError()
	}
	snap, err := s.readSnapshotRetry()
	if err != nil {
		return profile.Preset{}, err
	}
	if snap.DPI == nil || snap.Sensor == nil || snap.Buttons == nil {
		return profile.Preset{}, errors.New("デバイス設定を完全に読み取れなかったため、プロファイルを保存できません。再読み込みしてください。")
	}
	return s.presetStore.SaveSnapshot(id, name, description, snap)
}

// ApplyPreset writes all settings from a preset (DPI, Polling Rate, Sensor, Buttons) to the mouse.
func (s *MouseService) ApplyPreset(id string) (rapoo.DeviceSnapshot, error) {
	return s.ApplyPresetForGeneration(id, s.currentGeneration(), time.Time{})
}

// ApplyPresetForGeneration binds the whole multi-step apply to the generation
// the UI saw and stops between steps when the deadline expires.
func (s *MouseService) ApplyPresetForGeneration(id string, generation uint64, deadline time.Time) (rapoo.DeviceSnapshot, error) {
	if s.presetStore == nil {
		return rapoo.DeviceSnapshot{}, s.presetStoreError()
	}
	preset, ok, err := s.presetStore.Get(id)
	if err != nil {
		return rapoo.DeviceSnapshot{}, err
	}
	if !ok {
		return rapoo.DeviceSnapshot{}, errors.New("指定されたプロファイルが見つかりません")
	}
	if err := profile.ValidatePreset(preset); err != nil {
		return rapoo.DeviceSnapshot{}, err
	}

	var applied []string
	var expectedRate int
	client, actualGeneration, err := s.withWritableClient(generation, deadline, func(c *rapoo.Client) error {
		targetGen := c.Model().ProtocolGen
		if preset.ProtocolGen != 0 && targetGen != 0 && preset.ProtocolGen != targetGen {
			return errors.New("プロファイルのハードウェア世代が接続中のマウスと一致しません")
		}
		if targetGen == 1 {
			if (preset.Sensor.PressDebounceStep != nil && *preset.Sensor.PressDebounceStep > 6) ||
				(preset.Sensor.ReleaseDebounceStep != nil && *preset.Sensor.ReleaseDebounceStep > 6) {
				return errors.New("Gen-A のデバウンスは 0..6 の段階値で指定してください")
			}
		}

		// Clamp polling rate to connection capability (e.g. 1000Hz max when wired USB-C).
		rate := preset.PollingHz
		if rate > 0 {
			maxRate := c.Model().MaxRateHz
			if maxRate > 0 && rate > maxRate {
				rate = maxRate
			}
			expectedRate = rate
			if err := c.SetPollingRateDeadline(rate, deadline); err != nil {
				return wrapStepError(applied, fmt.Errorf("ポーリングレートの適用に失敗しました: %s: %w", s.friendlyError(err), err))
			}
			applied = append(applied, "pollingRate")
		}
		if preset.DPI.StageCount >= 1 {
			if err := c.SetDPIConfigDeadline(preset.DPI, deadline); err != nil {
				return wrapStepError(applied, fmt.Errorf("DPIの適用に失敗しました: %s: %w", s.friendlyError(err), err))
			}
			applied = append(applied, "dpi")
		}
		if err := c.UpdateSensorConfigDeadline(preset.Sensor, deadline); err != nil {
			return wrapStepError(applied, fmt.Errorf("センサー設定の適用に失敗しました: %s: %w", s.friendlyError(err), err))
		}
		applied = append(applied, "sensor")
		if len(preset.Buttons) > 0 {
			if err := c.SetAllButtonMappingsDeadline(preset.Buttons, deadline); err != nil {
				return wrapStepError(applied, fmt.Errorf("ボタン割り当ての適用に失敗しました: %s: %w", s.friendlyError(err), err))
			}
			applied = append(applied, "buttons")
		}
		return nil
	})
	if err != nil {
		return rapoo.DeviceSnapshot{}, err
	}
	return s.readAfterWrite(client, actualGeneration, applied, func(snap rapoo.DeviceSnapshot) error {
		return verifyPresetSnapshot(snap, preset, expectedRate)
	})
}

// DeletePreset deletes a user-saved preset.
func (s *MouseService) DeletePreset(id string) error {
	if s.presetStore == nil {
		return s.presetStoreError()
	}
	return s.presetStore.Delete(id)
}

// ExportPresetJSON returns the formatted JSON string for a preset.
func (s *MouseService) ExportPresetJSON(id string) (string, error) {
	if s.presetStore == nil {
		return "", s.presetStoreError()
	}
	return s.presetStore.ExportJSON(id)
}

// ImportPresetJSON imports and saves a preset from JSON text.
func (s *MouseService) ImportPresetJSON(rawJSON string) (profile.Preset, error) {
	if s.presetStore == nil {
		return profile.Preset{}, s.presetStoreError()
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
			if updaterguard.Held() {
				s.connectMu.Lock()
				s.closeClient()
				s.connectMu.Unlock()
				s.markDisconnected(errors.New("ファームウェア更新中です"))
				lastPoll = now
				continue
			}

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
	current := curClient.Endpoint()
	currentPresent := false
	bestPriority := -1
	for _, pair := range pairs {
		if pairMatchesEndpoint(pair, current) {
			currentPresent = true
		}
		if pair.Ambiguous {
			continue
		}
		if pair.Model.Priority > bestPriority {
			bestPriority = pair.Model.Priority
		}
	}
	if !currentPresent {
		return true
	}
	// Do not churn between two identical devices just because the map/sort
	// order changed. Only a strictly higher-priority endpoint (wired over
	// dongle) may replace a healthy current one.
	return bestPriority > current.Model.Priority
}

func pairMatchesEndpoint(pair, current rapoo.EndpointPair) bool {
	if current.Command != nil && pair.Command != nil &&
		pair.Command.Path == current.Command.Path {
		return true
	}
	if current.Report != nil && pair.Report != nil &&
		pair.Report.Path == current.Report.Path {
		return true
	}
	return false
}

func (s *MouseService) reconnectAndPoll() {
	s.connectMu.Lock()
	defer s.connectMu.Unlock()
	s.closeClient()
	s.ensureConnectedAndPollLocked()
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
	s.connectMu.Lock()
	defer s.connectMu.Unlock()
	s.ensureConnectedAndPollLocked()
}

func (s *MouseService) ensureConnectedAndPollLocked() {
	s.mu.RLock()
	hasClient := s.client != nil
	mode := s.state.Mode
	demo := s.demoMode
	s.mu.RUnlock()

	if !hasClient {
		if updaterguard.Held() {
			s.markDisconnected(errors.New("ファームウェア更新中です"))
			return
		}
		opts := rapoo.ClientOptions{
			PassiveOnly:  mode == ModePassiveOnly,
			OnAsyncEvent: s.handleAsyncEvent,
			OnAsyncError: s.handleAsyncError,
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
		s.cachedSnap = nil
		s.state.Generation++
		s.state.Connected = true
		s.state.DemoMode = demo
		s.state.DeviceName = model.Name
		s.state.Connection = model.Connection
		s.state.ProductID = model.ProductID
		s.state.MaxRateHz = model.MaxRateHz
		s.state.PermissionDenied = false
		s.state.LastError = ""
		snap := s.snapshotLocked()
		s.mu.Unlock()
		s.publish(snap)
	}

	if mode != ModePassiveOnly {
		s.pollOnceLocked()
	}
}

func (s *MouseService) pollOnce() {
	s.connectMu.Lock()
	defer s.connectMu.Unlock()
	s.pollOnceLocked()
}

func (s *MouseService) pollOnceLocked() {
	s.mu.RLock()
	c := s.client
	mode := s.state.Mode
	s.mu.RUnlock()

	if c == nil || mode == ModePassiveOnly {
		return
	}

	bat, err := s.readBatteryWithRetry(c, 3)
	if err != nil {
		if errors.Is(err, rapoo.ErrMouseAsleep) {
			_, _ = s.markSleepingIfCurrent(true, nil, 0)
			return
		}
		s.closeClient()
		s.markDisconnected(err)
		return
	}

	s.applyBattery(bat)
}

// readBatteryWithRetry gives the 4K dongle a few short chances before a
// transient "busy" response is treated as the mouse being asleep. A full
// settings snapshot can keep the dongle busy around the same time.
func (s *MouseService) readBatteryWithRetry(c *rapoo.Client, attempts int) (rapoo.BatteryState, error) {
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		bat, err := c.ReadBattery()
		if err == nil {
			return bat, nil
		}
		lastErr = err
		if !errors.Is(err, rapoo.ErrMouseAsleep) {
			return bat, err
		}
		if attempt+1 < attempts {
			time.Sleep(200 * time.Millisecond)
		}
	}
	return rapoo.BatteryState{}, lastErr
}

func (s *MouseService) markSleeping(transient bool) bool {
	applied, _ := s.markSleepingIfCurrent(transient, nil, 0)
	return applied
}

// markSleepingIfCurrent applies a sleep transition only when the optional
// client/generation pair is still current. It returns applied and current.
func (s *MouseService) markSleepingIfCurrent(
	transient bool,
	expected *rapoo.Client,
	generation uint64,
) (applied bool, current bool) {
	s.mu.Lock()
	if expected != nil && (s.client != expected || s.state.Generation != generation) {
		s.mu.Unlock()
		return false, false
	}
	if transient && !s.lastBatteryOK.IsZero() && time.Since(s.lastBatteryOK) < s.cfg.SleepGrace {
		s.mu.Unlock()
		// The dongle occasionally answers 0x02 while it is busy with a
		// settings read. Keep the recent online state and retry shortly
		// instead of flashing "sleeping" on the UI.
		time.AfterFunc(2*time.Second, s.RefreshNow)
		return false, true
	}
	s.state.MouseOnline = false
	s.state.Sleeping = true
	s.state.Charging = false
	s.state.LastError = ""
	snap := s.snapshotLocked()
	s.mu.Unlock()
	log.Printf("mouse marked sleeping")
	s.publish(snap)
	return true, true
}

func (s *MouseService) handleAsyncError(err error) {
	if err == nil {
		return
	}
	log.Printf("async report reader stopped: %v", err)
	s.NotifyDeviceChange()
}

func (s *MouseService) handleAsyncEvent(ev rapoo.AsyncEvent) {
	switch ev.Kind {
	case rapoo.EventBattery:
		if ev.Battery == nil {
			return
		}
		if !ev.Battery.Online {
			_, _ = s.markSleepingIfCurrent(false, nil, 0)
			return
		}
		s.applyBattery(*ev.Battery)

	case rapoo.EventWake, rapoo.EventProfile:
		s.mu.Lock()
		mode := s.state.Mode
		emit := s.cfg.OnEventEmit
		shouldRefresh := mode != ModePassiveOnly && time.Since(s.lastWakeAt) >= 1200*time.Millisecond
		if shouldRefresh {
			s.lastWakeAt = time.Now()
		}
		s.mu.Unlock()

		if shouldRefresh {
			s.RefreshNow()
			if emit != nil {
				emit("rapoo:wake", true)
			}
		}
	}
}

func (s *MouseService) applyBattery(bat rapoo.BatteryState) {
	_ = s.applyBatteryIfCurrent(bat, nil, 0)
}

// applyBatteryIfCurrent applies a battery result only when the client and
// generation still match. A nil expected client means the caller already holds
// connectMu and the current client is authoritative.
func (s *MouseService) applyBatteryIfCurrent(bat rapoo.BatteryState, expected *rapoo.Client, generation uint64) bool {
	s.mu.Lock()
	if expected != nil && (s.client != expected || s.state.Generation != generation) {
		s.mu.Unlock()
		return false
	}
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
	if bat.Online {
		s.lastBatteryOK = time.Now()
	}
	s.state.PermissionDenied = false
	s.state.LastError = ""
	snap := s.snapshotLocked()
	s.mu.Unlock()

	s.publish(snap)
	return true
}

func (s *MouseService) updateFromSnapshot(ds rapoo.DeviceSnapshot, expectedClient *rapoo.Client, expectedGeneration uint64) bool {
	s.mu.Lock()
	if s.client != expectedClient || s.state.Generation != expectedGeneration {
		s.mu.Unlock()
		return false
	}
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
	if ds.Battery.Online {
		s.lastBatteryOK = time.Now()
	}
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
	st := s.snapshotLocked()
	emit := s.cfg.OnEventEmit
	s.mu.Unlock()

	s.publish(st)
	if emit != nil {
		emit("rapoo:snapshot", ds)
	}
	return true
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

func normalizeLanguage(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "ja" || value == "jp" || strings.HasPrefix(value, "ja-") {
		return "ja"
	}
	return "en"
}

func (s *MouseService) friendlyError(err error) string {
	if err == nil {
		return ""
	}
	language, _ := s.language.Load().(string)
	japanese := normalizeLanguage(language) == "ja"
	switch {
	case errors.Is(err, rapoo.ErrDeviceNotFound):
		if japanese {
			return "Rapoo VT9 Pro 4K (4Kレシーバー または USBケーブル) が見つかりません。接続を確認するか、デモモードでお試しください。"
		}
		return "Rapoo VT9 Pro 4K was not found. Check the receiver/USB cable or try Demo mode."
	case errors.Is(err, rapoo.ErrMouseAsleep):
		if japanese {
			return "ワイヤレスマウスがスリープ中または電源OFFです。マウスを軽く動かして復帰させてください。"
		}
		return "The wireless mouse is asleep or off. Move it to wake it."
	case errors.Is(err, rapoo.ErrMissingLeftClick):
		if japanese {
			return "操作不能を防ぐため、少なくとも1つの物理ボタンに「左クリック」を残す必要があります。先に別のボタンへ左クリックを割り当ててください。"
		}
		return "To prevent an unusable mouse, keep Left Click on at least one physical button. Assign Left Click to another button first."
	case errors.Is(err, ErrBusy):
		if japanese {
			return "マウスの初期化中です。少し待って再試行してください。"
		}
		return "The mouse is initializing. Wait a moment and retry."
	case errors.Is(err, rapoo.ErrAmbiguousDevice):
		if japanese {
			return "同型のマウスが複数接続されており、対象を一意に特定できません。1台だけ接続して再試行してください。"
		}
		return "Multiple identical mice are connected and cannot be told apart. Connect only one and retry."
	case errors.Is(err, rapoo.ErrResultUnknown):
		if japanese {
			return "コマンドは送信されましたが、結果を確認できませんでした。設定を再読み込みしてください。"
		}
		return "The command was sent but its result could not be confirmed. Reload the settings."
	case errors.Is(err, hid.ErrDeviceBusy):
		if japanese {
			return "別のプロセス（CLI または旧アプリ）がマウスの書き込みチャンネルを使用中です。そのプロセスを終了してから再試行してください。"
		}
		return "Another process (the CLI or a legacy app) is using the mouse command channel. Close it and retry."
	case isPermissionError(err):
		if runtime.GOOS == "linux" {
			if japanese {
				return "Linux の /dev/hidraw デバイスへのアクセス権限がありません。設定タブの udev ルールを適用してください。"
			}
			return "No permission to access /dev/hidraw. Apply the udev rule in Settings."
		}
		if japanese {
			return "HID デバイスへのアクセス権限がありません。"
		}
		return "No permission to access the HID device."
	default:
		if japanese {
			return fmt.Sprintf("デバイス通信エラー: %v", err)
		}
		return fmt.Sprintf("Device communication error: %v", err)
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
	s.cachedSnap = nil
	if err != nil && !errors.Is(err, rapoo.ErrDeviceNotFound) {
		s.state.LastError = s.friendlyError(err)
	} else {
		s.state.LastError = ""
	}
	snap := s.snapshotLocked()
	s.mu.Unlock()

	if changed {
		s.publish(snap)
	}
}

// snapshotLocked takes a state copy with a revision assigned at mutation time.
// A delayed publish of an older snapshot therefore carries an older revision
// and cannot overtake a newer state.
func (s *MouseService) snapshotLocked() State {
	s.state.Revision++
	return s.state
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
