package gopdf2

import (
	"math"
	"os"
	"os/exec"
	"strings"
	"sync"
	"unicode/utf8"

	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// fitRun chooses the system font LibreOffice will actually draw and, when
// that font's advance differs from the PDF run, returns a size and a small
// per-glyph tracking (points). Tracking stays within a few percent of an em
// so glyphs are not condensed; a larger mismatch is a uniform size change.
func fitRun(family string, bold, italic bool, size, pdfWidth float64, text string) (string, float64, float64) {
	if size < 0.5 || pdfWidth < 0.5 || strings.TrimSpace(text) == "" {
		return family, size, 0
	}
	// Office cloud fonts are downloaded by Microsoft 365. Renaming them
	// to Noto Sans or Calibri draws a different face. The PDF advances
	// already are that face, so the size and spacing stay.
	if isOfficeCloudFont(family) {
		return family, size, 0
	}
	family = preferMetricFamily(family)
	path, resolved, ok := matchSystemFont(family, bold, italic)
	if !ok {
		return family, size, 0
	}
	measured, covered := measureText(path, size, text)
	if !covered || measured < 0.5 {
		return family, size, 0
	}
	use := family
	if !fontNamesClose(family, resolved) {
		use = resolved
	}
	sz, tr := trackingFor(text, size, pdfWidth, measured, func(sz float64) float64 {
		w, ok := measureText(path, sz, text)
		if !ok {
			return 0
		}
		return w
	})
	return use, sz, tr
}

// trackingFor returns the font size and per-character tracking (points) that
// make measured ink match pdfWidth. Spacing is applied between characters,
// not after the last one.

// officeCloudFonts are faces Microsoft 365 downloads on demand into
// AppData\Local\Microsoft\FontCache\4\CloudFonts. The PDF advances are
// that face, so the typeface name stays and spc stays near zero.
// Faces that are not in this list still fall back to Calibri or the
// local metric match.
var officeCloudFonts = []string{
	"lato",
	"anton",
	"montserrat",
	"open sans",
	"roboto",
	"poppins",
	"oswald",
	"raleway",
	"nunito",
	"inter",
	"source sans",
}

func isOfficeCloudFont(family string) bool {
	n := strings.ToLower(family)
	n = strings.ReplaceAll(n, "-", " ")
	for _, f := range officeCloudFonts {
		if strings.Contains(n, f) {
			return true
		}
	}
	return false
}

// fitCloudFontSpacing sets a:rPr spc from the real font, when that font
// is installed here, against the PDF width. Lato is not installed on the
// build machine; its PDF widths already match the cloud font, so spc
// stays 0. The typeface name is not rewritten.
func fitCloudFontSpacing(p *Page) {
	for i := range p.Elements {
		t := p.Elements[i].Text
		if t == nil || t.Vertical || !isOfficeCloudFont(t.FontName) || t.Width < 0.5 || strings.TrimSpace(t.Content) == "" {
			continue
		}
		path, resolved, ok := matchSystemFont(t.FontName, t.Bold, t.Italic)
		if !ok || !fontNamesClose(t.FontName, resolved) {
			t.Tracking = 0
			continue
		}
		measured, covered := measureText(path, t.FontSize, t.Content)
		if !covered || measured < 0.5 {
			t.Tracking = 0
			continue
		}
		gaps := utf8.RuneCountInString(t.Content) - 1
		if gaps < 1 || math.Abs(measured-t.Width)/t.Width < 0.015 {
			t.Tracking = 0
			continue
		}
		t.Tracking = math.Round((t.Width-measured)/float64(gaps)*100) / 100
	}
}

// preferMetricFamily maps faces that are not installed here onto a system
// family with a real bold and italic, instead of a faux slant of Cantarell.
func preferMetricFamily(family string) string {
	switch strings.ToLower(strings.TrimSpace(family)) {
	case "gill sans", "gill sans mt", "gillsans":
		return "Liberation Sans"
	case "trebuchet ms", "trebuchetms":
		return "Liberation Sans"
	case "palatino", "palatino-roman", "palatino linotype", "palatinolinotype":
		return "Liberation Serif"
	case "marker felt", "marker felt thin", "markerfelt", "markerfelt-thin":
		return "Liberation Sans"
	default:
		return family
	}
}

func trackingFor(text string, size, pdfWidth, measured float64, remeasure func(float64) float64) (float64, float64) {
	gaps := len([]rune(text)) - 1
	ratio := pdfWidth / measured
	if ratio > 0.985 && ratio < 1.015 {
		return size, 0
	}
	// Scale the whole em first. Negative tracking condenses the letters, which
	// is the "Gotham University" squeeze, so it is only a small remainder.
	sz := size * ratio
	if sz < size*0.82 {
		sz = size * 0.82
	}
	if sz > size*1.12 {
		sz = size * 1.12
	}
	if math.Abs(sz-size) > 0.05 {
		if w := remeasure(sz); w > 0.5 {
			measured = w
		}
	} else {
		sz = size
	}
	if gaps < 1 || measured < 0.5 {
		return sz, 0
	}
	tr := (pdfWidth - measured) / float64(gaps)
	limit := sz * 0.04
	if tr > limit {
		tr = limit
	}
	if tr < -limit {
		tr = -limit
	}
	if math.Abs(tr) < 0.03 {
		return sz, 0
	}
	return sz, tr
}

type fontHit struct {
	path, family string
	ok           bool
}

var (
	fontMu     sync.Mutex
	matchCache = map[string]fontHit{}
	faceCache  = map[string]*sfnt.Font{}
)

func matchSystemFont(family string, bold, italic bool) (path, resolved string, ok bool) {
	family = strings.TrimSpace(family)
	if family == "" {
		return "", "", false
	}
	key := family
	if bold {
		key += "|b"
	}
	if italic {
		key += "|i"
	}
	fontMu.Lock()
	if hit, found := matchCache[key]; found {
		fontMu.Unlock()
		return hit.path, hit.family, hit.ok
	}
	fontMu.Unlock()

	pattern := family
	switch {
	case bold && italic:
		pattern += ":weight=bold:slant=italic"
	case bold:
		pattern += ":weight=bold"
	case italic:
		pattern += ":slant=italic"
	}
	cmd := exec.Command("fc-match", "-f", "%{file}\t%{family}\n", pattern)
	out, err := cmd.Output()
	hit := fontHit{}
	if err == nil {
		line := strings.TrimSpace(string(out))
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) == 2 && parts[0] != "" {
			fam := parts[1]
			if i := strings.IndexByte(fam, ','); i >= 0 {
				fam = fam[:i]
			}
			hit = fontHit{path: parts[0], family: strings.TrimSpace(fam), ok: true}
		}
	}
	fontMu.Lock()
	matchCache[key] = hit
	fontMu.Unlock()
	return hit.path, hit.family, hit.ok
}

