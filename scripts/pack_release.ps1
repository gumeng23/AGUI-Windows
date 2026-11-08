#requires -Version 5.1
<#
.SYNOPSIS
    打包 Windows 发布压缩包（白名单）。
.DESCRIPTION
    只打包用户真正需要的东西：解释器、License、README、assets、docs 下的语言/签名文档与截图、examples。
    内部文档（例如 docs/promo.md）、调试符号、旧压缩包一律不会进入发布包，脚本会做一次泄漏检查。
.EXAMPLE
    .\scripts\pack_release.ps1
.EXAMPLE
    .\scripts\pack_release.ps1 -Version 0.9.2
#>
param(
    [string]$Version = ''
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
Push-Location $root
try {
    if (-not $Version) {
        # 版本号取 git tag（v0.9.1 → 0.9.1）。
        # 不要用 `agui.exe version` 的输出来推断：在 PowerShell 里捕获它的 stdout 不可靠。
        $tag = & git describe --tags --abbrev=0 2>$null
        if ($LASTEXITCODE -eq 0 -and $tag) { $Version = ($tag -replace '^v', '').Trim() }
    }
    if (-not $Version) {
        throw '无法确定版本号（没有 git tag），请用 -Version 指定，例如 -Version 0.9.2。'
    }

    $outDir = Join-Path $root 'build\release'
    $stage = Join-Path $outDir "stage-$Version"
    if (Test-Path $stage) { Remove-Item $stage -Recurse -Force }
    New-Item -ItemType Directory -Path $stage -Force | Out-Null

    # 白名单：新增文件必须同时加到这里，否则不会进包
    $files = @('agui.exe', 'aguiw.exe', 'LICENSE', 'README.md')
    foreach ($f in $files) {
        $src = Join-Path $root $f
        if (-not (Test-Path $src)) { throw "缺少 $f，请先运行 build.ps1。" }
        Copy-Item $src $stage
    }
    foreach ($d in @('assets', 'examples')) {
        Copy-Item (Join-Path $root $d) (Join-Path $stage $d) -Recurse
    }
    $docsOut = Join-Path $stage 'docs'
    New-Item -ItemType Directory -Path $docsOut -Force | Out-Null
    foreach ($f in @('language.md', 'signing.md')) {
        Copy-Item (Join-Path $root "docs\$f") (Join-Path $docsOut $f)
    }
    Copy-Item (Join-Path $root 'docs\images') (Join-Path $docsOut 'images') -Recurse

    # 泄漏检查：内部文档、构建垃圾都不该出现在用户下载的包里
    $leaks = @()
    $leaks += Get-ChildItem $stage -Recurse -File -Include 'promo.md'
    $leaks += Get-ChildItem $stage -Recurse -File -Include '*.pdb', '*.debug', '*.zip', '*.vsix'
    if ($leaks.Count -gt 0) {
        throw ('发布包里出现了不该有的文件：' + (($leaks | ForEach-Object { $_.FullName }) -join ', '))
    }

    $zip = Join-Path $outDir "Agui-$Version-win64.zip"
    if (Test-Path $zip) { Remove-Item $zip -Force }
    Compress-Archive -Path (Join-Path $stage '*') -DestinationPath $zip -CompressionLevel Optimal
    Remove-Item $stage -Recurse -Force

    $mb = [math]::Round((Get-Item $zip).Length / 1MB, 1)
    Write-Host "✓ 已生成 $zip（$mb MB）" -ForegroundColor Green
    Write-Host "  上传：gh release upload v$Version `"$zip`"" -ForegroundColor DarkGray
}
finally {
    Pop-Location
}
