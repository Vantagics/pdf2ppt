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

// joinCloseRTLPairs joins two neighboring runs whose boxes almost touch.
// Merge walks left to right, so "בניש" (left) was glued in front of
// "אילנה" (right) and "1,600,000" landed in front of בישראל. The right-hand
// run is the one that comes first in reading order. A wide overlap is a
// whole line whose own spaces already match the v1.5.0 text, and joining
// it dropped those words.
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
		line := refs[i:j]
		i = j
		for k := 0; k+1 < len(line); k++ {
			a, b := line[k].t, line[k+1].t
			if remove[line[k].i] || remove[line[k+1].i] {
				continue
			}
			if !closeRTLPair(a, b) {
				continue
			}
			// b is to the right of a.
			p.Elements[line[k].i].Text = joinRTLPair(b, a)
			remove[line[k+1].i] = true
			line[k].t = p.Elements[line[k].i].Text
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
