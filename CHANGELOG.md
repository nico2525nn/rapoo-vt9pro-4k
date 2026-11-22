# Changelog

## Unreleased

### Added

- WinUI notification-area icon with the live battery percentage, tooltip, and menu
- WinUI close-to-tray and Start with Windows settings
- WinUI battery row on the Overview page
- Tray menu for polling modes, demo mode, reload, open, and quit

### Fixed

- Both apps retry transient dongle-busy replies and debounce sleep reports, so the UI no longer flashes "sleeping" while the mouse is online
- WinUI draws the same transparent, pixel-aligned battery icon as the Wails tray instead of a dark generated square
- Wails and the standalone tray promote the notification icon in Windows 11
- Wails and WinUI default close-to-tray to off and migrate old preferences to an explicit opt-in
- Wails only hides to the tray after the notification icon has been created

## v1.1.0 — 2026-11-22

### Added

- English is the default UI language; Japanese is available in Settings → Language
- `RAPOO_LANG=en|ja` environment variable support for both UIs and the WinUI sidecar
- Language preference persistence; a saved Settings choice takes precedence over the environment default

### Changed

- README, changelog, and release notes are now in English
- Version bumped to 1.1.0

## v1.0.0 — 2026-11-22

### Added

- Native WinUI 3 app (`winui/RapooControl`) backed by the Go `rapoo-bridge` sidecar
  - Overview, DPI, sensor, buttons, profiles, and settings pages
  - 10-button remapping, keyboard assignments, and left-click protection
  - Profile save / apply / delete
  - Firmware version display and official updater launch (this app never flashes)
  - 0-TX (passive) mode, demo mode, factory reset
  - Icon-only Reload/Apply actions at the bottom-right of the sidebar
- Single-file WinUI distribution: the sidecar is embedded and extracted to `%LOCALAPPDATA%\rapoo-vt9pro-4k\bin` on first launch
- Wails palette aligned with WinUI 3 in dark and light themes
- `rapoo-bridge` sidecar package

### Fixed

- Windows HID close now waits for in-flight I/O
- Linux HID rejects invalid `HIDIOCGINPUT` lengths and only falls back on `ENOTTY`
- Linux command/report collections pair by physical device ID
- 0-TX closes the active HID client synchronously
- Polling mode is persisted and applied before the first HID open
- The official updater runs with HID in passive mode and restores the previous mode
- Button writes validate all 10 slots, protect left click, and read back
- Presets are validated before apply, saved atomically, and report load errors
- Presets persist the LED low-power setting

### Known limitations

- WinUI and Wails/tray cannot own HID at the same time.
- WinUI is unpackaged; the single-file executable extracts the sidecar on first launch.
- Some Linux hidraw multi-collection layouts are not supported.
- The official update server signature parameters are not known; use the official software.
- Macros are not supported. The onboard memory stores one 4-byte action per button.
