# Changelog

## 1.4.7

The KUST opening parenthesis was 32 pt because PowerPoint's character spacing (`Tc` ≈ 30) was counted as the glyph's width. `fitRun` then grew "(" to fill an 81 pt box, and that box covered `z₁`. The spacing after the last glyph is no longer ink, so "(" stays the same size as `z` on page 4. The formula is still `p₂ = p₁ + ρg(z₁ − z₂)`.

Canva Rotary titles were scrambled because a smaller letter sitting under a big one was taken as a subscript and pulled out of reading order. Only a digit is a subscript now. Page 2 reads `What is Canva?`, page 7 reads `Creating Designs`, and page 8 reads `Canva for Social`. canva-mahidol page 1 is unchanged.

canva-mahidol's 153rd concatenated token in v1.4.2 was `aaii`, from writing each letter of the style list twice (`  AAII`). The letters `AI` are still their own runs (the source line is `Gen AI`). Putting the duplicate letters back in that order would turn `Sustainability` into `SSuusstt…`, so that artifact is not restored. The unique-token count stays 152, inside the 1.5% floor.

## 1.4.6

The picture, size, token, and LibreOffice render gates run in GitHub Actions. The workflow downloads the manifest into `PDF2PPT_CORPUS` (cached on the manifest hash, four retries) and the tests read that directory. A missing required PDF fails the job. Three URLs are warnings: `tsmc-2q25-zh` (Cloudflare 403), `korea-privacy` (the attachment link returns an expired-download page), and `nrcs-cattle-distiller` (`www.nrcs.usda.gov` accepts TLS and then sends no bytes). The other 33 decks, including mit-ocw-ses1 and mit-datamgmt, have stored picture counts, v1.4.0 sizes, token counts, and a rendered page. Token counts may sit 1.5% (at least 2 tokens) under the baseline, which is the gap system font metrics open between machines. A rendered page may differ by 24 levels from the stored PNG, which is the gap between LibreOffice 24.2 and 26.2; a filled hole is about 90. The release job publishes binaries only after the OOXML run on that commit is green.

## 1.4.5

v1.4.4 gave every contour of a custom geometry its own `a:path`. PowerPoint fills each `a:path` by itself, so a second contour that was a hole was painted solid. That filled the WPS title frame (`#262626` over the card; page 1 SSIM 0.970 to 0.577), the WPS page 2 and page 4 rings, the Mahidol wordmark counters, the Beijing triangle, the McGill crest, and the Keynote lock. Every contour of one fill is one `a:path` again. An even-odd hole that winds the same way as the shape around it is reversed, so `fill="norm"` (nonzero) punches the same hole. The 0.15 pt grid snap and the Type 3 line merge stay; they do not split the path.

A logo JPEG of 400 px or less is stored as JPEG again. The PNG re-encode did not move nbs page 4 (SSIM 0.8472 to 0.8469) and grew mit-ocw-ses1 by 30.5% and mit-datamgmt by 14.4%. The 10% size gate did not fail that release. `pptx_size_baseline_v140.json` was recorded from the v1.4.0 local corpus and has no entry for those two decks, `corpusPDF` only opens a file it finds under `/tmp/corpus`, and the GitHub job checks out no corpus PDFs, so `TestPictureCountBaseline` skips.

`TestHoleStaysOnePath` requires a frame, a letter O, and a donut to emit a single `a:path` containing both contours. `TestRenderedSlidesMatchBaseline` renders wps pages 1, 2, and 4, canva-mahidol page 1, bjedu-2026 page 1, beamer-mcgill page 1, and psumac page 30 with LibreOffice and compares them to the stored PNGs. `TestHoleRendersOpen` renders those three holes and fails when the center is the fill. LibreOffice paints the split path solid too: the v1.4.4 WPS page 1 is mean error 96 against this render.

## 1.4.4

An outlined symbol keeps its character in the text box with no fill, so `a:t` still contains the bullet, `$`, `#`, `%`, `(`, and `@` that v1.4.3 dropped. A common punctuation mark or a bullet is outlined only when the embedded font has a handful of glyphs (the Keynote lock). A bullet in a normal text face stays real text. `TestPictureCountBaseline` fails if concatenated `a:t` tokens fall more than 0.3% under the stored v1.4.2 counts.

