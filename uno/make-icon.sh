#!/usr/bin/env bash
# Regenerate uno/RapooControl/Resources/app.ico from the two icon sources.
#
# The SVGs are composited over a transparent 1024 px canvas in a headless
# browser, so the ICO comes from the same geometry the app uses and keeps real
# alpha. Frames 16..128 are 32 bpp BMP with an AND mask; 256 is PNG compressed.
#
# Requires: Chrome or Chromium (CHROME=... to override), python3 with Pillow
# and numpy.
set -euo pipefail
cd "$(dirname "$0")/.."

CHROME=${CHROME:-$(command -v google-chrome || command -v chromium || command -v chromium-browser || true)}
if [[ -z "$CHROME" ]]; then
  echo "make-icon: Chrome/Chromium not found (set CHROME)" >&2
  exit 1
fi
if ! python3 -c 'import PIL, numpy' 2>/dev/null; then
  echo "make-icon: python3 needs Pillow and numpy" >&2
  exit 1
fi

ICONS=uno/RapooControl/Assets/Icons
OUT=uno/RapooControl/Resources/app.ico
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

cp "$ICONS/icon.svg" "$ICONS/icon_foreground.svg" "$TMP/"
cat >"$TMP/page.html" <<'HTML'
<!doctype html><meta charset="utf-8">
<style>
  html, body { margin: 0; padding: 0; overflow: hidden; background: transparent; }
  img { position: absolute; left: 0; top: 0; width: 1024px; height: 1024px; }
</style>
<img src="icon.svg" alt="">
<img src="icon_foreground.svg" alt="">
HTML

"$CHROME" --headless=new --disable-gpu --no-sandbox --hide-scrollbars \
  --default-background-color=00000000 --force-device-scale-factor=1 \
  --window-size=1024,1024 --screenshot="$TMP/icon.png" \
  "file://$TMP/page.html" >/dev/null 2>&1

python3 - "$TMP/icon.png" "$OUT" <<'PY'
import io
import struct
import sys

import numpy as np
from PIL import Image

src, out = sys.argv[1], sys.argv[2]
im = Image.open(src).convert("RGBA")
if im.size != (1024, 1024):
    sys.exit(f"make-icon: unexpected render size {im.size}")
if im.getpixel((0, 0))[3] != 0:
    sys.exit("make-icon: render is opaque; check --default-background-color")


def downscale(n: int) -> Image.Image:
    # Pillow premultiplies RGBA for resampling itself; doing it here as well
    # darkens the plate edge.
    return im.resize((n, n), Image.LANCZOS)


def bmp_frame(img: Image.Image) -> bytes:
    n = img.size[0]
    px = np.asarray(img)
    bgra = np.stack([px[..., 2], px[..., 1], px[..., 0], px[..., 3]], axis=-1)
    xor = bgra[::-1].tobytes()
    stride = ((n + 31) // 32) * 4
    mask = bytearray(stride * n)
    for y in range(n):
        row = px[n - 1 - y]
        for x in range(n):
            if row[x, 3] < 128:
                mask[y * stride + x // 8] |= 0x80 >> (x % 8)
    header = struct.pack("<IiiHHIIiiII", 40, n, n * 2, 1, 32, 0, len(xor), 0, 0, 0, 0)
    return header + xor + bytes(mask)


def png_frame(img: Image.Image) -> bytes:
    buf = io.BytesIO()
    img.save(buf, "PNG", optimize=True)
    return buf.getvalue()


sizes = [16, 20, 24, 32, 40, 48, 64, 128, 256]
frames = []
for n in sizes:
    img = downscale(n)
    frames.append((n, img, png_frame(img) if n == 256 else bmp_frame(img)))

header = struct.pack("<HHH", 0, 1, len(frames))
offset = 6 + 16 * len(frames)
dirs = b""
blob = b""
for n, _img, data in frames:
    entry = 0 if n == 256 else n
    dirs += struct.pack("<BBBBHHII", entry, entry, 0, 0, 1, 32, len(data), offset)
    blob += data
    offset += len(data)
open(out, "wb").write(header + dirs + blob)

print(f"make-icon: wrote {out} ({len(header + dirs + blob)} bytes)")
for n, img, data in frames:
    a = np.asarray(img)
    corner = int(a[0, 0, 3])
    bright = int(((a[..., 3] > 128) & (a[..., :3].sum(axis=-1) > 400)).sum())
    mask_ok = n == 256 or any(data[40 + n * n * 4:])
    print(f"  {n:3d}px corner_alpha={corner:3d} bright={bright:6d} bytes={len(data):6d}"
          f" kind={'png' if n == 256 else 'bmp'}")
    if corner > 8 or bright < 8 or not mask_ok:
        sys.exit(f"make-icon: frame {n} looks wrong")
PY

echo "make-icon: done"
