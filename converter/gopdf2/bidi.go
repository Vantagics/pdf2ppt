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

// joinCloseRTLPairs joins two neighboring Hebrew words, or a Hebrew word
// and a number, when their boxes almost touch. The right-hand word comes
// first. A whole line is not collapsed: that packed the 60 off the left
// edge and glued "OCR" to the Hebrew beside it. The 50 line stays in
// separate boxes at the PDF x, which is the reading order.
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
			// Only a Hebrew word and its neighbor (the name, or בישראל
			// next to 1,600,000). Joining a whole line packs the glyphs
			// and glues "OCR" to the following Hebrew.
			joinClosePairs(p, g, remove)
		}
	}
	padRTLBoundaries(p)
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

func mostlyNumber(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	digits := 0
	for _, r := range s {
		if r >= '0' && r <= '9' {
			digits++
		}
	}
	return digits >= 2 && digits*2 >= utf8.RuneCountInString(s)
}

func closeRTLPair(left, right *TextObject) bool {
	lrtl := hasRTLScript(left.Content)
	rrtl := hasRTLScript(right.Content)
	// Latin next to Hebrew ("OCR", "Microsoft") stays in its own box so
	// the PDF gap survives. A number next to Hebrew is the 1,600,000 pair.
	switch {
	case lrtl && rrtl:
	case lrtl && mostlyNumber(right.Content):
	case rrtl && mostlyNumber(left.Content):
	default:
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

func appendSpace(t *TextObject) {
	if strings.HasSuffix(t.Content, " ") {
		return
	}
	t.Content += " "
	if n := len(t.Runs); n > 0 {
		t.Runs[n-1].Content += " "
	}
}

// padRTLBoundaries keeps a trailing space on Hebrew, bullets, and
// colons, and stops merge from gluing the next word on. The space is
// inside the run, so the box is not shrunk to invent a gap.
func padRTLBoundaries(p *Page) {
	for i := range p.Elements {
		t := p.Elements[i].Text
		if t == nil || t.Vertical {
			continue
		}
		s := strings.TrimSpace(t.Content)
		if s == "" {
			continue
		}
		if hasRTLScript(t.Content) {
			t.holdX = true
			// A one-letter Hebrew prefix (ו, ב, ה) is its own glyph. A
			// trailing space would split "ולוודא" into two tokens.
			if utf8.RuneCountInString(s) >= 2 {
				appendSpace(t)
			}
		}
		if s == "•" || s == ":" || strings.HasSuffix(s, ":") {
			appendSpace(t)
		}
	}
}

// separateRTLNeighbors puts a space between a Hebrew or Arabic run and a
// Latin or digit run that sit in neighboring boxes. Concatenated a:t has
// no separator, so "Word" + "ניתנים" was one token and the source word
// dropped out of unique recall.
func separateRTLNeighbors(p *Page) {
	var prev *TextObject
	for i := range p.Elements {
		t := p.Elements[i].Text
		if t == nil || strings.TrimSpace(t.Content) == "" {
			continue
		}
		if prev != nil && scriptBoundary(prev.Content, t.Content) && !strings.HasSuffix(prev.Content, " ") && !strings.HasPrefix(t.Content, " ") {
			prev.Content += " "
			// The written a:t comes from the runs when a box has any.
			if n := len(prev.Runs); n > 0 {
				prev.Runs[n-1].Content += " "
			}
		}
		prev = t
	}
	for i := range p.Elements {
		t := p.Elements[i].Text
		if t == nil {
			continue
		}
		t.Content = spaceScriptTouches(t.Content)
		for r := range t.Runs {
			t.Runs[r].Content = spaceScriptTouches(t.Runs[r].Content)
		}
	}
}

// spaceScriptTouches inserts a space where a Hebrew or Arabic letter is
// glued to a Latin letter or a digit inside one run ("Aניווט", "2תוויות").
func spaceScriptTouches(s string) string {
	var b strings.Builder
	var prev rune
	for _, r := range s {
		if prev != 0 && !unicode.IsSpace(r) && !unicode.IsSpace(prev) && wordEdge(prev) && wordEdge(r) && rtlLetter(prev) != rtlLetter(r) {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
		prev = r
	}
	return b.String()
}

func wordEdge(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

func rtlLetter(r rune) bool {
	return unicode.Is(unicode.Hebrew, r) || unicode.Is(unicode.Arabic, r)
}

func scriptKind(r rune) int {
	switch {
	case unicode.Is(unicode.Hebrew, r) || unicode.Is(unicode.Arabic, r):
		return 1
	case unicode.IsLetter(r) || unicode.IsDigit(r):
		return 2
	default:
		return 0
	}
}

func edgeKind(s string, last bool) int {
	rs := []rune(s)
	if last {
		for i := len(rs) - 1; i >= 0; i-- {
			if k := scriptKind(rs[i]); k != 0 {
				return k
			}
		}
		return 0
	}
	for _, r := range rs {
		if k := scriptKind(r); k != 0 {
			return k
		}
	}
	return 0
}

func scriptBoundary(a, b string) bool {
	ka, kb := edgeKind(a, true), edgeKind(b, false)
	return ka != 0 && kb != 0 && ka != kb
}

func lineHasRTL(line []textRef) bool {
	for _, r := range line {
		if r.t.RTL || hasRTLScript(r.t.Content) {
			return true
		}
	}
	return false
}
