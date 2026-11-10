# Compatibility notes

## v1.5.8

`he-openu-a11y` page 13 footer is one paragraph again, and the gap after באיתור is 6.7 pt. Office cloud fonts keep their PDF names, from Microsoft's March 12, 2025 cloud-font table plus Lato, Anton, Montserrat, Open Sans, Roboto, Poppins, Raleway, Nunito, and Noto Sans. `psumac-keynote` pages 1 and 30 use Source Sans Pro at the PDF size; that change landed in v1.5.7.

## v1.5.7

Office cloud fonts keep their name: Lato, Anton, Montserrat, Open Sans, Roboto, Poppins, and a short list of the same kind. Calibri is only the fallback for a face that is not on that list. `canva-rotary` pages 8 and 16 stay in Lato, with no extra character spacing. Page 1 stays in Anton. `he-openu-a11y` page 13 has the 6.7 pt gap after באיתור again.

## v1.5.6

Lato runs are Calibri, spaced so the advance ends at the next glyph's PDF x. `canva-rotary` pages 1 and 16 keep the spaces in "Account for", "nonprofit organizations", and "not-for-profit basis". `kaist-xai` page 18 is unchanged from v1.5.3. The render gate fails when a slide's mean error rises more than 1.0 over the LibreOffice 26.2 baseline. Recall baselines: cdc-nbs-m365 84.11 on this tokenizer (84.47 independent), mit-ocw-ses1 90.40, nhsa-ypsw 98.99, who-tb-2024 86.52.

## v1.5.5

v1.5.4 is regressed. Text boxes stay at the PDF x, including the next word. `kaist-xai` page 18 matches v1.5.3 again. `canva-rotary` page 16 keeps the last letter without shifting the following words. `he-openu-a11y` page 13 no longer overlaps באיתור and בניווט. Page 2, the Hebrew recall of 1, and the UNDP caption spaces stay. Those slides are in the render gate. canva-rotary and he-openu page 13 fail above a mean error of 16. kaist-xai page 18 fails above 28, which is the LibreOffice 26.2 difference on that page. UNDP unique recall is recorded as 1.

## v1.5.4

Superseded by v1.5.5.

`canva-rotary` page 16 no longer clips the last letter of `nonprofit`, `Account`, `for`, or `not-for-profit`. The trailing space stays inside the box. `he-openu-a11y` pages 6, 10, and 13 keep a space between Latin and Hebrew, and unique recall against pdftotext is 1.000. Page 2's 50 and BCG lines are separate boxes again. The stored Hebrew recall baseline was 0.921444 from v1.5.0, which is why 98.54% did not fail the 0.3 pp gate. `uab-reo-m365` gate concat is 1419; an independent count of the same joined text is 1446.

## v1.5.3

`he-openu-a11y` page 2 first bullet is the v1.5.1 pair (`בישראל 1,600,000`, with `60` in its own box). The 50 line stays `הישראלי: מעל ל 50`, with a space after the bullet and after `BCG:`. `canva-rotary` page 16 keeps about one space between `not-for-profit` and `basis`. `undp-gender-ar` unique recall is 84.62% again. `uab-reo-m365` concatenated tokens are 1419.

## v1.5.2

`he-openu-a11y` page 2 reads `איגוד האינטרנט הישראלי: מעל ל 50`. `oicw-1996-scan` page 1 is fitted inside the slide, logo included. `canva-rotary` page 16 keeps the space between nonprofit and organizations. `wps-tutorial` page 4 reads `WPS Office`. `undp-gender-ar` page 10 body text is painted from the soft masks; that deck's size ceiling is the 10.8 MB file.

## v1.5.1

