<#
.SYNOPSIS
    Build the Windows release bundles (slim and portable).

.DESCRIPTION
    slim     - framework-dependent; requires the .NET 10 Desktop Runtime and
               the Windows App Runtime 1.8.
    portable - self-contained; no runtime installation required.

    Run this on Windows after uno/build-sidecars.sh has produced
    build\bin\rapoo-bridge.exe.
#>
param(
    [string]$Version = ""
)

$ErrorActionPreference = "Stop"

$root = Split-Path $PSScriptRoot
$project = Join-Path $root "uno\RapooControl\RapooControl.csproj"
$dist = Join-Path $root "dist"

if (-not $Version) {
    $Version = (Select-String -Path $project -Pattern '<Version>([^<]+)</Version>' |
        Select-Object -First 1).Matches[0].Groups[1].Value
}
if (-not $Version) {
    throw "Could not read the app version from $project"
}

$sidecar = Join-Path $root "build\bin\rapoo-bridge.exe"
if (-not (Test-Path $sidecar)) {
    throw "Missing $sidecar. Run ./uno/build-sidecars.sh on Linux first."
}

$dotnet = Join-Path $env:USERPROFILE ".dotnet\dotnet.exe"
if (-not (Test-Path $dotnet)) {
    $dotnet = "dotnet"
}

$sevenZip = Join-Path $env:ProgramFiles "7-Zip\7z.exe"
if (-not (Test-Path $sevenZip)) {
    $sevenZip = "7z"
}

$prefix = "rapoo-control-uno-v$Version-windows-amd64"
$common = @(
    "-f", "net10.0-windows10.0.26100",
    "-c", "Release",
    "-p:Version=$Version",
    "-p:InformationalVersion=$Version",
    "-p:ApplicationDisplayVersion=$Version"
)

function Publish-Bundle {
    param(
        [string]$Name,
        [string]$Profile,
        [bool]$SelfContained,
        [bool]$WindowsAppSdkSelfContained
    )

    $out = Join-Path $dist $Name
    if (Test-Path $out) {
        Remove-Item $out -Recurse -Force
    }

    Write-Host "==> $Name"
    & $dotnet publish $project @common `
        "-p:PublishProfile=$Profile" `
        "-p:SelfContained=$($SelfContained.ToString().ToLower())" `
        "-p:WindowsAppSDKSelfContained=$($WindowsAppSdkSelfContained.ToString().ToLower())" `
        "-p:PublishReadyToRun=false" `
        "-p:PublishTrimmed=false" `
        "-o" $out
    if ($LASTEXITCODE -ne 0) {
        throw "dotnet publish failed for $Name"
    }

    Copy-Item (Join-Path $root "README.md") $out
    Copy-Item (Join-Path $root "LICENSE") $out

    $sevenZipPath = Join-Path $dist "$prefix-$Name.7z"
    if (Test-Path $sevenZipPath) {
        Remove-Item $sevenZipPath -Force
    }
    & $sevenZip a -t7z -m0=lzma2 -mx=9 -md=64m -mfb=64 -ms=on `
        $sevenZipPath (Join-Path $out "*") | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw "7-Zip failed for $Name"
    }

    $zipPath = Join-Path $dist "$prefix-$Name.zip"
    if (Test-Path $zipPath) {
        Remove-Item $zipPath -Force
    }
    Compress-Archive -Path (Join-Path $out "*") -DestinationPath $zipPath -CompressionLevel Optimal
}

New-Item -ItemType Directory -Force -Path $dist | Out-Null

Publish-Bundle -Name "slim" -Profile "win-x64" -SelfContained $false -WindowsAppSdkSelfContained $false
Publish-Bundle -Name "portable" -Profile "win-x64-portable" -SelfContained $true -WindowsAppSdkSelfContained $true

Write-Host "==> checksums"
$assets = Get-ChildItem $dist -Filter "$prefix-*.7z" | Sort-Object Name
$assets += Get-ChildItem $dist -Filter "$prefix-*.zip" | Sort-Object Name
$assets | ForEach-Object {
    $hash = (Get-FileHash $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
    "$hash  $($_.Name)"
} | Set-Content -Encoding ASCII (Join-Path $dist "checksums-windows.txt")

Get-ChildItem $dist -Filter "$prefix-*" | Format-Table Name, Length
