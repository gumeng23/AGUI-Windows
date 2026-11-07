$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$env:ANDROID_HOME = Join-Path $root "build\sdk"
$env:ANDROID_SDK_ROOT = $env:ANDROID_HOME
$env:GOTMPDIR = Join-Path $root "build\gotmp"
$env:TMP = $env:GOTMPDIR
$env:TEMP = $env:GOTMPDIR
New-Item -ItemType Directory -Force $env:GOTMPDIR | Out-Null

$jdk = "C:\Users\WANDA\AppData\Roaming\.minecraft\runtime\java-runtime-delta\bin"
$gomobile = "C:\Users\WANDA\go\bin\gomobile.exe"
if (Test-Path $jdk) { $env:PATH = "$jdk;$env:PATH" }

Set-Location $root
& $gomobile bind -target=android/arm64 -androidapi 23 -javapkg "com.agui.runtime" -o "android/runtime/agui.aar" "./mobile"
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
Write-Host "AAR rebuilt: android/runtime/agui.aar"