Type 3 letters on one line share one shape. Coordinates snap to a 0.15 pt grid (an integer number of EMU), a flat cubic becomes a line, and each contour is its own `a:path`. cesar2025-ai page 14 goes from 192 custom geometries and 796 KB of slide XML to 98 geometries and 601 KB.

The KUST subscript is attached to the letter whose box ends next to the digit, and a small overlap with the following parenthesis no longer clips that letter away. The heading reads `p₂ = p₁ + ρg z₁( − z₂)`.

A logo JPEG at most 400 px on a side is stored as PNG. Page photos stay JPEG. The NBS marks are about 130 px wide; PNG keeps PowerPoint from softening the DCT.

## 1.4.3

cesar2025-ai page 14 is Type 3 text: each letter is a glyph outline, and Noto Sans in its place was the 0.686 slide. Those outlines are drawn, and the characters stay in the text box with no fill so recall is unchanged. CJK stays filled text.

A small glyph that sits just below a letter (a smaller size and a lower baseline) joins that line as a run with `baseline` around `-25000`. A same-size number such as 2024 does not. On the KUST formula that makes `p₂ = p₁ + ρg(z…)`.

The Keynote lock on psumac page 30 is a symbolic TrueType glyph whose code is `$`. An isolated non-letter from that kind of font is drawn as the glyph outline. A comma that joins a sentence stays text.

NBS logo JPEGs carried a soft mask that was solid white. That mask no longer forces a PNG re-encode, so the marks stay the original JPEG. A text box that runs through the next word on the same line is pulled back and clipped (`per Andy` no longer covers `Morodo`).

`mit-ocw-ses1` is in the corpus manifest. `fetch.py` records that `tsmc-2q25-zh` currently returns a Cloudflare 403.

## 1.4.2

A clip no longer decodes a JPEG and writes it back as a PNG. DCT bytes pass through, and the crop is an `a:srcRect`. A PNG is still cropped in the file, because keeping the uncropped bitmap was larger (the UAI repeats). A page-sized JPEG with no alpha is not flattened. A soft-masked photo that comes out opaque is a JPEG again. `TestPictureCountBaseline` fails when a deck is more than 10% larger than its stored v1.4.0 size.

On the local corpus the PPTX total is 213,162,773 bytes, against 256,019,440 for v1.4.0 (−16.7%). No deck grew by more than 10%.

Shape boxes include Bézier handles again, so DrawingML does not clamp a ring or a chevron flat. Clip boxes still use the curve itself, so the NeurIPS footer stays cropped. A clip that removes less than 2% of a picture is left alone (the UNDP frame). A pattern tile that mostly sits in its cell is not shaved. Tiles that miss the cell are still dropped.

`sh` paints an axial shading clipped to the current rectangle. The NBS/Alibaba category panels are that gradient (gray 56 down to black), not the flat black page.

The korea-privacy recall gap versus v1.3.9 is the widened text boxes from that release. Those boxes collided columns and stay at the v1.3.7 width. The eight missing pictures are tiles that miss their cell; the same rule is what keeps gzmtu and nbs from the duplicate-tile bloat. That PDF is not in the local corpus, so the count was not remeasured here.

## 1.4.1

A rectangular clip crops the picture to that rectangle again, so the NeurIPS adoption chart loses the footer and the junk URL that v1.4.0 let back in. A clip that is the page only trims what hangs off the slide, which keeps the Canva diamonds. A picture that misses the clip is not deleted. A pattern tile that misses its cell is still dropped, so gzmtu and nbs-alibaba are not bloated with duplicate tiles.

Rounded-corner clips use the curve itself, not the control points, so the box is not tall enough to keep the footer.

Text fragments are no longer stitched, so the v1.3.7 tokens (`and`, `ffi`, `ill`) stay in the file. A same-position copy still lends its lighter color to the first copy. The recall gate fails if a deck loses more than 0.3% of its stored tokens.

`CORPORATE OVERVIEW` is bold: the face is TencentSansW7 with FontWeight 700, which is not a "-Bold" suffix.

## 1.4.0

