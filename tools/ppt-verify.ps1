# Opens a PPTX in Microsoft PowerPoint via COM, reports whether PowerPoint
# had to repair it, and exports slides to PNG.
#
# Run on Windows, in an STA PowerShell host (the default for powershell.exe):
#
#   powershell -STA -File tools/ppt-verify.ps1 -Path deck.pptx -Slides 1,23 -OutDir ppt-verify
#
# -Slides accepts "1,2,23", "1 2 23", or a PowerShell array. A bare int array
# is not used: from an already-open PowerShell host, -File binds "1,2,23" as
# one string and [int] would read it as 1223.
#
# A hung repair dialog is stopped after -TimeoutSec. PowerPoint processes
# that this script started are closed. PowerPoint that was already open is
# left alone.
param(
    [Parameter(Mandatory = $true)][string]$Path,
    [string]$Slides = "1",
    [string]$OutDir = "ppt-verify",
    [int]$TimeoutSec = 90
)

$ErrorActionPreference = "Stop"
$Path = (Resolve-Path -LiteralPath $Path).Path
New-Item -ItemType Directory -Force -Path $OutDir | Out-Null
$OutDir = (Resolve-Path -LiteralPath $OutDir).Path

if ([Threading.Thread]::CurrentThread.GetApartmentState() -ne "STA") {
    Write-Output "RESULT: FAIL"
    Write-Output "MESSAGE: start this script with powershell -STA"
    exit 2
}

$existing = @(Get-Process POWERPNT -ErrorAction SilentlyContinue | ForEach-Object { $_.Id })
$killer = Start-Job -ArgumentList $TimeoutSec, $PID, $existing -ScriptBlock {
    param($sec, $parent, $already)
    Start-Sleep -Seconds $sec
    if (Get-Process -Id $parent -ErrorAction SilentlyContinue) {
        Get-Process POWERPNT -ErrorAction SilentlyContinue |
            Where-Object { $already -notcontains $_.Id } |
            Stop-Process -Force
    }
}

$ppt = $null
$pres = $null
try {
    $ppt = New-Object -ComObject PowerPoint.Application
    # ppAlertsNone = 1. A repair dialog must not block the export.
    $ppt.DisplayAlerts = 1
    try {
        # ReadOnly, WithWindow = false.
        $pres = $ppt.Presentations.Open($Path, -1, 0, 0)
    } catch {
        $hex = "0x{0:X8}" -f ($_.Exception.HResult -band 0xffffffff)
        Write-Output "RESULT: FAIL"
        Write-Output "HRESULT: $hex"
        Write-Output "MESSAGE: $($_.Exception.Message)"
        exit 1
    }

    $w = [double]$pres.PageSetup.SlideWidth
    $h = [double]$pres.PageSetup.SlideHeight
    Write-Output "RESULT: OK"
    Write-Output "SLIDES: $($pres.Slides.Count)"
    Write-Output ("WIDTH_PT: {0}" -f $w)
    Write-Output ("HEIGHT_PT: {0}" -f $h)

    $expW = 1920
    $expH = [int][Math]::Round(1920 * $h / [Math]::Max($w, 1))
    if ($expH -lt 1) { $expH = 1 }
    $slideNums = @()
    foreach ($part in ($Slides -split '[,\s;]+')) {
        if ($part -match '^\d+$') { $slideNums += [int]$part }
    }
    if ($slideNums.Count -eq 0) { $slideNums = @(1) }
    foreach ($n in $slideNums) {
        if ($n -lt 1 -or $n -gt $pres.Slides.Count) {
            Write-Output "EXPORT_FAIL: $n out of range"
            continue
        }
        $dest = Join-Path $OutDir ("slide-{0}.png" -f $n)
        $pres.Slides.Item($n).Export($dest, "PNG", $expW, $expH)
        Write-Output "EXPORT: $n=$dest"
    }
    exit 0
} catch {
    $hex = "0x{0:X8}" -f ($_.Exception.HResult -band 0xffffffff)
    Write-Output "RESULT: FAIL"
    Write-Output "HRESULT: $hex"
    Write-Output "MESSAGE: $($_.Exception.Message)"
    exit 1
} finally {
    Stop-Job $killer -ErrorAction SilentlyContinue
    Remove-Job $killer -Force -ErrorAction SilentlyContinue
    if ($pres -ne $null) {
        $pres.Close()
        [void][Runtime.InteropServices.Marshal]::ReleaseComObject($pres)
    }
    if ($ppt -ne $null) {
        $ppt.Quit()
        [void][Runtime.InteropServices.Marshal]::ReleaseComObject($ppt)
    }
    Get-Process POWERPNT -ErrorAction SilentlyContinue |
        Where-Object { $existing -notcontains $_.Id } |
        Stop-Process -Force -ErrorAction SilentlyContinue
}
