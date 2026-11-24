#!/usr/bin/env bash
# Launch a published Linux bundle under Xvfb and verify the sidecar demo
# protocol, the rendered window, the window icon, and that the UI still
# repaints. This is the release smoke check.
#
# Requires: Xvfb, xdotool, ImageMagick (import/identify/compare), python3.
set -euo pipefail

APP=${1:-dist/linux-portable/RapooControl}
if [[ ! -x "$APP" ]]; then
  echo "smoke-linux: $APP is not executable; publish the bundle first" >&2
  exit 1
fi
BRIDGE=${BRIDGE:-$(dirname "$APP")/rapoo-bridge}
if [[ ! -x "$BRIDGE" ]]; then
  echo "smoke-linux: sidecar $BRIDGE is not executable" >&2
  exit 1
fi

WORK=$(mktemp -d)
cleanup() {
  rm -rf "$WORK"
}
trap cleanup EXIT

export HOME="$WORK/home"
export XDG_CONFIG_HOME="$WORK/config"
export XDG_DATA_HOME="$WORK/data"
mkdir -p "$HOME" "$XDG_CONFIG_HOME" "$XDG_DATA_HOME"
export RAPOO_LANG=ja
export RAPOO_DEMO=1
# Never let the caller's environment redirect the bridge or swap in the mock.
unset RAPOO_BRIDGE_PATH RAPOO_USE_MOCK RAPOO_START_HIDDEN
if [[ -n "${DOTNET_ROOT:-}" ]]; then
  export DOTNET_ROOT
fi
SHOT=${SHOT:-${TMPDIR:-/tmp}/rapoo-smoke.png}
LOG=$WORK/app.log

cat >"$WORK/check_bridge.py" <<'PY'
import json
import sys

for line in sys.stdin:
    try:
        obj = json.loads(line)
    except json.JSONDecodeError:
        continue
    if obj.get("id") != 2:
        continue
    result = obj.get("result") or {}
    firmware = result.get("firmware") or {}
    assert result.get("demoMode") is True, result
    assert firmware.get("mouseString") == "1.25", firmware
    assert result.get("dpi") and result.get("sensor"), sorted(result)
    print("bridge-demo-ok")
    sys.exit(0)
sys.exit("no snapshot response from sidecar")
PY

# Read _NET_WM_ICON as CARDINAL/32 and require the real app icon: a 50 px or
# larger square, an opaque plate, a pale glyph spanning most of the canvas, and
# transparent rounded corners. xprop's preview output is not parseable, so talk
# to Xlib directly.
cat >"$WORK/check_icon.py" <<'PY'
import ctypes
import ctypes.util
import sys

wid = int(sys.argv[1], 0)
x11 = ctypes.CDLL(ctypes.util.find_library("X11") or "libX11.so.6")
x11.XOpenDisplay.restype = ctypes.c_void_p
x11.XOpenDisplay.argtypes = [ctypes.c_char_p]
x11.XInternAtom.restype = ctypes.c_ulong
x11.XInternAtom.argtypes = [ctypes.c_void_p, ctypes.c_char_p, ctypes.c_int]
x11.XGetWindowProperty.restype = ctypes.c_int
x11.XGetWindowProperty.argtypes = [
    ctypes.c_void_p, ctypes.c_ulong, ctypes.c_ulong, ctypes.c_long, ctypes.c_long,
    ctypes.c_int, ctypes.c_ulong, ctypes.POINTER(ctypes.c_ulong),
    ctypes.POINTER(ctypes.c_int), ctypes.POINTER(ctypes.c_ulong),
    ctypes.POINTER(ctypes.c_ulong), ctypes.POINTER(ctypes.POINTER(ctypes.c_ulong)),
]
x11.XFree.argtypes = [ctypes.c_void_p]

display = x11.XOpenDisplay(None)
if not display:
    sys.exit("smoke-linux: cannot open display for the icon check")
atom = x11.XInternAtom(display, b"_NET_WM_ICON", 0)
actual = ctypes.c_ulong()
fmt = ctypes.c_int()
count = ctypes.c_ulong()
after = ctypes.c_ulong()
data = ctypes.POINTER(ctypes.c_ulong)()
status = x11.XGetWindowProperty(
    display, wid, atom, 0, 1 << 20, 0, 0,
    ctypes.byref(actual), ctypes.byref(fmt), ctypes.byref(count),
    ctypes.byref(after), ctypes.byref(data))
if status != 0 or not data:
    sys.exit("smoke-linux: window has no _NET_WM_ICON")
try:
    # Check the type before reading the buffer: an 8/16-bit property would
    # otherwise be read as an array of CARDINALs.
    if actual.value != 6 or fmt.value != 32:
        sys.exit(
            "smoke-linux: _NET_WM_ICON is not CARDINAL/32 "
            f"(type={actual.value} format={fmt.value})")
    values = [data[i] for i in range(count.value)]
finally:
    x11.XFree(data)

if len(values) < 2:
    sys.exit("smoke-linux: _NET_WM_ICON is empty")
width, height = values[0], values[1]
if width != height or width < 50 or len(values) < 2 + width * height:
    sys.exit(f"smoke-linux: bad icon geometry {width}x{height}")
pixels = values[2:2 + width * height]
total = width * height

