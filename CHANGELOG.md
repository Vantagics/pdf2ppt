# Changelog

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
