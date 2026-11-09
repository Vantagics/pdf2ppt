# Compatibility notes (v1.1.0)

These are the decks that were actually opened, converted, and compared. External PDFs are not in the git tree; see `manifest.json` and `fetch.py`.

SSIM is the rendered source PDF page against the LibreOffice render of the PPTX we wrote. Text was also read back out of the PPTX XML.

| Sample | Producer (from PDF info) | What was wrong before | After |
| --- | --- | --- | --- |
| UAB “El treball de recerca”, pages 1–2 | Microsoft PowerPoint 2016 | Pictures and layout were already close (SSIM about 0.96–0.98) but WinAnsi accents were deleted: “Lluís” → “Llus”, “què és” → “qu s”, “consciència”, “bàsiques”, “tècniques”. | Accents restored. Page 2 SSIM 0.982. |
| UAB “Recursos Educatius Oberts”, title and summary | Microsoft PowerPoint for Microsoft 365 | Identity-H text with ToUnicode was already fine (“Ciència”, “Llicències”). The bold name used WinAnsi and lost “à” (“Damià” → “Dami”). The photo, logos, and blue bar were kept. | “Damià Gil” is in the text box. Photo still matches. |
| “Scripting the Unscriptable”, title | Keynote, macOS Quartz PDFContext | English text, paper background, and the circular badge survived. SSIM about 0.89 because the paper texture is a large JPEG that is re-encoded. | No further change. Text was already intact. |
| `slides.html` printed by headless Chrome | Skia/PDF m156 (Google Slides uses this same backend; this file is our HTML, not a downloaded Google Slides deck) | First attempt: black page (axial gradient dropped) and the −18° label sat below its white chip. SSIM 0.15. | Gradient, Chinese, rounded card, and the rotated label. SSIM 0.959. `TestChromeSkiaSlide` guards this. |
| Owner-password copy of one PowerPoint page, empty user password | pypdf | — | Opens and converts. A real user password is still rejected. |
| Two pages, 720×450 then 960×540 | PowerPoint 2016 + 365 | The presentation size followed page 1, so a larger later page would be clipped. | Slide size is the maximum of the pages. |

LibreOffice headless opened every PPTX produced here and wrote a PDF with no repair step we could see. The package XML parses. Microsoft PowerPoint itself was not available, so a PowerPoint “repair” dialog was not checked.

Not found: a WPS Presentation PDF we could legally fetch, and a LaTeX Beamer file in this environment. Tiling patterns other than axial shadings, Type 3 fonts, and encrypted files that require a user password are still out of scope.
