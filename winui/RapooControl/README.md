# RapooControl (WinUI 3 prototype)

WinUI 3 / Windows App SDK frontend prototype for the Rapoo VT9 Pro 4K Windows app.
This is a feasibility spike, not a port of the Wails UI.

## Status

- Unpackaged, self-contained x64 desktop app.
- .NET 9 (`net9.0-windows10.0.19041.0`), Windows App SDK 1.8.
- NavigationView shell with 概要 / DPI / センサー / ボタン / 設定.
- Overview and DPI pages are interactive; the DPI page writes to an in-memory mock service.
- Sensor and button data are read-only mock values. Settings are placeholders.
- **No HID access, no firmware flashing, and no mouse changes.**

## Build

Requires the .NET 9 SDK and NuGet access (the Windows App SDK and Windows SDK
build tools are restored as NuGet packages; Visual Studio is not required).

```powershell
dotnet build .\RapooControl.csproj -c Release -p:Platform=x64
```

The executable is written to:

```text
bin\x64\Release\net9.0-windows10.0.19041.0\win-x64\RapooControl.exe
```

## Structure

```text
RapooControl/
├─ App.xaml(.cs)              app bootstrap and shared styles
├─ MainWindow.xaml(.cs)       NavigationView shell and page routing
├─ Models/                    device summary, DPI stage/config records
├─ Services/                  IRapooService + MockRapooService
├─ ViewModels/                shell and DPI page state
└─ Views/                     header + five pages
```

## Replacing the mock with Go

`IRapooService` is deliberately small and UI-facing:

- `GetStatusAsync`
- `GetDpiConfigurationAsync`
- `ApplyDpiConfigurationAsync`
- `SetPollingRateAsync`
- `StartAsync` / `StopAsync`
- `StatusChanged`

Only `MockRapooService` is constructed, in `App.xaml.cs`. A real implementation
can call the existing Go HID engine without touching the pages. The recommended
first bridge is a local HTTP/JSON endpoint exposed by the Go process (or a named
pipe if avoiding a loopback port is preferred); a native DLL/cgo bridge is not
needed for this UI.
