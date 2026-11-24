// Command vt9ctl is a cross-platform (Windows & Linux) CLI for inspecting and
// configuring Rapoo VT9 Pro 4K mice using the reverse-engineered HID protocol.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/nico/rapoo-vt9pro-4k/internal/icon"
	"github.com/nico/rapoo-vt9pro-4k/pkg/hid"
	"github.com/nico/rapoo-vt9pro-4k/pkg/rapoo"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	sub := os.Args[1]
	args := os.Args[2:]

	var err error
	switch sub {
	case "list":
		err = runList(args)
	case "battery":
		err = runBattery(args)
	case "watch":
		err = runWatch(args)
	case "info":
		err = runInfo(args)
	case "dump-mem":
		err = runDumpMem(args)
	case "set-rate":
		err = runSetRate(args)
	case "set-dpi-stage":
		err = runSetDPIStage(args)
	case "set-motionsync":
		err = runSetMotionSync(args)
	case "set-sleep":
		err = runSetSleep(args)
	case "render-icon":
		err = runRenderIcon(args)
	case "-h", "--help", "help":
		printUsage()
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", sub)
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(`vt9ctl — Rapoo VT9 Pro 4K CLI (Windows & Linux, zero CGO)

Read-only / Passive commands:
  vt9ctl list                          List vendor HID endpoints (no packets sent)
  vt9ctl battery [-json]               Read battery percentage & charging status
  vt9ctl watch [-poll 0s]              Stream passive 0xBB/0xBD notifications
  vt9ctl info [-json]                  Read full device snapshot (FW, DPI, Sensor)
  vt9ctl render-icon -pct 85 -out a.ico Render a tray .ico file locally

Configuration commands (writes onboard memory):
  vt9ctl set-rate <125|250|500|1000|2000|4000>
  vt9ctl set-dpi-stage <1..7>
  vt9ctl set-motionsync <on|off>
  vt9ctl set-sleep <2..120>
`)
}

func runList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	asJSON := fs.Bool("json", false, "output JSON")
	_ = fs.Parse(args)

	prov := hid.NewProvider()
	infos, err := prov.Enumerate(rapoo.DefaultHIDFilter())
	if err != nil {
		return err
	}
	if *asJSON {
		return writeJSON(infos)
	}
	if len(infos) == 0 {
		fmt.Println("No Rapoo VT9 Pro 4K vendor HID endpoints found.")
		return nil
	}
	pairs := rapoo.MatchEndpoints(infos)
	for _, p := range pairs {
		fmt.Printf("%s (VID=%#04x PID=%#04x, %s)\n",
			p.Model.Name, p.Model.VendorID, p.Model.ProductID, p.Model.Connection)
		if p.Command != nil {
			fmt.Printf("  Command (0xBA, Usage=%#04x): %s\n", p.Command.Usage, p.Command.Path)
		}
		if p.Report != nil {
			fmt.Printf("  Report  (0xBB, Usage=%#04x): %s\n", p.Report.Usage, p.Report.Path)
		}
	}
	return nil
}

func runBattery(args []string) error {
	fs := flag.NewFlagSet("battery", flag.ExitOnError)
	asJSON := fs.Bool("json", false, "output JSON")
	_ = fs.Parse(args)

	client, err := rapoo.OpenFirst(nil, rapoo.ClientOptions{})
	if err != nil {
		return err
	}
	defer client.Close()

	bat, err := client.ReadBattery()
	if *asJSON {
		_ = writeJSON(bat)
		if errors.Is(err, rapoo.ErrMouseAsleep) {
			return nil
		}
		return err
	}
	if errors.Is(err, rapoo.ErrMouseAsleep) {
		fmt.Printf("%s (%s): wireless mouse is asleep / powered off\n",
			client.Model().Name, client.Model().Connection)
		return nil
	}
	if err != nil {
		return err
	}

	status := "discharging"
	if bat.Charging {
		status = "charging"
	}
	fmt.Printf("%s (%s): %d%% (%s)\n",
		client.Model().Name, client.Model().Connection, bat.Percentage, status)
	return nil
}

