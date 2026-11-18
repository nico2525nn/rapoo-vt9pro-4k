// Package profile manages named mouse configuration presets stored on disk,
// built-in gaming/desktop templates, and JSON import/export.
package profile

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/nico/rapoo-vt9pro-4k/pkg/rapoo"
)

// Preset captures a complete portable Rapoo VT9 Pro 4K configuration.
type Preset struct {
	ID          string                `json:"id"`
	Name        string                `json:"name"`
	Description string                `json:"description,omitempty"`
	BuiltIn     bool                  `json:"builtIn"`
	CreatedAt   time.Time             `json:"createdAt"`
	UpdatedAt   time.Time             `json:"updatedAt"`
	DPI         rapoo.DPIConfig       `json:"dpi"`
	PollingHz   int                   `json:"pollingHz"`
	Sensor      rapoo.SensorPatch     `json:"sensor"`
	Buttons     []rapoo.ButtonMapping `json:"buttons,omitempty"`
}

// Store persists user Presets in a JSON file inside dir.
type Store struct {
	mu       sync.RWMutex
	filePath string
	user     map[string]Preset
}

// NewStore opens or initializes a preset Store in configDir (defaults to os.UserConfigDir()/rapoo-vt9pro-4k).
func NewStore(configDir string) (*Store, error) {
	if configDir == "" {
		base, err := os.UserConfigDir()
		if err != nil || base == "" {
			base = os.TempDir()
		}
		configDir = filepath.Join(base, "rapoo-vt9pro-4k")
	}
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return nil, err
	}
	s := &Store{
		filePath: filepath.Join(configDir, "presets.json"),
		user:     make(map[string]Preset),
	}
	_ = s.load()
	return s, nil
}

func (s *Store) load() error {
	raw, err := os.ReadFile(s.filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	var list []Preset
	if err := json.Unmarshal(raw, &list); err != nil {
		return err
	}
	for _, p := range list {
		if p.ID != "" && !p.BuiltIn {
			s.user[p.ID] = p
		}
	}
	return nil
}

func (s *Store) saveLocked() error {
	list := make([]Preset, 0, len(s.user))
	for _, p := range s.user {
		list = append(list, p)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt.Before(list[j].CreatedAt)
	})
	raw, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.filePath + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.filePath)
}

// List returns built-in starter presets followed by user-saved presets.
func (s *Store) List() []Preset {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := BuiltInPresets()
	userList := make([]Preset, 0, len(s.user))
	for _, p := range s.user {
		userList = append(userList, p)
	}
	sort.Slice(userList, func(i, j int) bool {
		return userList[i].UpdatedAt.After(userList[j].UpdatedAt)
	})
	return append(out, userList...)
}

// Get returns a built-in or user preset by ID.
func (s *Store) Get(id string) (Preset, bool) {
	for _, b := range BuiltInPresets() {
		if b.ID == id {
			return b, true
		}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.user[id]
	return p, ok
}

// SaveSnapshot captures a live DeviceSnapshot into a named user Preset.
func (s *Store) SaveSnapshot(id, name, description string, snap rapoo.DeviceSnapshot) (Preset, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Preset{}, errors.New("プロファイル名を入力してください")
	}
	if snap.DPI == nil || snap.Sensor == nil {
		return Preset{}, errors.New("マウスの設定情報がまだ読み込まれていません")
	}

	now := time.Now().UTC()
	if id == "" || strings.HasPrefix(id, "builtin-") {
		id = fmt.Sprintf("preset-%d", now.UnixNano())
	}

	sc := snap.Sensor
	lod := sc.LiftOffLevel
	ms := sc.MotionSync
	pStep := sc.PressDebounceStep
	rStep := sc.ReleaseDebounceStep
	sleep := sc.SleepMinutes
	rip := sc.RippleControl
	lin := sc.LinearCorrection
	ang := int(sc.SensorAngleDeg)
	glass := sc.GlassMode

	preset := Preset{
		ID:          id,
		Name:        name,
		Description: strings.TrimSpace(description),
		BuiltIn:     false,
		CreatedAt:   now,
		UpdatedAt:   now,
		DPI:         *snap.DPI,
		PollingHz:   sc.PollingRateHz,
		Sensor: rapoo.SensorPatch{
			LiftOffLevel:        &lod,
			MotionSync:          &ms,
			PressDebounceStep:   &pStep,
			ReleaseDebounceStep: &rStep,
			SleepMinutes:        &sleep,
			RippleControl:       &rip,
			LinearCorrection:    &lin,
			SensorAngleDeg:      &ang,
			GlassMode:           &glass,
		},
		Buttons: snap.Buttons,
	}

	s.mu.Lock()
	if existing, ok := s.user[id]; ok {
		preset.CreatedAt = existing.CreatedAt
	}
	s.user[id] = preset
	err := s.saveLocked()
	s.mu.Unlock()
	return preset, err
}

