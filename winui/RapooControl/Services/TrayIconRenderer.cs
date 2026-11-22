using System.Drawing;
using System.Drawing.Imaging;
using System.Runtime.InteropServices;

namespace RapooControl.Services;

/// <summary>Inputs for the tray icon, mirroring the Go icon.VisualSpec.</summary>
internal readonly record struct TrayIconSpec(
    bool Connected,
    bool HasBattery,
    int Percentage,
    bool Charging,
    bool Sleeping);

/// <summary>
/// Draws the same crisp, transparent battery-percentage icon as the Wails dock
/// tray: large 7-segment digits with a dark halo and a colored bar along the
/// bottom. Rendering at the system small-icon size keeps the text readable.
/// </summary>
internal static class TrayIconRenderer
{
    private const int SmCxSmIcon = 49;

    private static readonly Color Outline = Color.FromArgb(245, 10, 15, 26);
    private static readonly Color BarTrack = Color.FromArgb(230, 15, 23, 42);
    private static readonly Color TextNormal = Color.FromArgb(255, 255, 255, 255);
    private static readonly Color TextCharging = Color.FromArgb(255, 125, 211, 252);
    private static readonly Color TextWarn = Color.FromArgb(255, 253, 224, 71);
    private static readonly Color TextLow = Color.FromArgb(255, 252, 165, 165);
    private static readonly Color TextSleep = Color.FromArgb(225, 203, 213, 225);
    private static readonly Color TextOffline = Color.FromArgb(220, 148, 163, 184);
    private static readonly Color BarGood = Color.FromArgb(255, 34, 197, 94);
    private static readonly Color BarCharging = Color.FromArgb(255, 56, 189, 248);
    private static readonly Color BarWarn = Color.FromArgb(255, 250, 204, 21);
    private static readonly Color BarLow = Color.FromArgb(255, 239, 68, 68);
    private static readonly Color BarSleep = Color.FromArgb(230, 100, 116, 139);

    [DllImport("user32.dll")]
    private static extern int GetSystemMetrics(int index);

    [DllImport("user32.dll")]
    private static extern bool DestroyIcon(IntPtr handle);

    public static int GetTraySize()
    {
        var size = GetSystemMetrics(SmCxSmIcon);
        return Math.Max(16, size);
    }

    public static Icon Render(TrayIconSpec spec) => Render(GetTraySize(), spec);

    public static Icon Render(int size, TrayIconSpec spec)
    {
        using var bitmap = RenderBitmap(size, spec);
        var handle = bitmap.GetHicon();
        try
        {
            using var temporary = Icon.FromHandle(handle);
            return (Icon)temporary.Clone();
        }
        finally
        {
            _ = DestroyIcon(handle);
        }
    }

    internal static Bitmap RenderBitmap(int size, TrayIconSpec spec)
    {
        size = Math.Max(16, size);
        var bitmap = new Bitmap(size, size, PixelFormat.Format32bppArgb);

        var text = "--";
        if (spec.Connected && spec.HasBattery)
        {
            text = FormatPercentage(Math.Clamp(spec.Percentage, 0, 100));
        }

        var (foreground, bar) = ChoosePalette(spec);

        var barHeight = Math.Max(2, size / 8);
        var barY = size - barHeight;
        var barPadX = Math.Max(1, size / 16);
        var barWidth = size - (2 * barPadX);

        // A dark track keeps the bar readable on light and dark taskbars.
        FillRectangle(bitmap, barPadX, barY - 1, barWidth, barHeight + 1, BarTrack);
        if (spec.Connected && spec.HasBattery)
        {
            var percentage = Math.Clamp(spec.Percentage, 0, 100);
            var fillWidth = barWidth * percentage / 100;
            if (percentage > 0 && fillWidth < 1)
            {
                fillWidth = 1;
            }
            if (fillWidth > 0)
            {
                FillRectangle(bitmap, barPadX, barY, fillWidth, barHeight, bar);
            }
        }

        DrawOutlinedDigits(bitmap, size, barY - 1, text, foreground);
        return bitmap;
    }

