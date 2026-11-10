#!/bin/sh
# Convert one PDF, or every PDF in a directory, to PPTX.
#   sh convert.sh input.pdf [output.pptx]
#   sh convert.sh input_dir [output_dir]
# Finds the binary on PATH, then ~/.local/bin, then the path recorded by install.sh.
set -eu

if [ $# -lt 1 ] || [ $# -gt 2 ]; then
  echo "usage: convert.sh input.pdf [output.pptx]" >&2
  echo "       convert.sh input_dir [output_dir]" >&2
  exit 2
fi

find_bin() {
  if command -v pdf2ppt >/dev/null 2>&1; then
    command -v pdf2ppt
    return 0
  fi
  if [ -x "${HOME}/.local/bin/pdf2ppt" ]; then
    echo "${HOME}/.local/bin/pdf2ppt"
    return 0
  fi
  marker="${HOME}/.local/share/pdf2ppt/pdf2ppt-bin"
  if [ -f "$marker" ]; then
    recorded=$(cat "$marker")
    if [ -x "$recorded" ]; then
      echo "$recorded"
      return 0
    fi
  fi
  return 1
}

if ! bin=$(find_bin); then
  echo "pdf2ppt is not installed. Run install.sh (see README.md)." >&2
  exit 127
fi

src=$1
if [ -d "$src" ]; then
  outdir=${2:-"$src/pptx-out"}
  mkdir -p "$outdir"
  found=0
  for pdf in "$src"/*.pdf "$src"/*.PDF; do
    [ -f "$pdf" ] || continue
    found=1
    base=$(basename "$pdf")
    base=${base%.*}
    echo "converting $pdf"
    "$bin" "$pdf" "$outdir/$base.pptx"
  done
  if [ "$found" -eq 0 ]; then
    echo "no PDF files in $src" >&2
    exit 1
  fi
  echo "wrote slides in $outdir"
  exit 0
fi

if [ ! -f "$src" ]; then
  echo "not a file: $src" >&2
  exit 1
fi

if [ $# -eq 2 ]; then
  dest=$2
else
  dest=${src%.*}.pptx
fi
"$bin" "$src" "$dest"