func fontNamesClose(want, got string) bool {
	w := strings.ToLower(strings.TrimSpace(want))
	g := strings.ToLower(strings.TrimSpace(got))
	if w == "" || g == "" {
		return false
	}
	return strings.Contains(g, w) || strings.Contains(w, g)
}

// timesWidth estimates a Times-compatible advance in points. PowerPoint
// draws the KUST formula with Times New Roman. Tinos uses the same widths,
// and fc-match is not there on the Windows build, so the fence nudge uses
// this table instead of a local font file.
func timesWidth(size float64, text string) float64 {
	if size <= 0 || text == "" {
		return 0
	}
	var units float64
	for _, r := range text {
		units += float64(timesUnit(r))
	}
	return units / 1000 * size
}

func timesUnit(r rune) int {
	switch r {
	case 'ρ':
		return 499
	case '−', '+':
		return 563
	case '–':
		return 500
	case '—':
		return 1000
	case '×':
		return 563
	case '·':
		return 333
	case '’':
		return 333
	case '“', '”':
		return 443
	}
	if r >= 32 && r <= 126 {
		return timesASCII[r-32]
	}
	return 500
}

// timesASCII is the Tinos advance of U+0020..U+007E in em/1000 units.
var timesASCII = [95]int{
	250, 333, 408, 500, 500, 833, 777, 180, 333, 333, 500, 563, 250, 333, 250, 277,
	500, 500, 500, 500, 500, 500, 500, 500, 500, 500, 277, 277, 563, 563, 563, 443,
	920, 722, 667, 667, 722, 610, 556, 722, 722, 333, 389, 722, 610, 889, 722, 722,
	556, 722, 667, 556, 610, 722, 722, 943, 722, 722, 610, 333, 277, 333, 469, 500,
	333, 443, 500, 443, 500, 443, 333, 500, 500, 277, 277, 500, 277, 777, 500, 500,
	500, 500, 333, 389, 277, 500, 500, 722, 500, 500, 443, 479, 200, 479, 541,
}