    private static string FormatPercentage(int value) =>
        value == 100 ? "100" : value >= 10 ? $"{value / 10}{value % 10}" : $"{value}";

    private static (Color Foreground, Color Bar) ChoosePalette(TrayIconSpec spec)
    {
        if (!spec.Connected || !spec.HasBattery)
        {
            return (TextOffline, BarSleep);
        }
        if (spec.Charging)
        {
            return (TextCharging, BarCharging);
        }
        if (spec.Sleeping)
        {
            return spec.Percentage <= 15 ? (TextLow, BarLow) : (TextSleep, BarSleep);
        }
        return spec.Percentage switch
        {
            <= 15 => (TextLow, BarLow),
            <= 30 => (TextWarn, BarWarn),
            _ => (TextNormal, BarGood),
        };
    }

    private static void DrawOutlinedDigits(Bitmap bitmap, int canvas, int availableHeight, string text, Color foreground)
    {
        var mask = new bool[canvas * canvas];
        var isThree = text.Length >= 3;
        int digitWidth;
        int digitHeight;
        int oneWidth;
        int stroke;
        int gap;

        switch (canvas)
        {
            case <= 17:
                digitHeight = 10;
                if (isThree)
                {
                    (oneWidth, digitWidth, stroke, gap) = (2, 4, 1, 1);
                }
                else
                {
                    (oneWidth, digitWidth, stroke, gap) = (3, 6, 2, 1);
                }
                break;
            case <= 21:
                digitHeight = 12;
                if (isThree)
                {
                    (oneWidth, digitWidth, stroke, gap) = (2, 5, 2, 1);
                }
                else
                {
                    (oneWidth, digitWidth, stroke, gap) = (3, 7, 2, 2);
                }
                break;
            case <= 26:
                digitHeight = 15;
                if (isThree)
                {
                    (oneWidth, digitWidth, stroke, gap) = (3, 6, 2, 1);
                }
                else
                {
                    (oneWidth, digitWidth, stroke, gap) = (4, 9, 3, 2);
                }
                break;
            default:
                var scale = canvas / 16;
                digitHeight = Math.Min(availableHeight - (2 * scale), 10 * scale);
                if (isThree)
                {
                    oneWidth = 2 * scale;
                    digitWidth = (4 * scale) + 1;
                    stroke = Math.Max(2, scale);
                    gap = Math.Max(1, scale);
                }
                else
                {
                    oneWidth = 3 * scale;
                    digitWidth = 6 * scale;
                    stroke = 2 * scale;
                    gap = Math.Max(2, scale);
                }
                break;
        }

        if (digitHeight > availableHeight - 2)
        {
            digitHeight = availableHeight - 2;
        }

        var totalWidth = 0;
        for (var i = 0; i < text.Length; i++)
        {
            if (i > 0)
            {
                totalWidth += gap;
            }
            totalWidth += text[i] == '1' ? oneWidth : digitWidth;
        }

        var startX = (canvas - totalWidth) / 2;
        var startY = (availableHeight - digitHeight) / 2;
        if (startY < 1)
        {
            startY = 1;
        }

        var currentX = startX;
        foreach (var character in text)
        {
            var width = character == '1' ? oneWidth : digitWidth;
            RasterizeGlyph(mask, canvas, currentX, startY, width, digitHeight, stroke, character);
            currentX += width + gap;
        }

        var outlineRadius = canvas >= 32 ? 2 : 1;
        for (var y = 0; y < canvas; y++)
        {
            for (var x = 0; x < canvas; x++)
            {
                if (mask[(y * canvas) + x])
                {
                    continue;
                }
                if (HasNeighbor(mask, canvas, x, y, outlineRadius))
                {
                    bitmap.SetPixel(x, y, Outline);
                }
            }
        }

        for (var y = 0; y < canvas; y++)
        {
            for (var x = 0; x < canvas; x++)
            {
                if (mask[(y * canvas) + x])
                {
                    bitmap.SetPixel(x, y, foreground);
                }
            }
        }
    }

