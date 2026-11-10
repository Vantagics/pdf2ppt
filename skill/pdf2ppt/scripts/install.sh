#!/usr/bin/env bash
# Install the pdf2ppt CLI. Safe to run twice.
# Order: existing binary, local clone, go install, GitHub Release asset.
set -euo pipefail

REPO="Vantagics/pdf2ppt"
MODULE="github.com/Vantagics/pdf2ppt/cmd/pdf2ppt"

have() { command -v "$1" >/dev/null 2>&1; }

if have pdf2ppt; then
  echo "pdf2ppt already on PATH: $(command -v pdf2ppt)"
  pdf2ppt -v || true
  exit 0
fi

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(cd "$script_dir/../../.." && pwd)
if [[ -f "$repo_root/go.mod" ]] && grep -q 'github.com/Vantagics/pdf2ppt' "$repo_root/go.mod" && have go; then
  echo "building pdf2ppt from $repo_root"
  (cd "$repo_root" && go install ./cmd/pdf2ppt)
  echo "installed $(go env GOPATH)/bin/pdf2ppt"
  echo "If that directory is not on PATH, add it:"
  echo "  export PATH=\"\$(go env GOPATH)/bin:\$PATH\""
  exit 0
fi

if have go; then
  echo "go install ${MODULE}@latest"
  if go install "${MODULE}@latest"; then
    echo "installed $(go env GOPATH)/bin/pdf2ppt"
    echo "If that directory is not on PATH, add it:"
    echo "  export PATH=\"\$(go env GOPATH)/bin:\$PATH\""
    exit 0
  fi
  echo "go install failed; trying a release binary" >&2
fi

os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m)
case "$arch" in
  x86_64|amd64) arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  *) echo "unsupported architecture: $arch" >&2; exit 1 ;;
esac
case "$os" in
  linux|darwin) ;;
  *) echo "unsupported OS: $os (use install.ps1 on Windows)" >&2; exit 1 ;;
esac

asset="pdf2ppt_${os}_${arch}"
url="https://github.com/${REPO}/releases/latest/download/${asset}"
dest_dir="${HOME}/.local/bin"
mkdir -p "$dest_dir"
dest="${dest_dir}/pdf2ppt"
echo "downloading $url"
curl -fL --retry 3 --retry-delay 2 -o "$dest" "$url"
chmod +x "$dest"
echo "installed $dest"
case ":$PATH:" in
  *":${dest_dir}:"*) ;;
  *) echo "Add it to PATH: export PATH=\"${dest_dir}:\$PATH\"" ;;
esac
"$dest" -v