v1.3.9 cropped every picture against the current clip, including photos that only hang off the edge of the slide. That cut the Canva diamonds and, on the UAI keynote, deleted 13 pictures whose boxes missed the clip. A clip now crops a picture only when it removes more than about a third of the picture (the NeurIPS tweet card). A picture that misses the clip entirely stays.

The Canva script wordmark is a letter outline filled by a tiling pattern that only paints an axial shading. That shading is now the fill, so the word is drawn.

Canva paints a white letter on top of a black copy in the same box. v1.3.9 kept the black copy and also joined neighboring words, which turned the Mahidol captions black and dropped tokens on the Rotary deck. Only a copy that sits on the same box is removed, and the lighter color is the one kept. Text boxes are no longer widened, so columns do not collide.

An Indexed color is looked up in its palette. A Separation tint is not painted as gray.

`TestPictureCountBaseline` also checks unique Latin tokens against the v1.3.7 counts. `TestCanvaScriptWordmarkIsDrawn` and `TestUAIPicturesAreNotClippedAway` cover the two decks when the PDFs are on the machine.

## 1.3.9

v1.3.8 composited every soft-masked picture into the image behind it and then deleted the picture. 24 decks lost logos and title art that way (the Mahidol Thai banner, the Railsware mark, the Beijing soft waves). That compositing is gone. A stored v1.3.7 baseline checks that `p:pic` counts do not fall; area may fall only where a clip crops overflow, and those decks are named in the baseline.

The Tencent wordmark PNG is white (255,255,255) with a straight alpha. v1.3.8 baked those white pixels into the background and PowerPoint still drew the mark black, so the bake was not the fix. The mark is its own picture again, with the alpha removed: the letters are opaque white on the blue sampled behind them (PNG color type 2, no alpha channel).

A rectangular clip is applied to the next image. The NeurIPS page 19 tweet is the top of a JPEG that the clip cuts off, and the page 8 ChatGPT icon no longer covers the leading 8 of "800 million". Text boxes stay `wrap="none"` with no autofit, and the box is wider than the PDF advance so the last glyph is not cut.

Canva paints each caption letter twice on the same spot. The second copy was starting a new box, so "Realistic" was drawn as "Re"+"ea"+… and the R disappeared. The duplicate is dropped and the letters join.

## 1.3.8

PDFMaker names a one-channel gray color space and then emits a single number. That number was stored as the blue channel, so bjedu-2026 and nbs-alibaba came out `#0000FF`. DeviceGray, CalGray, and an ICC profile with N=1 now consume one number and paint `(g,g,g)`.

The Tencent wordmark PNG is white with a soft mask. PowerPoint composites that alpha against black, so the logo stays black on the slide. The wordmark is composited onto the background image and the separate translucent picture is removed.

Marked content whose optional-content group is in the catalog OFF list is not painted.

## 1.3.7

Word spaces of about 0.2 em were dropped when the merge threshold moved to 0.22 em, so captions became "Gettyimages" and "AmadeuPons". The threshold is 0.15 em. Canva's per-letter boxes still overlap, so "Recognized" does not grow a space.

The Tencent wordmark is an indexed image. Opaque pixels are index 0, which is white. The lookup table was ignored, so the logo was painted black. Indexed color is applied.

The mine-safety "四" disc is a shading pattern whose color function is a sample table. That table is read, and the disc is a blue gradient (both ends the same blue).

A large character spacing on "()" in the KUST formula is split into separate glyphs, so the parentheses are not drawn as one pair in the middle of the expression.

## 1.3.6

PowerPoint on a 25-deck corpus stayed valid and the right size, and several slides were still wrong.

Google Slides paints a black rectangle with fill opacity 0 under a clipped image. That rectangle was written as opaque black, so it covered the teal page and the card text. A fill or stroke with opacity 0 is no longer emitted.

Acrobat PDFMaker attaches a two-byte codespace to a one-byte WinAnsi font. Even-length strings were decoded as one code per pair, so "CORPORATE OVERVIEW" became "CORP" and "Fourth Quarter of 2024" became "Four r of". The code width now follows the ToUnicode keys.

A path that covers the page and has a second subpath is a frame with a hole. It was absorbed as a solid page color, which is why the WPS white panel disappeared. Only a single rectangle becomes the page background.

