# Rapoo Control (Uno Platform)

A single Uno Platform project that hosts the existing Rapoo VT9 Pro 4K UI on
Windows (WinAppSDK) and Linux (Skia Desktop). The Go sidecar remains the only
process that touches HID; the UI talks to it over newline-delimited JSON.

The WinUI UI is intentionally a faithful port of the shipped app. Do not
redesign the shell or pages without an explicit product decision.

## Targets

- `net10.0-windows10.0.26100` — packaged/unpackaged WinAppSDK (win-x64)
- `net10.0-desktop` — Skia Desktop, verified on linux-x64
- `net10.0` — platform-neutral fallback used by tests

## Build

```bash
export PATH="$HOME/.dotnet:$PATH"
export DOTNET_ROOT="$HOME/.dotnet"

# Linux desktop
dotnet build uno/RapooControl/RapooControl.csproj -f net10.0-desktop -c Debug

# Windows (run on Windows)
dotnet build uno/RapooControl/RapooControl.csproj -f net10.0-windows10.0.26100 -c Debug
```

The bridge is copied next to the app when it exists at `build/bin/rapoo-bridge`
or `build/bin/rapoo-bridge.exe`. Build both sidecars with:

```bash
./uno/build-sidecars.sh
```

Publishing fails deliberately if the sidecar is missing or the release RID is
not explicit. Release publish therefore uses:

```bash
dotnet publish uno/RapooControl/RapooControl.csproj -f net10.0-desktop -c Release -r linux-x64
```

Both app heads disable trimming because the bridge protocol uses
reflection-based `System.Text.Json`.

## Run

```bash
RAPOO_USE_MOCK=1 dotnet run --project uno/RapooControl -f net10.0-desktop
RAPOO_DEMO=1 dotnet run --project uno/RapooControl -f net10.0-desktop
```

`RAPOO_USE_MOCK` keeps the UI entirely in-process. `RAPOO_DEMO` starts the Go
sidecar in demo mode and exercises the full bridge protocol without hardware.

On Linux the app uses XDG autostart, the sidecar under the app directory, and a
udev rule for `/dev/hidraw`. The Settings page shows the copyable rule. A Linux
tray is not implemented yet; close-to-tray and start-hidden are Windows-only.

## Tests

```bash
dotnet test uno/RapooControl.Tests/RapooControl.Tests.csproj
```

Tests cover DPI normalization/editing, polling-rate limits, bridge DTO mapping,
and desktop-entry escaping. They deliberately avoid starting the Uno runtime.

## Structure

- `App.xaml` / `App.xaml.cs` — shell resources, composition root, window sizing
- `MainPage.xaml` — title bar, navigation, page actions
- `Views/` — the six faithful WinUI pages
- `ViewModels/` — shell and DPI page view models
- `Services/` — bridge client, real/mock services, tray/autostart/preferences
- `Services/*.Windows.cs`, `*.Linux.cs`, `*.Other.cs` — platform partials