`jp-tategaki` page 1 is two `vert="eaVert"` columns, あいうえお on the right and 日本語 on the left, each taller than it is wide. `he-openu-a11y` page 1 reads אילנה בניש, and page 2 keeps 1,600,000, BCG, and 50 in `pdftotext -layout` order. `oicw-1996-scan` stays at or under the PDF size × 1.05 (21.0 MB from a 22.5 MB PDF). Page 1 fills the slide width. `wps-office2019` page 1 is one "WPS Office 2019" line, and the subtitle is not covered by white rectangles. `undp-gender-ar` page 10 body text is a 1-bit soft mask on a 2×2 image, not extractable text.

## v1.5.0

Six decks are fetched with the rest of the corpus: `jp-tategaki` (vertical Japanese, Identity-V), `he-openu-a11y` (Hebrew PowerPoint, ToUnicode), `who-tb2023` (chart slides), `arcourts-ppt` (widescreen handout with soft masks), `wps-office2019` (102-page WPS guide), `oicw-1996-scan` (74-page image-only scan). The gate count is 39. `tsmc-2q25-zh` concatenated tokens are 261; that is the footer `© 2025 TSMC, Ltd`, not a missing token.

## v1.4.9

KUST page 4 draws `ρg(z₁ − z₂)` with the parenthesis against the ink of `g`, and a trailing space so `ρg` does not glue to the next word. NeurIPS subscripts on pages 41, 42, and 45 stay inline, without the hole space. `tsmc-2q25-zh` was not remeasured (Cloudflare 403, no Wayback copy).

## v1.4.8

KUST page 4 draws `(` in its own box, ending where `z₁` starts, so the parenthesis is not painted on `z₁`. Equation letters at 22 pt or below are subscripts again. Canva Rotary titles are unchanged from v1.4.7. `TestTextRecallBaseline` fails if unique-token recall drops more than 0.3 percentage points. `tsmc-2q25-zh` was not remeasured.

## v1.4.7

The KUST "(" on page 4 is the same size as `z`. Canva Rotary titles on pages 2, 7, and 8 keep reading order; a smaller letter is not a subscript. canva-mahidol still has `AI` as its own runs. The v1.4.2 token `aaii` was the doubled form of that `AI`.

## v1.4.6

GitHub Actions fetches the corpus and runs `TestPictureCountBaseline` and `TestRenderedSlidesMatchBaseline` with `-v`. `PDF2PPT_CORPUS` is the directory. Fewer than the required decks fails the job. `tsmc-2q25-zh` (Cloudflare 403), `korea-privacy` (expired attachment link), and `nrcs-cattle-distiller` (USDA host sends no bytes after TLS) are warnings. The other 33 decks, including both MIT decks, are gated on picture count, v1.4.0 size, tokens, and a LibreOffice render. A release tag waits for that OOXML run to be green.

## v1.4.5

A fill with a hole is one `a:path`. PowerPoint fills each `a:path` separately, which is why v1.4.4 painted the WPS frame, the lock, the wordmark counters, the Beijing triangle, and the McGill crest solid. An even-odd hole that winds with the outer contour is reversed so nonzero fill matches. Logo JPEGs stay JPEG, including ones at most 400 px wide.

`TestHoleStaysOnePath` and `TestHoleRendersOpen` cover a frame, a letter O, and a donut. `TestRenderedSlidesMatchBaseline` compares LibreOffice renders of wps pages 1, 2, and 4, canva-mahidol page 1, bjedu-2026 page 1, beamer-mcgill page 1, and psumac page 30 with the stored PNGs.

## v1.4.4

Outlined symbols stay in `a:t` with no fill. Common punctuation and bullets are outlined only for a tiny symbolic font, so a body-font bullet stays text. Concatenated `a:t` tokens may not fall more than 0.3% under the v1.4.2 counts.

Type 3 glyphs on one line are one shape, snapped to a 0.15 pt grid, with one `a:path` per contour. cesar page 14 slide XML is 601 KB (was 796 KB) and 98 custom geometries (was 192).

The KUST `(z…)` subscript hangs on the letter it sits under. Small NBS logo JPEGs are stored as PNG.

## v1.4.3

