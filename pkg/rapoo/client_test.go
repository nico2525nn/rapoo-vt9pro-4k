package rapoo

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/nico/rapoo-vt9pro-4k/pkg/hid"
)

func TestClientBusyPollingAndAsyncNotifications(t *testing.T) {
	prov := hid.NewMockProvider()

	cmdDev := hid.NewMockDevice(hid.DeviceInfo{
		Path:            `\\?\hid#vid_24ae&pid_1211&mi_01&col04`,
		VendorID:        0x24AE,
		ProductID:       0x1211,
		InterfaceNumber: 1,
		UsagePage:       0xFF00,
		Usage:           0x000E,
		InputReportLen:  32,
		OutputReportLen: 32,
	})
	repDev := hid.NewMockDevice(hid.DeviceInfo{
		Path:            `\\?\hid#vid_24ae&pid_1211&mi_01&col05`,
		VendorID:        0x24AE,
		ProductID:       0x1211,
		InterfaceNumber: 1,
		UsagePage:       0xFF00,
		Usage:           0x0002,
		InputReportLen:  32,
	})

	var polls atomic.Int32
	cmdDev.OnGetInput = func(reportID byte, length int) ([]byte, error) {
		n := polls.Add(1)
		buf := make([]byte, length)
		if n < 3 {
			// Simulate RF busy status 0x02 for first 2 checks
			buf[0] = 0x02
			return buf, nil
		}
		buf[0] = 0x01
		buf[1] = 0x01 // Discharging
		buf[2] = 92   // 92%
		return buf, nil
	}

	prov.Register(cmdDev)
	prov.Register(repDev)

	asyncCh := make(chan AsyncEvent, 4)
	client, err := OpenFirst(prov, ClientOptions{
		BusyPollInterval: time.Millisecond,
		MaxBusyPolls:     10,
		OnAsyncEvent: func(ev AsyncEvent) {
			asyncCh <- ev
		},
	})
	if err != nil {
		t.Fatalf("OpenFirst: %v", err)
	}
	defer client.Close()

	bat, err := client.ReadBattery()
	if err != nil {
		t.Fatalf("ReadBattery: %v", err)
	}
	if bat.Percentage != 92 || bat.Charging || !bat.Online {
		t.Fatalf("unexpected battery: %+v", bat)
	}
	if polls.Load() != 3 {
		t.Fatalf("polls = %d, want 3", polls.Load())
	}

	// Push unsolicited 0xBB packet
	repDev.PushAsyncPacket([]byte{0xBB, 0x00, 0x00, 0x01, 0x00, 0x02, 95, 0x00})
	select {
	case ev := <-asyncCh:
		if ev.Battery == nil || ev.Battery.Percentage != 95 || !ev.Battery.Charging {
			t.Fatalf("unexpected async event: %+v", ev.Battery)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for async 0xBB event")
	}
}
