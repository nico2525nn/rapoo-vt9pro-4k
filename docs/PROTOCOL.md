# Rapoo VT9 Pro 4K HID Protocol Specification (`MouseForRealtek` / Chip `0x0E`)

This document describes the vendor HID protocol used by the **Rapoo VT9 Pro 4K** wireless/wired gaming mouse family, reverse-engineered from Rapoo's official Windows software (`RapooDevice.dll` in **Rapoo A-HUB v1.0.19** and **RapooGameDev v1.6.29**).

---

## 1. USB / HID Device Identification

| Model | Connection | Vendor ID | Product ID | Official Class | Opcode Gen | Target Byte |
| :--- | :--- | :---: | :---: | :--- | :---: | :---: |
| **VT9 Pro 4K** | Wireless (4K Dongle) | `0x24AE` | `0x1211` | `QDeviceVT9PRO_4K_Wireless` | **Gen-A (`0xAx`)** | `0xA5` |
| **VT9 Pro 4K** | Wired (USB-C) | `0x24AE` | `0x4411` | `QDeviceVT9PRO_4K` | **Gen-A (`0xAx`)** | `0xFF` |
| **VT9 Pro 4K (Rev B / EU)** | Wireless (4K Dongle) | `0x24AE` | `0x1227` | `MouseForRealtek` | **Gen-B (`0xBx`)** | `0xA5` |
| **VT9 Pro 4K (Rev B / EU)** | Wired (USB-C) | `0x24AE` | `0x4427` | `MouseForRealtek` | **Gen-B (`0xBx`)** | `0xFF` |

> **Related (Non-Realtek) VT9 Pro PIDs** (Nordic `Mouse` class, different framing):
> - `0x418A` (VT9 Pro Wired), `0x185A` (VT9 Pro 1K Dongle), `0x1601` (V2M 4K Dongle).

---

## 2. HID Interfaces & Top-Level Collections

The USB composite device exposes two USB interfaces:

1. **`MI_00` (`InterfaceNumber == 0`) — Standard Boot/4K Mouse Input**
   - `UsagePage = 0x0001` (Generic Desktop), `Usage = 0x0002` (Mouse).
   - Carries up to **4000 Hz** motion and button input reports.
   - **Gaming Safety Rule**: Never open, query, or touch `MI_00` (not even for enumeration metadata when avoidable).

2. **`MI_01` (`InterfaceNumber == 1`) — Vendor Control & Telemetry Collections**
   - Exposes multiple top-level collections (`COL01`..`COL07` on Windows):
   - **Command Channel (`path`)**:
     - `InterfaceNumber == 1`, `UsagePage >= 0xFF00` (Vendor-Defined), **`Usage == 0x000E` (`14`)**.
     - Used for request/response commands (`Report ID 0xBA`).
     - Opened with `FILE_SHARE_READ | FILE_SHARE_WRITE` (`GENERIC_READ | GENERIC_WRITE`) so multiple apps can coexist without locking each other out.
   - **Async Notification Channel (`path_report`)**:
     - `InterfaceNumber == 1`, `UsagePage >= 0xFF00` (Vendor-Defined), **`Usage == 0x0002`** (specifically `COL05` on Windows when multiple `Usage == 0x0002` collections exist).
     - Streams unsolicited interrupt `Input` reports (`Report ID 0xBB` battery/online changes, `Report ID 0xBD` DPI/profile/key notifications).
     - Can be opened **read-only** (`GENERIC_READ`, `FILE_SHARE_READ | FILE_SHARE_WRITE`) for 100% passive telemetry with zero outbound USB packets.

---

## 3. Command Transport (`Report ID 0xBA`)

### 3.1 Packet Framing
All outbound commands on the `Usage == 0x000E` collection are **32-byte (`0x20`)** logical frames (zero-padded to the OS `OutputReportByteLength` if larger):