opaque = bright = 0
pale_x = []
pale_y = []
for index, argb in enumerate(pixels):
    alpha = (argb >> 24) & 0xFF
    if alpha <= 128:
        continue
    opaque += 1
    if ((argb >> 16) & 0xFF) + ((argb >> 8) & 0xFF) + (argb & 0xFF) > 400:
        bright += 1
        pale_x.append(index % width)
        pale_y.append(index // width)

if opaque < total * 0.3:
    sys.exit(f"smoke-linux: icon plate is missing (opaque={opaque}/{total})")
if bright < total * 0.04:
    sys.exit(f"smoke-linux: icon glyph is missing (pale={bright}/{total})")
if max(pale_x) - min(pale_x) < width * 0.6 or max(pale_y) - min(pale_y) < height * 0.6:
    sys.exit("smoke-linux: icon glyph does not span the plate")
for x, y in ((0, 0), (width - 1, 0), (0, height - 1), (width - 1, height - 1)):
    if ((pixels[y * width + x] >> 24) & 0xFF) >= 16:
        sys.exit(f"smoke-linux: icon corner {x},{y} is not transparent")
print(f"window-icon-ok {width}x{height} bright={bright}")
PY

# Assert the sidecar reaches demo mode and can produce a complete snapshot
# without touching real HID or the user's profile directory.
printf '%s\n' \
  '{"id":1,"method":"demo","params":{"enabled":true}}' \
  '{"id":2,"method":"snapshot"}' |
  timeout 15 "$BRIDGE" 2>"$WORK/bridge.err" |
  python3 "$WORK/check_bridge.py"

# The app must repaint when the UI is driven. A SIGSTOPped or hung process
# keeps its old pixels, so a Settings click that changes nothing fails.
xvfb-run -a -s "-screen 0 1280x900x24" bash -c '
  set -euo pipefail
  "$0" >"$1" 2>&1 &
  app=$!
  cleanup() {
    # A SIGSTOPped child ignores SIGTERM until it is resumed.
    kill -CONT "$app" 2>/dev/null || true
    kill -TERM "$app" 2>/dev/null || true
    for _ in $(seq 1 20); do
      kill -0 "$app" 2>/dev/null || break
      state=$(awk "{print \$3}" "/proc/$app/stat" 2>/dev/null || true)
      [[ "$state" == Z ]] && break
      sleep 0.1
    done
    kill -KILL "$app" 2>/dev/null || true
    wait "$app" 2>/dev/null || true
  }
  trap cleanup EXIT
  wid=""
  for _ in $(seq 1 40); do
    wid=$(xdotool search --onlyvisible --name "Rapoo Control" 2>/dev/null | head -n1 || true)
    [[ -n "$wid" ]] && break
    sleep 0.5
  done
  if [[ -z "$wid" ]]; then
    echo "smoke-linux: app window did not appear" >&2
    exit 1
  fi
  sleep 2
  python3 "$3" "$wid"

  # The GUI must own a live sidecar. Probing the bundled sidecar independently
  # is not enough: the child owned by the GUI could have exited.
  bridge_pid=""
  for _ in $(seq 1 20); do
    for pid in $(cat /proc/"$app"/task/*/children 2>/dev/null || true); do
      if [[ "$(cat "/proc/$pid/comm" 2>/dev/null)" == rapoo-bridge* ]]; then
        bridge_pid=$pid
        break 2
      fi
    done
    sleep 0.25
  done
  if [[ -z "$bridge_pid" ]]; then
    echo "smoke-linux: the GUI did not start its sidecar" >&2
    exit 1
  fi

  import -window "$wid" "$2"
  test -s "$2"

  # Sixth sidebar item, "設定" / Settings.
  xdotool mousemove --window "$wid" 90 264 click 1
  changed=0
  for _ in $(seq 1 24); do
    sleep 0.25
    import -window "$wid" "$4"
    diff=$(compare -metric AE "$2" "$4" null: 2>&1 || true)
    diff=${diff%% *}
    if [[ "$diff" =~ ^[0-9]+$ ]] && (( diff > 1000 )); then
      changed=1
      break
    fi
  done
  if [[ "$changed" != 1 ]]; then
    echo "smoke-linux: UI did not react to the Settings click (frozen or hung?)" >&2
    exit 1
  fi

  kill -0 "$app" 2>/dev/null
  state=$(awk "/^State:/{print \$2}" "/proc/$app/status" 2>/dev/null || true)
  if [[ "$state" == T || "$state" == t ]]; then
    echo "smoke-linux: app process is stopped" >&2
    exit 1
  fi
  xdotool getwindowgeometry "$wid" >/dev/null
  sd=$(identify -format "%[fx:standard_deviation]" "$2")
  python3 -c "import sys; sys.exit(0 if float(sys.argv[1]) > 0.02 else 1)" "$sd"

  # Closing the app must also reap its sidecar.
  kill "$app" 2>/dev/null || true
  wait "$app" 2>/dev/null || true
  for _ in $(seq 1 30); do
    kill -0 "$bridge_pid" 2>/dev/null || break
    sleep 0.1
  done
  if kill -0 "$bridge_pid" 2>/dev/null; then
    echo "smoke-linux: the sidecar outlived the app" >&2
    exit 1
  fi
' "$APP" "$LOG" "$SHOT" "$WORK/check_icon.py" "$WORK/after.png"

echo "smoke-linux: OK ($SHOT)"
