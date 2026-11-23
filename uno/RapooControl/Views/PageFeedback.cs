using RapooControl.Services;
using RapooControl.ViewModels;

namespace RapooControl.Views;

public sealed class PageFeedback : ObservableObject
{
    public const string Applied = "適用しました。";
    public const string Reloaded = "再読み込みしました。";
    public const string NoChanges = "変更はありません。";

    private string _message = string.Empty;

    public bool IsBusy { get; private set; }

    public string Message
    {
        get => _message;
        set
        {
            value = LocalizationService.Instance.Error(value);
            if (SetProperty(ref _message, value))
            {
                OnPropertyChanged(nameof(HasMessage));
            }
        }
    }

    public bool HasMessage => Message.Length > 0;

    public async Task RunAsync(Func<Task> operation, string completed = "")
    {
        if (IsBusy)
        {
            return;
        }

        IsBusy = true;
        Message = LocalizationService.Instance.T("処理中…");
        OnPropertyChanged(nameof(IsBusy));
        try
        {
            await operation();
            Message = completed;
        }
        catch (Exception ex)
        {
            Message = LocalizationService.Instance.Error(ex.Message);
        }
        finally
        {
            IsBusy = false;
            OnPropertyChanged(nameof(IsBusy));
        }
    }
}
