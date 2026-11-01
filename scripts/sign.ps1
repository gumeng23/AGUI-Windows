param(
    [Parameter(Mandatory = $true)][string]$Exe,
    [string]$Subject = "CN=Agui Self-Signed",
    [string]$CertOut = ""
)

$ErrorActionPreference = "Stop"
$exePath = (Resolve-Path $Exe).Path

$cert = Get-ChildItem Cert:\CurrentUser\My -CodeSigningCert |
    Where-Object { $_.Subject -eq $Subject } |
    Select-Object -First 1
if (-not $cert) {
    Write-Host "Creating self-signed code signing certificate: $Subject"
    $cert = New-SelfSignedCertificate `
        -Type CodeSigningCert `
        -Subject $Subject `
        -CertStoreLocation Cert:\CurrentUser\My `
        -HashAlgorithm SHA256 `
        -KeyUsage DigitalSignature `
        -NotAfter (Get-Date).AddYears(5)
}

$sig = Set-AuthenticodeSignature -FilePath $exePath -Certificate $cert -HashAlgorithm SHA256
if ($sig.Status -notin @("Valid", "UnknownError")) {
    throw "Signing failed: $($sig.Status) $($sig.StatusMessage)"
}

if ([string]::IsNullOrWhiteSpace($CertOut)) {
    $CertOut = [System.IO.Path]::ChangeExtension($exePath, ".cer")
}
Export-Certificate -Cert $cert -FilePath $CertOut | Out-Null

Write-Host "Signed: $exePath"
Write-Host "Status: $($sig.Status) (UnknownError is expected until the certificate is trusted)"
Write-Host "Public certificate: $CertOut"
Write-Host ""
Write-Host "To trust the certificate on this machine (local testing only):"
Write-Host "  Import-Certificate -FilePath `"$CertOut`" -CertStoreLocation Cert:\CurrentUser\Root"
Write-Host "SmartScreen reputation is independent of self-signing; a new self-signed exe may still warn."