// Delete removes a user preset by ID. Built-in presets cannot be deleted.
func (s *Store) Delete(id string) error {
	if strings.HasPrefix(id, "builtin-") {
		return errors.New("標準プリセットは削除できません")
	}
	s.mu.Lock()
	delete(s.user, id)
	err := s.saveLocked()
	s.mu.Unlock()
	return err
}

// ExportJSON serializes a preset to formatted JSON for sharing or backup.
func (s *Store) ExportJSON(id string) (string, error) {
	p, ok := s.Get(id)
	if !ok {
		return "", errors.New("指定されたプロファイルが見つかりません")
	}
	raw, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// ImportJSON validates and stores a preset imported from JSON text.
func (s *Store) ImportJSON(rawJSON string) (Preset, error) {
	var p Preset
	if err := json.Unmarshal([]byte(rawJSON), &p); err != nil {
		return Preset{}, fmt.Errorf("JSON形式が正しくありません: %w", err)
	}
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		return Preset{}, errors.New("プロファイル名が含まれていません")
	}
	if p.DPI.StageCount < 1 || p.DPI.StageCount > rapoo.MaxDPIStage {
		return Preset{}, errors.New("DPIステージ数が不正です")
	}
	if len(p.Buttons) > 0 {
		if err := rapoo.ValidateButtonMappings(p.Buttons); err != nil {
			return Preset{}, errors.New("左クリックが1つも割り当てられていないプロファイルはインポートできません")
		}
	}
	now := time.Now().UTC()
	p.ID = fmt.Sprintf("preset-%d", now.UnixNano())
	p.BuiltIn = false
	p.CreatedAt = now
	p.UpdatedAt = now

	s.mu.Lock()
	s.user[p.ID] = p
	err := s.saveLocked()
	s.mu.Unlock()
	return p, err
}

func intPtr(v int) *int    { return &v }
func boolPtr(v bool) *bool { return &v }

