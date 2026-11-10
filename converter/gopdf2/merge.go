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
	remove := map[int]bool{}
	for _, r := range refs {
		if len(groups) == 0 {
			groups = append(groups, []textRef{r})
			continue
		}
		g := groups[len(groups)-1]
		if sameLine(g[0].t, r.t) && duplicateOverlap(g[len(g)-1].t, r.t) {
			// Same letters painted twice. Keep both runs so the original
			// fragment ("ffi", "and", "ill") stays in the text. The later
			// copy is the lighter one; give that color to the first copy
			// too, or a black shadow is what PowerPoint shows.
			prev := g[len(g)-1].t
			if luminance(r.t) > luminance(prev) {
				prev.ColorR, prev.ColorG, prev.ColorB = r.t.ColorR, r.t.ColorG, r.t.ColorB
			}
		}
		if sameLine(g[0].t, r.t) && contiguous(g, r.t) {
			groups[len(groups)-1] = append(g, r)
			continue
		}
		groups = append(groups, []textRef{r})
	}
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
	if a.NoFill != b.NoFill {
		return false
	}
	em := math.Max(a.FontSize, b.FontSize)
	return math.Abs(baseline(a)-baseline(b)) <= math.Max(1.25, em*0.18)
}

func luminance(t *TextObject) int {
	return int(t.ColorR) + int(t.ColorG) + int(t.ColorB)
}

func duplicateOverlap(prev, t *TextObject) bool {
	if prev.Content == "" || prev.Content != t.Content {
		return false
	}
	gap := t.X - (prev.X + prev.Width)
	if gap >= 0 {
		return false
	}
	shorter := math.Min(prev.Width, t.Width)
	return shorter > 0 && -gap > shorter*0.92
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
	allHidden := true
	for _, t := range cp {
		if !t.NoFill {
			allHidden = false
			break
		}
	}
	out.NoFill = allHidden
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
		math.Abs(a.FontSize-b.FontSize) < 1 && math.Abs(a.Baseline-b.Baseline) < 1
}

// attachSubscripts folds a small glyph that sits just below a letter into
// that letter's text box as a baseline-shifted run. A same-size number such
// as 2024 shares the body's baseline and size, so it stays its own text.
func attachSubscripts(p *Page) {
	type ref struct {
		i int
		t *TextObject
	}
	var refs []ref
	for i := range p.Elements {
		t := p.Elements[i].Text
		if t == nil || !horizontal(t.Rotation) || t.Content == "" {
			continue
		}
		refs = append(refs, ref{i, t})
	}
	remove := map[int]bool{}
	type pair struct {
		body *TextObject
		sub  *TextObject
		si   int
	}
	var pairs []pair
	for _, sub := range refs {
		if !subscriptCandidate(sub.t) {
			continue
		}
		for i := range refs {
			b := &refs[i]
			if b.i == sub.i || subscriptCandidate(b.t) || !subscriptHost(b.t, sub.t) {
				continue
			}
			pairs = append(pairs, pair{b.t, sub.t, sub.i})
			break
		}
	}
	// Rightmost subscript first so earlier cut indexes stay put.
	for i := 1; i < len(pairs); i++ {
		for j := i; j > 0 && pairs[j].sub.X > pairs[j-1].sub.X; j-- {
			pairs[j], pairs[j-1] = pairs[j-1], pairs[j]
		}
	}
	for _, pair := range pairs {
		if remove[pair.si] {
			continue
		}
		insertSubscript(pair.body, pair.sub)
		remove[pair.si] = true
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

func subscriptCandidate(t *TextObject) bool {
	n := utf8.RuneCountInString(strings.TrimSpace(t.Content))
	return n >= 1 && n <= 2
}

func subscriptHost(body, sub *TextObject) bool {
	if body.FontSize < 1 || sub.FontSize < 1 {
		return false
	}
	if sub.FontSize/body.FontSize > 0.75 {
		return false
	}
	delta := baseline(sub) - baseline(body)
	em := body.FontSize
	if delta < em*0.08 || delta > em*0.55 {
		return false
	}
	if sub.X+sub.Width < body.X-em*0.15 || sub.X > body.X+body.Width+em*0.4 {
		return false
	}
	return true
}

func insertSubscript(body, sub *TextObject) {
	runs := body.Runs
	if len(runs) == 0 {
		runs = []TextRun{{
			Content: body.Content, FontName: body.FontName, FontSize: body.FontSize,
			Bold: body.Bold, Italic: body.Italic,
			ColorR: body.ColorR, ColorG: body.ColorG, ColorB: body.ColorB,
		}}
	}
	total := 0
	for _, r := range runs {
		total += utf8.RuneCountInString(r.Content)
	}
	if total < 1 || body.Width < 0.5 {
		return
	}
	rel := (sub.X - body.X) / body.Width
	if rel < 0 {
		rel = 0
	}
	if rel > 1 {
		rel = 1
	}
	at := int(math.Round(rel * float64(total)))
	if at < 1 {
		at = 1
	}
	if at > total {
		at = total
	}
	drop := baseline(sub) - baseline(body)
	pct := -drop / body.FontSize * 100000
	if pct > -8000 {
		pct = -8000
	}
	if pct < -60000 {
		pct = -60000
	}
	subRun := TextRun{
		Content: sub.Content, FontName: sub.FontName, FontSize: sub.FontSize,
		Bold: sub.Bold, Italic: sub.Italic,
		ColorR: sub.ColorR, ColorG: sub.ColorG, ColorB: sub.ColorB,
		Baseline: pct,
	}
	var next []TextRun
	seen := 0
	inserted := false
	for _, r := range runs {
		n := utf8.RuneCountInString(r.Content)
		if inserted || seen+n < at {
			next = append(next, r)
			seen += n
			continue
		}
		cut := at - seen
		rs := []rune(r.Content)
		if cut > 0 {
			left := r
			left.Content = string(rs[:cut])
			next = append(next, left)
		}
		next = append(next, subRun)
		if cut < len(rs) {
			right := r
			right.Content = string(rs[cut:])
			next = append(next, right)
		}
		inserted = true
		seen += n
	}
	if !inserted {
		next = append(next, subRun)
	}
	body.Runs = next
	var text string
	for _, r := range next {
		text += r.Content
	}
	body.Content = text
}

// pullBackOverlaps shortens a run whose box runs through the next word on
// the same line. "per Andy" was wider than the gap before "Morodo", so
// PowerPoint drew "pMAndy".
func pullBackOverlaps(p *Page) {
	var refs []textRef
	for i := range p.Elements {
		t := p.Elements[i].Text
		if t == nil || !horizontal(t.Rotation) {
			continue
		}
		refs = append(refs, textRef{i, t})
	}
	sortRefs(refs)
	for i := 0; i+1 < len(refs); i++ {
		a, b := refs[i].t, refs[i+1].t
		if a.Content == "" || b.Content == "" || a.Content == b.Content {
			continue
		}
		em := math.Max(a.FontSize, b.FontSize)
		if math.Abs(baseline(a)-baseline(b)) > math.Max(1.5, em*0.35) {
			continue
		}
		if b.X <= a.X || b.X >= a.X+a.Width-0.8 {
			continue
		}
		old := a.Width
		nw := b.X - a.X - 0.4
		if nw < em*0.4 {
			continue
		}
		gaps := utf8.RuneCountInString(a.Content) - 1
		if gaps > 0 && len(a.Runs) == 0 {
			a.Tracking += (nw - old) / float64(gaps)
		}
		a.Width = nw
		a.Clip = true
	}
}
