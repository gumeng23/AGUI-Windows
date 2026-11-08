#requires -Version 5.1
<#
.SYNOPSIS
    构建 Agui 解释器。
.DESCRIPTION
    生成控制台版 agui.exe 和无控制台版 aguiw.exe。
    加 -Test 会先运行全部 Go 单元测试。
.EXAMPLE
    .\build.ps1
    .\build.ps1 -Test
#>
param(
    [switch]$Test
)

$ErrorActionPreference = 'Stop'
Push-Location $PSScriptRoot
try {
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        throw '没有找到 go，请先安装 Go 1.26+ 并加入 PATH。'
    }

    if ($Test) {
        Write-Host '==> go test ./...' -ForegroundColor Cyan
        go test ./...
        if ($LASTEXITCODE -ne 0) { throw '测试失败。' }
    }

    Write-Host '==> 构建 agui.exe（控制台版）' -ForegroundColor Cyan
    go build -trimpath -o agui.exe ./cmd/agui
    if ($LASTEXITCODE -ne 0) { throw '构建 agui.exe 失败。' }

    Write-Host '==> 构建 aguiw.exe（无控制台版）' -ForegroundColor Cyan
    go build -trimpath -ldflags "-H=windowsgui" -o aguiw.exe ./cmd/agui
    if ($LASTEXITCODE -ne 0) { throw '构建 aguiw.exe 失败。' }

    Write-Host ''
    Write-Host '构建完成：' -ForegroundColor Green
    Write-Host '  agui.exe   控制台版，适合运行/调试脚本'
    Write-Host '  aguiw.exe  无控制台版，agui build 默认用它打包'
    Write-Host ''
    Write-Host '试试：.\agui.exe run examples\hello.ag'
}
finally {
    Pop-Location
}
