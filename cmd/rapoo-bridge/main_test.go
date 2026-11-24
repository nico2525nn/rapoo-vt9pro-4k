package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/nico/rapoo-vt9pro-4k/internal/service"
	"github.com/nico/rapoo-vt9pro-4k/pkg/hid"
	"github.com/nico/rapoo-vt9pro-4k/pkg/rapoo"
)

func TestRunSkipsExpiredRequest(t *testing.T) {
	svc := service.NewMouseService(hid.NewMockProvider(), service.Config{PresetDir: t.TempDir()})
	var out bytes.Buffer
	b := &bridge{svc: svc, enc: json.NewEncoder(&out)}

	input := strings.Join([]string{
		`{"id":1,"method":"state","deadline":1}`,
		`{"id":2,"method":"state"}`,
	}, "\n") + "\n"
	if err := b.run(strings.NewReader(input)); err != nil {
		t.Fatalf("run: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("responses = %d, want 2: %s", len(lines), out.String())
	}
	var expired response
	if err := json.Unmarshal([]byte(lines[0]), &expired); err != nil {
		t.Fatalf("decode expired: %v", err)
	}
	if expired.ID != 1 || expired.Code != "deadline_exceeded" {
		t.Fatalf("expired response = %+v", expired)
	}
	var current response
	if err := json.Unmarshal([]byte(lines[1]), &current); err != nil {
		t.Fatalf("decode current: %v", err)
	}
	if current.ID != 2 || current.Error != "" {
		t.Fatalf("current response = %+v", current)
	}
}

func TestBridgeErrorCodes(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{service.ErrDeviceChanged, "device_changed"},
		{rapoo.ErrDeadlineExceeded, "deadline_exceeded"},
		{&service.PartialWriteError{Applied: []string{"dpi"}, Err: errors.New("boom")}, "partial"},
		{errors.New("boom"), "error"},
	}
	for _, tc := range cases {
		if got := bridgeErrorCode(tc.err); got != tc.want {
			t.Errorf("bridgeErrorCode(%v) = %q, want %q", tc.err, got, tc.want)
		}
	}
}