// textWidth is the font advance of text in points. A trailing space is
// included when it is part of text; callers that only want ink trim it.
func textWidth(family string, bold, italic bool, size float64, text string) (float64, bool) {
	if text == "" || size < 0.5 {
		return 0, true
	}
	path, _, ok := matchSystemFont(preferMetricFamily(family), bold, italic)
	if !ok {
		return 0, false
	}
	return measureText(path, size, text)
}

func measureText(path string, sizePt float64, text string) (width float64, covered bool) {
	face := loadFace(path)
	if face == nil || sizePt < 0.5 {
		return 0, false
	}
	var buf sfnt.Buffer
	ppem := fixed.Int26_6(sizePt * 64)
	runes := []rune(text)
	missing := 0
	letters := 0
	var adv fixed.Int26_6
	for _, r := range runes {
		if r == '\n' || r == '\r' {
			continue
		}
		letters++
		gi, err := face.GlyphIndex(&buf, r)
		if err != nil || gi == 0 {
			missing++
			continue
		}
		a, err := face.GlyphAdvance(&buf, gi, ppem, font.HintingNone)
		if err != nil {
			missing++
			continue
		}
		adv += a
	}
	if letters == 0 || missing*2 > letters {
		return 0, false
	}
	return float64(adv) / 64, true
}

func loadFace(path string) *sfnt.Font {
	fontMu.Lock()
	defer fontMu.Unlock()
	if f, ok := faceCache[path]; ok {
		return f
	}
	b, err := os.ReadFile(path)
	if err != nil {
		faceCache[path] = nil
		return nil
	}
	f, err := sfnt.Parse(b)
	if err != nil {
		faceCache[path] = nil
		return nil
	}
	faceCache[path] = f
	return f
}

// suppressDuplicateGlyphOutlines drops stroke-only letter contours that sit
// inside a text run. Some exports (Beamer via xdvipdfmx) paint the glyphs as
// text and again as stroked outlines. With a substituted font the two copies
// no longer coincide, so the title looks doubled.
func suppressDuplicateGlyphOutlines(els []PageElement) []PageElement {
	var texts []*TextObject
	for _, el := range els {
		if el.Text != nil && el.Text.Content != "" {
			texts = append(texts, el.Text)
		}
	}
	if len(texts) == 0 {
		return els
	}
	out := make([]PageElement, 0, len(els))
	for _, el := range els {
		if el.Shape != nil {
			if t := outlineHost(el.Shape, texts); t != nil {
				// The contour is part of the letter's weight (the light
				// "THIS IS A" next to a heavy "SAMPLE"). Keep it as a text
				// outline on the same run instead of a second, misaligned copy.
				absorbOutline(t, el.Shape)
				continue
			}
		}
		out = append(out, el)
	}
	return out
}

func outlineHost(s *ShapeObject, texts []*TextObject) *TextObject {
	if !glyphOutlineOfText(s, texts) {
		return nil
	}
	cx := s.X + s.Width/2
	cy := s.Y + s.Height/2
	const pad = 3.0
	for _, t := range texts {
		if t.FontSize < 4 || t.Width < 2 {
			continue
		}
		if cx < t.X-pad || cx > t.X+t.Width+pad || cy < t.Y-pad || cy > t.Y+t.Height+pad {
			continue
		}
		return t
	}
	return nil
}

