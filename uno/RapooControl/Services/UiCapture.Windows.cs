#if WINDOWS
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Media.Imaging;
using Windows.Graphics.Imaging;
using Windows.Storage.Streams;

namespace RapooControl.Services;

internal static class UiCapture
{
    public static async Task CaptureAsync(FrameworkElement element)
    {
        try
        {
            var bitmap = new RenderTargetBitmap();
            await bitmap.RenderAsync(element);
            var buffer = await bitmap.GetPixelsAsync();
            var pixels = new byte[buffer.Length];
            using (var reader = DataReader.FromBuffer(buffer))
            {
                reader.ReadBytes(pixels);
            }

            using var stream = new InMemoryRandomAccessStream();
            var encoder = await BitmapEncoder.CreateAsync(BitmapEncoder.PngEncoderId, stream);
            encoder.SetPixelData(
                BitmapPixelFormat.Bgra8,
                BitmapAlphaMode.Premultiplied,
                (uint)bitmap.PixelWidth,
                (uint)bitmap.PixelHeight,
                96,
                96,
                pixels);
            await encoder.FlushAsync();
            stream.Seek(0);

            using var dataReader = new DataReader(stream.GetInputStreamAt(0));
            await dataReader.LoadAsync((uint)stream.Size);
            var png = new byte[stream.Size];
            dataReader.ReadBytes(png);
            var dir = Path.Combine(
                Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
                "rapoo-vt9pro-4k");
            Directory.CreateDirectory(dir);
            await File.WriteAllBytesAsync(Path.Combine(dir, "uno-ui.png"), png);
        }
        catch (Exception ex)
        {
            System.Diagnostics.Debug.WriteLine($"ui capture: {ex}");
        }
    }
}
#endif
