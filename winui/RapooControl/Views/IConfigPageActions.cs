namespace RapooControl.Views;

public interface IConfigPageActions
{
    bool CanReload { get; }
    bool CanApply { get; }
    event EventHandler? ActionsChanged;
    Task ReloadAsync();
    Task ApplyAsync();
}
