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
	// An uncovered parenthesis keeps the x it was drawn at. Merging it
	// back into the run on either side paints it on top of z₁.
	if prev.holdX || t.holdX {
		return false
	}
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
		var best *ref
		bestEdge := math.MaxFloat64
		for i := range refs {
			b := &refs[i]
			if b.i == sub.i || subscriptCandidate(b.t) || !subscriptHost(b.t, sub.t) {
				continue
			}
			// The digit hangs on the letter it sits under. That letter's box
			// ends just to the right of the digit; a later parenthesis that
			// merely overlaps the same x is the wrong host.
			edge := b.t.X + b.t.Width - sub.t.X
			if edge < 0 || edge >= bestEdge {
				continue
			}
			bestEdge = edge
			best = b
		}
		if best != nil {
			pairs = append(pairs, pair{best.t, sub.t, sub.i})
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
	// A digit, or a short equation letter (B_s, x'). Canva paints a smaller
	// title letter (30 pt and up) just under a display cap; attaching that
	// letter pulled "What" and "Social" out of order.
	s := strings.TrimSpace(t.Content)
	if s == "" || utf8.RuneCountInString(s) > 2 {
		return false
	}
	for _, r := range s {
		if r >= '0' && r <= '9' {
			continue
		}
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
			if t.FontSize > 22 {
				return false
			}
			continue
		}
		return false
	}
	return true
}

// uncoverOverlaidGlyphs splits a run when another glyph is drawn in the
// middle of it. PowerPoint lays each box out from its own x, so the KUST
// "(" at 484 pt was painted on top of z₁, which still lived in "g z".
func uncoverOverlaidGlyphs(p *Page) {
	for iter := 0; iter < 8; iter++ {
		var texts []*TextObject
		for i := range p.Elements {
			t := p.Elements[i].Text
			if t != nil && horizontal(t.Rotation) && t.Content != "" {
				texts = append(texts, t)
			}
		}
		split := false
		for _, a := range texts {
			if utf8.RuneCountInString(a.Content) < 2 {
				continue
			}
			var intruder *TextObject
			for _, b := range texts {
				if b == a || subscriptCandidate(b) || !fenceGlyph(b.Content) {
					continue
				}
				if !sameLine(a, b) {
					continue
				}
				// The fence's origin sits in the body of this run, not on a
				// shared edge. A letter here used to cut the word in half.
				if b.X <= a.X+0.8 || b.X >= a.X+a.Width-1.5 {
					continue
				}
				if intruder == nil || b.X < intruder.X {
					intruder = b
				}
			}
			if intruder != nil && splitRunAround(p, a, intruder) {
				split = true
				break
			}
		}
		if !split {
			return
		}
	}
}

func splitRunAround(p *Page, a, b *TextObject) bool {
	rs := []rune(a.Content)
	shares := runeShares(rs, a.Width)
	var cum float64
	cut := -1
	dropSpace := false
	for i, r := range rs {
		start := a.X + cum
		end := start + shares[i]
		// Only a gap. Cutting a letter split "disease" and "Metrics".
		if unicode.IsSpace(r) && b.X >= start-0.3 && b.X < end-0.3 {
			cut = i
			dropSpace = true
			break
		}
		cum += shares[i]
	}
	if cut <= 0 {
		return false
	}
	rightRunes := rs[cut:]
	if dropSpace {
		rightRunes = rs[cut+1:]
	}
	leftRunes := rs[:cut]
	rightStr := strings.TrimLeft(string(rightRunes), " ")
	if len(leftRunes) == 0 || rightStr == "" {
		return false
	}
	leftW := 0.0
	for i := 0; i < cut; i++ {
		leftW += shares[i]
	}
	origRight := a.X + a.Width
	rightX := b.X + b.Width
	natural := a.X + leftW
	if dropSpace {
		natural += shares[cut]
	}
	if natural > rightX {
		rightX = natural
	}
	if rightX > origRight-0.4 {
		return false
	}
	leftRuns, rightRuns := splitRunSlice(a.Runs, cut, dropSpace)
	rightRuns = trimLeftRunSpaces(rightRuns)
	leftContent := string(leftRunes)
	if len(leftRuns) > 0 {
		leftContent = joinRunContent(leftRuns)
	}
	rightContent := rightStr
	if len(rightRuns) > 0 {
		rightContent = joinRunContent(rightRuns)
	}
	if leftContent == "" || rightContent == "" {
		return false
	}
	right := *a
	a.Content = leftContent
	a.Width = math.Min(leftW, b.X-a.X)
	if a.Width < 0.4 {
		a.Width = 0.4
	}
	a.Runs = leftRuns
	right.Content = rightContent
	right.X = rightX
	right.Width = origRight - rightX
	if right.Width < 0.4 {
		right.Width = 0.4
	}
	right.Runs = rightRuns
	b.holdX = true
	p.Elements = append(p.Elements, PageElement{Text: &right})
	return true
}

