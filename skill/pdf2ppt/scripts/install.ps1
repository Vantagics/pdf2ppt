# Install the pdf2ppt CLI on Windows. Safe to run twice.
# Order: existing binary, local clone, go install, GitHub Release asset.
$ErrorActionPreference = "Stop"
$Repo = "Vantagics/pdf2ppt"
$Module = "github.com/Vantagics/pdf2ppt/cmd/pdf2ppt"

function Have($name) {
    return [bool](Get-Command $name -ErrorAction SilentlyContinue)
}

if (Have "pdf2ppt") {
    $cmd = Get-Command pdf2ppt
    Write-Host "pdf2ppt already on PATH: $($cmd.Source)"
    & pdf2ppt -v
    exit 0
}

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Resolve-Path (Join-Path $scriptDir "..\..\..") -ErrorAction SilentlyContinue
$goMod = $null
if ($repoRoot) { $goMod = Join-Path $repoRoot "go.mod" }
if ($goMod -and (Test-Path $goMod) -and (Have "go")) {
    $modText = Get-Content $goMod -Raw
    if ($modText -match "github.com/Vantagics/pdf2ppt") {
        Write-Host "building pdf2ppt from $repoRoot"
        Push-Location $repoRoot
        try { go install ./cmd/pdf2ppt } finally { Pop-Location }
        $gopath = (go env GOPATH).Trim()
        Write-Host "installed $gopath\bin\pdf2ppt.exe"
        Write-Host "If that directory is not on PATH, add it for this session:"
        Write-Host "  `$env:Path = `"$gopath\bin;`$env:Path`""
        exit 0
    }
}

if (Have "go") {
    Write-Host "go install ${Module}@latest"
    try {
        go install "${Module}@latest"
        $gopath = (go env GOPATH).Trim()
        Write-Host "installed $gopath\bin\pdf2ppt.exe"
        Write-Host "If that directory is not on PATH, add it for this session:"
        Write-Host "  `$env:Path = `"$gopath\bin;`$env:Path`""
        exit 0
    } catch {
        Write-Warning "go install failed; trying a release binary"
    }
}

$arch = switch ($env:PROCESSOR_ARCHITECTURE) {
    "AMD64" { "amd64" }
    "ARM64" { "arm64" }
    default { "" }
}
if (-not $arch) {
    Write-Error "unsupported architecture: $env:PROCESSOR_ARCHITECTURE"
}
$asset = "pdf2ppt_windows_${arch}.exe"
$url = "https://github.com/$Repo/releases/latest/download/$asset"
$destDir = Join-Path $env:LOCALAPPDATA "pdf2ppt"
New-Item -ItemType Directory -Force -Path $destDir | Out-Null
$dest = Join-Path $destDir "pdf2ppt.exe"
Write-Host "downloading $url"
Invoke-WebRequest -Uri $url -OutFile $dest
Write-Host "installed $dest"
if ($env:Path -notlike "*$destDir*") {
    Write-Host "Add it to PATH for this session:"
    Write-Host "  `$env:Path = `"$destDir;`$env:Path`""
}
& $dest -v
