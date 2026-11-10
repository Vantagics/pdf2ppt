# Changelog

## 1.2.1

The Beamer title was not a heavier substitute font. xdvipdfmx paints those letters twice: a text run, then a stroked outline of each glyph, including a rectangular stem for "I". With Noto Sans in place of the embedded Trebuchet, the fill and the outline no longer sat on the same pixels, so the title looked ghosted. Those outlines are dropped when they sit inside the text run. Each run is then tracked (and, only for a large mismatch, resized) so the fallback font occupies the PDF advance instead of spilling into the next run. A TJ gap of exactly a quarter em, which xdvipdfmx uses as a word space, is a space again, so "You can now abuse fonts" is not one word. A wider gap is a second run, which keeps "155" and "8" in their own boxes. Vertical labels use a non-negative DrawingML angle; LibreOffice was ignoring the negative one and drawing them across the numbers.

`TestBeamerTitleIsSingleAndFitted`, `TestWordSpaceGap`, and `TestSuppressesGlyphOutlinesInsideText` cover the doubled title, the missing spaces, and a letter outline left inside a run. `TextOverlaps` flags two runs on one baseline whose boxes intersect.

## 1.2.0

PowerPoint often saves a slide background as a tiling pattern whose cell is one photo, with the pattern origin at the top of the page. The tile that covers the page is the one just below that origin. Unresolved pattern fills were painted with the default black, so a Chinese PowerPoint 2013 title (a building photo plus a gold title) came back as a black page. Those single-image tiles are now placed on the slide, clipped to the filled path.

Wingdings runs whose ToUnicode map uses the private-use area (U+F0E0 and the rest of U+F020–U+F0FF) are rewritten to Unicode, and the font is set to DejaVu Sans so the arrow draws without Wingdings installed. Text rendering mode 2 (fill and stroke) is an outline on the run, not a fake bold flag.

Measured at 60 dpi, first pages plus a middle page, source PDF render against the LibreOffice render of the PPTX. Character recall is the share of source characters present in the PPTX text. The other decks did not move; the gain is the PowerPoint 2013 deck.

| Deck | Producer | Mean SSIM before | Mean SSIM after | Text recall after |
| --- | --- | ---: | ---: | ---: |
| gzmtu courseware (zh) | PowerPoint 2013 | 0.300 (worst page 0.025) | 0.784 (worst page 0.684) | 1.00 |
| eight other slide PDFs | PowerPoint 2016/365, Keynote, Google, Beamer, Distiller | 0.921 | 0.921 | 0.99–1.00 |

Per-deck notes are in `tests/corpus/COMPATIBILITY.md`.

## 1.1.1

The Chrome/Skia slide gradient was mirrored top to bottom. The shading axis was transformed by both the pattern matrix and the paint-time CTM. Poppler applies the pattern matrix in the content stream's base space and cancels the CTM, so the extra multiply flipped the axis: yellow climbed into the top-right and the navy region shrank. Stop positions and the −18° label geometry were already right; the flipped background made the label look shifted. The white chip's centroid now matches the source within a fraction of a pixel.

LibreOffice's `a:lin` with `scaled="1"` matches this CSS-style axis on a 16:9 page (corner colors within about two levels). No separate PowerPoint-only gradient was required.

The same path covers other linear angles, a four-stop blend, and a centered radial shading (PDF type 3).

| Page | Before (v1.1.0) | After |
| --- | ---: | ---: |
| 120° gradient, rotated label, Chinese | 0.959 | 0.991 |
| 90° linear | — | 0.994 |
| 0° linear, three stops | — | 0.995 |
| 45° linear, four stops | — | 0.996 |
| centered radial | — | 0.996 |

## 1.1.0

Compatibility pass against real slide PDFs, not only the generated fixtures from 1.0.0.

Checked producer metadata and converted sample pages from Microsoft PowerPoint 2016, PowerPoint for Microsoft 365, Keynote, and a Skia/PDF (Chrome) slide with a gradient, rotated text, and Chinese. The external PDFs are listed in `tests/corpus/manifest.json` and are not committed.

- WinAnsi and MacRoman encodings are decoded, so PowerPoint text such as “Lluís”, “què és”, and “Damià” is no longer dropped when the font has no ToUnicode map.
- Axial shading patterns (Chrome/Skia gradients) become linear gradient fills.
- Rotated text is a rotated text box. A −18° Chrome label stays on its label.
- A deck whose pages are different sizes uses the largest page as the single OOXML slide size, so a later page is not clipped.
- An owner-password PDF with an empty user password opens. A user password is still rejected by the reader.

Measured SSIM against the rendered source page (72 dpi unless noted):

| Sample | Producer | Before this release | After |
| --- | --- | ---: | ---: |
| UAB ISC2, page 2 | PowerPoint 2016 | text missing à/è/é/í (“qu s”, “Llus”) | 0.982, accents restored |
| UAB REO, title | PowerPoint 365 | “Dami” instead of “Damià”; photo kept | accents restored, photo kept |
| MacAdmins talk, title | Keynote | 0.889 (paper texture re-encoded) | same class of result; text was already intact |
| Chrome fixture, page 1 | Skia/PDF | 0.15 (black page, gradient dropped, rotated label misplaced) | 0.959 |

No redistributable WPS Presentation PDF was found. LaTeX Beamer was not in this environment.

## 1.0.0

First release of the editable PDF → PPTX converter.

The previous reader matched `/Type /Page` and a few operators with regular expressions on the raw file bytes. LibreOffice (and almost every other producer) stores page content in Flate-compressed streams and encodes text with a custom character map, so text, Chinese characters, images, and vector drawings were dropped. Slide count and page size were the only things that survived.

This release replaces that reader with a content-stream interpreter and teaches the PPTX writer to emit real text boxes, pictures, freeform shapes, and solid backgrounds.

Round trip on the `tests/roundtrip` fixtures (PPTX → LibreOffice PDF → pdf2ppt → re-render), mean SSIM at 72 dpi (the resolution `TestRoundTrip` uses):

| Fixture | Before (no text recovered) | After |
| --- | ---: | ---: |
| text_styles | 0.927 | 0.9999 |
| cjk | 0.922 | 0.9992 |
| bullets | 0.947 | 0.9999 |
| images | 0.787 | 0.9989 |
| shapes | 0.836 | 0.9994 |
| table | 0.888 | 0.9992 |
| backgrounds | 0.347 | 0.9999 / 1.000 |
| aspect_4x3 | 0.910 | 1.000 |
| mixed | 0.806 | 0.9995 |

Before, extracted text was empty on every fixture. After, normalized text matches the original deck on every fixture.
