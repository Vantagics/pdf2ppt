package gopdf2

import (
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
		Path: opsToLocal(pts, minX, maxY),
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