// BuiltInPresets returns curated presets for FPS esports, balanced play, and battery saving.
func BuiltInPresets() []Preset {
	baseTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	fpsStages := []rapoo.DPIStage{
		{Index: 1, X: 400, Y: 400, Enabled: true},
		{Index: 2, X: 800, Y: 800, Enabled: true},
		{Index: 3, X: 1600, Y: 1600, Enabled: true},
		{Index: 4, X: 3200, Y: 3200, Enabled: false},
		{Index: 5, X: 3200, Y: 3200, Enabled: false},
		{Index: 6, X: 6400, Y: 6400, Enabled: false},
		{Index: 7, X: 1600, Y: 1600, Enabled: false},
	}

	stdStages := []rapoo.DPIStage{
		{Index: 1, X: 400, Y: 400, Enabled: true},
		{Index: 2, X: 800, Y: 800, Enabled: true},
		{Index: 3, X: 1200, Y: 1200, Enabled: true},
		{Index: 4, X: 1600, Y: 1600, Enabled: true},
		{Index: 5, X: 3200, Y: 3200, Enabled: true},
		{Index: 6, X: 6400, Y: 6400, Enabled: false},
		{Index: 7, X: 1000, Y: 1000, Enabled: false},
	}

	ecoStages := []rapoo.DPIStage{
		{Index: 1, X: 800, Y: 800, Enabled: true},
		{Index: 2, X: 1200, Y: 1200, Enabled: true},
		{Index: 3, X: 1600, Y: 1600, Enabled: true},
		{Index: 4, X: 2400, Y: 2400, Enabled: true},
		{Index: 5, X: 3200, Y: 3200, Enabled: false},
		{Index: 6, X: 6400, Y: 6400, Enabled: false},
		{Index: 7, X: 1000, Y: 1000, Enabled: false},
	}

	// FPS Safe Buttons: Disable DPI+/DPI- top buttons to prevent accidental mid-fight DPI changes
	fpsButtons := rapoo.DefaultButtonMappings()
	disabledRaw, _ := rapoo.EncodeButtonAction(rapoo.ButtonAction{Type: rapoo.ActionDisabled})
	for i := range fpsButtons {
		if fpsButtons[i].Slot.ID == rapoo.ButtonDPIPlus || fpsButtons[i].Slot.ID == rapoo.ButtonDPIMinus {
			fpsButtons[i].Action = rapoo.DecodeButtonAction(fpsButtons[i].Slot, disabledRaw)
		}
	}

	return []Preset{
		{
			ID:          "builtin-fps-4k",
			Name:        "FPS 競技用 (4000Hz・超低遅延)",
			Description: "800 DPI・4000Hz・デバウンス 1ms・直線/波紋補正OFF・誤爆防止のため天面DPIボタン無効化",
			BuiltIn:     true,
			CreatedAt:   baseTime,
			UpdatedAt:   baseTime,
			PollingHz:   4000,
			DPI: rapoo.DPIConfig{
				CurrentStage: 2,
				StageCount:   3,
				Stages:       fpsStages[:3],
				AllStages:    fpsStages,
			},
			Sensor: rapoo.SensorPatch{
				LiftOffLevel:        intPtr(0), // 1.0 mm
				MotionSync:          boolPtr(false),
				PressDebounceStep:   intPtr(1), // 1 ms
				ReleaseDebounceStep: intPtr(1), // 1 ms
				SleepMinutes:        intPtr(15),
				RippleControl:       boolPtr(false),
				LinearCorrection:    boolPtr(false),
				SensorAngleDeg:      intPtr(0),
				GlassMode:           boolPtr(false),
			},
			Buttons: fpsButtons,
		},
		{
			ID:          "builtin-balanced-1k",
			Name:        "オールラウンド標準 (1000Hz)",
			Description: "1600 DPI・1000Hz・Motion Sync ON・デバウンス 4ms・バッテリー持続約160時間のバランス設定",
			BuiltIn:     true,
			CreatedAt:   baseTime,
			UpdatedAt:   baseTime,
			PollingHz:   1000,
			DPI: rapoo.DPIConfig{
				CurrentStage: 4,
				StageCount:   5,
				Stages:       stdStages[:5],
				AllStages:    stdStages,
			},
			Sensor: rapoo.SensorPatch{
				LiftOffLevel:        intPtr(1), // 1.1 mm
				MotionSync:          boolPtr(true),
				PressDebounceStep:   intPtr(3), // 4 ms
				ReleaseDebounceStep: intPtr(3), // 4 ms
				SleepMinutes:        intPtr(10),
				RippleControl:       boolPtr(false),
				LinearCorrection:    boolPtr(false),
				SensorAngleDeg:      intPtr(0),
				GlassMode:           boolPtr(false),
			},
			Buttons: rapoo.DefaultButtonMappings(),
		},
		{
			ID:          "builtin-desktop-eco",
			Name:        "デスクトップ省電力 (500Hz・長寿命)",
			Description: "1200 DPI・500Hz・5分スリープ・作業やブラウジングで最大約185時間駆動する省電力設定",
			BuiltIn:     true,
			CreatedAt:   baseTime,
			UpdatedAt:   baseTime,
			PollingHz:   500,
			DPI: rapoo.DPIConfig{
				CurrentStage: 2,
				StageCount:   4,
				Stages:       ecoStages[:4],
				AllStages:    ecoStages,
			},
			Sensor: rapoo.SensorPatch{
				LiftOffLevel:        intPtr(2), // 1.2 mm
				MotionSync:          boolPtr(true),
				PressDebounceStep:   intPtr(4), // 8 ms
				ReleaseDebounceStep: intPtr(4), // 8 ms
				SleepMinutes:        intPtr(5),
				RippleControl:       boolPtr(false),
				LinearCorrection:    boolPtr(false),
				SensorAngleDeg:      intPtr(0),
				GlassMode:           boolPtr(false),
			},
			Buttons: rapoo.DefaultButtonMappings(),
		},
	}
}
