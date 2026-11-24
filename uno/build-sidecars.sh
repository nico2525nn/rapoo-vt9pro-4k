#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

mkdir -p build/bin
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
  go build -trimpath -ldflags='-s -w' -o build/bin/rapoo-bridge ./cmd/rapoo-bridge
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
  go build -trimpath -ldflags='-s -w' -o build/bin/rapoo-bridge.exe ./cmd/rapoo-bridge
chmod +x build/bin/rapoo-bridge

file build/bin/rapoo-bridge | grep -q 'x86-64'
file build/bin/rapoo-bridge.exe | grep -q 'x86-64'

# The stamp must describe what Go actually embedded. An untracked file is
# enough to make vcs.modified=true, so git status would disagree with the
# binaries and the Windows packager would reject the build.
read_stamp() {
  local bin=$1 field=$2
  go version -m "$bin" | sed -n "s/^[[:space:]]*build[[:space:]]\+vcs\.$field=//p" | head -n1
}

linux_rev=$(read_stamp build/bin/rapoo-bridge revision)
windows_rev=$(read_stamp build/bin/rapoo-bridge.exe revision)
linux_modified=$(read_stamp build/bin/rapoo-bridge modified)
windows_modified=$(read_stamp build/bin/rapoo-bridge.exe modified)

if [[ ! "$linux_rev" =~ ^[0-9a-f]{40}$ ]]; then
  echo "sidecars: no VCS revision embedded; build from a git checkout" >&2
  exit 1
fi
if [[ "$linux_rev" != "$windows_rev" || "$linux_modified" != "$windows_modified" ]]; then
  echo "sidecars: embedded VCS stamps disagree ($linux_rev/$linux_modified vs $windows_rev/$windows_modified)" >&2
  exit 1
fi
case "$linux_modified" in
  false) modified=0 ;;
  true) modified=1 ;;
  *) echo "sidecars: unexpected vcs.modified value '$linux_modified'" >&2; exit 1 ;;
esac

printf 'revision=%s\nmodified=%s\n' "$linux_rev" "$modified" > build/bin/sidecar-vcs.txt
echo "sidecars: $(file build/bin/rapoo-bridge build/bin/rapoo-bridge.exe | tr '\n' ' ')"
echo "sidecar-vcs: revision=$linux_rev modified=$modified"
if [[ "$modified" != 0 ]]; then
  echo "sidecars: WARNING built from a dirty tree (untracked files count); Windows packaging will reject this stamp" >&2
fi
