package gopdf2

import (
	"math"
	"strings"
	"unicode"
	"unicode/utf8"
)

// visualLTRToLogical turns a PDF text string that was painted left to right
// into logical order. Hebrew and Arabic glyphs are often stored in the order
// they appear on the page, so an LTR copy reads backwards. Digit and Latin
// runs keep their order.
func visualLTRToLogical(s string) string {
	type seg struct {
		rtl bool
		s   string
	}
	var segs []seg
	var b strings.Builder
	rtl := false
	flush := func() {
		if b.Len() == 0 {
			return
		}
		segs = append(segs, seg{rtl, b.String()})
		b.Reset()
	}
	for _, r := range s {
		isRTL := unicode.Is(unicode.Hebrew, r) || unicode.Is(unicode.Arabic, r)
		if b.Len() > 0 && isRTL != rtl {
			flush()
		}
		rtl = isRTL
		b.WriteRune(r)
	}
	flush()
	for i, j := 0, len(segs)-1; i < j; i, j = i+1, j-1 {
		segs[i], segs[j] = segs[j], segs[i]
	}
	var out strings.Builder
	for _, sg := range segs {
		if !sg.rtl {
			out.WriteString(sg.s)
			continue
		}
		rs := []rune(sg.s)
		for i, j := 0, len(rs)-1; i < j; i, j = i+1, j-1 {
			rs[i], rs[j] = rs[j], rs[i]
		}
		out.WriteString(string(rs))
	}
	return out.String()
}

func hasRTLScript(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Hebrew, r) || unicode.Is(unicode.Arabic, r) {
			return true
		}
	}
	return false
}

// reorderRTLLines builds one logical-order box for a mixed RTL line.
// Each fragment is already logical (digits and Latin keep their order).
// Joining those fragments left to right put "1,600,000" on the right of
// בישראל and glued "בניש" to "אילנה" with no space. Reading order is the
// right-hand fragment first, and a gap between fragments is a word space.
func reorderRTLLines(p *Page) {
	var refs []textRef
	for i := range p.Elements {
		t := p.Elements[i].Text
		if t == nil || t.Vertical || !horizontal(t.Rotation) {
			continue
		}
		refs = append(refs, textRef{i, t})
	}
	if len(refs) < 2 {
		return
	}
	sortRefs(refs)
	var lines [][]textRef
	for _, r := range refs {
		if len(lines) == 0 || !sameLine(lines[len(lines)-1][0].t, r.t) {
			lines = append(lines, []textRef{r})
			continue
		}
		lines[len(lines)-1] = append(lines[len(lines)-1], r)
	}
	remove := map[int]bool{}
	for _, line := range lines {
		if !lineHasRTL(line) {
			continue
		}
		var groups [][]textRef
		for _, r := range line {
			if len(groups) == 0 {
				groups = append(groups, []textRef{r})
				continue
			}
			prev := groups[len(groups)-1]
			last := prev[len(prev)-1].t
			gap := r.t.X - (last.X + last.Width)
			em := math.Max(last.FontSize, r.t.FontSize)
			if em < 1 {
				em = 12
			}
			if gap > em*1.6 {
				groups = append(groups, []textRef{r})
				continue
			}
			groups[len(groups)-1] = append(prev, r)
		}
		for _, g := range groups {
			if len(g) < 2 {
				continue
			}
			p.Elements[g[0].i].Text = joinRTLReadingOrder(g)
			for _, r := range g[1:] {
				remove[r.i] = true
			}
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

func lineHasRTL(line []textRef) bool {
	for _, r := range line {
		if r.t.RTL || hasRTLScript(r.t.Content) {
			return true
		}
	}
	return false
}

func joinRTLReadingOrder(g []textRef) *TextObject {
	items := make([]*TextObject, len(g))
	for i := range g {
		items[i] = g[i].t
	}
	best := items[0]
	for _, t := range items[1:] {
		if utf8.RuneCountInString(strings.TrimSpace(t.Content)) > utf8.RuneCountInString(strings.TrimSpace(best.Content)) {
			best = t
		}
	}
	var b strings.Builder
	var runs []TextRun
	var prev *TextObject
	push := func(content string, src *TextObject) {
		if content == "" {
			return
		}
		run := TextRun{
			Content: content, FontName: src.FontName, FontSize: src.FontSize,
			Bold: src.Bold, Italic: src.Italic,
			ColorR: src.ColorR, ColorG: src.ColorG, ColorB: src.ColorB,
		}
		if n := len(runs); n > 0 && sameStyle(runs[n-1], run) {
			runs[n-1].Content += content
			return
		}
		runs = append(runs, run)
	}
	for i := len(items) - 1; i >= 0; i-- {
		t := items[i]
		piece := strings.TrimSpace(t.Content)
		if piece == "" {
			continue
		}
		if prev != nil {
			gap := prev.X - (t.X + t.Width)
			em := math.Max(prev.FontSize, t.FontSize)
			if em < 1 {
				em = 12
			}
			if rtlWordGap(gap, em, strings.TrimSpace(prev.Content), piece) {
				b.WriteString(" ")
				push(" ", prev)
			}
		}
		b.WriteString(piece)
		push(piece, t)
		prev = t
	}
	out := *best
	out.Content = b.String()
	out.RTL = true
	out.Runs = runs
	out.holdX = false
	minX := items[0].X
	maxR := items[0].X + items[0].Width
	minY := items[0].Y
	maxB := items[0].Y + items[0].Height
	for _, t := range items[1:] {
		if t.X < minX {
			minX = t.X
		}
		if t.X+t.Width > maxR {
			maxR = t.X + t.Width
		}
		if t.Y < minY {
			minY = t.Y
		}
		if t.Y+t.Height > maxB {
			maxB = t.Y + t.Height
		}
	}
	out.X = minX
	out.Y = minY
	out.Width = maxR - minX
	out.Height = maxB - minY
	if out.Width < 1 {
		out.Width = 1
	}
	return &out
}

func rtlWordGap(gap, em float64, right, left string) bool {
	if gap > em*0.15 {
		return true
	}
	// Box padding eats the word space, so the two name boxes overlap by a
	// point or two and the gap is negative. A one-letter fragment is
	// kerning, not a word boundary. A deep overlap is one word painted twice.
	if utf8.RuneCountInString(right) > 1 && utf8.RuneCountInString(left) > 1 && gap > -0.35*em {
		return true
	}
	return false
}