func runWatch(args []string) error {
	fs := flag.NewFlagSet("watch", flag.ExitOnError)
	poll := fs.Duration("poll", 0, "optional active battery poll interval (default 0 = 100% passive)")
	_ = fs.Parse(args)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	passiveOnly := *poll <= 0
	client, err := rapoo.OpenFirst(nil, rapoo.ClientOptions{
		PassiveOnly: passiveOnly,
		OnAsyncEvent: func(_ *rapoo.Client, ev rapoo.AsyncEvent) {
			switch ev.Kind {
			case rapoo.EventBattery:
				fmt.Printf("[%s] async 0xBB: online=%v pct=%d%% charging=%v\n",
					time.Now().Format(time.TimeOnly),
					ev.Battery.Online, ev.Battery.Percentage, ev.Battery.Charging)
			case rapoo.EventProfile:
				fmt.Printf("[%s] async 0xBD: profile=%d\n",
					time.Now().Format(time.TimeOnly), ev.ProfileID)
			case rapoo.EventWake:
				fmt.Printf("[%s] async 0x06 (COL06 wake/status): % x\n",
					time.Now().Format(time.TimeOnly), ev.Raw)
			}
		},
	})
	if err != nil {
		return err
	}
	defer client.Close()

	fmt.Printf("Watching %s (%s), passiveOnly=%v. Press Ctrl+C to stop.\n",
		client.Model().Name, client.Model().Connection, passiveOnly)

	if passiveOnly {
		<-ctx.Done()
		return nil
	}

	ticker := time.NewTicker(*poll)
	defer ticker.Stop()
	for {
		if bat, berr := client.ReadBattery(); berr == nil {
			fmt.Printf("[%s] poll  0xBA: pct=%d%% charging=%v\n",
				time.Now().Format(time.TimeOnly), bat.Percentage, bat.Charging)
		} else if errors.Is(berr, rapoo.ErrMouseAsleep) {
			fmt.Printf("[%s] poll  0xBA: mouse asleep\n", time.Now().Format(time.TimeOnly))
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func runInfo(args []string) error {
	fs := flag.NewFlagSet("info", flag.ExitOnError)
	asJSON := fs.Bool("json", false, "output JSON")
	_ = fs.Parse(args)

	client, err := rapoo.OpenFirst(nil, rapoo.ClientOptions{})
	if err != nil {
		return err
	}
	defer client.Close()

	snap, err := client.ReadSnapshot()
	if err != nil && !errors.Is(err, rapoo.ErrMouseAsleep) {
		return err
	}
	if *asJSON {
		return writeJSON(snap)
	}

	fmt.Printf("Model:      %s (%s, PID=%#04x)\n", snap.Model.Name, snap.Model.Connection, snap.Model.ProductID)
	if snap.Firmware != nil {
		if snap.Firmware.MouseRaw != 0 {
			fmt.Printf("Mouse FW:   %s (raw %d)\n", snap.Firmware.MouseString, snap.Firmware.MouseRaw)
		}
		if snap.Firmware.DongleRaw != 0 {
			fmt.Printf("Dongle FW:  %s (raw %d)\n", snap.Firmware.DongleStr, snap.Firmware.DongleRaw)
		}
	}
	if snap.Battery.Sleeping {
		fmt.Println("Battery:    wireless mouse asleep")
		return nil
	}
	fmt.Printf("Battery:    %d%% (charging=%v, est. ~%.1f h)\n", snap.Battery.Percentage, snap.Battery.Charging, snap.EstimatedHours)

	if snap.DPI != nil {
		fmt.Printf("DPI Stages: active=%d/%d (independentXY=%v)\n", snap.DPI.CurrentStage, snap.DPI.StageCount, snap.DPI.IndependentXY)
		for _, st := range snap.DPI.Stages {
			mark := " "
			if st.Index == snap.DPI.CurrentStage {
				mark = "*"
			}
			fmt.Printf("  %s Stage %d: %d x %d DPI\n", mark, st.Index, st.X, st.Y)
		}
	}
	if snap.Sensor != nil {
		s := snap.Sensor
		fmt.Printf("Polling:    %d Hz\n", s.PollingRateHz)
		fmt.Printf("Sensor:     LOD=%.1fmm (lvl %d), MotionSync=%v, Ripple=%v, Linear=%v, Angle=%d°, Glass=%v\n",
			s.LiftOffDistanceMm, s.LiftOffLevel, s.MotionSync, s.RippleControl, s.LinearCorrection, s.SensorAngleDeg, s.GlassMode)
		fmt.Printf("Debounce:   press=%s, release=%s | Sleep=%d min\n",
			s.PressDebounceLabel, s.ReleaseDebounceLabel, s.SleepMinutes)
	}
	if len(snap.Buttons) > 0 {
		fmt.Println("Buttons:")
		for _, bm := range snap.Buttons {
			fmt.Printf("  [Key %2d | 0x%04X] %-24s -> %s (%s)\n",
				bm.Slot.ID, bm.Slot.Addr, bm.Slot.NameJA, bm.Action.LabelJA, bm.Action.Type)
		}
	}
	return nil
}

func runDumpMem(args []string) error {
	if len(args) != 2 {
		return errors.New("usage: vt9ctl dump-mem <hex_or_dec_addr> <byte_len>")
	}
	addr, err := strconv.ParseUint(args[0], 0, 32)
	if err != nil {
		return fmt.Errorf("invalid address: %w", err)
	}
	total, err := strconv.Atoi(args[1])
	if err != nil || total <= 0 || total > 512 {
		return errors.New("byte_len must be between 1 and 512")
	}

	client, err := rapoo.OpenFirst(nil, rapoo.ClientOptions{})
	if err != nil {
		return err
	}
	defer client.Close()

	// Ensure opcode generation is auto-detected first via ReadBattery (read-only).
	if _, berr := client.ReadBattery(); berr != nil && !errors.Is(berr, rapoo.ErrMouseAsleep) {
		return berr
	}

	cur := uint32(addr)
	rem := total
	for rem > 0 {
		chunk := rem
		if chunk > 24 {
			chunk = 24
		}
		buf, rerr := client.ReadMemory(cur, byte(chunk))
		if rerr != nil {
			return rerr
		}
		for off := 0; off < len(buf); off += 16 {
			end := off + 16
			if end > len(buf) {
				end = len(buf)
			}
			fmt.Printf("0x%04X:", cur+uint32(off))
			for _, b := range buf[off:end] {
				fmt.Printf(" %02X", b)
			}
			fmt.Println()
		}
		cur += uint32(chunk)
		rem -= chunk
	}
	return nil
}

func runSetRate(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: vt9ctl set-rate <125|250|500|1000|2000|4000>")
	}
	hz, err := strconv.Atoi(args[0])
	if err != nil {
		return err
	}
	client, err := rapoo.OpenFirst(nil, rapoo.ClientOptions{})
	if err != nil {
		return err
	}
	defer client.Close()
	if err := client.SetPollingRate(hz); err != nil {
		return err
	}
	fmt.Printf("Updated polling rate to %d Hz.\n", hz)
	return nil
}

func runSetDPIStage(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: vt9ctl set-dpi-stage <1..7>")
	}
	stage, err := strconv.Atoi(args[0])
	if err != nil {
		return err
	}
	client, err := rapoo.OpenFirst(nil, rapoo.ClientOptions{})
	if err != nil {
		return err
	}
	defer client.Close()
	if err := client.SetCurrentDPIGear(stage); err != nil {
		return err
	}
	fmt.Printf("Switched active DPI stage to %d.\n", stage)
	return nil
}

func runSetMotionSync(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: vt9ctl set-motionsync <on|off>")
	}
	var enable bool
	switch strings.ToLower(args[0]) {
	case "on", "1", "true":
		enable = true
	case "off", "0", "false":
		enable = false
	default:
		return errors.New("expected 'on' or 'off'")
	}
	client, err := rapoo.OpenFirst(nil, rapoo.ClientOptions{})
	if err != nil {
		return err
	}
	defer client.Close()
	if err := client.SetMotionSync(enable); err != nil {
		return err
	}
	fmt.Printf("Updated Motion Sync to %v.\n", enable)
	return nil
}

