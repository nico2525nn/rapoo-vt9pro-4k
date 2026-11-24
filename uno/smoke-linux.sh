#!/usr/bin/env bash
# Launch a published Linux bundle under Xvfb and verify both the sidecar demo
# protocol and the rendered window. This is the release smoke check.
#
# Requires: Xvfb, xdotool, ImageMagick (import/identify), python3.
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

# Assert the sidecar reaches demo mode and can produce a complete snapshot
# without touching real HID or the user's profile directory.
printf '%s\n' \
  '{"id":1,"method":"demo","params":{"enabled":true}}' \
  '{"id":2,"method":"snapshot"}' |
  timeout 15 "$BRIDGE" 2>"$WORK/bridge.err" |
  python3 "$WORK/check_bridge.py"

xvfb-run -a -s "-screen 0 1280x900x24" bash -c '
  set -euo pipefail
  "$0" >"$1" 2>&1 &
  app=$!
  trap "kill $app 2>/dev/null || true" EXIT
  have_icon=0
  for _ in $(seq 1 30); do
    for wid in $(xdotool search --name "Rapoo Control" 2>/dev/null || true); do
      icon=$(xprop -id "$wid" _NET_WM_ICON 2>/dev/null || true)
      if [[ -n "$icon" && "$icon" != *"not found"* && ${#icon} -gt 200 ]]; then
        have_icon=1
        break
      fi
    done
    [[ "$have_icon" == 1 ]] && break
    sleep 0.5
  done
  if [[ "$have_icon" != 1 ]]; then
    echo "smoke-linux: window has no _NET_WM_ICON (app icon not applied)" >&2
    exit 1
  fi
  sleep 2
  import -window root "$2"
  test -s "$2"
  test "$(stat -c%s "$2")" -gt 10000
' "$APP" "$LOG" "$SHOT"

echo "smoke-linux: OK ($SHOT)"
