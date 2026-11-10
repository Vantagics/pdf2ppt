package gopdf2

import (
	"fmt"
	"math"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

// type3Font is a PDF Type 3 font: each glyph is a content stream of paths.
// LibreOffice writes slide text this way. Substituting Noto Sans for those
// outlines is why a text-heavy slide misses the source.
type type3Font struct {
	mat   matrix
	proc  map[string][]byte
	glyph map[byte]string
}

func readType3(xrd *model.XRefTable, d types.Dict) *type3Font {
	f := &type3Font{
		mat:   matrix{0.001, 0, 0, 0.001, 0, 0},
		proc:  map[string][]byte{},
		glyph: map[byte]string{},
	}
	if m := d.ArrayEntry("FontMatrix"); len(m) == 6 {
		ok := true
		var mm [6]float64
		for i := 0; i < 6; i++ {
			n, okN := objNum(xrd, m[i])
			if !okN {
				ok = false
				break
			}
			mm[i] = n
		}
		if ok {
			f.mat = matrix{mm[0], mm[1], mm[2], mm[3], mm[4], mm[5]}
		}
	}
	if cp, err := xrd.DereferenceDict(d["CharProcs"]); err == nil && cp != nil {
		for name, obj := range cp {
			sd, _, err := xrd.DereferenceStreamDict(obj)
			if err != nil || sd == nil {
				continue
			}
			if err := sd.Decode(); err != nil || len(sd.Content) == 0 {
				continue
			}
			f.proc[name] = append([]byte(nil), sd.Content...)
		}
	}
	if len(f.proc) == 0 {
		return nil
	}
	encObj, ok := d.Find("Encoding")
	if !ok {
		return f
	}
	ed, err := xrd.DereferenceDict(encObj)
	if err != nil || ed == nil {
		return f
	}
	diffs := ed.ArrayEntry("Differences")
	code := 0
	for _, el := range diffs {
		el, _ = xrd.Dereference(el)
		switch v := el.(type) {
		case types.Integer:
			code = int(v)
		case types.Name:
			f.glyph[byte(code)] = v.Value()
			code++
		}
	}
	return f
}

// paintType3 appends one filled shape for the glyphs in raw and reports
// whether every glyph had an outline. The caller still emits the text so
// the characters stay in the file, with the fill turned off.
func (ip *interp) paintType3(raw []byte) bool {
	f := ip.gs.font
	if f == nil || f.t3 == nil || len(raw) == 0 || ip.gs.render == 3 {
		return false
	}
	tm := ip.tm
	var pts []pathOp
	for _, b := range raw {
		name := f.t3.glyph[b]
		if name == "" {
			name = string(rune(b))
		}
		proc := f.t3.proc[name]
		if len(proc) == 0 {
			return false
		}
		gpath := execType3Proc(proc)
		fs := ip.gs.fontSize
		if fs == 0 {
			fs = 1
		}
		scale := matrix{fs, 0, 0, fs, 0, 0}
		// concat(A, B) applies A first. Glyph space, then the font size,
		// then the text matrix, then the page CTM.
		m := concat(f.t3.mat, concat(scale, concat(tm, ip.gs.ctm)))
		for _, op := range gpath {
			op.x, op.y = m.apply(op.x, op.y)
			op.x1, op.y1 = m.apply(op.x1, op.y1)
			op.x2, op.y2 = m.apply(op.x2, op.y2)
			pts = append(pts, op)
		}
		adv := glyphAdvance(f.widthOf(int(b)), ip.gs.fontSize, ip.gs.charSp, ip.gs.wordSp, ip.gs.hScale, 0)
		tm[4] += adv * tm[0]
		tm[5] += adv * tm[1]
	}
	if len(pts) == 0 {
		return false
	}
	minX, minY, maxX, maxY, ok := pathBounds(pts)
	if !ok || maxX-minX < 0.2 || maxY-minY < 0.2 {
		return false
	}
	sh := &ShapeObject{
		X: minX - ip.originX, Y: (ip.originY + ip.pageH) - maxY,
		Width: maxX - minX, Height: maxY - minY,
		HasFill: true, FillAlpha: ip.gs.fillAlpha,
		Path:  quantizePath(opsToLocal(pts, minX, maxY)),
		Glyph: true,
	}
	sh.FillR, sh.FillG, sh.FillB = ip.gs.fill.bytes()
	if sh.FillAlpha <= 0 {
		sh.FillAlpha = 1
	}
	ip.elements = append(ip.elements, PageElement{Shape: sh})
	return true
}

func execType3Proc(data []byte) []pathOp {
	toks := tokenize(data)
	ctm := ident()
	var saved []matrix
	var ops []pathOp
	var st []token
	var cx, cy float64
	have := false
	flush := func() {
		st = nil
	}
	popN := func(n int) []float64 {
		if len(st) < n {
			return nil
		}
		out := make([]float64, n)
		base := len(st) - n
		for i := 0; i < n; i++ {
			if st[base+i].kind != tkNum {
				return nil
			}
			out[i] = st[base+i].num
		}
		st = st[:base]
		return out
	}
	for _, t := range toks {
		if t.kind != tkOp {
			st = append(st, t)
			continue
		}
		switch t.text {
		case "q":
			saved = append(saved, ctm)
			flush()
		case "Q":
			if n := len(saved); n > 0 {
				ctm = saved[n-1]
				saved = saved[:n-1]
			}
			flush()
		case "cm":
			nums := popN(6)
			if nums != nil {
				ctm = concat(matrix{nums[0], nums[1], nums[2], nums[3], nums[4], nums[5]}, ctm)
			}
		case "m":
			nums := popN(2)
			if nums != nil {
				x, y := ctm.apply(nums[0], nums[1])
				ops = append(ops, pathOp{op: 'M', x: x, y: y})
				cx, cy, have = x, y, true
			}
		case "l":
			nums := popN(2)
			if nums != nil {
				x, y := ctm.apply(nums[0], nums[1])
				ops = append(ops, pathOp{op: 'L', x: x, y: y})
				cx, cy, have = x, y, true
			}
		case "c":
			nums := popN(6)
			if nums != nil {
				x1, y1 := ctm.apply(nums[0], nums[1])
				x2, y2 := ctm.apply(nums[2], nums[3])
				x, y := ctm.apply(nums[4], nums[5])
				ops = append(ops, pathOp{op: 'C', x: x, y: y, x1: x1, y1: y1, x2: x2, y2: y2})
				cx, cy, have = x, y, true
			}
		case "h":
			ops = append(ops, pathOp{op: 'Z'})
		case "re":
			nums := popN(4)
			if nums != nil {
				x, y, w, h := nums[0], nums[1], nums[2], nums[3]
				p := [][2]float64{{x, y}, {x + w, y}, {x + w, y + h}, {x, y + h}}
				for i, pt := range p {
					px, py := ctm.apply(pt[0], pt[1])
					op := byte('L')
					if i == 0 {
						op = 'M'
					}
					ops = append(ops, pathOp{op: op, x: px, y: py})
				}
				ops = append(ops, pathOp{op: 'Z'})
			}
		case "d0", "d1":
			flush()
		default:
			flush()
		}
		_ = cx
		_ = cy
		_ = have
	}
	return ops
}

// quantizePath snaps coordinates to 0.15 pt (1905 EMU) and turns a cubic
// whose controls sit on the chord into a line. A Type 3 letter otherwise
// writes dozens of full-precision beziers.
func quantizePath(ops []PathOp) []PathOp {
	const grid = 0.15
	snap := func(v float64) float64 {
		return math.Round(v/grid) * grid
	}
	var out []PathOp
	var cx, cy float64
	have := false
	for _, op := range ops {
		op.X, op.Y = snap(op.X), snap(op.Y)
		op.X1, op.Y1 = snap(op.X1), snap(op.Y1)
		op.X2, op.Y2 = snap(op.X2), snap(op.Y2)
		switch op.Op {
		case "C":
			if have && flatCubic(cx, cy, op.X1, op.Y1, op.X2, op.Y2, op.X, op.Y) {
				op = PathOp{Op: "L", X: op.X, Y: op.Y}
			}
		}
		if have && (op.Op == "L" || op.Op == "M") && op.X == cx && op.Y == cy {
			continue
		}
		out = append(out, op)
		if op.Op == "M" || op.Op == "L" || op.Op == "C" {
			cx, cy = op.X, op.Y
			have = true
		}
	}
	return out
}

func flatCubic(x0, y0, x1, y1, x2, y2, x3, y3 float64) bool {
	// Controls within 0.2 pt of the straight chord.
	d := func(px, py float64) float64 {
		dx, dy := x3-x0, y3-y0
		l := math.Hypot(dx, dy)
		if l < 0.2 {
			return math.Hypot(px-x0, py-y0)
		}
		return math.Abs(dy*(px-x0)-dx*(py-y0)) / l
	}
	return d(x1, y1) < 0.2 && d(x2, y2) < 0.2
}

// coalesceGlyphShapes joins Type 3 glyphs that sit on one line into a single
// shape, so a word is one custGeom instead of a shape per letter.
func coalesceGlyphShapes(p *Page) {
	type item struct {
		i  int
		sh *ShapeObject
	}
	var items []item
	for i := range p.Elements {
		sh := p.Elements[i].Shape
		if sh != nil && sh.Glyph && len(sh.Path) > 0 {
			items = append(items, item{i, sh})
		}
	}
	if len(items) < 2 {
		return
	}
	sortItems := func() {
		for i := 1; i < len(items); i++ {
			for j := i; j > 0; j-- {
				a, b := items[j].sh, items[j-1].sh
				if a.Y < b.Y-1 || (math.Abs(a.Y-b.Y) <= 1 && a.X < b.X) {
					items[j], items[j-1] = items[j-1], items[j]
					continue
				}
				break
			}
		}
	}
	sortItems()
	remove := map[int]bool{}
	used := map[int]bool{}
	for i := 0; i < len(items); i++ {
		if used[items[i].i] {
			continue
		}
		base := items[i].sh
		group := []item{items[i]}
		used[items[i].i] = true
		right := base.X + base.Width
		for k := i + 1; k < len(items); k++ {
			if used[items[k].i] {
				continue
			}
			n := items[k].sh
			if n.FillR != base.FillR || n.FillG != base.FillG || n.FillB != base.FillB {
				continue
			}
			if math.Abs((n.Y+n.Height/2)-(base.Y+base.Height/2)) > math.Max(base.Height, n.Height)*0.45 {
				continue
			}
			if n.X > right+base.Height*0.8 {
				continue
			}
			group = append(group, items[k])
			used[items[k].i] = true
			if n.X+n.Width > right {
				right = n.X + n.Width
			}
		}
		if len(group) < 2 {
			continue
		}
		minX, minY := group[0].sh.X, group[0].sh.Y
		maxX, maxY := minX+group[0].sh.Width, minY+group[0].sh.Height
		var path []PathOp
		seen := map[string]bool{}
		for _, g := range group {
			sh := g.sh
			if sh.X < minX {
				minX = sh.X
			}
			if sh.Y < minY {
				minY = sh.Y
			}
			if sh.X+sh.Width > maxX {
				maxX = sh.X + sh.Width
			}
			if sh.Y+sh.Height > maxY {
				maxY = sh.Y + sh.Height
			}
		}
		for _, g := range group {
			sh := g.sh
			var rel strings.Builder
			for _, op := range sh.Path {
				fmt.Fprintf(&rel, "%s %.2f %.2f %.2f %.2f %.2f %.2f;", op.Op, op.X, op.Y, op.X1, op.Y1, op.X2, op.Y2)
			}
			key := fmt.Sprintf("%.2f,%.2f|%s", sh.X, sh.Y, rel.String())
			if seen[key] {
				continue
			}
			seen[key] = true
			for _, op := range sh.Path {
				op.X += sh.X - minX
				op.Y += sh.Y - minY
				op.X1 += sh.X - minX
				op.Y1 += sh.Y - minY
				op.X2 += sh.X - minX
				op.Y2 += sh.Y - minY
				path = append(path, op)
			}
			if g.i != items[i].i {
				remove[g.i] = true
			}
		}
		base.X, base.Y = minX, minY
		base.Width, base.Height = maxX-minX, maxY-minY
		base.Path = quantizePath(path)
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
