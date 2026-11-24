<#
.SYNOPSIS
    Build the Windows release bundles (slim and portable).

.DESCRIPTION
    slim     - framework-dependent; requires the .NET 10 Desktop Runtime and
               a Windows App Runtime 2.x (2.5.1 or later, x64).
    portable - self-contained; no runtime installation required.

    Run this on Windows after uno/build-sidecars.sh has produced
    build\bin\rapoo-bridge.exe and build\bin\sidecar-vcs.txt.

    Pass -ExpectedSidecarRevision <sha> to bind the package to a specific
    release commit.
#>
param(
    [string]$Version = "",
    [string]$ExpectedSidecarRevision = ""
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
if (-not $ExpectedSidecarRevision) {
    throw "Pass -ExpectedSidecarRevision <sha> from the release checkout; an unpinned sidecar is not allowed."
}

$sidecar = Join-Path $root "build\bin\rapoo-bridge.exe"
if (-not (Test-Path $sidecar)) {
    throw "Missing $sidecar. Run ./uno/build-sidecars.sh on Linux first."
}

$stampPath = Join-Path $root "build\bin\sidecar-vcs.txt"
if (-not (Test-Path $stampPath)) {
    throw "Missing $stampPath. Run ./uno/build-sidecars.sh on Linux and sync the stamp with the sidecar."
}
$stamp = Get-Content $stampPath | ConvertFrom-StringData
$revision = $stamp.revision
if ($revision -notmatch '^[0-9a-f]{40}$') {
    throw "Invalid sidecar revision '$revision' in $stampPath."
}
if ($stamp.modified -ne "0") {
    throw "The sidecar was built from a dirty tree (modified=$($stamp.modified))."
}
if ($ExpectedSidecarRevision -and
    $ExpectedSidecarRevision -ne $revision) {
    throw "The sidecar revision $revision does not match the expected $ExpectedSidecarRevision."
}
$sidecarText = [Text.Encoding]::UTF8.GetString([IO.File]::ReadAllBytes($sidecar))
if (-not $sidecarText.Contains("vcs.revision=$revision") -or
    -not $sidecarText.Contains("vcs.modified=false")) {
    throw "The sidecar binary does not match $stampPath."
}
Write-Host "sidecar revision $revision (clean)"

$newestGoSource = Get-ChildItem `
    (Join-Path $root "cmd"), (Join-Path $root "internal"), (Join-Path $root "pkg") `
    -Recurse -File -Filter *.go -ErrorAction SilentlyContinue |
    Sort-Object LastWriteTime -Descending | Select-Object -First 1
if ($newestGoSource -and
    (Get-Item $sidecar).LastWriteTime -lt $newestGoSource.LastWriteTime) {
    throw "The sidecar is older than $($newestGoSource.FullName). Rebuild it with ./uno/build-sidecars.sh."
}

$go = Get-Command go -ErrorAction SilentlyContinue
if ($go) {
    $vcs = & $go.Source version -m $sidecar | Select-String 'vcs\.(revision|modified)='
    if (-not $vcs) {
        throw "The sidecar has no VCS stamp; rebuild it from the release checkout."
    }
    if ($vcs -match 'vcs\.modified=true') {
        throw "The sidecar was built from a dirty tree; rebuild it from the release commit."
    }
    Write-Host ($vcs -join [Environment]::NewLine)
} else {
    Write-Warning "Go is not installed; the sidecar VCS stamp was not checked (freshness was checked)."
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

    if ($Name -eq "slim") {
        $depsPath = Join-Path $out "RapooControl.deps.json"
        $deps = Get-Content $depsPath -Raw | ConvertFrom-Json
        $wasdk = $deps.libraries.PSObject.Properties.Name |
            Where-Object { $_ -like "Microsoft.WindowsAppSDK/*" } |
            Select-Object -First 1
        if (-not $wasdk) {
            throw "No Microsoft.WindowsAppSDK entry in $depsPath"
        }
        $wasdkVersion = ($wasdk -split '/')[-1]
        $readmeText = Get-Content (Join-Path $out "README.md") -Raw
        $parts = $wasdkVersion -split '\.'
        if ($parts.Count -lt 2) {
            throw "Could not derive the Windows App Runtime series from $wasdkVersion."
        }
        $series = "$($parts[0]).$($parts[1])"
        $requirement = "Windows App Runtime $($parts[0]).x ($wasdkVersion or later, x64)"
        $installer = "https://aka.ms/windowsappsdk/$series/$wasdkVersion/windowsappruntimeinstall-x64.exe"
        if ($readmeText -notmatch [regex]::Escape($requirement) -or
            $readmeText -notmatch [regex]::Escape($installer)) {
            throw "README must contain '$requirement' and the $wasdkVersion x64 installer URL."
        }
        Write-Host "Windows App SDK $wasdkVersion; README runtime prerequisite matches."
    }

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