A rotated image keeps the angle of its transform instead of being placed in the axis-aligned box. CJK punctuation no longer grows an ASCII space. Wingdings chevrons use Segoe UI Symbol, which PowerPoint has. Symbol private-use letters map to Unicode math.

`tools/ppt-verify.ps1 -Slides` accepts `1,2,23` and `1 2 23`. `TestTextRecall` compares pdftotext words with the text stored in the PPTX.

## 1.3.5

PowerPoint is the renderer these files have to satisfy. Three things LibreOffice did not show:

The MIT title slide's watermark is a JPEG with a soft mask whose matte is black, so the stored pixels are already multiplied by that black. They were written out that way, at 50% alpha. PowerPoint then paints a gray page instead of the light blue-white watermark. The matte is removed before the PNG is written, and a page-sized image is composited onto the page color so the file does not depend on PNG alpha.

"post-it", "top laning", and "Jossey-Bass" were several text boxes, including a hyphen whose box was wider than the glyph. PowerPoint drew a gap inside the word. Runs on one baseline that touch are now one paragraph.

Page 23 is a letter page with `/Rotate 90`. The boxes were moved into the displayed page, but each text box kept its −90° angle, so PowerPoint drew the lines on their side. The page angle is added to the text, image, and path, and the glyph box stays in its own axes. The upright page is then letterboxed into the 16:9 slide.

`tools/ppt-verify.ps1` opens a PPTX in PowerPoint, reports a repair error, and exports slides to PNG.

## 1.3.4

Mixed page sizes used the largest width and the largest height as two separate numbers. The MIT Session 1 deck has 22 pages at 960×540. Its last page is a 612×792 letter box with `/Rotate 90`, which displays as 792×612. The deck became 960×612 (`sldSz` 12192000×7772400), a size no page has. The slide size is now the most common page, and a tie keeps the first page. A page that differs is scaled down to fit, aspect ratio kept, and centered. The MIT deck is 960×540 pt, `sldSz cx="12192000" cy="6858000"`.

## 1.3.3

PowerPoint's repair dialog on pdf2ppt files came from markup the ECMA-376 schemas reject. The presentation omitted the required `notesSz` and the slide-master relationship. Custom geometry used `fill="evenOdd"`, which is not a legal path-fill value. Zip entries were stamped 1980-00-00, an invalid DOS date. A text outline was written after the run fill; the schema requires `a:ln` first.

The package now includes a blank slide master, layout, theme, and document properties. `tools/ooxmlvalidate` runs the Open XML SDK validator. `TestOOXMLKitchenSink` fails if a generated file is not valid.

## 1.3.2

Skill install is one command. Linux and macOS: `curl -fsSL .../install.sh | sh`. Windows: `iex (irm .../install.ps1)`. The skill lands in whichever of `~/.cursor/skills`, `~/.claude/skills`, and `~/.codex/skills` already exist, or in `~/.cursor/skills` if none do. `PDF2PPT_SKILL_DIR` overrides that. `convert.sh` is gone; the command is `pdf2ppt input.pdf output.pptx`.

## 1.3.1

The skill install no longer needs Go or git. `install.sh` / `install.ps1` download the release binary, check `SHA256SUMS`, and put it in `~/.local/bin` or `%LOCALAPPDATA%\pdf2ppt`. The same command downloads the skill folder. README.md and README.zh-CN.md start with a block an agent can paste. Releases also attach `pdf2ppt-skill.tar.gz` and `pdf2ppt-skill.zip`.

## 1.3.0

The Beamer line "THIS IS A" is regular text plus a heavy glyph contour. Dropping the contour left it thin next to the bold "SAMPLE". That contour is now a text outline, and the face is set bold because LibreOffice draws the outline faintly. "Gotham University" was condensed by large negative character spacing on a faux italic. Spacing is capped at a few percent of an em. A wider fallback is scaled uniformly, and Gill Sans is drawn with Liberation Sans, which has a real italic.

`skill/pdf2ppt` is an agent skill for Cursor, Claude Code, and other SKILL.md agents. `scripts/install.sh` and `scripts/install.ps1` install the CLI. Release tags build linux, darwin, and windows binaries for amd64 and arm64.

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
