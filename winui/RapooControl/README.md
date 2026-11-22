# RapooControl (WinUI 3)

Native WinUI 3 / Windows App SDK frontend for the Rapoo VT9 Pro 4K.
The UI never touches HID directly: it launches the Go `rapoo-bridge` sidecar and
talks newline-delimited JSON over stdin/stdout.

## Status

- Unpackaged, self-contained x64 desktop app.
- .NET 9 (`net9.0-windows10.0.19041.0`), Windows App SDK 1.8.
- NavigationView shell with 概要 / DPI / センサー / ボタン / プロファイル / 設定.
- All settings pages read and write the real device through the bridge.
- Firmware versions are display-only. The update button launches the official
  Rapoo software; this app never flashes firmware.
- `RAPOO_DEMO=1` starts the sidecar in demo mode for testing.
- `RAPOO_USE_MOCK=1` uses the in-memory mock instead of the bridge.

## Build

Requires the .NET 9 SDK, NuGet access, and Go for the sidecar.

```powershell
# From the repository root, cross-build or build on Windows:
go build -o build\bin\rapoo-bridge.exe .\cmd\rapoo-bridge

# Build the WinUI app:
dotnet build .\winui\RapooControl\RapooControl.csproj -c Release -p:Platform=x64
```

`RapooControl.csproj` copies `build\bin\rapoo-bridge.exe` next to the app when
that file exists. Keep both executables in the same folder.

The executable is written to:

```text
bin\x64\Release\net9.0-windows10.0.19041.0\win-x64\RapooControl.exe
```

## Bridge protocol

The sidecar is `cmd/rapoo-bridge`. Requests are JSON lines:

```json
{"id":1,"method":"state"}
{"id":2,"method":"pollingRate","params":{"hz":4000}}
{"id":3,"method":"sensor","params":{"patch":{"motionSync":true}}}
```

Responses carry the same `id`; state changes arrive as unsolicited events:

```json
{"id":2,"result":{...}}
{"event":"state","data":{"connected":true,"demoMode":false,...}}
```

Methods: `state`, `snapshot`, `catalog`, `demo`, `pollingMode`, `pollingRate`,
`dpiConfig`, `sensor`, `button`, `resetButtons`, `factoryReset`, `presets`,
`savePreset`, `applyPreset`, `deletePreset`, `exportPreset`, `importPreset`,
`openUpdater`, `shutdown`.

Only one process can own the HID handle. Close the Wails app/tray before using
the WinUI app, and vice versa.

## Structure

```text
RapooControl/
├─ App.xaml(.cs)              app bootstrap and shared styles
├─ MainWindow.xaml(.cs)       NavigationView shell and page routing
├─ Models/                    device summary, DPI stage/config records
├─ Services/                  bridge client, real service, mock fallback, wire models
├─ ViewModels/                shell and DPI page state
└─ Views/                     six pages plus shared feedback
```