// closeSubscriptHoles drops a space that survived after a subscript.
// The subscript already occupies that gap, so the space was a second one.
func closeSubscriptHoles(p *Page) {
	for i := range p.Elements {
		t := p.Elements[i].Text
		if t == nil || len(t.Runs) < 2 {
			continue
		}
		next := make([]TextRun, 0, len(t.Runs))
		changed := false
		for i, r := range t.Runs {
			if r.Baseline < -1000 {
				trimmed := strings.TrimRight(r.Content, " ")
				if trimmed != r.Content && trimmed != "" {
					nextSpace := i+1 < len(t.Runs) && strings.HasPrefix(t.Runs[i+1].Content, " ")
					if !nextSpace {
						trimmed += " "
					}
					if trimmed != r.Content {
						changed = true
						r.Content = trimmed
					}
				}
			}
			if i > 0 && t.Runs[i-1].Baseline < -1000 {
				rs := []rune(r.Content)
				spaces := 0
				for spaces < len(rs) && unicode.IsSpace(rs[spaces]) {
					spaces++
				}
				rest := string(rs[spaces:])
				if rest == "" && i+1 < len(t.Runs) {
					rest = strings.TrimLeft(t.Runs[i+1].Content, " ")
				}
				word := rest != "" && unicode.IsLetter([]rune(rest)[0])
				if !word && spaces > 0 && strings.TrimSpace(r.Content) == "" {
					changed = true
					continue
				}
				if !word && spaces > 1 {
					changed = true
					r.Content = " " + string([]rune(r.Content)[spaces:])
				}
			}
			if r.Content == "" {
				changed = true
				continue
			}
			next = append(next, r)
		}
		if !changed {
			continue
		}
		t.Runs = next
		t.Content = joinRunContent(t.Runs)
	}
}

// tightenHeldFences pulls a parenthesis that was uncovered from a gap back
// to the ink of the preceding word. The PDF box is a little wider than the
// Times advance, which left "ρg (z" loose.
func tightenHeldFences(p *Page) {
	var texts []*TextObject
	for i := range p.Elements {
		t := p.Elements[i].Text
		if t != nil && horizontal(t.Rotation) && t.Content != "" {
			texts = append(texts, t)
		}
	}
	for _, fence := range texts {
		if !fence.holdX {
			continue
		}
		var pred *TextObject
		for _, t := range texts {
			if t == fence || t.X >= fence.X-0.2 || !sameLine(t, fence) {
				continue
			}
			if pred == nil || t.X > pred.X {
				pred = t
			}
		}
		if pred == nil {
			continue
		}
		ink := objectInk(pred)
		if ink < 0.5 {
			continue
		}
		inkEnd := pred.X + ink
		gap := fence.X - inkEnd
		if gap < 0.6 || gap > 8 {
			continue
		}
		oldRight := fence.X + fence.Width
		pred.Width = ink
		fence.X -= gap
		// The hole space was removed when the run was split, so "ρg" is now
		// the end of this box and glues to the next a:t. Put the boundary
		// back. It is past the ink, so the parenthesis still starts at g.
		if !strings.HasSuffix(pred.Content, " ") {
			pred.Content += " "
			if n := len(pred.Runs); n > 0 {
				pred.Runs[n-1].Content += " "
			}
		}
		for _, t := range texts {
			if t == fence || t == pred || !sameLine(t, fence) {
				continue
			}
			if math.Abs(t.X-oldRight) <= 1.2 {
				t.X -= gap
			}
		}
	}
}

// objectInk is the Times advance of the box, ignoring a trailing space that
// exists only so the next a:t does not glue to this one.
func objectInk(t *TextObject) float64 {
	runs := t.Runs
	if len(runs) == 0 {
		runs = []TextRun{{
			Content: t.Content, FontName: t.FontName, FontSize: t.FontSize,
			Bold: t.Bold, Italic: t.Italic,
		}}
	}
	var sum float64
	for i, r := range runs {
		text := r.Content
		if i == len(runs)-1 {
			text = strings.TrimRight(text, " ")
		}
		if text == "" || r.FontSize < 0.5 {
			continue
		}
		sum += timesWidth(r.FontSize, text)
	}
	return sum
}