    private static bool HasNeighbor(bool[] mask, int stride, int centerX, int centerY, int radius)
    {
        for (var dy = -radius; dy <= radius; dy++)
        {
            var y = centerY + dy;
            if (y < 0 || y >= stride)
            {
                continue;
            }
            for (var dx = -radius; dx <= radius; dx++)
            {
                if (dx == 0 && dy == 0)
                {
                    continue;
                }
                var x = centerX + dx;
                if (x < 0 || x >= stride)
                {
                    continue;
                }
                if (mask[(y * stride) + x])
                {
                    return true;
                }
            }
        }
        return false;
    }

    private static void RasterizeGlyph(bool[] mask, int stride, int x0, int y0, int width, int height, int stroke, char character)
    {
        void Set(int x, int y)
        {
            if (x >= 0 && x < stride && y >= 0 && y < stride)
            {
                mask[(y * stride) + x] = true;
            }
        }

        void Box(int x, int y, int boxWidth, int boxHeight)
        {
            for (var yy = y; yy < y + boxHeight; yy++)
            {
                for (var xx = x; xx < x + boxWidth; xx++)
                {
                    Set(xx, yy);
                }
            }
        }

        if (character == '-')
        {
            var middleY = y0 + ((height - stroke) / 2);
            Box(x0, middleY, width, stroke);
            return;
        }

        if (character == '1')
        {
            var stemX = x0 + width - stroke;
            if (stemX < x0)
            {
                stemX = x0;
            }
            Box(stemX, y0, Math.Min(stroke, width), height);
            if (width > stroke)
            {
                Box(x0, y0 + Math.Min(1, height / 6), width - stroke, stroke);
            }
            return;
        }

        var segments = character switch
        {
            '0' => SegA | SegB | SegC | SegD | SegE | SegF,
            '2' => SegA | SegB | SegG | SegE | SegD,
            '3' => SegA | SegB | SegG | SegC | SegD,
            '4' => SegF | SegG | SegB | SegC,
            '5' => SegA | SegF | SegG | SegC | SegD,
            '6' => SegA | SegF | SegG | SegE | SegC | SegD,
            '7' => SegA | SegB | SegC,
            '8' => SegA | SegB | SegC | SegD | SegE | SegF | SegG,
            '9' => SegA | SegF | SegB | SegG | SegC | SegD,
            _ => 0,
        };
        if (segments == 0)
        {
            return;
        }

        var middle = y0 + ((height - stroke) / 2);
        var topHeight = middle - y0 + stroke;
        var bottomHeight = (y0 + height) - middle;

        if ((segments & SegA) != 0)
        {
            Box(x0, y0, width, stroke);
        }
        if ((segments & SegG) != 0)
        {
            Box(x0, middle, width, stroke);
        }
        if ((segments & SegD) != 0)
        {
            Box(x0, y0 + height - stroke, width, stroke);
        }
        if ((segments & SegF) != 0)
        {
            Box(x0, y0, stroke, topHeight);
        }
        if ((segments & SegB) != 0)
        {
            Box(x0 + width - stroke, y0, stroke, topHeight);
        }
        if ((segments & SegE) != 0)
        {
            Box(x0, middle, stroke, bottomHeight);
        }
        if ((segments & SegC) != 0)
        {
            Box(x0 + width - stroke, middle, stroke, bottomHeight);
        }
    }

    private static void FillRectangle(Bitmap bitmap, int x, int y, int width, int height, Color color)
    {
        var right = Math.Min(bitmap.Width, x + width);
        var bottom = Math.Min(bitmap.Height, y + height);
        for (var yy = Math.Max(0, y); yy < bottom; yy++)
        {
            for (var xx = Math.Max(0, x); xx < right; xx++)
            {
                bitmap.SetPixel(xx, yy, color);
            }
        }
    }

    private const int SegA = 1 << 0;
    private const int SegB = 1 << 1;
    private const int SegC = 1 << 2;
    private const int SegD = 1 << 3;
    private const int SegE = 1 << 4;
    private const int SegF = 1 << 5;
    private const int SegG = 1 << 6;
}
