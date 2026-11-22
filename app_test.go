package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeLanguage(t *testing.T) {
	tests := map[string]string{
		"":        "en",
		"en":      "en",
		"en-US":   "en",
		"ja":      "ja",
		"ja-JP":   "ja",
		"jp":      "ja",
		"fr":      "en",
		" JA-jp ": "ja",
	}
	for input, want := range tests {
		if got := normalizeLanguage(input); got != want {
			t.Errorf("normalizeLanguage(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSetLanguagePersists(t *testing.T) {
	prefsPath := filepath.Join(t.TempDir(), "app-prefs.json")
	a := &App{
		prefsPath: prefsPath,
		prefs:     AppPreferences{Language: "en", Mode: "normal"},
	}
	t.Setenv("RAPOO_LANG", "en")

	a.SetLanguage("ja")

	raw, err := os.ReadFile(prefsPath)
	if err != nil {
		t.Fatalf("read prefs: %v", err)
	}
	var disk struct {
		Language string `json:"language"`
	}
	if err := json.Unmarshal(raw, &disk); err != nil {
		t.Fatalf("decode prefs: %v", err)
	}
	if disk.Language != "ja" {
		t.Fatalf("persisted language = %q, want ja", disk.Language)
	}
	if got := os.Getenv("RAPOO_LANG"); got != "ja" {
		t.Fatalf("RAPOO_LANG = %q, want ja", got)
	}
}
