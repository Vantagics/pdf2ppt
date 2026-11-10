package gopdf2

import (
	"math"
	"os"
	"os/exec"
	"strings"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// fitRun chooses the system font LibreOffice will actually draw and, when
// that font's advance differs from the PDF run, returns a size and per-glyph
// tracking (points, negative tightens) so the run occupies pdfWidth.
func fitRun(family string, bold, italic bool, size, pdfWidth float64, text string) (string, float64, float64) {
	if size < 0.5 || pdfWidth < 0.5 || strings.TrimSpace(text) == "" {
		return family, size, 0
	}
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
func trackingFor(text string, size, pdfWidth, measured float64, remeasure func(float64) float64) (float64, float64) {
	gaps := len([]rune(text)) - 1
	ratio := pdfWidth / measured
	if ratio > 0.985 && ratio < 1.015 {
		return size, 0
	}
	sz := size
	// Prefer tracking over a size change so runs that share a line stay the
	// same height. Only a large mismatch is worth scaling.
	if ratio < 0.70 || ratio > 1.40 {
		sz = size * ratio
		if sz < size*0.72 {
			sz = size * 0.72
		}
		if sz > size*1.2 {
			sz = size * 1.2
		}
		if w := remeasure(sz); w > 0.5 {
			measured = w
		}
	}
	if gaps < 1 || measured < 0.5 {
		return sz, 0
	}
	tr := (pdfWidth - measured) / float64(gaps)
	limit := sz * 0.45
	if tr > limit {
		tr = limit
	}
	if tr < -limit {
		tr = -limit
	}
	if math.Abs(tr) < 0.04 {
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
		if el.Shape != nil && glyphOutlineOfText(el.Shape, texts) {
			continue
		}
		out = append(out, el)
	}
	return out
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
