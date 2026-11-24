# Architecture and support boundary

## Shipped head

`uno/RapooControl` is the only supported UI. It hosts the faithful WinUI
layout on Windows (`net10.0-windows10.0.26100`, WinAppSDK) and Linux
(`net10.0-desktop`, Skia). The Go sidecar in `cmd/rapoo-bridge` is the only
process that opens HID command/report handles.

Mutating RPCs carry the device `generation` the UI observed. The sidecar checks
it immediately before every write while holding the connection lock, so a
reconnect, demo-mode switch, or endpoint replacement cannot redirect a stale
dialog/apply to another mouse. Each request also carries a deadline; the
sidecar skips queued work that can no longer be applied and reports partial
multi-step writes.

## Legacy heads

`winui/RapooControl` and the Wails app (`app.go`, `frontend/`) are frozen
reference implementations. They are not built into the release bundles. They
still share the Go wire protocol and HID device, so protocol changes must be
mirrored in their clients or they must be considered unsupported. Their state
handlers keep generation/revision guards, but the Uno head is the only one
covered by release checks.

The Wails frontend is built only for reference; its npm production dependency
audit is clean. The per-monitor/DPI and tray behavior of the legacy heads is
not part of the current support contract.

## Persistent state

Preferences and language files are written atomically. Preset reads surface
I/O and parse errors while retaining the last good cache; an empty library is
never reported for a corrupt file. Presets are only saved from complete
snapshots (DPI, sensor, and button table present).

Button presets store the four-byte onboard action, including a `0x05` macro
reference. The app preserves that reference when applying a preset, but it does
not read or write the onboard macro body; edits made to macro contents in the
official vendor tool are outside the profile restore contract.

## Release checks

- `go test ./...`, `go vet ./...`, and the .NET tests.
- `npm test` for the legacy frontend.
- `dotnet build` for the Windows head and the Uno Linux head.
- `uno/smoke-linux.sh` against the trimmed Linux bundle under Xvfb.
- `uno/package-linux.sh` / `uno/package-windows.ps1` for release assets.
