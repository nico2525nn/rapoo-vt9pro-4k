#!/usr/bin/env bash
# Build the Linux release bundles.
#
#   slim     - framework-dependent; requires the .NET 10 Desktop Runtime
#   portable - self-contained and IL-trimmed; no runtime installation
#
# Both bundles contain the Go sidecar and are published as tar.xz and tar.gz.
set -euo pipefail

cd "$(dirname "$0")/.."

VERSION=$(sed -n 's:.*<Version>\(.*\)</Version>.*:\1:p' uno/RapooControl/RapooControl.csproj | head -n1)
if [[ -z "$VERSION" ]]; then
  echo "package-linux: could not read the app version" >&2
  exit 1
fi

DOTNET=${DOTNET:-dotnet}
DIST=${DIST:-dist}
SLIM="$DIST/linux-slim"
PORTABLE="$DIST/linux-portable"
PROJECT=uno/RapooControl/RapooControl.csproj
PREFIX="rapoo-control-uno-v$VERSION-linux-amd64"

command -v "$DOTNET" >/dev/null || { echo "package-linux: dotnet not found" >&2; exit 1; }
command -v xz >/dev/null || { echo "package-linux: xz not found" >&2; exit 1; }

./uno/build-sidecars.sh

rm -rf "$SLIM" "$PORTABLE"
mkdir -p "$DIST"

echo "==> Linux slim (framework-dependent)"
"$DOTNET" publish "$PROJECT" \
  -f net10.0-desktop -c Release -r linux-x64 \
  --self-contained false -p:PublishReadyToRun=false \
  -o "$SLIM"

echo "==> Linux portable (self-contained, trimmed)"
"$DOTNET" publish "$PROJECT" \
  -f net10.0-desktop -c Release -r linux-x64 \
  --self-contained true -p:PublishTrimmed=true -p:PublishReadyToRun=false \
  -o "$PORTABLE"

for dir in "$SLIM" "$PORTABLE"; do
  cp README.md LICENSE "$dir/"
done

echo "==> archives"
tar -C "$SLIM" -cf - . | xz -9 -T0 > "$DIST/$PREFIX-slim.tar.xz"
tar -C "$SLIM" -cf - . | gzip -9 -n > "$DIST/$PREFIX-slim.tar.gz"
tar -C "$PORTABLE" -cf - . | xz -9 -T0 > "$DIST/$PREFIX-portable.tar.xz"
tar -C "$PORTABLE" -cf - . | gzip -9 -n > "$DIST/$PREFIX-portable.tar.gz"

echo "==> checksums"
(
  cd "$DIST"
  sha256sum "$PREFIX"-slim.tar.xz "$PREFIX"-slim.tar.gz \
            "$PREFIX"-portable.tar.xz "$PREFIX"-portable.tar.gz
) > "$DIST/checksums-linux.txt"

ls -lh "$DIST/$PREFIX"-*