Type 3 slide text is drawn from the glyph outlines (cesar2025-ai page 14). The characters stay in the text box without a fill. A subscript is a baseline-shifted run inside the same box; 2024 is not one. The psumac lock is the symbolic font's outline, not `$`. NBS logo JPEGs with a solid-white soft mask stay JPEG. A run that overlaps the next word is clipped back.

`mit-ocw-ses1` is in the manifest. `fetch.py` notes the Cloudflare 403 on `tsmc-2q25-zh`.

## v1.4.2

JPEG pictures stay JPEG. A clip is `a:srcRect` on a DCT image and a real crop on a PNG. A deck more than 10% larger than its v1.4.0 PPTX fails `TestPictureCountBaseline`. Local corpus total is 213 MB, down from 256 MB in v1.4.0.

UNDP rings and the chevron band use the handle box again, so the curves are not clamped. Clips still follow the curve, and a trim under 2% does not move the picture. NBS/Alibaba page 4 paints the four category panels as axial shadings.

korea-privacy is not in the local corpus. Its recall gap versus v1.3.9 is the widened text boxes, which stay rolled back. Tiles that miss their cell stay dropped.

## v1.4.1

Non-page clips crop the picture again (NeurIPS chart footer, PSUMAC). A page clip only trims slide overflow. Pattern tiles outside their cell are dropped, so gzmtu and nbs picture counts match v1.3.7. Text tokens match v1.3.7 on the Rotary, Mahidol, and WPS decks. TencentSansW7 is bold.

The KUST pressure formulas are still separate glyphs. Gluing a small digit onto the letter beside it also ate real numbers such as 2024.

## v1.4.0

Edge clips no longer crop page-sized photos, so the Canva diamond fields keep the v1.3.7 placement. The script “Canva” wordmark is the axial shading behind each letter outline. The UAI keynote is back to 253 pictures. Mahidol captions stay white: a same-position duplicate keeps the lighter color. Indexed palette entries are not treated as gray.

The NeurIPS page 19 tweet stays cropped, because that clip removes more than a third of the JPEG.

## v1.3.9

Soft-masked pictures are no longer folded into the background. Picture counts match v1.3.7 on the local corpus (Mahidol slide 1 is 4 pictures, SaaSOpen slide 1 is 5, Tencent slide 1 is 2). Rectangular clips crop images, which is why NeurIPS page 19 no longer shows the tweet above the Manhattan render and why the page 8 icon stops before the "8".

The Tencent wordmark is an opaque RGB picture, white on the blue behind it. The v1.3.8 background PNG already contained those white pixels; this build keeps them as a separate picture with no alpha.

Canva captions that were doubled per letter join into the word (`Realistic`, `Sustainability`).

Still open: UNDP Arabic labels whose ToUnicode map only contains comma and percent; a Keynote Type 1C icon with no ToUnicode is still `$`.

## v1.3.8

Ten more decks are in `manifest.json` (NeurIPS Google Slides, two Beijing statistics decks, NBS/Alibaba, Korean PowerPoint, UNDP Arabic, another WPS file, a Japanese Google deck, and a Canva workshop).

DeviceGray through a named color space or an ICC profile with N=1 is neutral gray, not blue. The Tencent wordmark is baked into the background image so PowerPoint cannot turn the soft-mask white into black.

Still open: UNDP Arabic labels whose ToUnicode map only contains comma and percent; the NeurIPS “800” is the character U+0038 in the file; a Keynote Type 1C icon with no ToUnicode is still `$`.

## v1.3.7

Caption word spaces around 0.2 em are kept (`Getty images`, `Amadeu Pons`). Indexed images use their palette, so the Tencent wordmark stays white. The mine-safety “四” badge is the blue shading from its sampled function. A widely spaced `()` in the KUST formula is split into the two parentheses.

Not in this pass: Keynote Type 1C icons with no ToUnicode (the lock is still `$`, because the same font’s other characters are real MacRoman text). The Canva script wordmark is not a text run. LibreOffice ADMIN/TUXEDO marks and Keynote dotted arrows are unchanged.