func joinRunContent(runs []TextRun) string {
	var b strings.Builder
	for _, r := range runs {
		b.WriteString(r.Content)
	}
	return b.String()
}

func trimLeftRunSpaces(runs []TextRun) []TextRun {
	for len(runs) > 0 {
		rs := []rune(runs[0].Content)
		i := 0
		for i < len(rs) && unicode.IsSpace(rs[i]) {
			i++
		}
		if i == 0 {
			break
		}
		if i >= len(rs) {
			runs = runs[1:]
			continue
		}
		runs[0].Content = string(rs[i:])
		break
	}
	return runs
}

// splitRunSlice breaks runs at a rune index. dropSpace discards the rune
// at cut, which is the gap the overlaid parenthesis occupied.
func splitRunSlice(runs []TextRun, cut int, dropSpace bool) (left, right []TextRun) {
	if len(runs) == 0 {
		return nil, nil
	}
	skip := cut
	if dropSpace {
		skip = cut + 1
	}
	seen := 0
	for _, r := range runs {
		rs := []rune(r.Content)
		n := len(rs)
		end := seen + n
		switch {
		case end <= cut:
			left = append(left, r)
		case seen >= skip:
			right = append(right, r)
		default:
			if cut > seen {
				lr := r
				lr.Content = string(rs[:cut-seen])
				left = append(left, lr)
			}
			from := skip - seen
			if from < n {
				rr := r
				rr.Content = string(rs[from:])
				right = append(right, rr)
			}
		}
		seen = end
	}
	return left, right
}

func runeShares(rs []rune, total float64) []float64 {
	w := make([]float64, len(rs))
	var sum float64
	for i, r := range rs {
		if unicode.IsSpace(r) {
			w[i] = 0.33
		} else {
			w[i] = 0.55
		}
		sum += w[i]
	}
	if sum == 0 || total <= 0 {
		each := 1.0
		if len(rs) > 0 {
			each = total / float64(len(rs))
		}
		for i := range w {
			w[i] = each
		}
		return w
	}
	for i := range w {
		w[i] = w[i] / sum * total
	}
	return w
}

func fenceGlyph(s string) bool {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) != 1 {
		return false
	}
	switch []rune(s)[0] {
	case '(', ')', '[', ']', '{', '}':
		return true
	default:
		return false
	}
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
	// Put the subscript after the glyph whose right edge is nearest it.
	at := 1
	bestDist := math.MaxFloat64
	for k := 1; k <= total; k++ {
		edge := body.X + float64(k)/float64(total)*body.Width
		d := math.Abs(edge - sub.X)
		if d < bestDist {
			bestDist = d
			at = k
		}
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
	flat := make([]rune, 0, total)
	for _, r := range runs {
		flat = append(flat, []rune(r.Content)...)
	}
	// A space just before the subscript is the hole it was painted in when
	// the next glyph is punctuation ("n .d" → nₕ.d). A space before a word
	// ("of 7") stays.
	hole := -1
	if at > 0 && at <= len(flat) && unicode.IsSpace(flat[at-1]) {
		nextLetter := at < len(flat) && unicode.IsLetter(flat[at])
		if !nextLetter {
			hole = at - 1
		}
	}
	var next []TextRun
	var pending strings.Builder
	var pendingRun TextRun
	havePending := false
	flush := func() {
		if !havePending || pending.Len() == 0 {
			pending.Reset()
			havePending = false
			return
		}
		pendingRun.Content = pending.String()
		next = append(next, pendingRun)
		pending.Reset()
		havePending = false
	}
	seen := 0
	inserted := false
	for _, r := range runs {
		for _, ch := range r.Content {
			if seen == hole {
				seen++
				continue
			}
			if !inserted && seen == at {
				flush()
				next = append(next, subRun)
				inserted = true
			}
			if !havePending {
				pendingRun = r
				havePending = true
			}
			pending.WriteRune(ch)
			seen++
		}
		flush()
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
		overlap := a.X + a.Width - b.X
		// A formula's last glyph may cross the next parenthesis by a few
		// points. Only a real collision (Andy through Morodo) is pulled back.
		if overlap < math.Max(em*1.2, old*0.18) {
			continue
		}
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
