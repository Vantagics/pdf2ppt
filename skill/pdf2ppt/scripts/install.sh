#!/bin/sh
# Install the pdf2ppt binary from a GitHub Release and the skill folder.
# No Go and no git required. Safe to run twice.
#
#   sh install.sh --skill-dir "$HOME/.cursor/skills"
#   curl -fsSL https://raw.githubusercontent.com/Vantagics/pdf2ppt/main/skill/pdf2ppt/scripts/install.sh | sh -s -- --skill-dir "$HOME/.cursor/skills"
#
# Options:
#   --skill-dir DIR   copy the skill to DIR/pdf2ppt (SKILL.md)
#   --from-source     build with Go instead of the release binary (optional)
set -eu

REPO="Vantagics/pdf2ppt"
SKILL_DIR=""
FROM_SOURCE=0

while [ $# -gt 0 ]; do
  case "$1" in
    --skill-dir)
      if [ $# -lt 2 ]; then
        echo "missing directory after --skill-dir" >&2
        exit 2
      fi
      SKILL_DIR=$2
      shift 2
      ;;
    --from-source)
      FROM_SOURCE=1
      shift
      ;;
    *)
      echo "unknown argument: $1" >&2
      echo "usage: install.sh [--skill-dir DIR] [--from-source]" >&2
      exit 2
      ;;
  esac
done

have() { command -v "$1" >/dev/null 2>&1; }

fail() {
  echo "pdf2ppt install failed: $*" >&2
  exit 1
}

os_name=$(uname -s 2>/dev/null || echo unknown)
case "$os_name" in
  Linux) os=linux ;;
  Darwin) os=darwin ;;
  *) fail "this script supports Linux and macOS (uname=$os_name). On Windows run install.ps1." ;;
esac

arch_raw=$(uname -m 2>/dev/null || echo unknown)
case "$arch_raw" in
  x86_64|amd64) arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  *) fail "unsupported architecture: $arch_raw" ;;
esac

asset="pdf2ppt_${os}_${arch}"
bindir="${HOME}/.local/bin"
share="${HOME}/.local/share/pdf2ppt"
binpath="${bindir}/pdf2ppt"
mkdir -p "$bindir" "$share"

if [ "$FROM_SOURCE" -eq 1 ]; then
  have go || fail "--from-source needs Go"
  echo "building from source (go install)"
  go install "github.com/${REPO}/cmd/pdf2ppt@latest" || fail "go install failed"
  gobin=$(go env GOPATH)/bin/pdf2ppt
  [ -x "$gobin" ] || fail "go install did not write $gobin"
  cp "$gobin" "$binpath"
  chmod +x "$binpath"
else
  work=$(mktemp -d)
  # shellcheck disable=SC2064
  trap "rm -rf '$work'" EXIT
  echo "downloading checksums and $asset"
  curl -fsSL --retry 3 --retry-delay 2 \
    -o "$work/SHA256SUMS" \
    "https://github.com/${REPO}/releases/latest/download/SHA256SUMS" \
    || fail "could not download SHA256SUMS from the latest GitHub Release"
  curl -fsSL --retry 3 --retry-delay 2 \
    -o "$work/$asset" \
    "https://github.com/${REPO}/releases/latest/download/${asset}" \
    || fail "could not download ${asset}. Check https://github.com/${REPO}/releases/latest"
  sumline=$(grep -E "^[0-9a-fA-F]{64}[[:space:]][[:space:]]${asset}$" "$work/SHA256SUMS" | head -n 1 || true)
  if [ -z "$sumline" ]; then
    fail "SHA256SUMS has no entry for ${asset}"
  fi
  (
    cd "$work"
    if have sha256sum; then
      printf '%s\n' "$sumline" | sha256sum -c - || fail "SHA256 mismatch for ${asset}; not installing it"
    elif have shasum; then
      printf '%s\n' "$sumline" | shasum -a 256 -c - || fail "SHA256 mismatch for ${asset}; not installing it"
    else
      fail "need sha256sum or shasum to verify ${asset}"
    fi
  )
  cp "$work/$asset" "$binpath"
  chmod +x "$binpath"
  trap - EXIT
  rm -rf "$work"
fi

printf '%s\n' "$binpath" > "$share/pdf2ppt-bin"
version=$("$binpath" -v) || fail "$binpath -v failed"
echo "PDF2PPT_BIN=$binpath"
echo "PDF2PPT_VERSION=$version"

stage=$(mktemp -d)
skill_ready="$stage/pdf2ppt"
if curl -fsSL --retry 3 --retry-delay 2 \
    -o "$stage/pdf2ppt-skill.tar.gz" \
    "https://github.com/${REPO}/releases/latest/download/pdf2ppt-skill.tar.gz"; then
  tar -xzf "$stage/pdf2ppt-skill.tar.gz" -C "$stage"
  echo "SKILL_SOURCE=release"
else
  echo "release has no pdf2ppt-skill.tar.gz; using the main branch tarball" >&2
  curl -fsSL --retry 3 --retry-delay 2 \
    -o "$stage/src.tar.gz" \
    "https://github.com/${REPO}/archive/refs/heads/main.tar.gz" \
    || fail "could not download the skill from GitHub"
  tar -xzf "$stage/src.tar.gz" -C "$stage"
  src_skill=$(echo "$stage"/pdf2ppt-*/skill/pdf2ppt)
  [ -d "$src_skill" ] || fail "skill folder missing in the main tarball"
  cp -R "$src_skill" "$skill_ready"
  echo "SKILL_SOURCE=main"
fi
[ -f "$skill_ready/SKILL.md" ] || fail "SKILL.md missing in the downloaded skill"

place_skill() {
  dest_parent=$1
  [ -n "$dest_parent" ] || return 0
  mkdir -p "$dest_parent"
  rm -rf "$dest_parent/pdf2ppt"
  cp -R "$skill_ready" "$dest_parent/pdf2ppt"
  [ -f "$dest_parent/pdf2ppt/SKILL.md" ] || fail "SKILL.md missing at $dest_parent/pdf2ppt/SKILL.md"
  echo "SKILL_MD=$dest_parent/pdf2ppt/SKILL.md"
}

mkdir -p "$share/skill"
place_skill "$share/skill"
if [ -n "$SKILL_DIR" ]; then
  place_skill "$SKILL_DIR"
fi
rm -rf "$stage"

echo "Install succeeded."
echo "Binary: $binpath"
echo "Version: $version"
case ":$PATH:" in
  *":${bindir}:"*) ;;
  *) echo "That directory is not on PATH. For this shell: export PATH=\"${bindir}:\$PATH\"" ;;
esac
echo "You can run it without changing PATH: \"$binpath\" -v"
