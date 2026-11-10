# Install the pdf2ppt release binary and this skill. No Go or git.
#   iex (irm https://raw.githubusercontent.com/Vantagics/pdf2ppt/main/skill/pdf2ppt/scripts/install.ps1)
# Optional: $env:PDF2PPT_SKILL_DIR = one skills directory (skips auto-detect).
$ErrorActionPreference = "Stop"
$Repo = "Vantagics/pdf2ppt"

function Fail($msg) { throw "pdf2ppt install failed: $msg" }

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
$work = Join-Path $env:TEMP ("pdf2ppt-" + [guid]::NewGuid().ToString("n"))
New-Item -ItemType Directory -Force -Path $work | Out-Null
try {
    Write-Host "downloading $asset"
    Invoke-WebRequest "https://github.com/$Repo/releases/latest/download/SHA256SUMS" -OutFile (Join-Path $work "SHA256SUMS")
    Invoke-WebRequest "https://github.com/$Repo/releases/latest/download/$asset" -OutFile (Join-Path $work $asset)
    $expected = $null
    foreach ($line in Get-Content (Join-Path $work "SHA256SUMS")) {
        if ($line -match "^([0-9a-fA-F]{64})\s+$([regex]::Escape($asset))$") { $expected = $Matches[1].ToLower() }
    }
    if (-not $expected) { Fail "SHA256SUMS has no entry for $asset" }
    $actual = (Get-FileHash -Algorithm SHA256 (Join-Path $work $asset)).Hash.ToLower()
    if ($actual -ne $expected) { Fail "SHA256 mismatch for $asset; not installing it" }
    Copy-Item -Force (Join-Path $work $asset) $bin

    $tgz = Join-Path $work "skill.tar.gz"
    $got = $true
    try { Invoke-WebRequest "https://github.com/$Repo/releases/latest/download/pdf2ppt-skill.tar.gz" -OutFile $tgz } catch { $got = $false }
    $src = Join-Path $work "pdf2ppt"
    if ($got) {
        tar -xzf $tgz -C $work
        if ($LASTEXITCODE -ne 0) { Fail "could not extract the skill archive" }
    } else {
        Write-Warning "using the main branch tarball for the skill"
        $srcTgz = Join-Path $work "src.tar.gz"
        Invoke-WebRequest "https://github.com/$Repo/archive/refs/heads/main.tar.gz" -OutFile $srcTgz
        tar -xzf $srcTgz -C $work
        if ($LASTEXITCODE -ne 0) { Fail "could not extract the main tarball" }
        $tree = Get-ChildItem $work -Directory -Filter "pdf2ppt-*" | Select-Object -First 1
        if (-not $tree) { Fail "skill folder missing in the main tarball" }
        Copy-Item -Recurse -Force (Join-Path $tree.FullName "skill\pdf2ppt") $src
    }
    if (-not (Test-Path (Join-Path $src "SKILL.md"))) { Fail "SKILL.md missing in the download" }

    $targets = @()
    if ($env:PDF2PPT_SKILL_DIR) {
        $targets = @($env:PDF2PPT_SKILL_DIR)
    } else {
        foreach ($d in @(
            (Join-Path $env:USERPROFILE ".cursor\skills"),
            (Join-Path $env:USERPROFILE ".claude\skills"),
            (Join-Path $env:USERPROFILE ".codex\skills")
        )) { if (Test-Path $d) { $targets += $d } }
        if ($targets.Count -eq 0) { $targets = @((Join-Path $env:USERPROFILE ".cursor\skills")) }
    }
    foreach ($parent in $targets) {
        New-Item -ItemType Directory -Force -Path $parent | Out-Null
        $dest = Join-Path $parent "pdf2ppt"
        if (Test-Path $dest) { Remove-Item -Recurse -Force $dest }
        Copy-Item -Recurse -Force $src $dest
        Write-Host "SKILL_MD=$dest\SKILL.md"
    }
} finally {
    Remove-Item -Recurse -Force $work -ErrorAction SilentlyContinue
}

$version = & $bin -v
if ($LASTEXITCODE -ne 0) { Fail "$bin -v failed" }
Write-Host "PDF2PPT_BIN=$bin"
Write-Host "PDF2PPT_VERSION=$version"
Write-Host "Install succeeded."
