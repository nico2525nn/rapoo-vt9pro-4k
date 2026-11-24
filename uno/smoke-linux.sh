#!/usr/bin/env bash
# Launch a published Linux bundle under Xvfb and verify that the window and
# demo-mode UI render. This is the same smoke check used for release review.
#
# Requires: Xvfb, xdotool, ImageMagick (import/identify).
set -euo pipefail

APP=${1:-dist/linux-portable/RapooControl}
if [[ ! -x "$APP" ]]; then
  echo "smoke-linux: $APP is not executable; publish the bundle first" >&2
  exit 1
fi

SHOT=${SHOT:-/tmp/opencode/rapoo-smoke.png}
export RAPOO_LANG=ja
export RAPOO_DEMO=1
if [[ -n "${DOTNET_ROOT:-}" ]]; then
  export DOTNET_ROOT
fi

xvfb-run -a -s "-screen 0 1280x900x24" bash -c '
  set -euo pipefail
  "$0" >/tmp/opencode/rapoo-smoke.log 2>&1 &
  app=$!
  trap "kill $app 2>/dev/null || true" EXIT
  for _ in $(seq 1 30); do
    if xdotool search --name "Rapoo Control" >/dev/null 2>&1; then
      break
    fi
    sleep 0.5
  done
  xdotool search --name "Rapoo Control" >/dev/null
  sleep 2
  import -window root "$1"
  test -s "$1"
  test "$(stat -c%s "$1")" -gt 10000
' "$APP" "$SHOT"

echo "smoke-linux: OK ($SHOT)"
