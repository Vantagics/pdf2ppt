---
name: pdf2ppt
description: >-
  Convert a PDF into an editable PowerPoint file (.pptx). Use when the user
  asks to convert PDF to PPT, PPTX, or PowerPoint, turn a slide-deck PDF into
  editable slides, recover text boxes and pictures from a PDF, or run pdf2ppt.
---

# pdf2ppt

Turn a PDF into an editable `.pptx`. Each page becomes one slide. Text stays in real text boxes, pictures stay pictures, and vector drawings become shapes.

The converter is the `pdf2ppt` command from [github.com/Vantagics/pdf2ppt](https://github.com/Vantagics/pdf2ppt). This skill only tells you how to install and run it. Do not reimplement the conversion.

## Check whether it is installed

```bash
command -v pdf2ppt && pdf2ppt -v
```

On Windows PowerShell:

```powershell
Get-Command pdf2ppt -ErrorAction SilentlyContinue
pdf2ppt -v
```

If the command is missing, install it with the script below. A version printed on stderr or stdout means the binary runs.

`go install` puts the binary in `$(go env GOPATH)/bin` (often `~/go/bin`). If `pdf2ppt -v` fails after install, add that directory to `PATH`.

## Install

From a clone of this repository the script builds the local module. Otherwise it runs `go install github.com/Vantagics/pdf2ppt/cmd/pdf2ppt@latest`. If Go is not available it downloads the matching binary from the latest GitHub Release.

Linux or macOS:

```bash
bash skill/pdf2ppt/scripts/install.sh
```

When this folder has been copied into an agent skills directory:

```bash
bash ~/.cursor/skills/pdf2ppt/scripts/install.sh
```

Windows PowerShell:

```powershell
powershell -ExecutionPolicy Bypass -File skill/pdf2ppt/scripts/install.ps1
```

Install by hand if you prefer:

```bash
go install github.com/Vantagics/pdf2ppt/cmd/pdf2ppt@latest
```

Release archives are named `pdf2ppt_<os>_<arch>` (`linux`, `darwin`, `windows` and `amd64`, `arm64`). Windows builds end in `.exe`. The checksum file on the release is `SHA256SUMS`.

## Convert

One file:

```bash
pdf2ppt report.pdf report.pptx
```

Or the helper, which also accepts a directory:

```bash
bash skill/pdf2ppt/scripts/convert.sh report.pdf
bash skill/pdf2ppt/scripts/convert.sh ./decks ./decks-pptx
```

A directory of PDFs writes one `.pptx` per file into the output directory (default: `./pptx-out` next to the current directory). The command prints `wrote <path> (N slides)` and page progress on stderr. Exit code 0 means the file was written. A non-PDF, a missing path, or a PDF that needs a user password exits non-zero and prints the reason.

The desktop window (`wails build`) is optional. Agents should use the CLI.

## Verify

1. Confirm the `.pptx` exists and is a zip (`unzip -t report.pptx`).
2. If LibreOffice is installed, render a page and look at it:

```bash
soffice --headless --convert-to pdf --outdir /tmp report.pptx
pdftoppm -png -r 72 -f 1 -l 1 /tmp/report.pdf /tmp/report-slide
```

Open `/tmp/report-slide-1.png`. Text should be selectable in Impress or PowerPoint, not a single picture of the page, unless the source page itself was a scan.

## Known limitations

- One slide size per file. The canvas is the largest page.
- Text is one box per PDF text run, not the original text frame. Tables and bullets come back as shapes plus text, not native PowerPoint tables or bullet lists.
- The font name is recorded. The font file is not embedded, so a missing family is substituted.
- Axial and radial shadings and a one-image tiling pattern (a common PowerPoint background) are kept. Other patterns, Type 3 fonts, optional content, annotations, and user-password encryption are skipped or rejected.
- Owner-password files with an empty user password open. A real user password does not.

## Troubleshooting

| Symptom | What to do |
| --- | --- |
| `pdf2ppt: command not found` after `go install` | Add `$(go env GOPATH)/bin` to `PATH` and open a new shell. |
| `go: module github.com/Vantagics/pdf2ppt` cannot be downloaded | Use the release binary. The install script does that when `go install` fails. |
| `不是有效的 PDF 格式` or "not a valid PDF" | The input is not a PDF, or it is truncated. `pdfinfo input.pdf` should print a page count. |
| Conversion succeeds but the slide looks like the wrong font | Install the family named in the PDF, or accept the substitute. The tool does not embed fonts. |
| LibreOffice will not open the file | `unzip -t` the pptx. If that fails, the write was interrupted. Convert again. |
| A page is blank | The page may be a scan (one image) or a construct this version skips. Check the source with `pdftotext -f N -l N input.pdf -`. |
