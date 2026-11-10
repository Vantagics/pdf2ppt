#!/bin/sh
# Install the pdf2ppt release binary and this skill. No Go or git.
#   curl -fsSL https://raw.githubusercontent.com/Vantagics/pdf2ppt/main/skill/pdf2ppt/scripts/install.sh | sh
# Optional: PDF2PPT_SKILL_DIR or --skill-dir DIR (one directory; skips auto-detect).
set -eu

REPO="Vantagics/pdf2ppt"
SKILL_DIR="${PDF2PPT_SKILL_DIR:-}"

while [ $# -gt 0 ]; do
  case "$1" in
    --skill-dir)
      [ $# -ge 2 ] || { echo "missing directory after --skill-dir" >&2; exit 2; }
      SKILL_DIR=$2
      shift 2
      ;;
    *)
      echo "unknown argument: $1" >&2
      exit 2
      ;;
  esac
done

fail() { echo "pdf2ppt install failed: $*" >&2; exit 1; }
have() { command -v "$1" >/dev/null 2>&1; }

case "$(uname -s 2>/dev/null || echo unknown)" in
  Linux) os=linux ;;
  Darwin) os=darwin ;;
  *) fail "Linux and macOS only. On Windows: iex (irm https://raw.githubusercontent.com/Vantagics/pdf2ppt/main/skill/pdf2ppt/scripts/install.ps1)" ;;
esac
case "$(uname -m 2>/dev/null || echo unknown)" in
  x86_64|amd64) arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  *) fail "unsupported architecture: $(uname -m)" ;;
esac

have curl || fail "curl is required"
have tar || fail "tar is required"

asset="pdf2ppt_${os}_${arch}"
bindir="${HOME}/.local/bin"
binpath="${bindir}/pdf2ppt"
mkdir -p "$bindir"

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
echo "downloading $asset"
curl -fsSL --retry 3 --retry-delay 2 -o "$work/SHA256SUMS" \
  "https://github.com/${REPO}/releases/latest/download/SHA256SUMS" \
  || fail "could not download SHA256SUMS"
curl -fsSL --retry 3 --retry-delay 2 -o "$work/$asset" \
  "https://github.com/${REPO}/releases/latest/download/${asset}" \
  || fail "could not download ${asset}"
sumline=$(grep -E "^[0-9a-fA-F]{64}[[:space:]][[:space:]]${asset}$" "$work/SHA256SUMS" | head -n 1 || true)
[ -n "$sumline" ] || fail "SHA256SUMS has no entry for ${asset}"
(
  cd "$work"
  if have sha256sum; then
    printf '%s\n' "$sumline" | sha256sum -c -
  else
    printf '%s\n' "$sumline" | shasum -a 256 -c -
  fi
) || fail "SHA256 mismatch for ${asset}; not installing it"
cp "$work/$asset" "$binpath"
chmod +x "$binpath"
"$binpath" -v >/dev/null || fail "$binpath -v failed"

if curl -fsSL --retry 3 --retry-delay 2 -o "$work/skill.tar.gz" \
    "https://github.com/${REPO}/releases/latest/download/pdf2ppt-skill.tar.gz"; then
  rm -rf "$work/pdf2ppt"
  tar -xzf "$work/skill.tar.gz" -C "$work"
else
  echo "using the main branch tarball for the skill" >&2
  curl -fsSL --retry 3 --retry-delay 2 -o "$work/src.tar.gz" \
    "https://github.com/${REPO}/archive/refs/heads/main.tar.gz" \
    || fail "could not download the skill"
  tar -xzf "$work/src.tar.gz" -C "$work"
  src=$(echo "$work"/pdf2ppt-*/skill/pdf2ppt)
  [ -d "$src" ] || fail "skill folder missing in the main tarball"
  rm -rf "$work/pdf2ppt"
  cp -R "$src" "$work/pdf2ppt"
fi
[ -f "$work/pdf2ppt/SKILL.md" ] || fail "SKILL.md missing in the download"

place() {
  mkdir -p "$1"
  rm -rf "$1/pdf2ppt"
  cp -R "$work/pdf2ppt" "$1/pdf2ppt"
  echo "SKILL_MD=$1/pdf2ppt/SKILL.md"
}

if [ -n "$SKILL_DIR" ]; then
  place "$SKILL_DIR"
else
  n=0
  for d in "$HOME/.cursor/skills" "$HOME/.claude/skills" "$HOME/.codex/skills"; do
    if [ -d "$d" ]; then
      place "$d"
      n=$((n + 1))
    fi
  done
  if [ "$n" -eq 0 ]; then
    place "$HOME/.cursor/skills"
  fi
fi

echo "PDF2PPT_BIN=$binpath"
echo "PDF2PPT_VERSION=$("$binpath" -v)"
echo "Install succeeded."
