// Command rapoo-bridge exposes the Rapoo device service to the WinUI app over
// newline-delimited JSON on stdin/stdout. The UI spawns this process as a
// sidecar, so HID ownership stays in Go and the UI never touches raw reports.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/nico/rapoo-vt9pro-4k/internal/service"
	"github.com/nico/rapoo-vt9pro-4k/internal/updaterguard"
	"github.com/nico/rapoo-vt9pro-4k/pkg/rapoo"
)

const bridgeVersion = "1"

type request struct {
	ID       int             `json:"id"`
	Method   string          `json:"method"`
	Params   json.RawMessage `json:"params,omitempty"`
	Deadline int64           `json:"deadline,omitempty"` // Unix milliseconds
}

type response struct {
	ID     int    `json:"id"`
	Result any    `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
	Code   string `json:"code,omitempty"`
}

type event struct {
	Event string `json:"event"`
	Data  any    `json:"data,omitempty"`
}

type bridgePrefs struct {
	Mode service.PollingMode `json:"mode"`
}

type bridge struct {
	svc       *service.MouseService
	prefsPath string
	prefs     bridgePrefs

	outMu sync.Mutex
	enc   *json.Encoder
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("rapoo-bridge: ")
	log.SetOutput(os.Stderr)

	if len(os.Args) >= 5 && os.Args[1] == "--updater-guard" {
		vendor, vendorErr := strconv.Atoi(os.Args[3])
		productIDs, productErr := parseProductIDs(os.Args[4])
		if vendorErr != nil || productErr != nil {
			log.Printf("updater guard: invalid device id")
			os.Exit(1)
		}
		if err := updaterguard.Run(os.Args[2], uint16(vendor), productIDs, func() {
			fmt.Println("ready")
			_ = os.Stdout.Sync()
		}); err != nil {
			log.Printf("updater guard: %v", err)
			os.Exit(1)
		}
		return
	}

	b, err := newBridge(os.Stdout)
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		b.svc.Shutdown()
		os.Exit(0)
	}()

	if err := b.run(os.Stdin); err != nil && !errors.Is(err, io.EOF) {
		log.Printf("bridge stopped: %v", err)
	}
	b.svc.Shutdown()
}

func parseProductIDs(value string) ([]uint16, error) {
	parts := strings.Split(value, ",")
	ids := make([]uint16, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.Atoi(part)
		if err != nil || id < 0 || id > 0xFFFF {
			return nil, errors.New("invalid product id")
		}
		ids = append(ids, uint16(id))
	}
	if len(ids) == 0 {
		return nil, errors.New("no product id")
	}
	return ids, nil
}

func newBridge(out io.Writer) (*bridge, error) {
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		base = os.TempDir()
	}
	cfgDir := filepath.Join(base, "rapoo-vt9pro-4k")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		return nil, fmt.Errorf("create config dir: %w", err)
	}

	prefsPath := filepath.Join(cfgDir, "bridge-prefs.json")
	var prefs bridgePrefs
	if raw, err := os.ReadFile(prefsPath); err == nil {
		_ = json.Unmarshal(raw, &prefs)
	}
	// A bridge restart must not silently leave the device in passive mode.
	// Only a user-selected low-frequency mode is persisted.
	if prefs.Mode != service.ModeLowFrequency {
		prefs.Mode = service.ModeNormal
	}

	svc := service.NewMouseService(nil, service.Config{
		InitialMode: prefs.Mode,
		Language:    os.Getenv("RAPOO_LANG"),
		PresetDir:   cfgDir,
		StartInDemo: os.Getenv("RAPOO_DEMO") == "1",
	})

	b := &bridge{svc: svc, prefsPath: prefsPath, prefs: prefs, enc: json.NewEncoder(out)}
	svc.Startup(context.Background())
	if os.Getenv("RAPOO_DEMO") == "1" {
		svc.EnsureDemoReady()
	}
	svc.Subscribe(func(state service.State) {
		b.emit("state", state)
	})
	b.emit("ready", map[string]any{
		"version":  bridgeVersion,
		"platform": runtime.GOOS,
	})
	return b, nil
}

func (b *bridge) run(in io.Reader) error {
	dec := json.NewDecoder(in)
	for {
		var req request
		if err := dec.Decode(&req); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		if req.Method == "" {
			b.respond(response{ID: req.ID, Error: "empty method", Code: "invalid_request"})
			continue
		}
		if req.Deadline > 0 && time.Now().UnixMilli() > req.Deadline {
			b.respond(response{
				ID:    req.ID,
				Error: "request expired before execution",
				Code:  "deadline_exceeded",
			})
			continue
		}

		result, err := b.handle(req)
		if err != nil {
			b.respond(response{ID: req.ID, Error: err.Error(), Code: bridgeErrorCode(err)})
		} else {
			b.respond(response{ID: req.ID, Result: result})
		}
		if req.Method == "shutdown" {
			return nil
		}
	}
}

func (b *bridge) handle(req request) (any, error) {
	switch req.Method {
	case "state":
		return b.svc.GetState(), nil
	case "catalog":
		return b.svc.GetCatalog(), nil
	case "snapshot":
		return b.svc.ReadSnapshot()
	case "refresh":
		return b.svc.RefreshSnapshot(requestDeadline(req))
	case "demo":
		var p struct {
			Enabled bool `json:"enabled"`
		}
		if err := decodeParams(req, &p); err != nil {
			return nil, err
		}
		return b.svc.SetDemoMode(p.Enabled)
	case "setLanguage":
		var p struct {
			Language string `json:"language"`
		}
		if err := decodeParams(req, &p); err != nil {
			return nil, err
		}
		b.svc.SetLanguage(p.Language)
		return b.svc.GetState(), nil
	case "pollingMode":
		var p struct {
			Mode string `json:"mode"`
		}
		if err := decodeParams(req, &p); err != nil {
			return nil, err
		}
		mode := service.PollingMode(p.Mode)
		switch mode {
		case service.ModeNormal, service.ModeLowFrequency, service.ModePassiveOnly:
		default:
			return nil, fmt.Errorf("unknown polling mode %q", p.Mode)
		}
		if b.svc.UpdaterActive() && mode != service.ModePassiveOnly {
			return nil, errors.New("firmware update in progress")
		}
		b.svc.SetPollingMode(mode)
		b.setMode(mode)
		return b.svc.GetState(), nil
	case "pollingRate":
		var p struct {
			Hz         int    `json:"hz"`
			Generation uint64 `json:"generation"`
		}
		if err := decodeParams(req, &p); err != nil {
			return nil, err
		}
		return b.svc.SetPollingRateForGeneration(p.Hz, p.Generation, requestDeadline(req))
	case "dpiGear":
		var p struct {
			Stage      int    `json:"stage"`
			Generation uint64 `json:"generation"`
		}
		if err := decodeParams(req, &p); err != nil {
			return nil, err
		}
		return b.svc.SetCurrentDPIGearForGeneration(p.Stage, p.Generation, requestDeadline(req))
	case "dpiConfig":
		var p struct {
			Config     rapoo.DPIConfig `json:"config"`
			Generation uint64          `json:"generation"`
		}
		if err := decodeParams(req, &p); err != nil {
			return nil, err
		}
		return b.svc.SetDPIConfigForGeneration(p.Config, p.Generation, requestDeadline(req))
	case "sensor":
		var p struct {
			Patch      rapoo.SensorPatch `json:"patch"`
			Generation uint64            `json:"generation"`
		}
		if err := decodeParams(req, &p); err != nil {
			return nil, err
		}
		return b.svc.UpdateSensorConfigForGeneration(p.Patch, p.Generation, requestDeadline(req))
	case "button":
		var p struct {
			ButtonID   int                `json:"buttonId"`
			Action     rapoo.ButtonAction `json:"action"`
			Generation uint64             `json:"generation"`
		}
		if err := decodeParams(req, &p); err != nil {
			return nil, err
		}
		return b.svc.SetButtonMappingForGeneration(p.ButtonID, p.Action, p.Generation, requestDeadline(req))
	case "resetButtons":
		var p struct {
			Generation uint64 `json:"generation"`
		}
		if len(req.Params) > 0 {
			if err := decodeParams(req, &p); err != nil {
				return nil, err
			}
		}
		return b.svc.ResetButtonsToDefaultForGeneration(p.Generation, requestDeadline(req))
	case "factoryReset":
		var p struct {
			Generation uint64 `json:"generation"`
		}
		if len(req.Params) > 0 {
			if err := decodeParams(req, &p); err != nil {
				return nil, err
			}
		}
		return b.svc.FactoryResetForGeneration(p.Generation, requestDeadline(req))
	case "presets":
		presets, err := b.svc.ListPresets()
		if err != nil {
			return nil, err
		}
		return presets, nil
	case "savePreset":
		var p struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		if err := decodeParams(req, &p); err != nil {
			return nil, err
		}
		return b.svc.SavePresetWithDeadline(p.ID, p.Name, p.Description, requestDeadline(req))
	case "applyPreset":
		var p struct {
			ID         string `json:"id"`
			Generation uint64 `json:"generation"`
		}
		if err := decodeParams(req, &p); err != nil {
			return nil, err
		}
		return b.svc.ApplyPresetForGeneration(p.ID, p.Generation, requestDeadline(req))
	case "deletePreset":
		var p struct {
			ID string `json:"id"`
		}
		if err := decodeParams(req, &p); err != nil {
			return nil, err
		}
		return nil, b.svc.DeletePreset(p.ID)
	case "exportPreset":
		var p struct {
			ID string `json:"id"`
		}
		if err := decodeParams(req, &p); err != nil {
			return nil, err
		}
		return b.svc.ExportPresetJSON(p.ID)
	case "importPreset":
		var p struct {
			RawJSON string `json:"rawJson"`
		}
		if err := decodeParams(req, &p); err != nil {
			return nil, err
		}
		return b.svc.ImportPresetJSON(p.RawJSON)
	case "openUpdater":
		var p struct {
			Path string `json:"path"`
		}
		if err := decodeParams(req, &p); err != nil {
			return nil, err
		}
		return b.openUpdater(p.Path)
	case "shutdown":
		return map[string]bool{"ok": true}, nil
	default:
		return nil, fmt.Errorf("unknown method %q", req.Method)
	}
}

func decodeParams(req request, dst any) error {
	if len(req.Params) == 0 || string(req.Params) == "null" {
		return errors.New("missing params")
	}
	if err := json.Unmarshal(req.Params, dst); err != nil {
		return fmt.Errorf("invalid params: %w", err)
	}
	return nil
}

func requestDeadline(req request) time.Time {
	if req.Deadline <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(req.Deadline)
}

func bridgeErrorCode(err error) string {
	var partial *service.PartialWriteError
	switch {
	case errors.As(err, &partial):
		return "partial"
	case errors.Is(err, service.ErrBusy):
		return "busy"
	case errors.Is(err, service.ErrDeviceChanged):
		return "device_changed"
	case errors.Is(err, service.ErrDeadlineExceeded), errors.Is(err, rapoo.ErrDeadlineExceeded):
		return "deadline_exceeded"
	case errors.Is(err, rapoo.ErrResultUnknown):
		return "result_unknown"
	default:
		return "error"
	}
}

func (b *bridge) respond(resp response) {
	b.outMu.Lock()
	defer b.outMu.Unlock()
	if err := b.enc.Encode(resp); err != nil {
		log.Printf("write response: %v", err)
	}
}

func (b *bridge) emit(name string, data any) {
	b.outMu.Lock()
	defer b.outMu.Unlock()
	if b.enc == nil {
		return
	}
	if err := b.enc.Encode(event{Event: name, Data: data}); err != nil {
		log.Printf("write event: %v", err)
	}
}

func (b *bridge) setMode(mode service.PollingMode) {
	b.prefs.Mode = mode
	if mode != service.ModeLowFrequency {
		b.prefs.Mode = service.ModeNormal
	}
	raw, err := json.MarshalIndent(b.prefs, "", "  ")
	if err != nil {
		log.Printf("encode prefs: %v", err)
		return
	}
	tmp := b.prefsPath + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		log.Printf("write prefs: %v", err)
		return
	}
	if err := os.Rename(tmp, b.prefsPath); err != nil {
		log.Printf("replace prefs: %v", err)
	}
}

func (b *bridge) openUpdater(requested string) (string, error) {
	if runtime.GOOS != "windows" {
		return "", errors.New("firmware update is only available with the official Windows software")
	}

	if requested == "" {
		return "", errors.New("select the official updater executable explicitly")
	}
	candidates := []string{requested}
	for _, path := range candidates {
		if path == "" {
			continue
		}
		if !filepath.IsAbs(path) {
			return "", errors.New("the updater path must be absolute")
		}
		switch strings.ToLower(filepath.Base(path)) {
		case "rapoogamedev.exe", "firmwareexe.exe":
		default:
			return "", errors.New("select RapooGameDev.exe or FirmwareExe.exe")
		}
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			continue
		}

		if err := verifyUpdaterSignature(path); err != nil {
			return "", err
		}
		if err := b.svc.BeginUpdater(); err != nil {
			return "", err
		}
		previous := b.svc.GetState().Mode
		b.svc.SetPollingMode(service.ModePassiveOnly)
		self, selfErr := os.Executable()
		if selfErr != nil {
			b.svc.RestorePollingMode(previous)
			b.svc.EndUpdater()
			b.svc.RefreshNow()
			return "", selfErr
		}
		productIDs := rapoo.SupportedProductIDs()
		productArgs := make([]string, len(productIDs))
		for i, id := range productIDs {
			productArgs[i] = strconv.Itoa(int(id))
		}
		cmd := exec.Command(self, "--updater-guard", path,
			strconv.Itoa(0x24AE), strings.Join(productArgs, ","))
		cmd.Dir = filepath.Dir(self)
		updaterguard.ConfigureCommand(cmd)
		stdout, pipeErr := cmd.StdoutPipe()
		if pipeErr != nil {
			b.svc.RestorePollingMode(previous)
			b.svc.EndUpdater()
			b.svc.RefreshNow()
			return "", pipeErr
		}
		if err := cmd.Start(); err != nil {
			b.svc.RestorePollingMode(previous)
			b.svc.EndUpdater()
			b.svc.RefreshNow()
			return "", err
		}
		ready := make(chan error, 1)
		go func() {
			scanner := bufio.NewScanner(stdout)
			if scanner.Scan() && strings.TrimSpace(scanner.Text()) == "ready" {
				ready <- nil
				return
			}
			ready <- errors.New("the official updater did not start")
		}()
		select {
		case err := <-ready:
			if err != nil {
				_ = cmd.Wait()
				b.svc.RestorePollingMode(previous)
				b.svc.EndUpdater()
				b.svc.RefreshNow()
				return "", err
			}
		case <-time.After(15 * time.Second):
			// Do not kill the guard here: the updater may have started just
			// before the deadline. Let it finish and release the locks.
			go func() {
				_ = cmd.Wait()
				b.svc.RestorePollingMode(previous)
				b.svc.EndUpdater()
				b.svc.RefreshNow()
			}()
			return "", errors.New("timed out waiting for the official updater")
		}
		go func() {
			_ = cmd.Wait()
			b.svc.RestorePollingMode(previous)
			b.svc.EndUpdater()
			b.svc.RefreshNow()
		}()
		return path, nil
	}
	return "", errors.New("the official Rapoo software was not found")
}
