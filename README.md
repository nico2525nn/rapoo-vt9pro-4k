# Rapoo VT9 Pro 4K

Windows tools and a HID protocol implementation for the Rapoo VT9 Pro 4K.

## Download

[Releases](https://github.com/nico2525nn/rapoo-vt9pro-4k/releases)

- `rapoo-control-winui-v1.1.0-windows-amd64.exe` — native WinUI 3 app (Windows, single file)
- `rapoo-vt9pro-4k-v1.1.0-bundle.zip` — Wails app (Windows/Linux), `vt9tray`, `vt9ctl`, README, changelog
- `checksums.txt` — SHA256 checksums

## Usage

Extract the bundle for the command-line and tray tools:

```text
windows/rapoo-control.exe   Wails app
windows/vt9tray.exe         battery tray
windows/vt9ctl.exe          CLI
linux/rapoo-control         Wails app
linux/vt9ctl                CLI
```

- `vt9tray.exe`: run it. Right-click for polling mode and exit.
- `rapoo-control.exe`: configure DPI, sensor, buttons, profiles, and app settings from the sidebar.
- `vt9ctl.exe`: `list`, `battery`, `info`, `watch`.

Features: DPI, polling rate, lift-off distance, Motion Sync, debounce, 10 buttons, profiles, 0-TX (gaming lock), and demo mode.

### WinUI 3 app

Run `rapoo-control-winui-v1.1.0-windows-amd64.exe`. It is a single file; the Go HID sidecar is embedded and extracted to `%LOCALAPPDATA%\rapoo-vt9pro-4k\bin` on first launch.

- All pages read and write the device: DPI, sensor, 10 buttons, profiles, settings.
- Firmware update is read-only in this app. It launches the official Rapoo software and never flashes firmware itself.
- Use the icon-only Reload and Apply buttons at the bottom-right of the sidebar to apply staged changes.
- Only one process can own the HID handle. Do not run the WinUI app and the Wails app/tray at the same time.

## Language

English is the default. Change the language in Settings → Language: **English** or **日本語**.

To set the initial language before the first launch:

```bat
setx RAPOO_LANG en
setx RAPOO_LANG ja
```

`RAPOO_LANG` is the default for a fresh profile. A language selected in Settings is saved and takes precedence on later launches. The value is also passed to the WinUI sidecar.

## UI

- Linux / Windows Wails UI: React 19 + Tailwind CSS v4 + current shadcn/ui
- Windows native: `winui/RapooControl` (WinUI 3 / Windows App SDK, real device access via the Go sidecar)

Both UIs use the WinUI 3 color tokens for surfaces, cards, text, and accent states.

## Build

```bash
go test ./...

# Windows tray / CLI
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-H=windowsgui -s -w" -o dist/vt9tray.exe ./cmd/vt9tray
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o dist/vt9ctl.exe ./cmd/vt9ctl
GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o dist/vt9ctl ./cmd/vt9ctl

# WinUI sidecar
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o build/bin/rapoo-bridge.exe ./cmd/rapoo-bridge

# Wails app
go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.1
wails build -platform windows/amd64
wails build -platform linux/amd64 -tags webkit2_41

# Frontend
cd frontend && npm ci && npm test && npm run build

# WinUI 3 (Windows + .NET 9 SDK)
dotnet build winui/RapooControl/RapooControl.csproj -c Release -p:Platform=x64

# Single-file WinUI release (embeds the sidecar)
dotnet publish winui/RapooControl/RapooControl.csproj -c Release -p:Platform=x64 -p:PublishSingleFile=true -p:RapooBridgePath=C:\path\to\rapoo-bridge.exe -o dist/winui
```

## Troubleshooting

- Not detected: check the 4K receiver or the USB-C cable.
- Cannot change settings: disable 0-TX (gaming lock).
- Linux permission error: install the hidraw udev rule.

  ```udev
  SUBSYSTEM=="hidraw", ATTRS{idVendor}=="24ae", MODE="0660", TAG+="uaccess"
  ```

- Firmware update: this project never flashes firmware. Use the official Rapoo software.

## Protocol

[docs/PROTOCOL.md](docs/PROTOCOL.md)

## License

MIT
