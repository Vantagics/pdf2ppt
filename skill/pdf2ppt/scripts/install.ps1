# Install the pdf2ppt binary from a GitHub Release and the skill folder.
# No Go and no git required. Safe to run twice.
#
#   powershell -ExecutionPolicy Bypass -File install.ps1 -SkillDir "$env:USERPROFILE\.cursor\skills"
param(
    [string]$SkillDir = "",
    [switch]$FromSource
)

$ErrorActionPreference = "Stop"
$Repo = "Vantagics/pdf2ppt"

function Fail($msg) {
    Write-Error "pdf2ppt install failed: $msg"
    exit 1
}

$arch = switch ($env:PROCESSOR_ARCHITECTURE) {
    "AMD64" { "amd64" }
    "ARM64" { "arm64" }
    default { "" }
}
if (-not $arch) { Fail "unsupported architecture: $env:PROCESSOR_ARCHITECTURE" }

$destDir = Join-Path $env:LOCALAPPDATA "pdf2ppt"
New-Item -ItemType Directory -Force -Path $destDir | Out-Null
$bin = Join-Path $destDir "pdf2ppt.exe"
$asset = "pdf2ppt_windows_${arch}.exe"

if ($FromSource) {
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) { Fail "--FromSource needs Go" }
    Write-Host "building from source (go install)"
    go install "github.com/$Repo/cmd/pdf2ppt@latest"
    if ($LASTEXITCODE -ne 0) { Fail "go install failed" }
    $gobin = Join-Path (go env GOPATH).Trim() "bin\pdf2ppt.exe"
    if (-not (Test-Path $gobin)) { Fail "go install did not write $gobin" }
    Copy-Item -Force $gobin $bin
} else {
    $work = Join-Path $env:TEMP ("pdf2ppt-install-" + [guid]::NewGuid().ToString("n"))
    New-Item -ItemType Directory -Force -Path $work | Out-Null
    try {
        $sumsUrl = "https://github.com/$Repo/releases/latest/download/SHA256SUMS"
        $assetUrl = "https://github.com/$Repo/releases/latest/download/$asset"
        Write-Host "downloading checksums and $asset"
        Invoke-WebRequest -Uri $sumsUrl -OutFile (Join-Path $work "SHA256SUMS")
        Invoke-WebRequest -Uri $assetUrl -OutFile (Join-Path $work $asset)
        $expected = $null
        foreach ($line in Get-Content (Join-Path $work "SHA256SUMS")) {
            if ($line -match "^([0-9a-fA-F]{64})\s+$([regex]::Escape($asset))$") {
                $expected = $Matches[1].ToLower()
            }
        }
        if (-not $expected) { Fail "SHA256SUMS has no entry for $asset" }
        $actual = (Get-FileHash -Algorithm SHA256 (Join-Path $work $asset)).Hash.ToLower()
        if ($actual -ne $expected) { Fail "SHA256 mismatch for $asset; not installing it" }
        Copy-Item -Force (Join-Path $work $asset) $bin
    } finally {
        Remove-Item -Recurse -Force $work -ErrorAction SilentlyContinue
    }
}

Set-Content -Path (Join-Path $destDir "pdf2ppt-bin.txt") -Value $bin -Encoding ascii
$version = & $bin -v
if ($LASTEXITCODE -ne 0) { Fail "$bin -v failed" }
Write-Host "PDF2PPT_BIN=$bin"
Write-Host "PDF2PPT_VERSION=$version"

function Install-SkillTree([string]$parent) {
    if (-not $parent) { return }
    New-Item -ItemType Directory -Force -Path $parent | Out-Null
    $dest = Join-Path $parent "pdf2ppt"
    if (Test-Path $dest) { Remove-Item -Recurse -Force $dest }
    $stage = Join-Path $env:TEMP ("pdf2ppt-skill-" + [guid]::NewGuid().ToString("n"))
    New-Item -ItemType Directory -Force -Path $stage | Out-Null
    try {
        $tgz = Join-Path $stage "pdf2ppt-skill.tar.gz"
        $url = "https://github.com/$Repo/releases/latest/download/pdf2ppt-skill.tar.gz"
        $gotRelease = $true
        try {
            Invoke-WebRequest -Uri $url -OutFile $tgz
        } catch {
            $gotRelease = $false
        }
        if ($gotRelease) {
            tar -xzf $tgz -C $parent
            if ($LASTEXITCODE -ne 0) { Fail "could not extract pdf2ppt-skill.tar.gz" }
            Write-Host "SKILL_SOURCE=release"
        } else {
            Write-Warning "release has no pdf2ppt-skill.tar.gz; using the main branch tarball"
            $src = Join-Path $stage "src.tar.gz"
            Invoke-WebRequest -Uri "https://github.com/$Repo/archive/refs/heads/main.tar.gz" -OutFile $src
            tar -xzf $src -C $stage
            if ($LASTEXITCODE -ne 0) { Fail "could not extract the main tarball" }
            $skill = Get-ChildItem -Path $stage -Directory -Filter "pdf2ppt-*" | Select-Object -First 1
            if (-not $skill) { Fail "skill folder missing in the main tarball" }
            $srcSkill = Join-Path $skill.FullName "skill\pdf2ppt"
            if (-not (Test-Path $srcSkill)) { Fail "skill folder missing in the main tarball" }
            Copy-Item -Recurse -Force $srcSkill $dest
            Write-Host "SKILL_SOURCE=main"
        }
    } finally {
        Remove-Item -Recurse -Force $stage -ErrorAction SilentlyContinue
    }
    $md = Join-Path $parent "pdf2ppt\SKILL.md"
    if (-not (Test-Path $md)) { Fail "SKILL.md missing at $md" }
    Write-Host "SKILL_MD=$md"
}

$shareSkill = Join-Path $destDir "skill"
Install-SkillTree $shareSkill
if ($SkillDir) { Install-SkillTree $SkillDir }

Write-Host "Install succeeded."
Write-Host "Binary: $bin"
Write-Host "Version: $version"
Write-Host "Run it without changing PATH: & `"$bin`" -v"
if ($env:Path -notlike "*$destDir*") {
    Write-Host "That directory is not on PATH. For this session: `$env:Path = `"$destDir;`$env:Path`""
}
