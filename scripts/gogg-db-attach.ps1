param(
    [string]$Distro = "Ubuntu",
    [switch]$AttachOnly
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$expectedDiskSerial = "1951LU401119"
$expectedDiskUniqueId = "5001B448B1E5DC21"
$expectedDiskModel = "WD My Passport 25F3"
$expectedDiskSize = 1024175636480
$expectedVolumeGuid = "\\?\Volume{014df7cc-19b7-435f-b071-ecc00c741678}\"
$vhdxPath = "F:\gogg-data\gogg-db.vhdx"
$wslMountHelper = "/home/zrt/apps/gogg/scripts/gogg-db-wsl-mount.sh"

function Get-DatabaseDevice {
    $previousPreference = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    $output = & "$env:SystemRoot\System32\wsl.exe" `
        -d $Distro -u root -- /bin/bash $wslMountHelper device 2>&1
    $exitCode = $LASTEXITCODE
    $ErrorActionPreference = $previousPreference

    return [PSCustomObject]@{
        ExitCode = $exitCode
        Output = ($output | Out-String).Trim()
    }
}

$identity = [Security.Principal.WindowsIdentity]::GetCurrent()
$principal = [Security.Principal.WindowsPrincipal]::new($identity)
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    throw "Run this script from an elevated PowerShell window."
}

$targetPartition = Get-Partition -DriveLetter F -ErrorAction Stop
$targetDisk = Get-Disk -Number $targetPartition.DiskNumber -ErrorAction Stop
$targetVolume = Get-Volume -DriveLetter F -ErrorAction Stop

if (($targetDisk.SerialNumber.Trim()) -ne $expectedDiskSerial -or
    $targetDisk.UniqueId -ne $expectedDiskUniqueId -or
    $targetDisk.FriendlyName -ne $expectedDiskModel -or
    $targetDisk.BusType -ne "USB" -or
    $targetDisk.Size -ne $expectedDiskSize) {
    throw "Safety stop: F: is not the verified GOGG database disk."
}
if ($targetPartition.AccessPaths -notcontains $expectedVolumeGuid) {
    throw "Safety stop: F: volume GUID does not match."
}
if ($targetDisk.IsBoot -or $targetDisk.IsSystem -or
    $targetDisk.IsOffline -or $targetDisk.IsReadOnly) {
    throw "Safety stop: F: has a protected or unusable disk state."
}
if ($targetVolume.FileSystem -ne "NTFS" -or
    $targetVolume.FileSystemLabel -ne "GOGG-DATA" -or
    $targetVolume.HealthStatus -ne "Healthy" -or
    $targetVolume.OperationalStatus -ne "OK") {
    throw "Safety stop: F: is not the healthy GOGG-DATA NTFS volume."
}
if (-not (Test-Path -LiteralPath $vhdxPath -PathType Leaf)) {
    throw "Safety stop: $vhdxPath is missing."
}

if (-not $AttachOnly) {
    $device = Get-DatabaseDevice
    if ($device.ExitCode -eq 0) {
        Write-Output "GOGG database VHDX is already attached as $($device.Output)."
        exit 0
    }
}

$previousPreference = $ErrorActionPreference
$ErrorActionPreference = "Continue"
$attachOutput = & "$env:SystemRoot\System32\wsl.exe" `
    --mount $vhdxPath --vhd --bare 2>&1
$attachExitCode = $LASTEXITCODE
$ErrorActionPreference = $previousPreference
if ($attachExitCode -ne 0) {
    throw "Unable to attach $vhdxPath.`n$($attachOutput | Out-String)"
}

if ($AttachOnly) {
    Write-Output "Attached $vhdxPath; Ubuntu will mount it by UUID."
    exit 0
}

for ($attempt = 0; $attempt -lt 40; $attempt++) {
    Start-Sleep -Milliseconds 500
    $device = Get-DatabaseDevice
    if ($device.ExitCode -eq 0) {
        Write-Output "Attached $vhdxPath as $($device.Output)."
        exit 0
    }
}

throw "Attached VHDX did not expose the expected ext4 UUID."
