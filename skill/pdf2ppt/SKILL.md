---
name: pdf2ppt
description: >-
  Convert a PDF into an editable PowerPoint file (.pptx). Use when the user
  asks to convert a PDF to PPT, PPTX, or PowerPoint, or to turn a slide deck
  into editable slides.
---

# pdf2ppt

Convert one PDF into an editable PowerPoint file. Do not reimplement it.

```bash
pdf2ppt input.pdf output.pptx
```

If that command is not found:

```bash
"$HOME/.local/bin/pdf2ppt" input.pdf output.pptx
```

Windows PowerShell:

```powershell
& "$env:LOCALAPPDATA\pdf2ppt\pdf2ppt.exe" input.pdf output.pptx
```

The command prints `wrote <path> (N slides)`. The file is a zip (it starts with `PK`).

## Install if it is missing

Linux or macOS:

```bash
curl -fsSL https://raw.githubusercontent.com/Vantagics/pdf2ppt/main/skill/pdf2ppt/scripts/install.sh | sh
```

Windows PowerShell:

```powershell
iex (irm https://raw.githubusercontent.com/Vantagics/pdf2ppt/main/skill/pdf2ppt/scripts/install.ps1)
```

No Go or git. This downloads the release binary and this skill. Set `PDF2PPT_SKILL_DIR` first only when the skill should go in one specific directory.

## Limitations

One slide size per file. Text is one box per PDF run, not the original frame. Tables and bullets are shapes plus text. Fonts are named, not embedded. User-password PDFs are rejected.