## v1.3.6, checked in Microsoft PowerPoint

Fifteen more decks are listed in `manifest.json` (Google Slides, Canva, WPS, Keynote, LibreOffice, PDFMaker, and several PowerPoint years). They are not committed. A 25-deck pass opened every PPTX in PowerPoint 16 with no repair dialog and the dominant page size.

What that pass showed, and what changed:

- Google Slides (saasopen) covered the page and the cards with opaque black. Those rectangles are fill opacity 0. They are not written.
- Tencent PDFMaker dropped letters inside even-length runs (`CORPORATE OVERVIEW` → `CORP`). The ToUnicode map is one byte wide; decoding follows that.
- WPS page 2 lost its white panel because a full-page frame with a hole was stored as a solid background. A path with more than one subpath stays a shape.
- Rotated pictures keep the image transform's angle.
- A space is not inserted after CJK punctuation. Wingdings `➢` is drawn with Segoe UI Symbol. Symbol private-use math maps to Unicode.

Still open after this pass: a Keynote Type 1C icon with no ToUnicode is still the MacRoman character (`$` for the lock on psumac page 30). Drawing that glyph needs the CFF outline. Canva's script "Canva" wordmark is not a text run in the PDF (it is outlined or a Type 3 font) and is not reconstructed as letters. Some formula subscripts and a few missing numerals in dense PDFMaker body text can still sit in the wrong box.

`TestTextRecall` fails when pdftotext words are missing from the PPTX. It runs when the corpus PDFs are on disk.

## v1.2.1

v1.3.0 makes the Beamer line "THIS IS A" bold again (the dropped contour was the weight; it is also a text outline) and stops condensing "Gotham University". Spacing is only a few percent of an em; a wider fallback is scaled instead. Gill Sans is drawn with Liberation Sans, which has a real italic. Side-by-side: the title page in the 1.3.0 artifacts.

v1.2.1 fixes three things the v1.2.0 SSIM table did not show:

- Beamer Flip page 1. The title "THIS IS A / SAMPLE / TALK TITLE" was a filled run plus a stroked copy of each letter. It is one run now. The face is still Noto Sans, not the embedded Trebuchet, so the letters are a little heavier and tighter than the source. SSIM on that page is about 0.81.
- Beamer Flip page 2. "You can now abuse fonts" had lost its spaces (a TJ gap of exactly 0.25 em). The spaces are back. "Marker Felt" is not installed here, so that sentence is Noto Sans rather than the marker face. "XeLaTeX" stays italic.
- NRCS cattlemen page 2. The green-box labels were horizontal and "155" ran into "8" because the rotation was negative and the two numbers were one run. The labels are vertical again and the numbers sit in their own boxes.

Pages that still differ when you look at them, after this fix:

| Page | What you still see |
| --- | --- |
| Beamer Flip title | Noto Sans instead of Trebuchet. One title, not two. |
| Beamer Flip page 2 | Marker Felt sentence is Noto Sans. Layout and spaces match. |
| Beamer McGill | Theme fonts substituted. Text is intact. |
| gzmtu title | Photo and title match. The gold title is a fill plus a stroke, which is what the PDF draws. |
| gzmtu table | Same layout. SSIM stays near 0.67 because white and blue Chinese sit on a gradient bitmap. |
| gzmtu callout | Arrow, pale-green bubble, and red outline are present. Callout text is a bit tighter than the source. |
| Keynote title | Paper texture is a re-encoded JPEG. Text and the badge match. |
| CDC OneLab title | Logo, type, and the diagonal bars match. A band SSIM dip is antialiasing. |
| Google Slides (AutoPkg) | Pages scored stay above 0.92. |
| PowerPoint 2016 and 365 (UAB) | Accents and pictures match. A few bullet runs sit within a point of the next word; the ink does not cross it. |
| NRCS page 2 | Vertical labels and separated numbers match the source. |

The v1.2.0 numbers below were measured before this fix.

## Earlier measurements (v1.2.0)

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
