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
	if err := s.load(); err != nil {
		return nil, fmt.Errorf("presets.json: %w", err)
	}
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
	tmp, err := os.CreateTemp(filepath.Dir(s.filePath), "presets-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, s.filePath)
}

// List returns user-saved presets.
func (s *Store) List() []Preset {
	s.mu.RLock()
	defer s.mu.RUnlock()

	userList := make([]Preset, 0, len(s.user))
	for _, p := range s.user {
		userList = append(userList, p)
	}
	sort.Slice(userList, func(i, j int) bool {
		return userList[i].UpdatedAt.After(userList[j].UpdatedAt)
	})
	return userList
}

// Get returns a user preset by ID.
func (s *Store) Get(id string) (Preset, bool) {
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
	if id == "" {
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
	led := sc.LEDLowPowerMode

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
			LEDLowPowerMode:     &led,
		},
		Buttons: snap.Buttons,
	}
	if err := ValidatePreset(preset); err != nil {
		return Preset{}, err
	}

	s.mu.Lock()
	old, existed := s.user[id]
	if existed {
		preset.CreatedAt = old.CreatedAt
	}
	s.user[id] = preset
	err := s.saveLocked()
	if err != nil {
		if existed {
			s.user[id] = old
		} else {
			delete(s.user, id)
		}
	}
	s.mu.Unlock()
	return preset, err
}

// Delete removes a user preset by ID.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	old, existed := s.user[id]
	delete(s.user, id)
	err := s.saveLocked()
	if err != nil && existed {
		s.user[id] = old
	}
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
	if err := ValidatePreset(p); err != nil {
		return Preset{}, err
	}
	now := time.Now().UTC()
	p.ID = fmt.Sprintf("preset-%d", now.UnixNano())
	p.BuiltIn = false
	p.CreatedAt = now
	p.UpdatedAt = now

	s.mu.Lock()
	s.user[p.ID] = p
	err := s.saveLocked()
	if err != nil {
		delete(s.user, p.ID)
	}
	s.mu.Unlock()
	return p, err
}

func ValidatePreset(p Preset) error {
	if p.PollingHz < 0 || p.PollingHz > 8000 {
		return fmt.Errorf("ポーリングレート %d Hz が不正です", p.PollingHz)
	}
	if p.DPI.StageCount < 1 || p.DPI.StageCount > rapoo.MaxDPIStage {
		return errors.New("DPIステージ数が不正です")
	}
	if p.DPI.CurrentStage < 1 || p.DPI.CurrentStage > p.DPI.StageCount {
		return errors.New("現在のDPIステージが不正です")
	}
	if len(p.DPI.Stages) < p.DPI.StageCount {
		return errors.New("DPIステージの値が不足しています")
	}
	stages := p.DPI.Stages
	if len(p.DPI.AllStages) > 0 {
		if len(p.DPI.AllStages) < rapoo.MaxDPIStage {
			return errors.New("DPI全ステージの値が不足しています")
		}
		stages = p.DPI.AllStages
	}
	for i, st := range stages {
		if st.X < 50 || st.X > 26000 || st.Y < 50 || st.Y > 26000 {
			return fmt.Errorf("DPIステージ %d の値が範囲外です", i+1)
		}
	}
	if p.Sensor.LiftOffLevel != nil && (*p.Sensor.LiftOffLevel < 0 || *p.Sensor.LiftOffLevel > 10) {
		return errors.New("リフトオフディスタンスが範囲外です")
	}
	if p.Sensor.SleepMinutes != nil && (*p.Sensor.SleepMinutes < 2 || *p.Sensor.SleepMinutes > 120) {
		return errors.New("スリープ時間が範囲外です")
	}
	if p.Sensor.SensorAngleDeg != nil && (*p.Sensor.SensorAngleDeg < -30 || *p.Sensor.SensorAngleDeg > 30) {
		return errors.New("センサー角度が範囲外です")
	}
	if p.Sensor.PressDebounceStep != nil && (*p.Sensor.PressDebounceStep < 0 || *p.Sensor.PressDebounceStep > 6) {
		return errors.New("押下デバウンスが範囲外です")
	}
	if p.Sensor.ReleaseDebounceStep != nil && (*p.Sensor.ReleaseDebounceStep < 0 || *p.Sensor.ReleaseDebounceStep > 6) {
		return errors.New("解放デバウンスが範囲外です")
	}
	if p.Sensor.LEDLowPowerMode != nil && *p.Sensor.LEDLowPowerMode > 1 {
		return errors.New("LED 省電力モードの値が不正です")
	}
	if len(p.Buttons) > 0 {
		if err := rapoo.ValidateButtonMappings(p.Buttons); err != nil {
			return fmt.Errorf("ボタン割り当てが不正です: %w", err)
		}
	}
	return nil
}
