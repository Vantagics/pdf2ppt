# Changelog

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