| Byte | Name | Value / Description |
| :---: | :--- | :--- |
| `0` | `ReportID` | Always `0xBA` |
| `1` | `Target` | `0xA5` = Remote Wireless Mouse (via 4K Dongle)<br>`0xFF` = Local Device (Wired USB Mouse, or the 4K Dongle MCU itself) |
| `2` | `Opcode` | **Gen-A (`0x1211`/`0x4411`)**: `0xA1`..`0xAA` (`0xA3`=FW, `0xA4`=ReadMem, `0xA5`=WriteMem, `0xAA`=Battery)<br>**Gen-B (`0x1227`/`0x4427`)**: `0xB1`..`0xBA` (`0xB3`=FW, `0xB4`=ReadMem, `0xB5`=WriteMem, `0xBA`=Battery) |
| `3..31` | `Payload` | Command-specific arguments, padded with `0x00` (no checksum is used) |

### 3.2 Control-Pipe Request / Response Handshake (`SET_REPORT` + `GET_REPORT`)
Crucially, the `COL04` (`Usage == 0x000E`) firmware processes `0xBA` commands on the **USB Control Pipe (`SET_REPORT (Output)` + `GET_REPORT (Input)`)**, **NOT** on the Interrupt OUT pipe (`WriteFile` is ignored and leaves the `0xBA` input buffer as `00 00...`!):

1. **Send Output Report via USB Control Pipe (`SET_REPORT (Output)`)**:
   - **Windows**: `HidD_SetOutputReport(h, buf, 32)` (`IOCTL_HID_SET_OUTPUT_REPORT`).
   - **Linux**: `ioctl(fd, HIDIOCSOUTPUT(32), buf)` (`_IOWR('H', 0x0B, 32)`).
2. Wait RF round-trip delay (`35–50 ms` wireless / `5 ms` wired).
3. **Read Input Report via USB Control Pipe (`GET_REPORT (Input)`, Report ID `0xBA`)**:
   - Prepare 32-byte buffer with `buf[0] = 0xBA`.
   - **Windows**: `HidD_GetInputReport(h, buf, 32)` on a synchronous (non-overlapped) handle.
   - **Linux**: `ioctl(fd, HIDIOCGINPUT(33), buf)` (`_IOWR('H', 0x0A, 33)`).
   - **OS Prefix Normalization**:
     - On Windows, `hidclass.sys` copies the firmware's raw USB report payload directly starting at `buf[0]` (`[status, b1, b2, ...]`, e.g. `01 01 1e ...` for 30% battery).
     - On Linux, `usbhid` preserves `buf[0] == 0xBA` and writes the USB payload at `buf[1..]` (`[0xBA, status, b1, b2, ...]`).
     - Stripping a leading `0xBA` byte (`if raw[0] == 0xBA { resp = raw[1:] }`) yields an identical normalized `resp` slice on both Windows and Linux.
5. **Status & Busy-Polling**:
   - `resp[0] == 0x01`: **Success** — payload follows in `resp[1..]`.
   - `resp[0] == 0x02`: **Pending / RF Timeout** — while the 4K dongle is waiting for an RF ACK from the wireless mouse, it initially returns `0x02`. Sleep `2 ms` and repeat `GET_INPUT_REPORT` (up to ~80–250 polls = `160–500 ms`). If it remains `0x02`, the wireless mouse is **asleep or powered off** while the 4K dongle remains plugged in.
   - Any other value: Transport/protocol error.

---

## 4. Command Reference

### 4.1 Read Battery & Charging State (`Opcode 0xBA`)
- **Request**:
  ```text
  [0xBA, Target, 0xBA, 0x00, ... 0x00]
  ```
- **Normalized Response (`resp`)**:
  - `resp[0]`: Status (`0x01` = Online, `0x02` = Wireless mouse asleep/offline)
  - `resp[1]`: `BatStatus`
    - `0x01` = Discharging (running on battery)
    - `> 0x01` (typically `0x02`) = Charging
  - `resp[2]`: `Electricity` — Battery percentage `0..100` (`uint8`, clamp to `100`).

