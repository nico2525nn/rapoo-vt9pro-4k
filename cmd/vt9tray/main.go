// Command vt9tray displays the Rapoo VT9 Pro 4K mouse battery percentage as
// crisp HiDPI numbers in the Windows system tray.
package main

import (
	"context"
	"errors"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nico/rapoo-vt9pro-4k/internal/service"
	"github.com/nico/rapoo-vt9pro-4k/internal/tray"
)

func main() {
	var (
		passive  = flag.Bool("passive", false, "start in passive-only mode (0 outbound HID packets; COL05 notifications only)")
		interval = flag.Duration("interval", 60*time.Second, "normal mode battery poll interval")
	)
	flag.Parse()

	initMode := service.ModeNormal
	if *passive {
		initMode = service.ModePassiveOnly
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	svc := service.NewMouseService(nil, service.Config{
		InitialMode:    initMode,
		NormalInterval: *interval,
	})
	svc.Startup(ctx)
	defer svc.Shutdown()

	if err := tray.Run(ctx, svc); err != nil && !errors.Is(err, tray.ErrAlreadyRunning) {
		os.Exit(1)
	}
}
