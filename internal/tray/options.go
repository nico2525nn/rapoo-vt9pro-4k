package tray

// Options customizes system tray behavior when embedded inside a GUI application
// (such as Rapoo Control Wails) vs standalone vt9tray.
type Options struct {
	MutexName         string // Defaults to Local\RapooVT9Pro4KTrayMutex
	WindowClass       string // Defaults to RapooVT9Pro4KTrayWndClass
	RegistryValueName string // Defaults to RapooVT9Tray
	OpenMenuLabel     string // Optional menu item (e.g. "Rapoo Control を開く(&O)")
	OnOpenWindow      func() // Invoked on left-click or OpenMenuLabel when set
	OnQuit            func() // Invoked after tray exits via Quit menu item
}
