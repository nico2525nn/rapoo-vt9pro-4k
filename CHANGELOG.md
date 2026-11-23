# Changelog

## v1.3.0 — 2026-11-23

### Added

- WinUI and Wails "Start in the notification area" setting
- Auto-start registry commands now carry `--hidden` when that setting is enabled
- Wails uses its own `RapooControlWails` Run entry so it cannot overwrite the WinUI auto-start
- Wails can read UTF-8 BOM preference files written by Windows PowerShell

### Changed

- Version bumped to 1.3.0

### Fixed

- Closing WinUI no longer kills an in-flight official firmware updater child process
- Hidden startup falls back to a visible window when the notification icon cannot be created
- WinUI and Wails use separate auto-start Run values and migrate legacy values only when they belong to that app
- Wails tray auto-start includes `--hidden`; startup-setting failures are reported instead of silently ignored
- Single-instance show requests are queued until the window is ready; hidden second launches do not force a show
- 0-TX reconnects and polls are serialized under one owner; the official updater locks out normal mode changes
- The writable HID command endpoint is owned by one process through a named mutex
- The sidecar restarts with per-process state, timeout-aware writes, embedded-sidecar hash/protocol checks, and publish-time validation
- Presets record the hardware generation, validate Gen-A steps vs. Rev B milliseconds, and use a cross-process lock file
- WinUI waits for a valid sidecar handshake before the first RPC and isolates restarts from old read loops
- Auto-start migration recognizes the documented v1.2.0 filenames and new per-app markers
- Early second-launch requests use a manual-reset event and synchronized Wails state
- Firmware updates run under a detached, console-less guard process that holds every supported device's HID command mutex and waits for existing handles to close
- The guard reports readiness only after the official updater actually starts, so the UI no longer claims success on a failed launch
- Polling mode is restored before the updater guard is cleared, so a rejected/retried update cannot leave 0-TX behind
- WinUI serializes bridge startup, waits for the handshake before any RPC, and retries after a launch failure
- Early show requests use a locked handoff; the Wails tray auto-start toggle goes through the app's migration and legacy cleanup
- WinUI uses an explicit sleeping state, respects an explicit `IsPromoted=0`, writes preferences atomically, and clamps wired DPI rates
- Wails preference files with a UTF-8 BOM are accepted

## v1.2.0 — 2026-11-22

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

### Changed

- The notification icon is rendered at the system small-icon size as a transparent 7-segment battery with a level bar
- Version bumped to 1.2.0

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
