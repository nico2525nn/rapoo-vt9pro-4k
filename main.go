package main

import (
	"embed"
	"fmt"
	"os"
	"runtime"
	"strconv"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"

	"github.com/nico/rapoo-vt9pro-4k/internal/updaterguard"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIconPNG []byte

func containsHiddenArg(args []string) bool {
	for _, argument := range args {
		if argument == "--hidden" || argument == "--tray" {
			return true
		}
	}
	return false
}

func main() {
	if len(os.Args) >= 5 && os.Args[1] == "--updater-guard" {
		vendor, vendorErr := strconv.Atoi(os.Args[3])
		product, productErr := strconv.Atoi(os.Args[4])
		if vendorErr != nil || productErr != nil {
			println("updater guard: invalid device id")
			os.Exit(1)
		}
		if err := updaterguard.Run(os.Args[2], uint16(vendor), uint16(product), func() {
			fmt.Println("ready")
			_ = os.Stdout.Sync()
		}); err != nil {
			println("updater guard:", err.Error())
			os.Exit(1)
		}
		return
	}

	app := NewApp()
	startHidden := false
	if runtime.GOOS == "windows" {
		for _, argument := range os.Args[1:] {
			if argument == "--hidden" || argument == "--tray" {
				startHidden = true
			}
		}
	}
	app.startHidden = startHidden

	err := wails.Run(&options.App{
		Title:       "Rapoo Control — VT9 Pro 4K",
		Width:       1140,
		Height:      760,
		MinWidth:    880,
		MinHeight:   620,
		MaxWidth:    1600,
		MaxHeight:   1000,
		Frameless:   true,
		StartHidden: startHidden,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 10, G: 12, B: 18, A: 255},
		OnStartup:        app.startup,
		OnBeforeClose:    app.beforeClose,
		OnShutdown:       app.shutdown,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "rapoo-vt9pro-4k-control-hub-v1",
			OnSecondInstanceLaunch: func(data options.SecondInstanceData) {
				if containsHiddenArg(data.Args) {
					return
				}
				app.ShowWindow()
			},
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			Theme:                windows.Dark,
		},
		Linux: &linux.Options{
			Icon:                appIconPNG,
			WindowIsTranslucent: false,
			ProgramName:         "Rapoo Control",
		},
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}