### 4.2 Passive Async Notifications (`Usage == 0x0002`, Interrupt `ReadFile`)
Reading interrupt Input packets from the `Usage == 0x0002` (`COL05`) collection requires **zero writes**:

1. **Report ID `0xBB` — Battery / Online / Status Push**:
   | Byte | Field | Meaning |
   | :---: | :--- | :--- |
   | `0` | `ReportID` | `0xBB` |
   | `3` | `DeviceStatus` | Online/connection indicator (`0` = offline/sleep, non-zero = active) |
   | `4` | `SubStatus` | Auxiliary status byte |
   | `5` | `BatStatus` | `1` = Discharging, `> 1` = Charging |
   | `6` | `Electricity` | Battery percentage `0..100` |

2. **Report ID `0xBD` — Profile / DPI / Button Event Push**:
   - `raw[0] == 0xBD`, `raw[1] == 0x01`:
     - `(raw[2] >> 4) & 0x0F`: Active onboard profile index (`CurConfigID`).

### 4.3 Read Firmware Version (`Opcode 0xB3` Mouse / `Opcode 0xB1` Dongle)
- **Mouse Firmware (`0xB3`)**:
  - TX: `[0xBA, Target, 0xB3, 0x00, ...]`
  - RX (`resp[0] == 0x01`): `ver := binary.LittleEndian.Uint32(resp[1:5])`
    - Example: `ver == 222` (`0x000000DE`) displays as `2.22` (`V222` / `fwVersion: 222` in official config JSON).
- **Dongle Firmware (`0xB1`)**:
  - TX: `[0xBA, 0xFF, 0xB1, 0x00, ...]`
  - RX (`resp[0] == 0x01`): `ver := binary.LittleEndian.Uint32(resp[1:5])`

### 4.4 Read Onboard Memory (`Opcode 0xB4`)
- **Request**:
  ```text
  Byte 0:    0xBA
  Byte 1:    Target (0xA5 wireless / 0xFF wired)
  Byte 2:    0xB4
  Byte 3:    Count (number of bytes to read, e.g. 4, 8, 20, 24)
  Byte 4..7: Address (uint32 little-endian)
  Byte 8..31: 0x00
  ```
- **Normalized Response (`resp`)**:
  - `resp[0]`: `0x01` (OK)
  - `resp[1..3]`: Header / echo bytes
  - **`resp[4 : 4+Count]`**: Read memory bytes `[mem[Addr], mem[Addr+1], ...]`.

### 4.5 Write Onboard Memory (`Opcode 0xB5`)
- **Request**:
  ```text
  Byte 0:           0xBA
  Byte 1:           Target (0xA5 wireless / 0xFF wired)
  Byte 2:           0xB5
  Byte 3:           Count (number of data bytes, <= 24)
  Byte 4..7:        Address (uint32 little-endian)
  Byte 8..8+Count:  Data bytes to write
  Remaining..31:    0x00
  ```
- **Normalized Response (`resp`)**:
  - `resp[0] == 0x01` indicates write committed.
  - Note: Official software performs **read-modify-write** on 4-byte aligned words (e.g. reading 4 bytes, replacing 1 byte, writing 4 bytes back) to preserve adjacent flags in flash/SRAM.

