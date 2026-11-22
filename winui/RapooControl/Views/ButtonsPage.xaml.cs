using Microsoft.UI.Xaml.Controls;

namespace RapooControl.Views;

public sealed record ButtonAssignment(string Button, string Action);

public sealed partial class ButtonsPage : Page
{
    public IReadOnlyList<ButtonAssignment> Assignments { get; } =
    [
        new("左クリック", "左クリック"),
        new("右クリック", "右クリック"),
        new("ホイール", "中クリック"),
        new("進む", "ブラウザー: 進む"),
        new("戻る", "ブラウザー: 戻る"),
        new("DPI +", "DPI ステージ +"),
        new("DPI -", "DPI ステージ -"),
        new("ボタン 8", "キーボード: Ctrl+C (モック)"),
        new("ボタン 9", "キーボード: Ctrl+V (モック)"),
        new("ボタン 10", "マクロ: 未割り当て"),
    ];

    public ButtonsPage()
    {
        InitializeComponent();
    }
}
