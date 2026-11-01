param([string]$Exe = ".\agui.exe", [string]$Script = "examples\table_type_test.ag")
$p = Start-Process -FilePath $Exe -ArgumentList @("run", $Script) -PassThru -RedirectStandardOutput "build\type.out" -RedirectStandardError "build\type.err"
Start-Sleep -Seconds 2
Add-Type -AssemblyName System.Windows.Forms
[System.Windows.Forms.SendKeys]::SendWait("777{ENTER}")
Wait-Process -Id $p.Id -Timeout 10
Get-Content "build\type.out"
Get-Content "build\type.err"
