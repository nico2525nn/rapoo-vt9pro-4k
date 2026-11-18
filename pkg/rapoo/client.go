package rapoo

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/nico/rapoo-vt9pro-4k/pkg/hid"
)

// ClientOptions customizes timeouts and passive/active behavior for a Client.
type ClientOptions struct {
	// PassiveOnly opens only the read-only Usage 0x0002 (COL06/COL05) notification
	// endpoint and refuses any outbound HID control reports.
	PassiveOnly bool
	// IOTimeout bounds individual SetOutputReport / GetInputReport calls.
	IOTimeout time.Duration
	// MaxBusyPolls caps GET_INPUT_REPORT retries while status == 0x00/0x02.
	MaxBusyPolls int
	// BusyPollInterval is the wait between GET_INPUT_REPORT polls (default 25ms).
	BusyPollInterval time.Duration
	// MaxCommandRetries is the outer send/recv retry count on transient error (default 2).
	MaxCommandRetries int
	// OnAsyncEvent receives unsolicited COL06 / COL05 notifications.
	OnAsyncEvent func(AsyncEvent)
}

func (o *ClientOptions) applyDefaults() {
	if o.IOTimeout <= 0 {
		o.IOTimeout = 250 * time.Millisecond
	}
	if o.MaxBusyPolls <= 0 {
		o.MaxBusyPolls = 6
	}
	if o.BusyPollInterval <= 0 {
		o.BusyPollInterval = 25 * time.Millisecond
	}
	if o.MaxCommandRetries <= 0 {
		o.MaxCommandRetries = 2
	}
}

// Client manages communication with a connected Rapoo VT9 Pro 4K mouse or dongle.
// All outbound commands are strictly serialized and paced; passive notifications
// are read concurrently on the dedicated report HID handle.
type Client struct {
	provider hid.Provider
	endpoint EndpointPair
	opts     ClientOptions

	cmdMu      sync.Mutex
	opcodes    OpcodeSet
	cmdDev     hid.Device
	reportDev  hid.Device
	lastCmdEnd time.Time

	stopCh    chan struct{}
	wg        sync.WaitGroup
	closeOnce sync.Once
}

// OpenFirst discovers the highest-priority connected Rapoo VT9 Pro 4K device
// (Wired USB-C preferred over 4K Wireless Dongle when both are attached) and opens it.
func OpenFirst(provider hid.Provider, opts ClientOptions) (*Client, error) {
	if provider == nil {
		provider = hid.NewProvider()
	}
	infos, err := provider.Enumerate(DefaultHIDFilter())
	if err != nil {
		return nil, err
	}
	pairs := MatchEndpoints(infos)
	if len(pairs) == 0 {
		return nil, ErrDeviceNotFound
	}
	return OpenEndpoint(provider, pairs[0], opts)
}

// OpenEndpoint opens a specific matched EndpointPair.
func OpenEndpoint(provider hid.Provider, pair EndpointPair, opts ClientOptions) (*Client, error) {
	opts.applyDefaults()

	c := &Client{
		provider: provider,
		endpoint: pair,
		opts:     opts,
		opcodes:  pair.Model.Opcodes,
		stopCh:   make(chan struct{}),
	}

	if !opts.PassiveOnly && pair.Command != nil {
		dev, err := provider.Open(*pair.Command, true)
		if err != nil {
			return nil, fmt.Errorf("open command endpoint: %w", err)
		}
		c.cmdDev = dev
	}

	if pair.Report != nil {
		rdev, err := provider.Open(*pair.Report, false)
		if err != nil {
			if c.cmdDev == nil {
				return nil, fmt.Errorf("open async report endpoint: %w", err)
			}
		} else {
			c.reportDev = rdev
		}
	}

	if c.cmdDev == nil && c.reportDev == nil {
		return nil, ErrDeviceNotFound
	}

	if c.reportDev != nil && opts.OnAsyncEvent != nil {
		c.wg.Add(1)
		go c.readReportLoop()
	}

	return c, nil
}

