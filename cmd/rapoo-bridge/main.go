// Command rapoo-bridge exposes the Rapoo device service to the WinUI app over
// newline-delimited JSON on stdin/stdout. The UI spawns this process as a
// sidecar, so HID ownership stays in Go and the UI never touches raw reports.
package main

import (
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
	"strings"
	"sync"
	"syscall"

	"github.com/nico/rapoo-vt9pro-4k/internal/service"
	"github.com/nico/rapoo-vt9pro-4k/pkg/rapoo"
)

const bridgeVersion = "1"

type request struct {
	ID     int             `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

type response struct {
	ID     int    `json:"id"`
	Result any    `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
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
	})

	b := &bridge{svc: svc, prefsPath: prefsPath, prefs: prefs, enc: json.NewEncoder(out)}
	svc.Startup(context.Background())
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
			b.respond(response{ID: req.ID, Error: "empty method"})
			continue
		}

		result, err := b.handle(req)
		if err != nil {
			b.respond(response{ID: req.ID, Error: err.Error()})
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
	case "demo":
		var p struct {
			Enabled bool `json:"enabled"`
		}
		if err := decodeParams(req, &p); err != nil {
			return nil, err
		}
		return b.svc.SetDemoMode(p.Enabled)
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
		b.svc.SetPollingMode(mode)
		b.setMode(mode)
		return b.svc.GetState(), nil
	case "pollingRate":
		var p struct {
			Hz int `json:"hz"`
		}
		if err := decodeParams(req, &p); err != nil {
			return nil, err
		}
		return b.svc.SetPollingRate(p.Hz)
	case "dpiGear":
		var p struct {
			Stage int `json:"stage"`
		}
		if err := decodeParams(req, &p); err != nil {
			return nil, err
		}
		return b.svc.SetCurrentDPIGear(p.Stage)
	case "dpiConfig":
		var p struct {
			Config rapoo.DPIConfig `json:"config"`
		}
		if err := decodeParams(req, &p); err != nil {
			return nil, err
		}
		return b.svc.SetDPIConfig(p.Config)
	case "sensor":
		var p struct {
			Patch rapoo.SensorPatch `json:"patch"`
		}
		if err := decodeParams(req, &p); err != nil {
			return nil, err
		}
		return b.svc.UpdateSensorConfig(p.Patch)
	case "button":
		var p struct {
			ButtonID int                `json:"buttonId"`
			Action   rapoo.ButtonAction `json:"action"`
		}
		if err := decodeParams(req, &p); err != nil {
			return nil, err
		}
		return b.svc.SetButtonMapping(p.ButtonID, p.Action)
	case "resetButtons":
		return b.svc.ResetButtonsToDefault()
	case "factoryReset":
		return b.svc.FactoryReset()
	case "presets":
		return b.svc.ListPresets(), nil
	case "savePreset":
		var p struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		if err := decodeParams(req, &p); err != nil {
			return nil, err
		}
		return b.svc.SavePreset(p.ID, p.Name, p.Description)
	case "applyPreset":
		var p struct {
			ID string `json:"id"`
		}
		if err := decodeParams(req, &p); err != nil {
			return nil, err
		}
		return b.svc.ApplyPreset(p.ID)
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

	candidates := []string{requested}
	if requested == "" {
		candidates = officialUpdaterCandidates()
	}
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

		previous := b.svc.GetState().Mode
		b.svc.SetPollingMode(service.ModePassiveOnly)
		cmd := exec.Command(path)
		cmd.Dir = filepath.Dir(path)
		if err := cmd.Start(); err != nil {
			b.svc.SetPollingMode(previous)
			return "", err
		}
		go func() {
			_ = cmd.Wait()
			b.svc.SetPollingMode(previous)
		}()
		return path, nil
	}
	return "", errors.New("the official Rapoo software was not found")
}

func officialUpdaterCandidates() []string {
	roots := []string{
		os.Getenv("ProgramFiles(x86)"),
		os.Getenv("ProgramFiles"),
		filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs"),
		os.Getenv("LOCALAPPDATA"),
		os.Getenv("APPDATA"),
	}
	seen := make(map[string]bool)
	var out []string
	for _, exe := range []string{"RapooGameDev.exe", "FirmwareExe.exe"} {
		for _, root := range roots {
			if root == "" {
				continue
			}
			path := filepath.Join(root, "Rapoo", "RapooGameDev", exe)
			if !seen[path] {
				seen[path] = true
				out = append(out, path)
			}
		}
	}
	return out
}
