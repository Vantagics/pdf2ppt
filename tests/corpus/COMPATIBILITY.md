# Compatibility notes (v1.2.0)

External PDFs are not in the git tree. `manifest.json` lists the URL, producer, and why the file is not committed. `fetch.py` downloads them into `tests/corpus/cache/`.

SSIM compares a poppler render of the source page with a LibreOffice render of the PPTX, at 60 dpi. Each file contributed its first two pages and, when the deck is longer than six pages, the middle page. Text recall is the share of source characters (whitespace removed) that appear in the PPTX `<a:t>` runs. The "before" column is v1.1.1, measured on the same pages before the tiling-pattern change.

| Sample | Producer | Pages scored | Mean SSIM before | Mean SSIM after | Min SSIM after | Text recall after |
| --- | --- | ---: | ---: | ---: | ---: | ---: |
| gzmtu “PPT教学课件制作与演示技巧” | PowerPoint 2013 | 3 | 0.300 | 0.784 | 0.684 | 1.00 |
| UAB ISC2 (Catalan) | PowerPoint 2016 | 3 | 0.980 | 0.980 | 0.976 | 1.00 |
| UAB REO | PowerPoint for Microsoft 365 | 3 | 0.943 | 0.943 | 0.897 | 1.00 |
| CDC NBS overview | PowerPoint for Microsoft 365 | 3 | 0.921 | 0.921 | 0.897 | 1.00 |
| NRCS cattlemen update (letter print of a PowerPoint file) | Acrobat Distiller 24 | 3 | 0.943 | 0.944 | 0.885 | 1.00 |
| MacAdmins AutoPkg | Google (Slides export; no Producer string) | 3 | 0.983 | 0.983 | 0.969 | 1.00 |
| MacAdmins “Scripting the unscriptable” | Keynote / Quartz PDFContext | 3 | 0.906 | 0.906 | 0.889 | 0.99 |
| Flip beamer theme | LaTeX Beamer 3.10, xdvipdfmx | 3 | 0.781 | 0.781 | 0.721 | 0.99 |
| McGill beamer theme | LaTeX Beamer 3.24, pdfTeX | 2 | 0.909 | 0.909 | 0.879 | 1.00 |
| `slides.html` via headless Chrome (not a downloaded Google deck) | Skia/PDF m156 | 5 | 0.994 (v1.1.1, 72 dpi) | unchanged | 0.991 | gated by `TestChromeSkiaSlide` |

Aggregate over the nine downloaded decks: mean SSIM 0.852 → 0.906. The minimum page SSIM 0.025 → 0.684. Text recall after the change is 0.996. The eight decks other than the PowerPoint 2013 courseware did not move, so nothing else regressed. The old word-token overlap used in earlier notes under-counted short tokens; character recall on those decks was already about 1.

What changed on the PowerPoint 2013 deck:

- Page 1 was a black slide with a gold title (SSIM 0.025). The building photo is a PatternType 1 cell, one JPEG, matrix pinning the cell origin to the top of the page. The tile that covers the page is the one below the origin. It is now the background image (SSIM 0.869). The title is fill-and-stroke (PDF text rendering mode 2); the outline is a real run stroke.
- Page 2 (table on a blue gradient bitmap) was 0.301 and is 0.684. The layout, Chinese text, and orange header match. The remaining gap is antialiasing of white and blue Chinese on the gradient, not a missing object.
- Page 3 (callout over a screenshot) was 0.573 and is 0.800. The Wingdings arrow U+F0E0 in “布局 → 文字” is a Unicode arrow. The pale-green callout fill and red outline are both present. Text inside the callout is still a bit tighter than the source.

Other producers, unchanged and still the main gaps:

- Keynote title stays near 0.89 because the paper texture is a JPEG that is re-encoded.
- Beamer titles use a display face that is not embedded. LibreOffice substitutes another family, so the title is heavier and the author line is tighter (SSIM about 0.72–0.88) even though the characters are there.
- Google Slides (Creator: Google, 720×405) was already 0.97–0.99 on the pages scored.
- PowerPoint 2016 and Microsoft 365 pages stay in the 0.90–0.98 range. Charts and SmartArt in these exports are mostly already images or ordinary paths.

Not in this table, because a confirmed slide PDF could not be fetched:

- WPS Presentation. A two-page WPS Writer file (A4, creator “WPS 文字”) was downloaded and not scored as a deck.
- Canva. No export with a Canva producer string was found.
- A published LibreOffice Impress deck. The generated round-trip fixtures are still the Impress coverage (`TestRoundTrip`, SSIM ≥ 0.998).
- Most CDC ACIP slide URLs returned 403 from this environment. The one CDC deck above is a public NBS overview.

LibreOffice headless opened every PPTX from this pass. Microsoft PowerPoint was not available, so a PowerPoint repair dialog was not checked.