func absorbOutline(t *TextObject, s *ShapeObject) {
	if s.StrokeWidth < 0.35 {
		return
	}
	// A run that was already fill-and-stroke (PowerPoint mode 2) keeps its
	// own weight. Only a plain fill that was doubled by a heavy contour
	// needs the bold face; LibreOffice draws a:ln too faintly to carry it.
	hadStroke := t.Stroke
	if !t.Stroke || t.StrokeWidth < s.StrokeWidth {
		t.Stroke = true
		t.StrokeOnly = false
		t.StrokeWidth = s.StrokeWidth
		t.StrokeR, t.StrokeG, t.StrokeB = s.StrokeR, s.StrokeG, s.StrokeB
	}
	if hadStroke || t.Bold || s.StrokeWidth < t.FontSize*0.025 {
		return
	}
	t.Bold = true
	target := t.Width - math.Min(t.FontSize*0.08, 1.2)
	if target < t.FontSize*0.4 {
		target = t.Width
	}
	fam, sz, tr := fitRun(t.FontName, true, t.Italic, t.FontSize, target, t.Content)
	if fam != "" {
		t.FontName = fam
	}
	if sz > 0.5 && math.Abs(sz-t.FontSize) > 0.05 {
		t.Y += t.FontSize - sz
		t.FontSize = sz
	}
	t.Tracking = tr
}

func glyphOutlineOfText(s *ShapeObject, texts []*TextObject) bool {
	if s == nil || s.HasFill || !s.HasStroke {
		return false
	}
	if s.Height < 4 || s.Width < 1.5 {
		return false
	}
	// A rectangular "I" stem is a simple rect. Horizontal rules are not.
	if s.SimpleRect || len(s.Path) < 6 {
		if s.Height < s.Width*1.3 {
			return false
		}
	}
	// Rules and underlines are short. Letter contours are most of an em tall.
	if s.Height*4 < s.Width {
		return false
	}
	cx := s.X + s.Width/2
	cy := s.Y + s.Height/2
	const pad = 3.0
	for _, t := range texts {
		if t.FontSize < 4 || t.Width < 2 {
			continue
		}
		if s.Height < t.FontSize*0.35 {
			continue
		}
		if cx < t.X-pad || cx > t.X+t.Width+pad || cy < t.Y-pad || cy > t.Y+t.Height+pad {
			continue
		}
		if s.X < t.X-pad || s.Y < t.Y-pad {
			continue
		}
		if s.X+s.Width > t.X+t.Width+pad || s.Y+s.Height > t.Y+t.Height+pad {
			continue
		}
		return true
	}
	return false
}

// InkWidth is the width the resolved system font will paint, including tracking.
func InkWidth(t *TextObject) float64 {
	if t == nil {
		return 0
	}
	path, _, ok := matchSystemFont(t.FontName, t.Bold, t.Italic)
	if !ok {
		return t.Width
	}
	w, covered := measureText(path, t.FontSize, t.Content)
	if !covered {
		return t.Width
	}
	n := len([]rune(t.Content))
	if n > 1 {
		w += t.Tracking * float64(n-1)
	}
	return w
}

// TextOverlaps reports runs on the same baseline whose boxes intersect.
// A wider fallback font that was not fitted shows up here once the box
// width follows the measured ink.
func TextOverlaps(texts []*TextObject) []string {
	var msgs []string
	for i := 0; i < len(texts); i++ {
		a := texts[i]
		if a == nil || strings.TrimSpace(a.Content) == "" {
			continue
		}
		for j := i + 1; j < len(texts); j++ {
			b := texts[j]
			if b == nil || strings.TrimSpace(b.Content) == "" {
				continue
			}
			if !sameBaseline(a, b) {
				continue
			}
			left := math.Max(a.X, b.X)
			right := math.Min(a.X+a.Width, b.X+b.Width)
			if right-left > 1.5 {
				msgs = append(msgs, a.Content+" ~ "+b.Content)
			}
		}
	}
	return msgs
}

func sameBaseline(a, b *TextObject) bool {
	// The box top sits one em above the PDF baseline.
	ba := a.Y + a.FontSize
	bb := b.Y + b.FontSize
	tol := 0.28 * math.Min(a.FontSize, b.FontSize)
	if tol < 1.5 {
		tol = 1.5
	}
	return math.Abs(ba-bb) <= tol
}
