// Package service provides a Wails-ready application service that manages
// device discovery, hotplug reconnection, passive 0xBB notifications, gentle
// battery polling, and thread-safe state subscriptions.
package service

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/nico/rapoo-vt9pro-4k/pkg/hid"
	"github.com/nico/rapoo-vt9pro-4k/pkg/rapoo"
)

// PollingMode controls how frequently outbound 0xBA battery queries are sent.
// Passive 0xBB notifications from COL05 remain active in all modes.
type PollingMode string

const (
	// ModeNormal polls every 60s in addition to instant passive 0xBB updates.
	ModeNormal PollingMode = "normal"
	// ModeLowFrequency polls every 5m in addition to passive 0xBB updates.
	ModeLowFrequency PollingMode = "low_freq"
	// ModePassiveOnly opens only the read-only COL05 report handle (zero TX packets).
	ModePassiveOnly PollingMode = "passive_only"
)

// State represents the observable UI state for system tray icons or a Wails frontend.
type State struct {
	Connected   bool                 `json:"connected"` // Dongle or USB-C cable present
	DeviceName  string               `json:"deviceName,omitempty"`
	Connection  rapoo.ConnectionMode `json:"connection,omitempty"`
	ProductID   uint16               `json:"productId,omitempty"`
	HasBattery  bool                 `json:"hasBattery"`  // True once at least one valid % was read
	Percentage  int                  `json:"percentage"`  // 0..100
	Charging    bool                 `json:"charging"`    // True when charging
	MouseOnline bool                 `json:"mouseOnline"` // False when wireless mouse is sleeping
	Sleeping    bool                 `json:"sleeping"`
	Mode        PollingMode          `json:"mode"`
	UpdatedAt   time.Time            `json:"updatedAt"`
	LastError   string               `json:"lastError,omitempty"`
}

// Config configures a MouseService instance.
type Config struct {
	InitialMode       PollingMode
	NormalInterval    time.Duration
	LowFreqInterval   time.Duration
	ReconnectInterval time.Duration
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

// MouseService manages lifecycle, battery telemetry, and device settings.
// Its exported methods are structured so it can be bound directly to Wails.
type MouseService struct {
	provider hid.Provider
	cfg      Config

	mu          sync.RWMutex
	client      *rapoo.Client
	state       State
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
	return &MouseService{
		provider:    provider,
		cfg:         cfg,
		state:       State{Mode: cfg.InitialMode},
		subscribers: make(map[int]Listener),
		refreshCh:   make(chan struct{}, 1),
		reopenCh:    make(chan struct{}, 1),
	}
}

// Startup starts background discovery, passive telemetry listening, and polling.
// Compatible with Wails OnStartup lifecycle hook.
func (s *MouseService) Startup(ctx context.Context) {
	runCtx, cancel := context.WithCancel(ctx)
	s.mu.Lock()
	s.cancel = cancel
	s.mu.Unlock()

	s.wg.Add(1)
	go s.loop(runCtx)
}

// Shutdown cleanly closes open HID handles and stops background workers.
// Compatible with Wails OnShutdown lifecycle hook.
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
// Immediately invokes fn with the current state snapshot.
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

// NotifyDeviceChange triggers immediate re-enumeration (e.g. on Win32 WM_DEVICECHANGE
// when plugging/unplugging the USB-C charging cable or 4K receiver).
func (s *MouseService) NotifyDeviceChange() {
	select {
	case s.reopenCh <- struct{}{}:
	default:
	}
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
		// Reopen client to attach or detach the writable command handle.
		s.NotifyDeviceChange()
	} else if !isPassive {
		s.RefreshNow()
	}
}

// ReadSnapshot queries full device details (FW, DPI, Sensor) on demand for Wails UI.
func (s *MouseService) ReadSnapshot() (rapoo.DeviceSnapshot, error) {
	s.mu.RLock()
	c := s.client
	s.mu.RUnlock()
	if c == nil {
		return rapoo.DeviceSnapshot{}, rapoo.ErrDeviceNotFound
	}
	return c.ReadSnapshot()
}

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
			// Brief settle delay so Windows PnP finishes creating HID collections on hotplug.
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

			// Also check periodically whether a higher-priority connection appeared
			// (e.g. USB-C cable plugged in while wireless dongle stayed attached).
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
	s.mu.RUnlock()
	if curClient == nil {
		return false
	}

	infos, err := s.provider.Enumerate(rapoo.DefaultHIDFilter())
	if err != nil {
		return false
	}
	pairs := rapoo.MatchEndpoints(infos)
	if len(pairs) == 0 {
		// Current device was unplugged!
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
	s.mu.RUnlock()

	if !hasClient {
		client, err := rapoo.OpenFirst(s.provider, rapoo.ClientOptions{
			PassiveOnly:  mode == ModePassiveOnly,
			OnAsyncEvent: s.handleAsyncEvent,
		})
		if err != nil {
			s.markDisconnected(err)
			return
		}

		model := client.Model()
		s.mu.Lock()
		s.client = client
		s.state.Connected = true
		s.state.DeviceName = model.Name
		s.state.Connection = model.Connection
		s.state.ProductID = model.ProductID
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
			s.mu.Lock()
			s.state.MouseOnline = false
			s.state.Sleeping = true
			s.state.Charging = false
			s.state.LastError = ""
			snap := s.state
			s.mu.Unlock()
			s.publish(snap)
			return
		}
		// Underlying HID handle likely invalidated by unplug/sleep -> reopen next cycle.
		s.closeClient()
		s.markDisconnected(err)
		return
	}

	s.applyBattery(bat)
}

func (s *MouseService) handleAsyncEvent(ev rapoo.AsyncEvent) {
	switch ev.Kind {
	case rapoo.EventBattery:
		if ev.Battery == nil {
			return
		}
		if !ev.Battery.Online {
			s.mu.Lock()
			s.state.MouseOnline = false
			s.state.Sleeping = true
			s.state.Charging = false
			snap := s.state
			s.mu.Unlock()
			s.publish(snap)
			return
		}
		s.applyBattery(*ev.Battery)

	case rapoo.EventWake:
		// COL06 status/wake packet: refresh battery if not in passive-only mode,
		// debounced to at most once every 2 seconds.
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
	s.state.Connected = true
	s.state.HasBattery = true
	s.state.Percentage = bat.Percentage
	s.state.Charging = bat.Charging
	s.state.MouseOnline = bat.Online
	s.state.Sleeping = bat.Sleeping
	s.state.UpdatedAt = bat.UpdatedAt
	s.state.LastError = ""
	snap := s.state
	s.mu.Unlock()

	s.publish(snap)
}

func (s *MouseService) markDisconnected(err error) {
	s.mu.Lock()
	changed := s.state.Connected || s.state.MouseOnline
	s.state.Connected = false
	s.state.MouseOnline = false
	s.state.Sleeping = false
	s.state.Charging = false
	s.state.DeviceName = ""
	s.state.Connection = ""
	s.state.ProductID = 0
	if err != nil && !errors.Is(err, rapoo.ErrDeviceNotFound) {
		s.state.LastError = err.Error()
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
