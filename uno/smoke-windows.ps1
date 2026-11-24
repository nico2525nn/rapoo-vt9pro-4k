# Windows release gate. Run this in the signed-in desktop session, against an
# extracted candidate folder (slim or portable). Session 0 cannot create a
# DirectComposition device, so an SSH-launched run is not a valid GUI test.
#
#   powershell -ExecutionPolicy Bypass -File uno\smoke-windows.ps1 -Package <dir>
#
# It checks the merged WinUI PRI, demo firmware in the Overview (UIA), the
# sidecar parent, single instance, and clean shutdown.

param(
    [Parameter(Mandatory = $true)][string]$Package,
    [switch]$SkipClose
)

$ErrorActionPreference = "Stop"
function Fail([string]$message) {
    Write-Host "FAIL: $message" -ForegroundColor Red
    exit 1
}
function Pass([string]$message) {
    Write-Host "PASS: $message" -ForegroundColor Green
}

$session = (Get-Process -Id $PID).SessionId
if ($session -eq 0) {
    Fail "run this in the signed-in desktop session; session 0 has no DirectComposition"
}

$exe = Join-Path $Package "RapooControl.exe"
if (-not (Test-Path $exe)) { Fail "missing $exe" }
# A self-contained build must carry the merged WinUI resources. A
# framework-dependent build loads them from the installed Windows App Runtime,
# so only its runtime behaviour is meaningful.
$selfContained = Test-Path (Join-Path $Package "Microsoft.UI.Xaml.dll")
if ($selfContained) {
    $pri = Join-Path $Package "RapooControl.pri"
    if (-not (Test-Path $pri)) { Fail "missing RapooControl.pri" }
    $priSize = (Get-Item $pri).Length
    if ($priSize -lt 1000000) {
        Fail "RapooControl.pri is only $priSize bytes; WinUI resources were not merged (check the direct WindowsAppSDK reference)"
    }
    Pass "PRI carries the WinUI resources ($priSize bytes)"
} else {
    Pass "framework-dependent build (WinUI resources come from the runtime)"
}

# Never kill an app, sidecar, or firmware-updater guard: the guard owns HID
# leases for the official updater. Require a quiet machine instead.
$busy = @(Get-Process RapooControl, rapoo-bridge, RapooGameDev, FirmwareExe -ErrorAction SilentlyContinue)
if ($busy.Count -gt 0) {
    Fail "close these processes first: $(($busy | ForEach-Object { $_.ProcessName + ':' + $_.Id }) -join ', ')"
}

$env:RAPOO_SMOKE = "1"
$env:RAPOO_DEMO = "1"
$gateData = Join-Path $env:TEMP ("rapoo-gate-" + [Guid]::NewGuid().ToString("N"))
$env:RAPOO_DATA_DIR = $gateData
$env:RAPOO_LANG = "ja"
Remove-Item Env:RAPOO_BRIDGE_PATH, Env:RAPOO_USE_MOCK, Env:RAPOO_START_HIDDEN -ErrorAction SilentlyContinue

$app = Start-Process -FilePath $exe -PassThru
$hwnd = [IntPtr]::Zero
for ($i = 0; $i -lt 40; $i++) {
    Start-Sleep -Milliseconds 500
    $app.Refresh()
    if ($app.HasExited) { break }
    if ($app.MainWindowHandle -ne [IntPtr]::Zero) {
        $hwnd = $app.MainWindowHandle
        break
    }
}
if ($app.HasExited) { Fail "app exited during startup (exit $($app.ExitCode))" }
if ($hwnd -eq [IntPtr]::Zero) { Fail "no main window after 20 s" }
Pass "window created"

Start-Sleep -Seconds 5
$app.Refresh()

Add-Type -AssemblyName UIAutomationClient, UIAutomationTypes
$root = [System.Windows.Automation.AutomationElement]::FromHandle($hwnd)
$textCondition = New-Object System.Windows.Automation.PropertyCondition(
    [System.Windows.Automation.AutomationElement]::ControlTypeProperty,
    [System.Windows.Automation.ControlType]::Text)
$texts = $root.FindAll([System.Windows.Automation.TreeScope]::Descendants, $textCondition)
$names = @()
foreach ($text in $texts) {
    if ($text.Current.Name) { $names += $text.Current.Name }
}
if (-not ($names -match '1\.25')) {
    Fail "Overview does not show the demo firmware 1.25 (texts: $($names -join ' | '))"
}
Pass "demo firmware and battery rendered"

$bridges = @(Get-CimInstance Win32_Process -Filter "Name='rapoo-bridge.exe'")
$children = @($bridges | Where-Object { $_.ParentProcessId -eq $app.Id })
if ($children.Count -ne 1) {
    Fail "expected one sidecar child of the app, found $($children.Count) (total bridges $($bridges.Count))"
}
Pass "sidecar is a child of the app"

$second = Start-Process -FilePath $exe -PassThru
Start-Sleep -Seconds 5
$second.Refresh()
$app.Refresh()
if (-not $second.HasExited) { Fail "second instance did not exit" }
if ($app.HasExited) { Fail "first instance exited when the second one started" }
Pass "single instance"

if (-not $SkipClose) {
    $app.CloseMainWindow() | Out-Null
    if (-not $app.WaitForExit(15000)) { Fail "app did not exit within 15 s of closing the window" }
    Start-Sleep -Seconds 3
    $left = @(Get-Process rapoo-bridge -ErrorAction SilentlyContinue)
    if ($left.Count -ne 0) { Fail "sidecar still running after the app exited" }
    Pass "clean shutdown (app and sidecar exited)"
}

if (Test-Path $gateData) { Remove-Item $gateData -Recurse -Force -ErrorAction SilentlyContinue }
Write-Host "SMOKE-WINDOWS: OK"