func runSetSleep(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: vt9ctl set-sleep <2..120>")
	}
	mins, err := strconv.Atoi(args[0])
	if err != nil {
		return err
	}
	if mins < 2 || mins > 120 {
		return errors.New("sleep timeout must be 2..120 minutes")
	}
	client, err := rapoo.OpenFirst(nil, rapoo.ClientOptions{})
	if err != nil {
		return err
	}
	defer client.Close()
	if err := client.SetSleepMinutes(mins); err != nil {
		return err
	}
	fmt.Printf("Updated wireless sleep timeout to %d minutes.\n", mins)
	return nil
}

func runRenderIcon(args []string) error {
	fs := flag.NewFlagSet("render-icon", flag.ExitOnError)
	pct := fs.Int("pct", 85, "battery percentage (0..100)")
	charging := fs.Bool("charging", false, "charging state")
	sleeping := fs.Bool("sleeping", false, "sleeping state")
	offline := fs.Bool("offline", false, "disconnected state")
	out := fs.String("out", "vt9tray-preview.ico", "output .ico path")
	_ = fs.Parse(args)

	ico := icon.RenderICO(icon.VisualSpec{
		Connected:  !*offline,
		HasBattery: !*offline,
		Percentage: *pct,
		Charging:   *charging,
		Sleeping:   *sleeping,
	}, 16, 20, 24, 32)
	if err := os.WriteFile(*out, ico, 0o644); err != nil {
		return err
	}
	fmt.Printf("Wrote %d bytes to %s\n", len(ico), *out)
	return nil
}

func writeJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
