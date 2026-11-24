package hid

import (
	"sync"
	"time"
)

// MockProvider is an in-memory HID Provider for deterministic unit testing of
// protocol clients and Wails application services without physical hardware.
type MockProvider struct {
	mu      sync.Mutex
	Devices map[string]*MockDevice
}

// NewMockProvider constructs an empty MockProvider.
func NewMockProvider() *MockProvider {
	return &MockProvider{
		Devices: make(map[string]*MockDevice),
	}
}

// Register adds a MockDevice to the provider.
func (m *MockProvider) Register(dev *MockDevice) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Devices[dev.DeviceInfo.Path] = dev
}

// Enumerate returns all registered devices matching filter.
func (m *MockProvider) Enumerate(filter Filter) ([]DeviceInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []DeviceInfo
	for _, dev := range m.Devices {
		info := dev.DeviceInfo
		if !filter.MatchesVIDPID(info.VendorID, info.ProductID) {
			continue
		}
		if filter.InterfaceNumber >= 0 && info.InterfaceNumber != filter.InterfaceNumber {
			continue
		}
		out = append(out, info)
	}
	return out, nil
}

// Open returns the registered MockDevice for info.Path.
func (m *MockProvider) Open(info DeviceInfo, _ bool) (Device, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	dev, ok := m.Devices[info.Path]
	if !ok {
		return nil, ErrClosed
	}
	dev.reopen()
	return dev, nil
}

// MockDevice simulates a single HID collection.
type MockDevice struct {
	DeviceInfo DeviceInfo

	// OnSetOutput is called whenever SetOutputReport is invoked.
	OnSetOutput func(data []byte) error
	// OnGetInput is called whenever GetInputReport is invoked.
	OnGetInput func(reportID byte, length int) ([]byte, error)

	mu         sync.Mutex
	Written    [][]byte
	AsyncQueue chan []byte
	closedCh   chan struct{}
	closed     bool
}

// NewMockDevice constructs a ready-to-use MockDevice.
func NewMockDevice(info DeviceInfo) *MockDevice {
	return &MockDevice{
		DeviceInfo: info,
		AsyncQueue: make(chan []byte, 32),
		closedCh:   make(chan struct{}),
	}
}

func (d *MockDevice) reopen() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		d.closed = false
		d.closedCh = make(chan struct{})
	}
}

// Info returns the device's static metadata.
func (d *MockDevice) Info() DeviceInfo {
	return d.DeviceInfo
}

// SetOnSetOutput safely replaces the output handler while a test service is
// running.
func (d *MockDevice) SetOnSetOutput(fn func([]byte) error) {
	d.mu.Lock()
	d.OnSetOutput = fn
	d.mu.Unlock()
}

// SetOnGetInput safely replaces the input handler while a test service is
// running.
func (d *MockDevice) SetOnGetInput(fn func(byte, int) ([]byte, error)) {
	d.mu.Lock()
	d.OnGetInput = fn
	d.mu.Unlock()
}

// SetOutputReport records the report and invokes OnSetOutput if configured.
func (d *MockDevice) SetOutputReport(data []byte, _ time.Duration) error {
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return ErrClosed
	}
	cp := make([]byte, len(data))
	copy(cp, data)
	d.Written = append(d.Written, cp)
	fn := d.OnSetOutput
	d.mu.Unlock()

	if fn != nil {
		return fn(cp)
	}
	return nil
}

// GetInputReport invokes OnGetInput if configured.
func (d *MockDevice) GetInputReport(reportID byte, length int, _ time.Duration) ([]byte, error) {
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return nil, ErrClosed
	}
	fn := d.OnGetInput
	d.mu.Unlock()

	if fn != nil {
		return fn(reportID, length)
	}
	buf := make([]byte, length)
	buf[0] = 0x01
	return buf, nil
}

// PushAsyncPacket enqueues an interrupt Input packet for ReadInputPacket.
func (d *MockDevice) PushAsyncPacket(pkt []byte) {
	cp := make([]byte, len(pkt))
	copy(cp, pkt)
	d.AsyncQueue <- cp
}

// ReadInputPacket waits for a packet from AsyncQueue, timeout, or Close.
func (d *MockDevice) ReadInputPacket(timeout time.Duration) ([]byte, error) {
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return nil, ErrClosed
	}
	closedCh := d.closedCh
	d.mu.Unlock()

	if timeout <= 0 {
		select {
		case pkt := <-d.AsyncQueue:
			return pkt, nil
		case <-closedCh:
			return nil, ErrClosed
		}
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case pkt := <-d.AsyncQueue:
		return pkt, nil
	case <-closedCh:
		return nil, ErrClosed
	case <-timer.C:
		return nil, ErrTimeout
	}
}

// Close marks the mock device closed and unblocks pending reads.
func (d *MockDevice) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.closed {
		d.closed = true
		close(d.closedCh)
	}
	return nil
}
