package gopdf2

import (
	"math"
	"strings"
	"unicode"
	"unicode/utf8"
)

// mergeTextLines joins PDF runs that share a baseline and sit next to each
// other into one text box. Separate boxes keep their own fallback widths, and
// PowerPoint then paints a gap in the middle of "post-it" or "Jossey-Bass".
// One paragraph lets PowerPoint place the glyphs. A wide gap stays a separate
// box, so columns do not collapse into one line.
type textRef struct {
	i int
	t *TextObject
}

func mergeTextLines(p *Page) {
	var refs []textRef
	for i := range p.Elements {
		t := p.Elements[i].Text
		if t == nil || !horizontal(t.Rotation) {
			continue
		}
		refs = append(refs, textRef{i, t})
	}
	if len(refs) < 2 {
		return
	}
	sortRefs(refs)
	var groups [][]textRef
	for _, r := range refs {
		if len(groups) == 0 {
			groups = append(groups, []textRef{r})
			continue
		}
		g := groups[len(groups)-1]
		if sameLine(g[0].t, r.t) && contiguous(g, r.t) {
			groups[len(groups)-1] = append(g, r)
			continue
		}
		groups = append(groups, []textRef{r})
	}
	remove := map[int]bool{}
	for _, g := range groups {
		if len(g) < 2 {
			continue
		}
		ts := make([]*TextObject, len(g))
		for i := range g {
			ts[i] = g[i].t
		}
		p.Elements[g[0].i].Text = buildMerged(ts)
		for _, r := range g[1:] {
			remove[r.i] = true
		}
	}
	if len(remove) == 0 {
		return
	}
	kept := p.Elements[:0]
	for i, el := range p.Elements {
		if remove[i] {
			continue
		}
		kept = append(kept, el)
	}
	p.Elements = kept
}

func sortRefs(refs []textRef) {
	for i := 1; i < len(refs); i++ {
		for j := i; j > 0; j-- {
			if refLess(refs[j], refs[j-1]) {
				refs[j], refs[j-1] = refs[j-1], refs[j]
				continue
			}
			break
		}
	}
}

func refLess(a, b textRef) bool {
	ba, bb := baseline(a.t), baseline(b.t)
	if math.Abs(ba-bb) > 0.4 {
		return ba < bb
	}
	return a.t.X < b.t.X
}

func horizontal(rot float64) bool {
	r := normRot(rot)
	return r < 1 || r > 359
}

func baseline(t *TextObject) float64 {
	return t.Y + t.FontSize
}

func sameLine(a, b *TextObject) bool {
	if a.Stroke != b.Stroke || a.StrokeOnly != b.StrokeOnly {
		return false
	}
	em := math.Max(a.FontSize, b.FontSize)
	return math.Abs(baseline(a)-baseline(b)) <= math.Max(1.25, em*0.18)
}

func contiguous(g []textRef, t *TextObject) bool {
	prev := g[len(g)-1].t
	gap := t.X - (prev.X + prev.Width)
	em := math.Max(prev.FontSize, t.FontSize)
	if em < 1 {
		em = 1
	}
	if gap < 0 {
		overlap := -gap
		shorter := math.Min(prev.Width, t.Width)
		if shorter > 0 && overlap > shorter*0.55 {
			return false
		}
	}
	return gap <= em*0.85
}

