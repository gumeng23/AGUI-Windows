# Agui exe 签名说明

Agui 打包出的 exe 默认没有 Authenticode 签名，Windows SmartScreen 可能提示“未知发布者”。签名不能消除所有提示，但可以证明文件未被篡改，并让系统显示发布者名称。

## 本机自签名（开发测试）

仓库提供 `scripts/sign.ps1`：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\sign.ps1 `
  -Exe .\hello.exe `
  -CertOut .\hello.cer
```

脚本会：

1. 在 `Cert:\CurrentUser\My` 查找 `CN=Agui Self-Signed` 代码签名证书；
2. 没有则用 `New-SelfSignedCertificate` 创建；
3. 用 `Set-AuthenticodeSignature` 给 exe 签名；
4. 导出公钥证书 `.cer`。

签名后可以用下面的命令确认：

```powershell
Get-AuthenticodeSignature .\hello.exe | Format-List Status,StatusMessage,SignerCertificate
```

自签名证书默认不受信任，状态通常是 `UnknownError`。仅在本机测试时，可以把证书加入当前用户的“受信任的根证书颁发机构”：

```powershell
Import-Certificate -FilePath .\hello.cer -CertStoreLocation Cert:\CurrentUser\Root
```

> 只应在自己的测试机上信任自签名证书；不要把它分发给最终用户，也不要把私钥提交到仓库。

## 正式签名

发布给最终用户时，建议购买可信的代码签名证书（OV/EV），并用 Windows SDK 的 `signtool.exe`：

```powershell
signtool sign /fd SHA256 /tr http://timestamp.digicert.com /td SHA256 `
  /f my-cert.pfx /p <密码> .\hello.exe
```

EV 证书通常配合硬件令牌或云签名服务。签名后仍需时间积累 SmartScreen 声誉；新证书首次发布也可能被提示。

## 与 `agui build` 的顺序

先打包，再签名：

```powershell
.\agui.exe build app.ag -o app.exe --icon assets\app.ico --file-version 1.0.0.0
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\sign.ps1 -Exe app.exe
```

不要先签名再执行 `agui build`，因为打包会重写 exe，使原签名失效。
