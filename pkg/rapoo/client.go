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
	// OnAsyncError reports a non-timeout failure of the notification reader.
	// The service should re-enumerate and reopen the device.
	OnAsyncError func(error)
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
	ambiguous := false
	for _, pair := range pairs {
		if pair.Ambiguous {
			ambiguous = true
			continue
		}
		return OpenEndpoint(provider, pair, opts)
	}
	if ambiguous {
		return nil, ErrAmbiguousDevice
	}
	return nil, ErrDeviceNotFound
}

// OpenEndpoint opens a specific matched EndpointPair.
func OpenEndpoint(provider hid.Provider, pair EndpointPair, opts ClientOptions) (*Client, error) {
	if pair.Ambiguous {
		return nil, ErrAmbiguousDevice
	}
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
			if c.opts.OnAsyncError != nil {
				c.opts.OnAsyncError(err)
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
	return c.sendAndRecvLockedDeadline(tx, time.Time{}, true)
}

func (c *Client) sendAndRecvLockedDeadline(tx []byte, deadline time.Time, retryAfterSend bool) ([]byte, error) {
	if c.opts.PassiveOnly || c.cmdDev == nil {
		return nil, errors.New("rapoo: command channel unavailable (passive-only mode)")
	}
	if err := checkDeadline(deadline); err != nil {
		return nil, err
	}

	stepDelay := c.endpoint.Model.StepDelay
	if stepDelay <= 0 {
		stepDelay = 35 * time.Millisecond
	}

	var lastErr error
	sent := false
	for attempt := 0; attempt < c.opts.MaxCommandRetries; attempt++ {
		if err := checkDeadline(deadline); err != nil {
			if sent {
				return nil, fmt.Errorf("%w: %v", ErrResultUnknown, err)
			}
			return nil, err
		}
		if since := time.Since(c.lastCmdEnd); since < stepDelay {
			time.Sleep(stepDelay - since)
		}
		if err := checkDeadline(deadline); err != nil {
			return nil, err
		}
		sendTimeout, err := ioTimeoutForDeadline(deadline, c.opts.IOTimeout)
		if err != nil {
			return nil, err
		}

		if err := c.cmdDev.SetOutputReport(tx, sendTimeout); err != nil {
			lastErr = err
			continue
		}

		sent = true
		time.Sleep(stepDelay)
		if err := checkDeadline(deadline); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrResultUnknown, err)
		}

		for poll := 0; poll < c.opts.MaxBusyPolls; poll++ {
			if err := checkDeadline(deadline); err != nil {
				return nil, fmt.Errorf("%w: %v", ErrResultUnknown, err)
			}
			raw, err := c.cmdDev.GetInputReport(ReportIDCommand, PacketLength, sendTimeout)
			c.lastCmdEnd = time.Now()
			if err != nil {
				if sent && !retryAfterSend {
					return nil, fmt.Errorf("%w: %v", ErrResultUnknown, err)
				}
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
	if sent && !retryAfterSend {
		return nil, fmt.Errorf("%w: %v", ErrResultUnknown, lastErr)
	}
	return nil, lastErr
}

func (c *Client) sendAndRecv(tx []byte) ([]byte, error) {
	c.cmdMu.Lock()
	defer c.cmdMu.Unlock()
	return c.sendAndRecvLockedDeadline(tx, time.Time{}, true)
}

func (c *Client) sendAndRecvDeadline(tx []byte, deadline time.Time) ([]byte, error) {
	c.cmdMu.Lock()
	defer c.cmdMu.Unlock()
	return c.sendAndRecvLockedDeadline(tx, deadline, false)
}

func ioTimeoutForDeadline(deadline time.Time, fallback time.Duration) (time.Duration, error) {
	if deadline.IsZero() {
		return fallback, nil
	}
	remaining := time.Until(deadline)
	if remaining <= 0 {
		return 0, ErrDeadlineExceeded
	}
	if remaining < fallback {
		return remaining, nil
	}
	return fallback, nil
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

// ReadDPIConfig reads all 7 configured X/Y DPI stages and the active stage index.
func (c *Client) ReadDPIConfig() (DPIConfig, error) {
	memX, err := c.ReadMemory(AddrDPIStagesX, 24)
	if err != nil {
		return DPIConfig{}, err
	}
	memY, err := c.ReadMemory(AddrDPIStagesY, 24)
	if err != nil {
		return DPIConfig{}, err
	}
	return DecodeDPIConfig(memX, memY)
}

// ReadSensorConfig reads polling rate, LOD, Motion Sync, debounce, sleep timeout, and sensor flags.
func (c *Client) ReadSensorConfig() (SensorConfig, error) {
	c.cmdMu.Lock()
	genA := c.opcodes.GenADebounce
	c.cmdMu.Unlock()

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
	return DecodeSensorConfigForGeneration(mem880, mem884, mem8C0, genA)
}

// ReadButtonMappings reads all 10 physical button assignments from 0x0600..0x0637.
func (c *Client) ReadButtonMappings() ([]ButtonMapping, error) {
	b1, err := c.ReadMemory(AddrButtonBlock1, 24)
	if err != nil {
		return nil, err
	}
	b2, err := c.ReadMemory(AddrButtonBlock2, 24)
	if err != nil {
		return nil, err
	}
	b3, err := c.ReadMemory(AddrButtonBlock3, 8)
	if err != nil {
		return nil, err
	}
	full := make([]byte, 0, ButtonTableTotalSize)
	full = append(full, b1...)
	full = append(full, b2...)
	full = append(full, b3...)
	return DecodeButtonTable(full)
}

// ReadSnapshot collects battery, firmware, DPI, sensor, and 10-button mappings in one call.
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
		snap.EstimatedHours = EstimateBatteryHours(bat.Percentage, sensor.PollingRateHz)
	} else {
		snap.EstimatedHours = EstimateBatteryHours(bat.Percentage, 1000)
	}
	if buttons, bErr := c.ReadButtonMappings(); bErr == nil {
		snap.Buttons = buttons
	}
	return snap, nil
}

// ============================================================================
// Mutating Configuration Commands
// ============================================================================

func checkDeadline(deadline time.Time) error {
	if !deadline.IsZero() && time.Now().After(deadline) {
		return ErrDeadlineExceeded
	}
	return nil
}

func (c *Client) writeMemory(addr uint32, data []byte) error {
	return c.writeMemoryDeadline(addr, data, time.Time{})
}

func (c *Client) writeMemoryDeadline(addr uint32, data []byte, deadline time.Time) error {
	if err := checkDeadline(deadline); err != nil {
		return err
	}
	c.cmdMu.Lock()
	op := c.opcodes.WriteMemory
	c.cmdMu.Unlock()

	tx, err := BuildWriteMemory(c.endpoint.Model.TargetByte, op, addr, data)
	if err != nil {
		return err
	}
	resp, err := c.sendAndRecvDeadline(tx, deadline)
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
	return c.SetPollingRateDeadline(hz, time.Time{})
}

// SetPollingRateDeadline is SetPollingRate with a deadline checked before the
// write, after the read-modify-write read.
func (c *Client) SetPollingRateDeadline(hz int, deadline time.Time) error {
	if err := checkDeadline(deadline); err != nil {
		return err
	}
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
	if err := checkDeadline(deadline); err != nil {
		return err
	}
	wiredCode := cur[2]
	if c.endpoint.Model.Connection == ConnectionWired {
		wiredCode = code
	}
	next := []byte{code, cur[1], wiredCode, cur[3]}
	return c.writeMemoryDeadline(AddrPollingRate, next, deadline)
}

// SetCurrentDPIGear switches the active DPI stage (1-based: 1..7) via
// read-modify-write at 0x0898.
func (c *Client) SetCurrentDPIGear(stage int) error {
	return c.SetCurrentDPIGearDeadline(stage, time.Time{})
}

// SetCurrentDPIGearDeadline is SetCurrentDPIGear with a deadline checked before
// the write.
func (c *Client) SetCurrentDPIGearDeadline(stage int, deadline time.Time) error {
	if err := checkDeadline(deadline); err != nil {
		return err
	}
	if stage < 1 || stage > MaxDPIStage {
		return fmt.Errorf("rapoo: DPI stage %d out of range (1..7)", stage)
	}
	cur, err := c.ReadMemory(AddrCurrentDPIGear, 4)
	if err != nil {
		return err
	}
	if err := checkDeadline(deadline); err != nil {
		return err
	}
	next := []byte{byte(stage - 1), cur[1], cur[2], cur[3]}
	return c.writeMemoryDeadline(AddrCurrentDPIGear, next, deadline)
}

// SetDPIConfig writes all 7 X/Y DPI stages, stage count (1..7), and active DPI stage (1..stageCount).
func (c *Client) SetDPIConfig(cfg DPIConfig) error {
	return c.SetDPIConfigDeadline(cfg, time.Time{})
}

// SetDPIConfigDeadline is SetDPIConfig with a deadline between the X, Y, and
// active-stage writes. A failure after a write returns a PartialError.
func (c *Client) SetDPIConfigDeadline(cfg DPIConfig, deadline time.Time) error {
	if err := checkDeadline(deadline); err != nil {
		return err
	}
	stageCount := cfg.StageCount
	if stageCount < 1 || stageCount > MaxDPIStage {
		return fmt.Errorf("rapoo: DPI stageCount %d out of range (1..7)", stageCount)
	}
	curStage := cfg.CurrentStage
	if curStage < 1 || curStage > stageCount {
		curStage = 1
	}

	srcStages := cfg.AllStages
	if len(srcStages) < stageCount {
		srcStages = cfg.Stages
	}
	// Preserve any trailing onboard slots (> len(srcStages)) by reading current slots if needed.
	if len(srcStages) < MaxDPIStage {
		if existing, err := c.ReadDPIConfig(); err == nil && len(existing.AllStages) == MaxDPIStage {
			merged := make([]DPIStage, MaxDPIStage)
			copy(merged, existing.AllStages)
			copy(merged, srcStages)
			srcStages = merged
		}
	}

	// Encode and validate the whole payload before the first write.
	blockX, blockY, err := EncodeDPIBlocks(srcStages, stageCount)
	if err != nil {
		return err
	}

	var applied []string
	if err := c.writeMemoryDeadline(AddrDPIStagesX, blockX[:], deadline); err != nil {
		return err
	}
	applied = append(applied, "dpiX")
	if err := checkDeadline(deadline); err != nil {
		return &PartialError{Applied: applied, Err: err}
	}
	if err := c.writeMemoryDeadline(AddrDPIStagesY, blockY[:], deadline); err != nil {
		return &PartialError{Applied: applied, Err: err}
	}
	applied = append(applied, "dpiY")
	if err := checkDeadline(deadline); err != nil {
		return &PartialError{Applied: applied, Err: err}
	}
	if err := c.SetCurrentDPIGearDeadline(curStage, deadline); err != nil {
		return &PartialError{Applied: applied, Err: err}
	}
	return nil
}

// SetMotionSync toggles sensor Motion Sync via 4-byte aligned read-modify-write at 0x0884 (mem[0x0885]).
func (c *Client) SetMotionSync(enabled bool) error {
	cur, err := c.ReadMemory(AddrSilentHeight, 4)
	if err != nil {
		return err
	}
	var flag byte
	if enabled {
		flag = 1
	}
	next := []byte{cur[0], flag, cur[2], cur[3]}
	return c.writeMemory(AddrSilentHeight, next)
}

// SetLiftOffLevel updates Lift-Off Distance level (0..10 corresponding to 1.0mm..2.0mm in 0.1mm steps).
func (c *Client) SetLiftOffLevel(level int) error {
	if level < 0 || level > 10 {
		return fmt.Errorf("rapoo: lift-off level %d out of range (0..10)", level)
	}
	cur, err := c.ReadMemory(AddrSilentHeight, 4)
	if err != nil {
		return err
	}
	next := []byte{byte(level + 1), cur[1], cur[2], cur[3]}
	return c.writeMemory(AddrSilentHeight, next)
}

// SetSleepMinutes updates the wireless idle sleep timeout (2..120 minutes) via
// 8-byte aligned read-modify-write at 0x08C0 (mem[0x08C2]).
func (c *Client) SetSleepMinutes(minutes int) error {
	if minutes < 2 || minutes > 120 {
		return fmt.Errorf("rapoo: sleep timeout %d minutes out of range (2..120)", minutes)
	}
	return c.UpdateSensorConfig(SensorPatch{SleepMinutes: &minutes})
}

// UpdateSensorConfig atomically applies one or more SensorPatch fields to 0x0884 and/or 0x08C0.
func (c *Client) UpdateSensorConfig(patch SensorPatch) error {
	return c.UpdateSensorConfigDeadline(patch, time.Time{})
}

// UpdateSensorConfigDeadline validates the entire patch before the first write
// and checks the deadline between the two memory blocks. A failure after the
// first block returns a PartialError listing what was applied.
func (c *Client) UpdateSensorConfigDeadline(patch SensorPatch, deadline time.Time) error {
	if err := checkDeadline(deadline); err != nil {
		return err
	}
	c.cmdMu.Lock()
	genA := c.opcodes.GenADebounce
	c.cmdMu.Unlock()

	// Validate every field before touching the device. This prevents an invalid
	// trailing field (for example sleep=0) from leaving earlier fields applied.
	var pressRaw, releaseRaw byte
	if patch.PressDebounceStep != nil {
		if genA {
			raw, err := EncodeGenADebounceStep(*patch.PressDebounceStep)
			if err != nil {
				return err
			}
			pressRaw = raw
		} else {
			ms := *patch.PressDebounceStep
			if ms < 0 || ms > 32 {
				return fmt.Errorf("rapoo: press debounce %d ms out of range (0..32)", ms)
			}
			pressRaw = byte(ms + 2)
		}
	}
	if patch.ReleaseDebounceStep != nil {
		if genA {
			raw, err := EncodeGenADebounceStep(*patch.ReleaseDebounceStep)
			if err != nil {
				return err
			}
			releaseRaw = raw
		} else {
			ms := *patch.ReleaseDebounceStep
			if ms < 0 || ms > 32 {
				return fmt.Errorf("rapoo: release debounce %d ms out of range (0..32)", ms)
			}
			releaseRaw = byte(ms + 2)
		}
	}
	if patch.LiftOffLevel != nil && (*patch.LiftOffLevel < 0 || *patch.LiftOffLevel > 10) {
		return fmt.Errorf("rapoo: lift-off level %d out of range (0..10)", *patch.LiftOffLevel)
	}
	if patch.SleepMinutes != nil && (*patch.SleepMinutes < 2 || *patch.SleepMinutes > 120) {
		return fmt.Errorf("rapoo: sleep timeout %d minutes out of range (2..120)", *patch.SleepMinutes)
	}
	if patch.SensorAngleDeg != nil && (*patch.SensorAngleDeg < -30 || *patch.SensorAngleDeg > 30) {
		return fmt.Errorf("rapoo: sensor angle %d° out of range (-30..+30)", *patch.SensorAngleDeg)
	}
	if patch.LEDLowPowerMode != nil && *patch.LEDLowPowerMode > 1 {
		return fmt.Errorf("rapoo: LED low-power mode %d out of range (0..1)", *patch.LEDLowPowerMode)
	}

	has884 := patch.LiftOffLevel != nil || patch.MotionSync != nil
	has8C0 := patch.PressDebounceStep != nil || patch.ReleaseDebounceStep != nil ||
		patch.SleepMinutes != nil || patch.RippleControl != nil ||
		patch.LinearCorrection != nil || patch.SensorAngleDeg != nil ||
		patch.GlassMode != nil || patch.LEDLowPowerMode != nil
	if !has884 && !has8C0 {
		return nil
	}

	var applied []string
	if has884 {
		if err := checkDeadline(deadline); err != nil {
			return err
		}
		cur884, err := c.ReadMemory(AddrSilentHeight, 4)
		if err != nil {
			return err
		}
		next884 := append([]byte(nil), cur884...)
		if patch.LiftOffLevel != nil {
			next884[0] = byte(*patch.LiftOffLevel + 1)
		}
		if patch.MotionSync != nil {
			if *patch.MotionSync {
				next884[1] = 1
			} else {
				next884[1] = 0
			}
		}
		if err := checkDeadline(deadline); err != nil {
			return err
		}
		if err := c.writeMemoryDeadline(AddrSilentHeight, next884, deadline); err != nil {
			return err
		}
		applied = append(applied, "sensor884")
	}

	if has8C0 {
		if err := checkDeadline(deadline); err != nil {
			return &PartialError{Applied: applied, Err: err}
		}
		cur8C0, err := c.ReadMemory(AddrSensorParameters, 8)
		if err != nil {
			if len(applied) > 0 {
				return &PartialError{Applied: applied, Err: err}
			}
			return err
		}
		next8C0 := append([]byte(nil), cur8C0...)
		if patch.PressDebounceStep != nil {
			next8C0[0] = pressRaw
		}
		if patch.ReleaseDebounceStep != nil {
			next8C0[1] = releaseRaw
		}
		if patch.SleepMinutes != nil {
			next8C0[2] = byte(*patch.SleepMinutes)
		}
		if patch.RippleControl != nil || patch.LinearCorrection != nil {
			curFlags := next8C0[3]
			ripple := ((^curFlags) & 0x01) != 0
			linear := (((curFlags >> 1) ^ 0x01) & 0x01) != 0
			if patch.RippleControl != nil {
				ripple = *patch.RippleControl
			}
			if patch.LinearCorrection != nil {
				linear = *patch.LinearCorrection
			}
			next8C0[3] = EncodeRippleLinearByte(ripple, linear)
		}
		if patch.SensorAngleDeg != nil {
			next8C0[4] = byte(int8(*patch.SensorAngleDeg))
		}
		if patch.GlassMode != nil {
			if *patch.GlassMode {
				next8C0[5] = 1
			} else {
				next8C0[5] = 0
			}
		}
		if patch.LEDLowPowerMode != nil {
			next8C0[6] = *patch.LEDLowPowerMode
		}
		if err := checkDeadline(deadline); err != nil {
			return &PartialError{Applied: applied, Err: err}
		}
		if err := c.writeMemoryDeadline(AddrSensorParameters, next8C0, deadline); err != nil {
			if len(applied) == 0 {
				return err
			}
			return &PartialError{Applied: applied, Err: err}
		}
	}

	return nil
}

// SetButtonMapping updates a single physical button's onboard action, enforcing
// the Left-Click safety guard so at least one clickable button retains Left-Click.
func (c *Client) SetButtonMapping(buttonID ButtonID, action ButtonAction) error {
	return c.SetButtonMappingDeadline(buttonID, action, time.Time{})
}

// SetButtonMappingDeadline is SetButtonMapping with a deadline checked before
// the write.
func (c *Client) SetButtonMappingDeadline(buttonID ButtonID, action ButtonAction, deadline time.Time) error {
	if err := checkDeadline(deadline); err != nil {
		return err
	}
	slot, ok := LookupButtonSlot(buttonID)
	if !ok {
		return fmt.Errorf("rapoo: unknown button ID %d", buttonID)
	}
	raw, err := EncodeButtonAction(action)
	if err != nil {
		return err
	}

	// Read current table to enforce Left-Click safety guard before mutating onboard memory.
	current, err := c.ReadButtonMappings()
	if err != nil {
		return err
	}
	for i := range current {
		if current[i].Slot.ID == buttonID {
			current[i].Action = DecodeButtonAction(slot, raw)
		}
	}
	if err := ValidateButtonMappings(current); err != nil {
		return err
	}
	if err := checkDeadline(deadline); err != nil {
		return err
	}

	return c.writeMemory(slot.Addr, raw[:])
}

// SetAllButtonMappings validates and writes the changed button mappings.
// New Left Click assignments are written first so a transient failure cannot
// leave the user without any clickable button. Unchanged slots are not
// re-encoded, which preserves onboard macros authored by the official tool.
func (c *Client) SetAllButtonMappings(mappings []ButtonMapping) error {
	return c.SetAllButtonMappingsDeadline(mappings, time.Time{})
}

// SetAllButtonMappingsDeadline is SetAllButtonMappings with a deadline between
// slot writes. A failure after a write returns a PartialError.
func (c *Client) SetAllButtonMappingsDeadline(mappings []ButtonMapping, deadline time.Time) error {
	if err := checkDeadline(deadline); err != nil {
		return err
	}
	if err := ValidateButtonMappings(mappings); err != nil {
		return err
	}

	current, err := c.ReadButtonMappings()
	if err != nil {
		return err
	}
	currentById := make(map[ButtonID]ButtonMapping, len(current))
	for _, m := range current {
		currentById[m.Slot.ID] = m
	}

	// Only touch slots whose action actually changes. This avoids re-encoding
	// an onboard macro that the official tool wrote into an unchanged slot.
	changed := make([]ButtonMapping, 0, len(mappings))
	encoded := make(map[ButtonID][4]byte, len(mappings))
	for _, m := range mappings {
		if buttonActionsEqual(m.Action, currentById[m.Slot.ID].Action) {
			continue
		}
		raw, err := EncodeButtonAction(m.Action)
		if err != nil {
			return err
		}
		changed = append(changed, m)
		encoded[m.Slot.ID] = raw
	}
	if len(changed) == 0 {
		return nil
	}

	var applied []string
	write := func(m ButtonMapping) error {
		if err := checkDeadline(deadline); err != nil {
			if len(applied) == 0 {
				return err
			}
			return &PartialError{Applied: applied, Err: err}
		}
		slot, _ := LookupButtonSlot(m.Slot.ID)
		raw := encoded[m.Slot.ID]
		if err := c.writeMemoryDeadline(slot.Addr, raw[:], deadline); err != nil {
			if len(applied) == 0 {
				return err
			}
			return &PartialError{Applied: applied, Err: err}
		}
		applied = append(applied, fmt.Sprintf("button %d", m.Slot.ID))
		return nil
	}

	// Write new Left Click targets before removing any existing Left Click.
	for _, m := range changed {
		if m.Action.Type != ActionLeftClick {
			continue
		}
		if currentById[m.Slot.ID].Action.Type == ActionLeftClick {
			continue
		}
		if err := write(m); err != nil {
			return err
		}
	}
	for _, m := range changed {
		if m.Action.Type == ActionLeftClick && currentById[m.Slot.ID].Action.Type != ActionLeftClick {
			continue
		}
		if err := write(m); err != nil {
			return err
		}
	}

	verify, err := c.ReadButtonMappings()
	if err != nil {
		return &PartialError{Applied: applied, Err: fmt.Errorf("rapoo: verify button mappings: %w", err)}
	}
	verifyById := make(map[ButtonID]ButtonMapping, len(verify))
	for _, m := range verify {
		verifyById[m.Slot.ID] = m
	}
	for _, expected := range changed {
		actual, ok := verifyById[expected.Slot.ID]
		if !ok || !buttonActionsEqual(expected.Action, actual.Action) {
			return &PartialError{
				Applied: applied,
				Err:     fmt.Errorf("rapoo: button %d did not match after write", expected.Slot.ID),
			}
		}
	}
	return nil
}

// ButtonActionsEqual reports whether two button actions are semantically equal.
func ButtonActionsEqual(a, b ButtonAction) bool {
	return buttonActionsEqual(a, b)
}

func buttonActionsEqual(a, b ButtonAction) bool {
	if a.Type != b.Type {
		return false
	}
	switch a.Type {
	case ActionKeyboard:
		return a.KeyCode == b.KeyCode && a.Modifiers.Mask() == b.Modifiers.Mask()
	case ActionUnknown:
		if len(a.Raw) != len(b.Raw) {
			return false
		}
		for i := range a.Raw {
			if a.Raw[i] != b.Raw[i] {
				return false
			}
		}
	}
	return true
}

// FactoryReset sends the hardware factory reset command (0xAD 0x52 0x3D on Gen-A / 0xD0 on Gen-B).
func (c *Client) FactoryReset() error {
	return c.FactoryResetDeadline(time.Time{})
}

// FactoryResetDeadline is FactoryReset with a deadline checked before sending.
func (c *Client) FactoryResetDeadline(deadline time.Time) error {
	if err := checkDeadline(deadline); err != nil {
		return err
	}
	c.cmdMu.Lock()
	ops := c.opcodes
	c.cmdMu.Unlock()

	tx := BuildFactoryReset(c.endpoint.Model.TargetByte, ops)
	resp, err := c.sendAndRecvDeadline(tx, deadline)
	if err != nil {
		return err
	}
	if resp[0] == StatusPending {
		return ErrMouseAsleep
	}
	if resp[0] != StatusOK {
		return fmt.Errorf("%w: factory reset status %#02x", ErrInvalidResponse, resp[0])
	}
	return nil
}
