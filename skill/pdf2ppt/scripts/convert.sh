#!/usr/bin/env bash
# Convert one PDF, or every PDF in a directory, to PPTX.
#   convert.sh input.pdf [output.pptx]
#   convert.sh input_dir [output_dir]
set -euo pipefail

if [[ $# -lt 1 || $# -gt 2 ]]; then
  echo "usage: convert.sh input.pdf [output.pptx]" >&2
  echo "       convert.sh input_dir [output_dir]" >&2
  exit 2
fi

if ! command -v pdf2ppt >/dev/null 2>&1; then
  script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
  echo "pdf2ppt is not on PATH. Install it first:" >&2
  echo "  bash \"$script_dir/install.sh\"" >&2
  exit 127
fi

src=$1
if [[ -d "$src" ]]; then
  outdir=${2:-"$src/pptx-out"}
  mkdir -p "$outdir"
  shopt -s nullglob
  found=0
  for pdf in "$src"/*.pdf "$src"/*.PDF; do
    [[ -f "$pdf" ]] || continue
    found=1
    base=$(basename "$pdf")
    base=${base%.*}
    echo "converting $pdf"
    pdf2ppt "$pdf" "$outdir/$base.pptx"
  done
  if [[ $found -eq 0 ]]; then
    echo "no PDF files in $src" >&2
    exit 1
  fi
  echo "wrote slides in $outdir"
  exit 0
fi

if [[ ! -f "$src" ]]; then
  echo "not a file: $src" >&2
  exit 1
fi

if [[ $# -eq 2 ]]; then
  dest=$2
else
  dest=${src%.*}.pptx
fi
pdf2ppt "$src" "$dest"
