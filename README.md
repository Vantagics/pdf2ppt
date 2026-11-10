# pdf2ppt

Converts a PDF into an editable PowerPoint file (`.pptx`). Each PDF page becomes one slide. Text stays in real text boxes (not a flattened picture of the page), pictures stay pictures, and vector drawings become shapes.

There are two ways to run it:

- **Desktop app** — a [Wails](https://wails.io) window (`go build` / `wails build` from the repo root). Choose a PDF, then convert.
- **Command line** — no window required:

```bash
go run ./cmd/pdf2ppt input.pdf output.pptx
```

```text
pdf2ppt 1.0.0
usage: pdf2ppt input.pdf output.pptx
```

## What the conversion keeps

The reader walks the PDF content stream (including compressed streams) and rebuilds a slide from the drawing operators:

| PDF feature | PPTX result |
| --- | --- |
| Page size | Slide size (points converted to EMU). Widescreen and 4:3 both keep their aspect ratio. |
| Text runs, including Chinese | Editable text boxes with font family, size, bold, italic, and RGB color. |
| Embedded images | PNG or JPEG pictures at the original position and size. Soft masks become PNG alpha. |
| Filled / stroked paths | Rectangles or freeform shapes, in paint order. A full-page fill becomes the slide background. |
| Tables, bullets | Editable text plus the rectangles and bullet glyphs the PDF actually contains. |

Font names are taken from the PDF `BaseFont` (subset prefixes like `ABCDEF+` are removed) and mapped back to a family LibreOffice can resolve, for example `LiberationSans-Bold` → Liberation Sans, bold, and `WenQuanYiMicroHei` → WenQuanYi Micro Hei. Character codes go through the font's ToUnicode map, so Chinese text is recovered as Unicode rather than as raw glyph ids.

## Round trip

A practical fidelity check is:

1. Start from a PPTX.
2. Export it to PDF (LibreOffice Impress headless is what the tests use).
3. Convert that PDF back with pdf2ppt.
4. Compare the two PPTX files.

On the fixtures in `tests/roundtrip`, that round trip keeps the slide count, the slide aspect ratio (within 1.5 pt), and the text (ignoring whitespace and bullet glyphs). Re-rendering both decks and comparing pixels with SSIM scores **0.998 or higher** on every slide. See the changelog for the before/after numbers.

The tests are `TestRoundTrip` in `converter/roundtrip_test.go`. They need:

- Go 1.22+
- LibreOffice Impress (`soffice`)
- Poppler (`pdftoppm`)
- Python 3 with `python-pptx`, `pillow`, `numpy`, and `scikit-image`

```bash
go test ./converter -run 'TestRoundTrip|TestRejectsNonPDF' -count=1 -timeout 20m
```

`tests/roundtrip/generate_samples.py` builds the fixtures (text styles, CJK, bullets, images, shapes, a table, solid and picture backgrounds, a 4:3 slide, and a mixed slide). `tests/roundtrip/compare.py` checks one pair and prints JSON. The Go test wires those together with the converter.

Parser checks that do not need LibreOffice:

```bash
go test ./converter/gopdf2 ./converter -count=1 -run 'TestNormalize|TestToUnicode|TestTokenizer|TestRejects|TestEmpty'
```

## Real slide decks

`tests/corpus/manifest.json` lists presentation PDFs whose producer metadata was checked (PowerPoint 2016, PowerPoint for Microsoft 365, Keynote, and a Skia/PDF print of `tests/corpus/slides.html`, which is the same PDF backend Google Slides uses). Those downloaded decks are not committed: the license was not clear, or the file is someone else's talk. `tests/corpus/fetch.py` downloads them into `tests/corpus/cache/`.

```bash
python3 tests/corpus/fetch.py
go test ./converter -run 'TestRoundTrip|TestChromeSkiaSlide' -count=1 -timeout 20m
```

On the decks that were checked, PowerPoint and Keynote pages keep their pictures and layout. Accented text (Catalan à è é í ó) needed WinAnsi, which PowerPoint uses when there is no ToUnicode map. Chrome/Skia gradients are axial shadings; rotated text is a real rotated text box. PowerPoint 2013 often stores the slide background as a one-image tiling pattern; that photo is placed on the slide instead of a black page. Wingdings private-use arrows become Unicode arrows. `tests/corpus/COMPATIBILITY.md` has the per-deck SSIM table. No redistributable WPS Presentation export was found.

## Known limitations

- **One slide size per file.** OOXML stores a single `sldSz`. The presentation size is the largest page, so a later larger page is not clipped. A smaller page keeps its own box and sits in the corner of that canvas.
- **Text is one box per PDF text run.** A wrapped paragraph or a mixed Latin/Chinese line usually comes back as several boxes on the same baseline, not as the original single text frame. The characters and their positions are preserved.
- **Tables and bullets are not native PowerPoint objects.** A table comes back as cell rectangles plus text boxes. A bullet list comes back as a bullet character and a text box per line. Both stay editable.
- **Preset shapes are outlines.** Rectangles stay rectangles. Ovals, rounded rectangles, diamonds, and arbitrary curves become freeform geometry that matches the PDF path. Drop shadows that LibreOffice rasterizes are kept as images.
- **Baseline placement assumes a horizontal run.** The box is positioned so LibreOffice draws the baseline on the PDF baseline. Rotated text is a rotated text box. Vertical writing mode is not reconstructed.
- **Fonts have to be installed to look the same.** The file records the family name; it does not embed the font. A missing family is substituted by the viewer.
- **Not every PDF construct is modeled.** Axial and radial shadings become gradient fills, and a tiling pattern that is a single image (a common PowerPoint background) is placed as that image. Other tiling patterns, Type 3 fonts, optional content, annotations, and files that need a user password are still skipped or rejected. Inline images are not read.

## Layout

```text
cmd/pdf2ppt/          CLI
converter/            conversion pipeline
converter/gopdf2/     PDF content interpreter
converter/goppt/      PPTX writer
frontend/             Wails UI
tests/roundtrip/      fixture generator and SSIM comparison
```
