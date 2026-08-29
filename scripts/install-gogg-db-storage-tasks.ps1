$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$attachTaskName = "GOGG Database VHDX Attach"
$attachScript = "F:\gogg-data\attach-gogg-db.ps1"

$identity = [Security.Principal.WindowsIdentity]::GetCurrent()
$principalCheck = [Security.Principal.WindowsPrincipal]::new($identity)
if (-not $principalCheck.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    throw "Installer must run elevated."
}
if (-not (Test-Path -LiteralPath $attachScript -PathType Leaf)) {
    throw "Attach script is missing: $attachScript"
}

$attachAction = New-ScheduledTaskAction `
    -Execute "$env:SystemRoot\System32\WindowsPowerShell\v1.0\powershell.exe" `
    -Argument '-NoProfile -WindowStyle Hidden -ExecutionPolicy Bypass -File "F:\gogg-data\attach-gogg-db.ps1" -AttachOnly'
$attachPrincipal = New-ScheduledTaskPrincipal `
    -UserId $identity.Name -LogonType Interactive -RunLevel Highest

$settings = New-ScheduledTaskSettingsSet `
    -AllowStartIfOnBatteries `
    -DontStopIfGoingOnBatteries `
    -StartWhenAvailable `
    -MultipleInstances IgnoreNew `
    -ExecutionTimeLimit (New-TimeSpan -Minutes 5)

$attachTask = New-ScheduledTask `
    -Action $attachAction `
    -Principal $attachPrincipal `
    -Settings $settings `
    -Description "Validate F: and attach the GOGG database VHDX when Ubuntu requests it."

Register-ScheduledTask `
    -TaskName $attachTaskName `
    -InputObject $attachTask `
    -Force | Out-Null

$legacyMountTask = Get-ScheduledTask -TaskName "GOGG Database VHDX Mount" -ErrorAction SilentlyContinue
if ($null -ne $legacyMountTask) {
    Unregister-ScheduledTask -TaskName $legacyMountTask.TaskName -Confirm:$false
}

Write-Output "Registered $attachTaskName as an on-demand task with no logon trigger."
