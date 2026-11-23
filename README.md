# Rapoo VT9 Pro 4K

A cross-platform Rapoo VT9 Pro 4K controller for Windows and Linux, built with
Uno Platform and WinUI. The UI is the existing WinUI layout; the Go sidecar
remains the only process that touches HID.

## Download

[Releases](https://github.com/nico2525nn/rapoo-vt9pro-4k/releases)

- `rapoo-control-uno-v1.4.0-windows-amd64.zip` — Windows app (self-contained, sidecar included)
- `rapoo-control-uno-v1.4.0-linux-amd64.tar.gz` — Linux app (self-contained, sidecar included)
- `checksums.txt` — SHA256 checksums

The v1.3.0 WinUI/Wails releases remain available for reference.

## Screenshots

| Windows | Linux |
|:---:|:---:|
| <img src="docs/images/winui-overview.png" width="560" alt="Windows overview"> | <img src="docs/images/uno-linux-overview.png" width="560" alt="Linux DPI and polling"> |
| Overview (Japanese) | Overview (Japanese) |

| Battery icon | Notification-area menu |
|:---:|:---:|
| <img src="docs/images/tray-icon.png" width="280" alt="Battery percentage tray icon"> | <img src="docs/images/tray-menu.png" width="260" alt="Notification-area menu"> |
| Transparent 7-segment icon with a level bar | Right-click modes, startup, and quit |

## Usage

### Windows

Extract the zip and run `RapooControl.exe`. The Go sidecar is included next to
the app; no installer or .NET runtime is required.

- DPI, polling rate, lift-off distance, Motion Sync, debounce, 10 buttons,
  profiles, 0-TX (gaming lock), and demo mode.
- The notification area shows the live battery percentage. Right-click for
  modes, demo, startup, and quit.
- Closing the window exits by default. Enable close-to-tray in Settings to
  keep it running in the notification area.
- "Start in the notification area" launches hidden when Windows signs in.
- The old WinUI and Wails apps use separate startup entries and instance
  locks. Exit them before running this release; only one process can own the
  HID command handle.

### Linux

Extract the tarball and run `./RapooControl`. The bundle is self-contained.

- The app uses XDG autostart when enabled in Settings.
- A Linux tray is not implemented yet; close-to-tray and start-hidden are
  Windows-only.
- For `/dev/hidraw` access, apply the udev rule shown in Settings (also below):

  ```udev
  SUBSYSTEM=="hidraw", ATTRS{idVendor}=="24ae", MODE="0660", TAG+="uaccess"
  ```

## Language

The app follows the system language on a fresh profile; Japanese and English
are available in Settings. To seed the first launch:

```bash
RAPOO_LANG=ja ./RapooControl
```

The selected language is saved and takes precedence later. `RAPOO_LANG` is
also passed to the sidecar.

## UI

- Windows: Uno Platform / WinAppSDK head, using the existing WinUI layout.
- Linux: Uno Platform / Skia desktop head, same pages and controls.
- The Go sidecar lives in `cmd/rapoo-bridge` and is shared by both heads.
- `winui/` and the Wails frontend are legacy reference implementations.

## Build

```bash
# Go backend and both sidecars
go test ./...
./uno/build-sidecars.sh

# Windows / Linux heads
export PATH="$HOME/.dotnet:$PATH"
export DOTNET_ROOT="$HOME/.dotnet"
dotnet build uno/RapooControl/RapooControl.csproj -f net10.0-desktop -c Debug
dotnet build uno/RapooControl/RapooControl.csproj -f net10.0-windows10.0.26100 -c Debug

# Tests (DPI, bridge faults/restart, state ordering, autostart, migration)
dotnet test uno/RapooControl.Tests/RapooControl.Tests.csproj

# Windows release (run on Windows)
dotnet publish uno/RapooControl/RapooControl.csproj -f net10.0-windows10.0.26100 -c Release -p:PublishProfile=win-x64

# Linux self-contained release
dotnet publish uno/RapooControl/RapooControl.csproj -f net10.0-desktop -c Release -r linux-x64 --self-contained true -o dist/linux
```

Publishing requires the sidecar and an explicit RID. Both heads disable
trimming because the bridge protocol uses reflection-based `System.Text.Json`.

## Troubleshooting

- Not detected: check the 4K receiver or the USB-C cable.
- Cannot change settings: disable 0-TX (gaming lock).
- Linux permission error: apply the udev rule above, then reconnect the device.
- Firmware update: the app launches the official Rapoo software and never
  flashes firmware itself.

## Protocol

[docs/PROTOCOL.md](docs/PROTOCOL.md)

## License

MIT
