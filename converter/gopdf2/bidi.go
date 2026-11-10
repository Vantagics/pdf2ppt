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

// joinCloseRTLPairs rebuilds one RTL line in reading order.
// Fragments are already logical. Visual order is left to right, so the
// paragraph is the right-hand fragment first. A colon is a neutral and
// stays against the word on its right ("הישראלי:"). A European number is
// a weak LTR run and keeps its digits ("מעל ל 50"). Pairing only the
// boxes that almost touched left "מעל ל" on the wrong side of the name.
func joinCloseRTLPairs(p *Page) {
	var refs []textRef
	for i := range p.Elements {
		t := p.Elements[i].Text
		if t == nil || t.Vertical || !horizontal(t.Rotation) || strings.TrimSpace(t.Content) == "" {
			continue
		}
		refs = append(refs, textRef{i, t})
	}
	if len(refs) < 2 {
		return
	}
	sortRefs(refs)
	remove := map[int]bool{}
	for i := 0; i < len(refs); {
		j := i + 1
		for j < len(refs) && sameLine(refs[i].t, refs[j].t) {
			j++
		}
		line := append([]textRef(nil), refs[i:j]...)
		i = j
		if len(line) < 2 || !lineHasRTL(line) {
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
			// A wide positive gap is the next column. An overlap is just
			// a box that is wider than its ink.
			if gap > em*2 {
				groups = append(groups, []textRef{r})
				continue
			}
			groups[len(groups)-1] = append(prev, r)
		}
		for _, g := range groups {
			if len(g) < 2 || !lineHasRTL(g) {
				continue
			}
			// A colon is a neutral that has to move with the embedding
			// direction ("הישראלי: מעל ל 50"). Other lines stay as the
			// v1.5.1 pairs: one box for the whole bullet packs the glyphs
			// and pulls 60 off the left edge.
			if lineHasColon(g) {
				p.Elements[g[0].i].Text = joinRTLReading(g)
				for _, r := range g[1:] {
					remove[r.i] = true
				}
				continue
			}
			joinClosePairs(p, g, remove)
		}
	}
	if len(remove) == 0 {
		return
	}
	kept := p.Elements[:0]
	for idx, el := range p.Elements {
		if remove[idx] {
			continue
		}
		kept = append(kept, el)
	}
	p.Elements = kept
}

func lineHasColon(line []textRef) bool {
	for _, r := range line {
		if strings.Contains(r.t.Content, ":") {
			return true
		}
	}
	return false
}

// joinClosePairs joins two neighboring runs whose boxes almost touch.
// The right-hand run comes first. A chain is not collapsed, so the rest
// of the bullet stays at its PDF x.
func joinClosePairs(p *Page, line []textRef, remove map[int]bool) {
	for k := 0; k+1 < len(line); k++ {
		if remove[line[k].i] || remove[line[k+1].i] {
			continue
		}
		left, right := line[k].t, line[k+1].t
		if !closeRTLPair(left, right) {
			continue
		}
		p.Elements[line[k].i].Text = joinRTLPair(right, left)
		remove[line[k+1].i] = true
		line[k].t = p.Elements[line[k].i].Text
	}
}

func closeRTLPair(left, right *TextObject) bool {
	if !hasRTLScript(left.Content) && !hasRTLScript(right.Content) {
		return false
	}
	if utf8.RuneCountInString(strings.TrimSpace(left.Content)) < 2 || utf8.RuneCountInString(strings.TrimSpace(right.Content)) < 2 {
		return false
	}
	gap := right.X - (left.X + left.Width)
	em := math.Max(left.FontSize, right.FontSize)
	if em < 1 {
		em = 12
	}
	return gap > -0.3*em && gap < 0.55*em
}

func joinRTLPair(right, left *TextObject) *TextObject {
	out := *right
	rs := strings.TrimSpace(right.Content)
	ls := strings.TrimSpace(left.Content)
	out.Content = rs + " " + ls
	out.RTL = true
	out.Runs = nil
	out.holdX = true
	if left.X < out.X {
		out.X = left.X
	}
	r := right.X + right.Width
	if left.X+left.Width > r {
		r = left.X + left.Width
	}
	out.Width = r - out.X
	if left.Y < out.Y {
		out.Y = left.Y
	}
	if left.Y+left.Height > out.Y+out.Height {
		out.Height = left.Y + left.Height - out.Y
	}
	return &out
}

func lineHasRTL(line []textRef) bool {
	for _, r := range line {
		if r.t.RTL || hasRTLScript(r.t.Content) {
			return true
		}
	}
	return false
}

func joinRTLReading(g []textRef) *TextObject {
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
	var prev *TextObject
	for i := len(items) - 1; i >= 0; i-- {
		t := items[i]
		piece := strings.TrimSpace(t.Content)
		if piece == "" {
			continue
		}
		if prev != nil && rtlWordSeparator(prev, t) {
			b.WriteByte(' ')
		}
		b.WriteString(piece)
		prev = t
	}
	out := *best
	out.Content = b.String()
	out.RTL = true
	out.Runs = nil
	out.holdX = true
	minX, minY := items[0].X, items[0].Y
	maxR, maxB := items[0].X+items[0].Width, items[0].Y+items[0].Height
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
	out.X, out.Y = minX, minY
	out.Width, out.Height = maxR-minX, maxB-minY
	if out.Width < 1 {
		out.Width = 1
	}
	return &out
}

// rtlWordSeparator reports a word space between the fragment already
// written (the one to the right) and the next fragment to its left.
// A colon is a neutral: it takes the embedding direction and sticks to
// the Hebrew word on its right. A deep overlap is one word painted
// twice, not a missing space.
func rtlWordSeparator(right, left *TextObject) bool {
	piece := strings.TrimSpace(left.Content)
	if piece == "" {
		return false
	}
	rightS := strings.TrimSpace(right.Content)
	// A bullet and a colon take a following space ("• איגוד", "BCG: ישראל").
	if rightS == "•" || strings.HasSuffix(rightS, ":") {
		return true
	}
	if neutralPunct(piece) {
		return false
	}
	gap := right.X - (left.X + left.Width)
	shorter := math.Min(right.Width, left.Width)
	if shorter > 1 && -gap > shorter*0.5 {
		return false
	}
	return true
}

func neutralPunct(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.IsPunct(r) && r != ':' {
			return false
		}
	}
	return true
}