func buildMerged(ts []*TextObject) *TextObject {
	cp := make([]*TextObject, len(ts))
	for i, t := range ts {
		c := *t
		cp[i] = &c
	}
	for i, t := range cp {
		if !isSpacer(t.Content) {
			continue
		}
		body := nearestBody(cp, i)
		if body == nil || body == t {
			continue
		}
		if body.FontSize > 0 && math.Abs(t.FontSize-body.FontSize)/body.FontSize > 0.12 {
			t.FontSize = body.FontSize
			t.FontName = body.FontName
			t.Bold = body.Bold
			t.Italic = body.Italic
			t.ColorR, t.ColorG, t.ColorB = body.ColorR, body.ColorG, body.ColorB
		}
	}
	base := baseline(cp[0])
	maxSize := cp[0].FontSize
	minX := cp[0].X
	maxR := cp[0].X + cp[0].Width
	maxH := cp[0].Height
	for _, t := range cp {
		if b := baseline(t); !isSpacer(t.Content) {
			base = b
		}
		if t.FontSize > maxSize {
			maxSize = t.FontSize
		}
		if t.X < minX {
			minX = t.X
		}
		if t.X+t.Width > maxR {
			maxR = t.X + t.Width
		}
		if t.Height > maxH {
			maxH = t.Height
		}
	}
	runs := coalesce(cp)
	var text string
	for _, r := range runs {
		text += r.Content
	}
	out := &TextObject{
		Content:  text,
		X:        minX,
		Y:        base - maxSize,
		Width:    math.Max(maxR-minX, 1),
		Height:   math.Max(maxH, maxSize*1.15),
		Rotation: cp[0].Rotation,
		FontName: runs[0].FontName,
		FontSize: runs[0].FontSize,
		Bold:     runs[0].Bold,
		Italic:   runs[0].Italic,
		ColorR:   runs[0].ColorR,
		ColorG:   runs[0].ColorG,
		ColorB:   runs[0].ColorB,
	}
	if len(runs) > 1 {
		out.Runs = runs
		// The box size is the largest run. The first run keeps its own size.
		out.FontSize = maxSize
	}
	return out
}

func nearestBody(ts []*TextObject, i int) *TextObject {
	for dist := 1; dist < len(ts); dist++ {
		if i-dist >= 0 && !isSpacer(ts[i-dist].Content) {
			return ts[i-dist]
		}
		if i+dist < len(ts) && !isSpacer(ts[i+dist].Content) {
			return ts[i+dist]
		}
	}
	return nil
}

func isSpacer(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return true
	}
	if utf8.RuneCountInString(s) > 2 {
		return false
	}
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func coalesce(ts []*TextObject) []TextRun {
	var runs []TextRun
	push := func(r TextRun) {
		if r.Content == "" {
			return
		}
		if n := len(runs); n > 0 && sameStyle(runs[n-1], r) {
			runs[n-1].Content += r.Content
			return
		}
		runs = append(runs, r)
	}
	for i, t := range ts {
		if i > 0 {
			prev := ts[i-1]
			gap := t.X - (prev.X + prev.Width)
			em := prev.FontSize
			if em < 1 {
				em = t.FontSize
			}
			// 0.15 em is under a normal word space (~0.25 em) and above the
			// negative gaps between Canva's per-letter boxes. 0.22 em dropped
			// "Getty images" (gap 0.21 em) and "Amadeu Pons".
			if gap > em*0.15 && !strings.HasSuffix(prev.Content, " ") && !strings.HasPrefix(t.Content, " ") && !cjkBoundary(prev.Content, t.Content) {
				push(TextRun{
					Content: " ", FontName: prev.FontName, FontSize: prev.FontSize,
					Bold: prev.Bold, Italic: prev.Italic,
					ColorR: prev.ColorR, ColorG: prev.ColorG, ColorB: prev.ColorB,
				})
			}
		}
		push(TextRun{
			Content: t.Content, FontName: t.FontName, FontSize: t.FontSize,
			Bold: t.Bold, Italic: t.Italic,
			ColorR: t.ColorR, ColorG: t.ColorG, ColorB: t.ColorB,
		})
	}
	if len(runs) == 0 {
		runs = []TextRun{{Content: ""}}
	}
	return runs
}

func cjkBoundary(left, right string) bool {
	return endsWithCJK(left) || startsWithCJK(right)
}

func startsWithCJK(s string) bool {
	if s == "" {
		return false
	}
	r, _ := utf8.DecodeRuneInString(s)
	return r >= 0x2E80 && r <= 0x9FFF || r >= 0x3000 && r <= 0x303F || r >= 0xFF00 && r <= 0xFFEF
}

func sameStyle(a, b TextRun) bool {
	return a.FontName == b.FontName && a.Bold == b.Bold && a.Italic == b.Italic &&
		a.ColorR == b.ColorR && a.ColorG == b.ColorG && a.ColorB == b.ColorB &&
		math.Abs(a.FontSize-b.FontSize) < 1
}
