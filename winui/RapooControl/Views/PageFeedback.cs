using RapooControl.ViewModels;

namespace RapooControl.Views;

public sealed class PageFeedback : ObservableObject
{
    private string _message = string.Empty;

    public bool IsBusy { get; private set; }

    public string Message
    {
        get => _message;
        set
        {
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
        Message = "処理中…";
        OnPropertyChanged(nameof(IsBusy));
        try
        {
            await operation();
            Message = completed;
        }
        catch (Exception ex)
        {
            Message = ex.Message;
        }
        finally
        {
            IsBusy = false;
            OnPropertyChanged(nameof(IsBusy));
        }
    }
}