// Model returns static metadata for the connected device.
func (c *Client) Model() ModelSpec {
	c.cmdMu.Lock()
	ops := c.opcodes
	c.cmdMu.Unlock()
	m := c.endpoint.Model
	m.Opcodes = ops
	return m
}

// Endpoint returns the underlying matched HID endpoints.
func (c *Client) Endpoint() EndpointPair {
	return c.endpoint
}

// Close stops background listeners and closes all open HID handles.
func (c *Client) Close() error {
	var firstErr error
	c.closeOnce.Do(func() {
		close(c.stopCh)
		if c.reportDev != nil {
			if err := c.reportDev.Close(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		c.wg.Wait()

		c.cmdMu.Lock()
		if c.cmdDev != nil {
			if err := c.cmdDev.Close(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		c.cmdMu.Unlock()
	})
	return firstErr
}

func (c *Client) readReportLoop() {
	defer c.wg.Done()
	for {
		select {
		case <-c.stopCh:
			return
		default:
		}

		pkt, err := c.reportDev.ReadInputPacket(500 * time.Millisecond)
		if err != nil {
			if errors.Is(err, hid.ErrTimeout) {
				continue
			}
			if errors.Is(err, hid.ErrClosed) {
				return
			}
			return
		}

		if ev, ok := ParseAsyncReport(pkt, time.Now()); ok && c.opts.OnAsyncEvent != nil {
			c.opts.OnAsyncEvent(*ev)
		}
	}
}

// sendAndRecvLocked executes a single 0xBA control-report exchange while cmdMu is held.
func (c *Client) sendAndRecvLocked(tx []byte) ([]byte, error) {
	if c.opts.PassiveOnly || c.cmdDev == nil {
		return nil, errors.New("rapoo: command channel unavailable (passive-only mode)")
	}

	stepDelay := c.endpoint.Model.StepDelay
	if stepDelay <= 0 {
		stepDelay = 35 * time.Millisecond
	}

	var lastErr error
	for attempt := 0; attempt < c.opts.MaxCommandRetries; attempt++ {
		if since := time.Since(c.lastCmdEnd); since < stepDelay {
			time.Sleep(stepDelay - since)
		}

		if err := c.cmdDev.SetOutputReport(tx, c.opts.IOTimeout); err != nil {
			lastErr = err
			continue
		}

		time.Sleep(stepDelay)

		for poll := 0; poll < c.opts.MaxBusyPolls; poll++ {
			raw, err := c.cmdDev.GetInputReport(ReportIDCommand, PacketLength, c.opts.IOTimeout)
			c.lastCmdEnd = time.Now()
			if err != nil {
				lastErr = err
				break
			}
			resp := NormalizeCommandResponse(raw)
			if len(resp) == 0 {
				lastErr = ErrInvalidResponse
				break
			}
			if resp[0] == StatusOK {
				return resp, nil
			}
			if resp[0] == StatusPending {
				if poll+1 < c.opts.MaxBusyPolls {
					time.Sleep(c.opts.BusyPollInterval)
					continue
				}
				return resp, nil
			}
			if resp[0] == StatusUnsupported {
				// Either opcode belongs to the other firmware generation or RF reply hasn't landed yet
				if poll < 2 {
					time.Sleep(c.opts.BusyPollInterval)
					continue
				}
				return resp, nil
			}
			lastErr = fmt.Errorf("%w: status %#02x (raw=% x)", ErrInvalidResponse, resp[0], raw)
			break
		}
	}

	if lastErr == nil {
		lastErr = ErrInvalidResponse
	}
	return nil, lastErr
}

func (c *Client) sendAndRecv(tx []byte) ([]byte, error) {
	c.cmdMu.Lock()
	defer c.cmdMu.Unlock()
	return c.sendAndRecvLocked(tx)
}

// ============================================================================
// Read-Only Queries (Safe, Non-Mutating)
// ============================================================================

// ReadBattery queries the current battery level and charging status (0xAA on Gen-A,
// 0xBA on Gen-B, with automatic firmware-generation fallback if status == 0x00).
func (c *Client) ReadBattery() (BatteryState, error) {
	c.cmdMu.Lock()
	defer c.cmdMu.Unlock()

	tx := BuildBatteryQuery(c.endpoint.Model.TargetByte, c.opcodes.GetBattery)
	resp, err := c.sendAndRecvLocked(tx)
	if err != nil {
		return BatteryState{}, err
	}

	if len(resp) > 0 && resp[0] == StatusUnsupported {
		alt := alternateOpcodes(c.opcodes)
		altTX := BuildBatteryQuery(c.endpoint.Model.TargetByte, alt.GetBattery)
		if altResp, altErr := c.sendAndRecvLocked(altTX); altErr == nil && len(altResp) > 0 && altResp[0] != StatusUnsupported {
			c.opcodes = alt
			resp = altResp
		}
	}

	return ParseBatteryResponse(resp, time.Now())
}

// ReadFirmwareInfo queries mouse and optional 4K receiver firmware versions.
func (c *Client) ReadFirmwareInfo() (FirmwareInfo, error) {
	var info FirmwareInfo

	c.cmdMu.Lock()
	ops := c.opcodes
	c.cmdMu.Unlock()

	if c.endpoint.Model.Connection == ConnectionWireless4K {
		dongleTX := BuildFirmwareQuery(TargetLocalOrWired, ops.DongleVersion)
		if resp, err := c.sendAndRecv(dongleTX); err == nil {
			if raw, perr := ParseFirmwareResponse(resp); perr == nil {
				info.DongleRaw = raw
				info.DongleStr = FormatFirmwareVersion(raw)
			}
		}
	}

	mouseTX := BuildFirmwareQuery(c.endpoint.Model.TargetByte, ops.MouseVersion)
	resp, err := c.sendAndRecv(mouseTX)
	if err != nil {
		return info, err
	}
	raw, err := ParseFirmwareResponse(resp)
	if err != nil {
		return info, err
	}
	info.MouseRaw = raw
	info.MouseString = FormatFirmwareVersion(raw)
	return info, nil
}

// ReadMemory reads count bytes from onboard memory address addr (0xA4 / 0xB4).
func (c *Client) ReadMemory(addr uint32, count byte) ([]byte, error) {
	c.cmdMu.Lock()
	op := c.opcodes.ReadMemory
	c.cmdMu.Unlock()

	tx, err := BuildReadMemory(c.endpoint.Model.TargetByte, op, addr, count)
	if err != nil {
		return nil, err
	}
	resp, err := c.sendAndRecv(tx)
	if err != nil {
		return nil, err
	}
	return ParseReadMemory(resp, int(count))
}

// ReadDPIConfig reads all configured X/Y DPI stages and the active stage index.
func (c *Client) ReadDPIConfig() (DPIConfig, error) {
	memX, err := c.ReadMemory(AddrDPIStagesX, 24)
	if err != nil {
		return DPIConfig{}, err
	}
	memY, err := c.ReadMemory(AddrDPIStagesY, 20)
	if err != nil {
		return DPIConfig{}, err
	}
	return DecodeDPIConfig(memX, memY)
}

// ReadSensorConfig reads polling rate, LOD, Motion Sync, debounce, sleep timeout, and sensor flags.
func (c *Client) ReadSensorConfig() (SensorConfig, error) {
	mem880, err := c.ReadMemory(AddrPollingRate, 4)
	if err != nil {
		return SensorConfig{}, err
	}
	mem884, err := c.ReadMemory(AddrSilentHeight, 4)
	if err != nil {
		return SensorConfig{}, err
	}
	mem8C0, err := c.ReadMemory(AddrSensorParameters, 8)
	if err != nil {
		return SensorConfig{}, err
	}
	return DecodeSensorConfig(mem880, mem884, mem8C0)
}

// ReadSnapshot collects battery, firmware, DPI, and sensor state in one call.
func (c *Client) ReadSnapshot() (DeviceSnapshot, error) {
	bat, err := c.ReadBattery()
	snap := DeviceSnapshot{
		Model:   c.Model(),
		Battery: bat,
	}
	if c.endpoint.Command != nil {
		snap.CommandHID = c.endpoint.Command.Path
	}
	if c.endpoint.Report != nil {
		snap.ReportHID = c.endpoint.Report.Path
	}
	if err != nil && !errors.Is(err, ErrMouseAsleep) {
		return snap, err
	}

	if fw, fwErr := c.ReadFirmwareInfo(); fwErr == nil || fw.DongleRaw != 0 {
		snap.Firmware = &fw
	}
	if errors.Is(err, ErrMouseAsleep) {
		return snap, ErrMouseAsleep
	}

	if dpi, dpiErr := c.ReadDPIConfig(); dpiErr == nil {
		snap.DPI = &dpi
	}
	if sensor, sErr := c.ReadSensorConfig(); sErr == nil {
		snap.Sensor = &sensor
	}
	return snap, nil
}

// ============================================================================
// Mutating Configuration Commands (For CLI / Future Wails Settings UI)
// ============================================================================

func (c *Client) writeMemory(addr uint32, data []byte) error {
	c.cmdMu.Lock()
	op := c.opcodes.WriteMemory
	c.cmdMu.Unlock()

	tx, err := BuildWriteMemory(c.endpoint.Model.TargetByte, op, addr, data)
	if err != nil {
		return err
	}
	resp, err := c.sendAndRecv(tx)
	if err != nil {
		return err
	}
	if resp[0] == StatusPending {
		return ErrMouseAsleep
	}
	if resp[0] != StatusOK {
		return fmt.Errorf("%w: memory write status %#02x", ErrInvalidResponse, resp[0])
	}
	return nil
}

// SetPollingRate updates the mouse polling rate (125, 250, 500, 1000, 2000, 4000 Hz)
// via read-modify-write at 0x0880, preserving adjacent bytes 0x0881 and 0x0883.
func (c *Client) SetPollingRate(hz int) error {
	if hz > c.endpoint.Model.MaxRateHz {
		return fmt.Errorf("rapoo: %d Hz exceeds %s maximum (%d Hz)", hz, c.endpoint.Model.Name, c.endpoint.Model.MaxRateHz)
	}
	code, err := EncodePollingRate(hz)
	if err != nil {
		return err
	}
	cur, err := c.ReadMemory(AddrPollingRate, 4)
	if err != nil {
		return err
	}
	next := []byte{code, cur[1], code, cur[3]}
	return c.writeMemory(AddrPollingRate, next)
}

// SetCurrentDPIGear switches the active DPI stage (1-based: 1..7) via
// read-modify-write at 0x0898.
func (c *Client) SetCurrentDPIGear(stage int) error {
	if stage < 1 || stage > 7 {
		return fmt.Errorf("rapoo: DPI stage %d out of range (1..7)", stage)
	}
	cur, err := c.ReadMemory(AddrCurrentDPIGear, 4)
	if err != nil {
		return err
	}
	next := []byte{byte(stage - 1), cur[1], cur[2], cur[3]}
	return c.writeMemory(AddrCurrentDPIGear, next)
}

// SetMotionSync toggles sensor Motion Sync via read-modify-write at 0x0885.
func (c *Client) SetMotionSync(enabled bool) error {
	cur, err := c.ReadMemory(AddrMotionSync, 4)
	if err != nil {
		return err
	}
	var flag byte
	if enabled {
		flag = 1
	}
	next := []byte{flag, cur[1], cur[2], cur[3]}
	return c.writeMemory(AddrMotionSync, next)
}

// SetSleepMinutes updates the wireless idle sleep timeout (1..120 minutes) via
// read-modify-write at 0x08C2.
func (c *Client) SetSleepMinutes(minutes int) error {
	if minutes < 1 || minutes > 120 {
		return fmt.Errorf("rapoo: sleep timeout %d minutes out of range (1..120)", minutes)
	}
	cur, err := c.ReadMemory(AddrSleepTimeout, 4)
	if err != nil {
		return err
	}
	next := []byte{byte(minutes), cur[1], cur[2], cur[3]}
	return c.writeMemory(AddrSleepTimeout, next)
}
