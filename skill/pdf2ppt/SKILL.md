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

Do not assume `pdf2ppt` is on `PATH`. Use the absolute path the installer prints (`PDF2PPT_BIN=`).

Linux or macOS:

```bash
test -x "$HOME/.local/bin/pdf2ppt" && "$HOME/.local/bin/pdf2ppt" -v
test -f "$HOME/.cursor/skills/pdf2ppt/SKILL.md" && echo SKILL_OK
```

Windows PowerShell:

```powershell
Test-Path "$env:LOCALAPPDATA\pdf2ppt\pdf2ppt.exe"
& "$env:LOCALAPPDATA\pdf2ppt\pdf2ppt.exe" -v
Test-Path "$env:USERPROFILE\.cursor\skills\pdf2ppt\SKILL.md"
```

A version number means the binary runs. `SKILL_OK` or `True` means the skill file is in place.

## Install

Follow the "Install for agents" section in the repository README (English: `README.md`, Chinese: `README.zh-CN.md`). Those blocks are complete. They download this skill and a release binary, and they check the SHA256 checksum. They do not need Go or git.

If this skill is already on disk, the same installer is:

```bash
sh "$HOME/.cursor/skills/pdf2ppt/scripts/install.sh" --skill-dir "$HOME/.cursor/skills"
```

Windows:

```powershell
powershell -ExecutionPolicy Bypass -File "$env:USERPROFILE\.cursor\skills\pdf2ppt\scripts\install.ps1" -SkillDir "$env:USERPROFILE\.cursor\skills"
```

## Convert

Use the absolute binary. Replace the two paths if the user named different files.

Linux or macOS:

```bash
"$HOME/.local/bin/pdf2ppt" report.pdf report.pptx
```

Windows PowerShell:

```powershell
& "$env:LOCALAPPDATA\pdf2ppt\pdf2ppt.exe" report.pdf report.pptx
```

A directory of PDFs (Linux or macOS):

```bash
sh "$HOME/.cursor/skills/pdf2ppt/scripts/convert.sh" ./decks ./decks-pptx
```

The command prints `wrote <path> (N slides)`. Exit code 0 means the file was written. The output is a zip; the first two bytes are `PK`. A non-PDF, a missing path, or a PDF that needs a user password exits non-zero and prints the reason.

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
| `pdf2ppt: command not found` | Do not rely on PATH. Run `"$HOME/.local/bin/pdf2ppt"` or `& "$env:LOCALAPPDATA\pdf2ppt\pdf2ppt.exe"`. |
| SHA256 mismatch | The download was corrupt. Run the README install block again. Do not execute the rejected file. |
| `不是有效的 PDF 格式` or "not a valid PDF" | The input is not a PDF, or it is truncated. `pdfinfo input.pdf` should print a page count. |
| Conversion succeeds but the slide looks like the wrong font | Install the family named in the PDF, or accept the substitute. The tool does not embed fonts. |
| LibreOffice will not open the file | `unzip -t` the pptx. If that fails, the write was interrupted. Convert again. |
| A page is blank | The page may be a scan (one image) or a construct this version skips. Check the source with `pdftotext -f N -l N input.pdf -`. |
