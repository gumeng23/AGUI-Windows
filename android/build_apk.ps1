param(
    [string]$Sdk = "",
    [string]$JavaHome = ""
)

$ErrorActionPreference = "Stop"
$Root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$Sdk = if ($Sdk) { $Sdk } else { Join-Path $Root "build\android\sdk" }
$App = Join-Path $Root "android\app\src\main"
$Build = Join-Path $Root "build\android\out"
$Web = Join-Path $Root "web"

function Find-JavaHome {
    $candidates = @()
    if ($JavaHome) { $candidates += $JavaHome }
    if ($env:JAVA_HOME) { $candidates += $env:JAVA_HOME }
    $candidates += "C:\Users\WANDA\AppData\Roaming\.minecraft\runtime\java-runtime-delta"
    $candidates += "C:\Program Files\Android\Android Studio\jbr"
    foreach ($c in $candidates) {
        if ($c -and (Test-Path (Join-Path $c "bin\javac.exe"))) { return $c }
    }
    throw "找不到 JDK。请安装 JDK 17/21，或用 -JavaHome 指定。"
}

$JavaHome = Find-JavaHome
$env:JAVA_HOME = $JavaHome
$Javac = Join-Path $JavaHome "bin\javac.exe"
$Keytool = Join-Path $JavaHome "bin\keytool.exe"
$Jar = Join-Path $JavaHome "bin\jar.exe"

$aapt2 = Join-Path $Sdk "build-tools\35.0.0\aapt2.exe"
$d8 = Join-Path $Sdk "build-tools\35.0.0\d8.bat"
$zipalign = Join-Path $Sdk "build-tools\35.0.0\zipalign.exe"
$apksigner = Join-Path $Sdk "build-tools\35.0.0\apksigner.bat"
$androidJar = Join-Path $Sdk "platforms\android-35\android.jar"
$keystore = Join-Path $Root "build\android\debug.keystore"
$outApk = Join-Path $Root "build\android\agui-webview.apk"

foreach ($tool in @($aapt2, $d8, $zipalign, $apksigner, $androidJar)) {
    if (-not (Test-Path $tool)) { throw "Android 工具链缺少：$tool" }
}

Write-Host "==> 构建 Agui WASM"
Push-Location $Root
try {
    $env:GOOS = "js"
    $env:GOARCH = "wasm"
    & go build -o (Join-Path $Web "agui.wasm") ./cmd/aguiweb
    if ($LASTEXITCODE -ne 0) { throw "go build WASM 失败" }
} finally {
    Remove-Item Env:GOOS -ErrorAction SilentlyContinue
    Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
    Pop-Location
}

$sourcePath = Join-Path $Web "game.ag"
$source = [System.IO.File]::ReadAllText($sourcePath, [System.Text.Encoding]::UTF8)
$json = $source | ConvertTo-Json -Compress
$data = "window.__AGUI_SOURCE__ = " + $json + ";`n"
[System.IO.File]::WriteAllText((Join-Path $Web "game-data.js"), $data, (New-Object System.Text.UTF8Encoding($false)))

$assetsWeb = Join-Path $App "assets\web"
New-Item -ItemType Directory -Force -Path $assetsWeb | Out-Null
Copy-Item (Join-Path $Web "agui.wasm") $assetsWeb -Force
Copy-Item (Join-Path $Web "game-data.js") $assetsWeb -Force
Copy-Item (Join-Path $Web "game.ag") $assetsWeb -Force
Copy-Item (Join-Path $Web "index.html") $assetsWeb -Force
Copy-Item (Join-Path $Web "oc.svg") $assetsWeb -Force
Copy-Item (Join-Path $Web "style.css") $assetsWeb -Force
Copy-Item (Join-Path $Web "wasm_exec.js") $assetsWeb -Force

if (Test-Path $Build) { Remove-Item $Build -Recurse -Force }
New-Item -ItemType Directory -Force -Path (Join-Path $Build "gen") | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $Build "classes") | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $Build "dex") | Out-Null

Write-Host "==> 编译资源与 Java"
& $aapt2 compile --dir (Join-Path $App "res") -o (Join-Path $Build "res.zip")
if ($LASTEXITCODE -ne 0) { throw "aapt2 compile 失败" }
& $aapt2 link -o (Join-Path $Build "app-unsigned.apk") -I $androidJar `
    --manifest (Join-Path $App "AndroidManifest.xml") `
    -R (Join-Path $Build "res.zip") `
    --java (Join-Path $Build "gen") --auto-add-overlay `
    --min-sdk-version 23 --target-sdk-version 34
if ($LASTEXITCODE -ne 0) { throw "aapt2 link 失败" }

& $Javac -source 8 -target 8 -Xlint:-options -bootclasspath $androidJar `
    -d (Join-Path $Build "classes") (Join-Path $App "java\com\agui\webview\MainActivity.java")
if ($LASTEXITCODE -ne 0) { throw "javac 失败" }

$classes = Get-ChildItem (Join-Path $Build "classes") -Recurse -Filter *.class | ForEach-Object { $_.FullName }
& $d8 --lib $androidJar --min-api 23 --output (Join-Path $Build "dex") $classes
if ($LASTEXITCODE -ne 0) { throw "d8 失败" }

Write-Host "==> 组装 APK"
$stage = Join-Path $Build "stage"
New-Item -ItemType Directory -Force -Path $stage | Out-Null
Copy-Item (Join-Path $App "assets") $stage -Recurse -Force
Copy-Item (Join-Path $Build "dex\classes.dex") $stage -Force
$packed = Join-Path $Build "app-packed.apk"
Copy-Item (Join-Path $Build "app-unsigned.apk") $packed
& $Jar --update --file $packed -C $stage .
if ($LASTEXITCODE -ne 0) { throw "jar 打包 assets 失败" }

$aligned = Join-Path $Build "app-aligned.apk"
& $zipalign -p -f 4 $packed $aligned
if ($LASTEXITCODE -ne 0) { throw "zipalign 失败" }

if (-not (Test-Path $keystore)) {
    New-Item -ItemType Directory -Force -Path (Split-Path $keystore) | Out-Null
    & $Keytool -genkeypair -keystore $keystore -storepass android -keypass android `
        -alias androiddebugkey -dname "CN=Android Debug,O=Android,C=US" `
        -keyalg RSA -keysize 2048 -validity 10000
    if ($LASTEXITCODE -ne 0) { throw "生成 debug keystore 失败" }
}

& $apksigner sign --ks $keystore --ks-pass pass:android --key-pass pass:android `
    --out $outApk $aligned
if ($LASTEXITCODE -ne 0) { throw "apksigner 失败" }
& $apksigner verify --verbose $outApk | Select-Object -First 8
& $aapt2 dump badging $outApk | Select-Object -First 8
Write-Host "APK: $outApk"