### 4.6 Other Direct Opcodes (`MessageManager` Table)
| Key | Opcode | Purpose in `RapooDevice.dll` |
| :---: | :---: | :--- |
| `0x07D1` / `0x07E5` | `0xB1` | Read receiver/device ID / dongle firmware version |
| `0x07D2` | `0xB2` | Read RF pairing / address status |
| `0x07D3` | `0xB3` | Read mouse firmware version |
| `0x07D4` | `0xB4` | Read onboard memory (`[count, addr_le32]`) |
| `0x07D5` | `0xB5` | Write onboard memory (`[count, addr_le32, data...]`) |
| `0x07D7` | `0xB7` | Get active onboard profile (`CurConfig`) |
| `0x07D8` | `0xB8` | Set active onboard profile (`CurConfig`) |
| `0x07DA` | `0xBA` | Get battery percentage & charging status (`getElectricity`) |
| `0x07DF` | `0xD0` | Factory reset (`setReset`) |
| `0x0803`..`0x080A` | `0xC0`..`0xC7` | 4K pairing / RF channel / dongle LED control commands |

---

## 5. Onboard Memory Map (`MouseForRealtek`)

| Address | Size | Field / Layout | Encoding Details |
| :---: | :---: | :--- | :--- |
| `0x0880` | `4`–`8` B | **Polling Rate (`ReturnRate`)** | `mem[0x880]` & `mem[0x882]` = Rate code:<br>`0x08`=125 Hz, `0x04`=250 Hz, `0x02`=500 Hz, `0x01`=1000 Hz,<br>`0x84`=2000 Hz, `0x82`=4000 Hz, `0x81`=8000 Hz.<br>`mem[0x881]` = `Wireless8KSwitch` flag (preserve on RMW). |
| `0x0884` | `1` B | **Lift-Off Distance (`SilentHeight`)** | Raw byte (`0`, `1`, `2` -> 1.0mm..2.0mm depending on sensor). |
| `0x0885` | `1` B | **Motion Sync** | `0x00` = Off, `0x01` = On (RMW 4 bytes at `0x0885`). |
| `0x0888` | `24` B | **X-DPI Table & Stage Count** | `mem[0x888 + i*2]` (`i=0..6`): `uint16` LE X-DPI for stages 1..7.<br>`mem[0x896]` (`+14`): `stageCount - 1` (`0..6` -> 1..7 stages).<br>`mem[0x898]` (`+16`): Current DPI stage (`0..6`). |
| `0x0898` | `4` B | **Current DPI Stage (`CurDPIGear`)** | `mem[0x898]`: `0..6` (0-based index for stages 1..7; preserve `0x899..0x89B` on RMW). |
| `0x08C0` | `1` B | **Key Press Debounce** | `(raw - 2)` ms (`raw >= 2`). |
| `0x08C1` | `1` B | **Key Release Debounce** | `(raw - 2)` ms (`raw >= 2`). |
| `0x08C2` | `1` B | **Wireless Sleep Timeout** | Minutes (`uint8`, e.g. `2`..`120`, default `10`). |
| `0x08C3` | `1` B | **Sensor Correction Flags** | Bit 0 inverted (`(~b)&1`) = Ripple Control;<br>Bit 1 inverted (`((b>>1)^1)&1`) = Angle Snapping / Linear Correction. |
| `0x08C4` | `1` B | **Sensor Angle** | `int8` degrees (`-30`..`+30`). |
| `0x08C5` | `1` B | **Glass Tracking Mode** | `0x00` = Off, non-zero = On. |
| `0x08C6` | `1` B | **Dongle Indicator / Low-Battery LED Mode** | Mode byte. |
| `0x08C8` | `20` B | **Y-DPI Table** | `mem[0x8C8 + i*2]` (`i=0..6`): `uint16` LE Y-DPI for stages 1..7. |
| `0x0600` + `N*0x400` | `0x400` B | **Profile `N` (`N=0..7`) Key Bindings** | Base `0x0600`, `0x0A00`, `0x0E00`, `0x1200`, `0x1600`, `0x1A00`, `0x1E00`, `0x2200`.<br>4-byte key entries at `+0x00, +0x04, +0x08, +0x0C, +0x10, +0x14, +0x18, +0x24, +0x28, +0x2C, +0x30, +0x34, +0x38`. |
| `0x1000` | — | **Macro Storage Area** | Onboard macro data region. |
