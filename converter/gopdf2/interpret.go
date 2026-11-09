package gopdf2

import (
	"math"
	"strconv"
	"unicode/utf16"

	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

type matrix [6]float64

func ident() matrix { return matrix{1, 0, 0, 1, 0, 0} }

// concat returns M × m (PDF cm operator: new = operand × current).
func concat(M, m matrix) matrix {
	a, b, c, d, e, f := M[0], M[1], M[2], M[3], M[4], M[5]
	return matrix{
		a*m[0] + b*m[2],
		a*m[1] + b*m[3],
		c*m[0] + d*m[2],
		c*m[1] + d*m[3],
		e*m[0] + f*m[2] + m[4],
		e*m[1] + f*m[3] + m[5],
	}
}

func (m matrix) apply(x, y float64) (float64, float64) {
	return m[0]*x + m[2]*y + m[4], m[1]*x + m[3]*y + m[5]
}

func (m matrix) applyVec(x, y float64) (float64, float64) {
	return m[0]*x + m[2]*y, m[1]*x + m[3]*y
}

type color3 struct{ r, g, b float64 }

func (c color3) bytes() (uint8, uint8, uint8) {
	return toByte(c.r), toByte(c.g), toByte(c.b)
}

func toByte(v float64) uint8 {
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	return uint8(math.Round(v * 255))
}

type gstate struct {
	ctm                    matrix
	fill, stroke           color3
	fillN, strokeN         int
	lineWidth              float64
	fillAlpha, strokeAlpha float64
	fontName               string
	fontSize               float64
	charSp, wordSp, hScale float64
	leading, rise          float64
	render                 int
	font                   *pdfFont
}

func defaultGS() gstate {
	return gstate{
		ctm:         ident(),
		fill:        color3{},
		stroke:      color3{},
		fillN:       1,
		strokeN:     1,
		lineWidth:   1,
		fillAlpha:   1,
		strokeAlpha: 1,
		hScale:      100,
	}
}

type resources struct {
	xrd    *model.XRefTable
	dict   types.Dict
	parent *resources
	fonts  map[string]*pdfFont
	xobjs  map[string]*xobject
	ext    map[string]extGS
}

type extGS struct {
	fillAlpha   *float64
	strokeAlpha *float64
	lineWidth   *float64
}

type xobject struct {
	kind    string
	stream  *types.StreamDict
	content []byte
	res     *resources
	matrix  matrix
}

func newResources(xrd *model.XRefTable, d types.Dict, parent *resources) *resources {
	if d == nil {
		d = types.NewDict()
	}
	return &resources{
		xrd:    xrd,
		dict:   d,
		parent: parent,
		fonts:  map[string]*pdfFont{},
		xobjs:  map[string]*xobject{},
		ext:    map[string]extGS{},
	}
}

func (r *resources) sub(key string) types.Dict {
	if r == nil || r.dict == nil {
		return nil
	}
	o, ok := r.dict.Find(key)
	if !ok || o == nil {
		return nil
	}
	d, err := r.xrd.DereferenceDict(o)
	if err != nil {
		return nil
	}
	return d
}

func (r *resources) font(name string) *pdfFont {
	if r == nil || name == "" {
		return nil
	}
	if f, ok := r.fonts[name]; ok {
		return f
	}
	if fd := r.sub("Font"); fd != nil {
		if o, ok := fd.Find(name); ok {
			f := parseFont(r.xrd, o)
			r.fonts[name] = f
			return f
		}
	}
	if r.parent != nil {
		return r.parent.font(name)
	}
	r.fonts[name] = nil
	return nil
}

func (r *resources) xobject(name string) *xobject {
	if r == nil || name == "" {
		return nil
	}
	if xo, ok := r.xobjs[name]; ok {
		return xo
	}
	xd := r.sub("XObject")
	if xd == nil {
		if r.parent != nil {
			return r.parent.xobject(name)
		}
		r.xobjs[name] = nil
		return nil
	}
	o, ok := xd.Find(name)
	if !ok {
		if r.parent != nil {
			return r.parent.xobject(name)
		}
		r.xobjs[name] = nil
		return nil
	}
	sd, _, err := r.xrd.DereferenceStreamDict(o)
	if err != nil || sd == nil {
		r.xobjs[name] = nil
		return nil
	}
	xo := &xobject{stream: sd, matrix: ident()}
	sub := ""
	if s := sd.NameEntry("Subtype"); s != nil {
		sub = *s
	}
	switch sub {
	case "Image":
		xo.kind = "image"
	case "Form":
		xo.kind = "form"
		if err := sd.Decode(); err == nil {
			xo.content = append([]byte(nil), sd.Content...)
		}
		if m := sd.ArrayEntry("Matrix"); len(m) == 6 {
			nums := make([]float64, 6)
			okm := true
			for i := 0; i < 6; i++ {
				n, ok := objNum(r.xrd, m[i])
				if !ok {
					okm = false
					break
				}
				nums[i] = n
			}
			if okm {
				xo.matrix = matrix{nums[0], nums[1], nums[2], nums[3], nums[4], nums[5]}
			}
		}
		if o, ok := sd.Find("Resources"); ok {
			if rd, err := r.xrd.DereferenceDict(o); err == nil {
				xo.res = newResources(r.xrd, rd, r)
			}
		}
	default:
		xo.kind = ""
	}
	r.xobjs[name] = xo
	return xo
}

func (r *resources) extGS(name string) (extGS, bool) {
	if r == nil {
		return extGS{}, false
	}
	if g, ok := r.ext[name]; ok {
		return g, true
	}
	ed := r.sub("ExtGState")
	if ed == nil {
		if r.parent != nil {
			return r.parent.extGS(name)
		}
		return extGS{}, false
	}
	o, ok := ed.Find(name)
	if !ok {
		if r.parent != nil {
			return r.parent.extGS(name)
		}
		return extGS{}, false
	}
	d, err := r.xrd.DereferenceDict(o)
	if err != nil || d == nil {
		return extGS{}, false
	}
	var g extGS
	if n, ok := objNum(r.xrd, mustObj(d, "ca")); ok {
		g.fillAlpha = &n
	}
	if n, ok := objNum(r.xrd, mustObj(d, "CA")); ok {
		g.strokeAlpha = &n
	}
	if n, ok := objNum(r.xrd, mustObj(d, "LW")); ok {
		g.lineWidth = &n
	}
	r.ext[name] = g
	return g, true
}

func mustObj(d types.Dict, key string) types.Object {
	o, _ := d.Find(key)
	return o
}

type pathOp struct {
	op     byte
	x, y   float64
	x1, y1 float64
	x2, y2 float64
}

type interp struct {
	res              *resources
	gs               gstate
	stack            []gstate
	tm, tlm          matrix
	inText           bool
	ops              []pathOp
	curX, curY       float64
	startX, startY   float64
	haveCur          bool
	pageW, pageH     float64
	originX, originY float64
	seenContent      bool
	hasBg            bool
	bg               color3
	elements         []PageElement
	formDepth        int
}

func interpretPage(content []byte, res *resources, originX, originY, width, height float64) *Page {
	ip := &interp{
		res:     res,
		gs:      defaultGS(),
		tm:      ident(),
		tlm:     ident(),
		pageW:   width,
		pageH:   height,
		originX: originX,
		originY: originY,
	}
	if len(content) > 0 {
		ip.execContent(content)
	}
	page := &Page{
		Width:         width,
		Height:        height,
		HasBackground: ip.hasBg,
		Elements:      ip.elements,
	}
	if ip.hasBg {
		page.BgR, page.BgG, page.BgB = ip.bg.bytes()
	}
	for _, el := range ip.elements {
		if el.Text != nil {
			page.TextObjects = append(page.TextObjects, el.Text)
		}
		if el.Image != nil {
			page.Images = append(page.Images, el.Image)
		}
	}
	return page
}

func (ip *interp) execContent(content []byte) {
	toks := tokenize(content)
	st := make([]token, 0, 16)
	for _, t := range toks {
		if t.kind != tkOp {
			st = append(st, t)
			continue
		}
		ip.exec(t.text, &st)
	}
}

func (ip *interp) exec(op string, st *[]token) {
	switch op {
	case "q":
		ip.stack = append(ip.stack, ip.gs)
	case "Q":
		if n := len(ip.stack); n > 0 {
			ip.gs = ip.stack[n-1]
			ip.stack = ip.stack[:n-1]
		}
	case "cm":
		a, b, c, d, e, f := popN(st, 6)
		ip.gs.ctm = concat(matrix{a, b, c, d, e, f}, ip.gs.ctm)
	case "w":
		ip.gs.lineWidth = pop1(st)
	case "J", "j", "M", "ri", "i":
		pop1(st)
	case "d":
		pop1(st)
		if len(*st) > 0 {
			*st = (*st)[:len(*st)-1]
		}
	case "gs":
		name := popName(st)
		if g, ok := ip.res.extGS(name); ok {
			if g.fillAlpha != nil {
				ip.gs.fillAlpha = *g.fillAlpha
			}
			if g.strokeAlpha != nil {
				ip.gs.strokeAlpha = *g.strokeAlpha
			}
			if g.lineWidth != nil {
				ip.gs.lineWidth = *g.lineWidth
			}
		}
	case "g":
		gray := pop1(st)
		ip.gs.fill = color3{gray, gray, gray}
		ip.gs.fillN = 1
	case "G":
		gray := pop1(st)
		ip.gs.stroke = color3{gray, gray, gray}
		ip.gs.strokeN = 1
	case "rg":
		b, g, r := popN3(st)
		ip.gs.fill = color3{r, g, b}
		ip.gs.fillN = 3
	case "RG":
		b, g, r := popN3(st)
		ip.gs.stroke = color3{r, g, b}
		ip.gs.strokeN = 3
	case "k":
		k, y, m, c := popN4(st)
		ip.gs.fill = cmyk(c, m, y, k)
		ip.gs.fillN = 4
	case "K":
		k, y, m, c := popN4(st)
		ip.gs.stroke = cmyk(c, m, y, k)
		ip.gs.strokeN = 4
	case "cs":
		name := popName(st)
		ip.gs.fillN = csComponents(name)
	case "CS":
		name := popName(st)
		ip.gs.strokeN = csComponents(name)
	case "sc", "scn":
		ip.popColor(st, ip.gs.fillN, true)
	case "SC", "SCN":
		ip.popColor(st, ip.gs.strokeN, false)
	case "m":
		y, x := popN2(st)
		ip.moveTo(x, y)
	case "l":
		y, x := popN2(st)
		ip.lineTo(x, y)
	case "c":
		x1, y1, x2, y2, x3, y3 := popN(st, 6)
		ip.curveTo(x1, y1, x2, y2, x3, y3)
	case "v":
		y3, x3, y2, x2 := popN4(st)
		ip.curveTo(ip.curX, ip.curY, x2, y2, x3, y3)
	case "y":
		y3, x3, y1, x1 := popN4(st)
		ip.curveTo(x1, y1, x3, y3, x3, y3)
	case "h":
		ip.closePath()
	case "re":
		h, w, y, x := popN4(st)
		ip.rectPath(x, y, w, h)
	case "W", "W*":
		// clipping is recorded only so the following paint/n consumes the path
	case "n":
		ip.ops = nil
		ip.haveCur = false
	case "S":
		ip.paint(false, true, false)
	case "s":
		ip.closePath()
		ip.paint(false, true, false)
	case "f", "F":
		ip.paint(true, false, false)
	case "f*":
		ip.paint(true, false, true)
	case "B":
		ip.paint(true, true, false)
	case "B*":
		ip.paint(true, true, true)
	case "b":
		ip.closePath()
		ip.paint(true, true, false)
	case "b*":
		ip.closePath()
		ip.paint(true, true, true)
	case "BT":
		ip.inText = true
		ip.tm = ident()
		ip.tlm = ident()
	case "ET":
		ip.inText = false
	case "Tc":
		ip.gs.charSp = pop1(st)
	case "Tw":
		ip.gs.wordSp = pop1(st)
	case "Tz":
		ip.gs.hScale = pop1(st)
	case "TL":
		ip.gs.leading = pop1(st)
	case "Tr":
		ip.gs.render = int(pop1(st))
	case "Ts":
		ip.gs.rise = pop1(st)
	case "Tf":
		size := pop1(st)
		name := popName(st)
		ip.gs.fontName = name
		ip.gs.fontSize = size
		ip.gs.font = ip.res.font(name)
	case "Td":
		y, x := popN2(st)
		ip.td(x, y)
	case "TD":
		y, x := popN2(st)
		ip.gs.leading = -y
		ip.td(x, y)
	case "Tm":
		a, b, c, d, e, f := popN(st, 6)
		ip.tm = matrix{a, b, c, d, e, f}
		ip.tlm = ip.tm
	case "T*":
		ip.td(0, -ip.gs.leading)
	case "Tj":
		ip.showString(popStr(st), 0)
	case "TJ":
		ip.showTJ(popArray(st))
	case "'":
		raw := popStr(st)
		ip.td(0, -ip.gs.leading)
		ip.showString(raw, 0)
	case "\"":
		raw := popStr(st)
		ac := pop1(st)
		aw := pop1(st)
		ip.gs.wordSp = aw
		ip.gs.charSp = ac
		ip.td(0, -ip.gs.leading)
		ip.showString(raw, 0)
	case "Do":
		ip.doXObject(popName(st))
	case "BMC", "MP":
		popName(st)
	case "BDC", "DP":
		if len(*st) > 0 {
			*st = (*st)[:len(*st)-1]
		}
		popName(st)
	case "EMC":
	default:
		// drop a trailing operand if this was a no-op we don't model
	}
}

func csComponents(name string) int {
	switch name {
	case "DeviceGray", "G":
		return 1
	case "DeviceCMYK", "CMYK":
		return 4
	default:
		return 3
	}
}

func cmyk(c, m, y, k float64) color3 {
	return color3{
		r: 1 - math.Min(1, c+k),
		g: 1 - math.Min(1, m+k),
		b: 1 - math.Min(1, y+k),
	}
}

func (ip *interp) popColor(st *[]token, n int, fill bool) {
	if n < 1 {
		n = 1
	}
	// Named pattern/separation colors end with a name; ignore those.
	if len(*st) > 0 && (*st)[len(*st)-1].kind == tkName {
		*st = (*st)[:len(*st)-1]
	}
	vals := make([]float64, n)
	for i := n - 1; i >= 0; i-- {
		vals[i] = pop1(st)
	}
	var c color3
	switch n {
	case 1:
		c = color3{vals[0], vals[0], vals[0]}
	case 4:
		c = cmyk(vals[0], vals[1], vals[2], vals[3])
	default:
		r, g, b := 0.0, 0.0, 0.0
		if len(vals) > 0 {
			r = vals[0]
		}
		if len(vals) > 1 {
			g = vals[1]
		}
		if len(vals) > 2 {
			b = vals[2]
		}
		c = color3{r, g, b}
	}
	if fill {
		ip.gs.fill = c
	} else {
		ip.gs.stroke = c
	}
}

func (ip *interp) moveTo(x, y float64) {
	px, py := ip.gs.ctm.apply(x, y)
	ip.ops = append(ip.ops, pathOp{op: 'M', x: px, y: py})
	ip.curX, ip.curY = x, y
	ip.startX, ip.startY = x, y
	ip.haveCur = true
}

func (ip *interp) lineTo(x, y float64) {
	if !ip.haveCur {
		ip.moveTo(x, y)
		return
	}
	px, py := ip.gs.ctm.apply(x, y)
	ip.ops = append(ip.ops, pathOp{op: 'L', x: px, y: py})
	ip.curX, ip.curY = x, y
}

func (ip *interp) curveTo(x1, y1, x2, y2, x3, y3 float64) {
	if !ip.haveCur {
		ip.moveTo(x1, y1)
	}
	ax, ay := ip.gs.ctm.apply(x1, y1)
	bx, by := ip.gs.ctm.apply(x2, y2)
	cx, cy := ip.gs.ctm.apply(x3, y3)
	ip.ops = append(ip.ops, pathOp{op: 'C', x: cx, y: cy, x1: ax, y1: ay, x2: bx, y2: by})
	ip.curX, ip.curY = x3, y3
}

func (ip *interp) closePath() {
	if !ip.haveCur {
		return
	}
	ip.ops = append(ip.ops, pathOp{op: 'Z'})
	ip.curX, ip.curY = ip.startX, ip.startY
}

func (ip *interp) rectPath(x, y, w, h float64) {
	ip.moveTo(x, y)
	ip.lineTo(x+w, y)
	ip.lineTo(x+w, y+h)
	ip.lineTo(x, y+h)
	ip.closePath()
}

func (ip *interp) paint(fill, stroke, evenOdd bool) {
	ops := ip.ops
	ip.ops = nil
	ip.haveCur = false
	if len(ops) == 0 {
		return
	}
	minX, minY, maxX, maxY, ok := pathBounds(ops)
	if !ok {
		return
	}
	if fill && !ip.seenContent && ip.coversPage(minX, minY, maxX, maxY) && !hasCurve(ops) {
		ip.hasBg = true
		ip.bg = ip.gs.fill
		return
	}
	ip.seenContent = true
	sw := 0.0
	if stroke {
		sx := math.Hypot(ip.gs.ctm[0], ip.gs.ctm[1])
		sy := math.Hypot(ip.gs.ctm[2], ip.gs.ctm[3])
		scale := (sx + sy) / 2
		if scale == 0 {
			scale = 1
		}
		sw = ip.gs.lineWidth * scale
		if sw < 0.25 {
			sw = 0.25
		}
		pad := sw / 2
		minX -= pad
		minY -= pad
		maxX += pad
		maxY += pad
	}
	bw := maxX - minX
	bh := maxY - minY
	if bw < 0.4 {
		mid := (minX + maxX) / 2
		minX = mid - 0.2
		maxX = mid + 0.2
		bw = 0.4
	}
	if bh < 0.4 {
		mid := (minY + maxY) / 2
		minY = mid - 0.2
		maxY = mid + 0.2
		bh = 0.4
	}
	sh := &ShapeObject{
		X:           minX - ip.originX,
		Y:           (ip.originY + ip.pageH) - maxY,
		Width:       bw,
		Height:      bh,
		HasFill:     fill,
		HasStroke:   stroke,
		StrokeWidth: sw,
		EvenOdd:     evenOdd,
		FillAlpha:   ip.gs.fillAlpha,
	}
	if fill {
		sh.FillR, sh.FillG, sh.FillB = ip.gs.fill.bytes()
	}
	if stroke {
		sh.StrokeR, sh.StrokeG, sh.StrokeB = ip.gs.stroke.bytes()
	}
	if isSimpleRect(ops) && !hasCurve(ops) {
		sh.SimpleRect = true
	} else {
		sh.Path = opsToLocal(ops, minX, maxY)
	}
	if sh.FillAlpha <= 0 {
		sh.FillAlpha = 1
	}
	ip.elements = append(ip.elements, PageElement{Shape: sh})
}

func (ip *interp) coversPage(minX, minY, maxX, maxY float64) bool {
	bw, bh := maxX-minX, maxY-minY
	if bw < ip.pageW*0.94 || bh < ip.pageH*0.94 {
		return false
	}
	if minX > ip.originX+ip.pageW*0.06 {
		return false
	}
	if minY > ip.originY+ip.pageH*0.06 {
		return false
	}
	return true
}

func hasCurve(ops []pathOp) bool {
	for _, o := range ops {
		if o.op == 'C' {
			return true
		}
	}
	return false
}

func pathBounds(ops []pathOp) (minX, minY, maxX, maxY float64, ok bool) {
	minX, minY = math.MaxFloat64, math.MaxFloat64
	maxX, maxY = -math.MaxFloat64, -math.MaxFloat64
	consider := func(x, y float64) {
		ok = true
		if x < minX {
			minX = x
		}
		if y < minY {
			minY = y
		}
		if x > maxX {
			maxX = x
		}
		if y > maxY {
			maxY = y
		}
	}
	for _, o := range ops {
		switch o.op {
		case 'M', 'L':
			consider(o.x, o.y)
		case 'C':
			consider(o.x1, o.y1)
			consider(o.x2, o.y2)
			consider(o.x, o.y)
		}
	}
	return
}

func isSimpleRect(ops []pathOp) bool {
	var pts [][2]float64
	for _, o := range ops {
		switch o.op {
		case 'M', 'L':
			pts = append(pts, [2]float64{o.x, o.y})
		case 'C':
			return false
		}
	}
	if len(pts) < 4 || len(pts) > 6 {
		return false
	}
	minX, minY, maxX, maxY, ok := pathBounds(ops)
	if !ok || maxX-minX < 0.5 || maxY-minY < 0.5 {
		return false
	}
	const eps = 0.8
	for _, p := range pts {
		onX := math.Abs(p[0]-minX) < eps || math.Abs(p[0]-maxX) < eps
		onY := math.Abs(p[1]-minY) < eps || math.Abs(p[1]-maxY) < eps
		if !onX || !onY {
			return false
		}
	}
	return true
}

func opsToLocal(ops []pathOp, minX, maxY float64) []PathOp {
	rel := func(x, y float64) (float64, float64) {
		return x - minX, maxY - y
	}
	out := make([]PathOp, 0, len(ops))
	for _, o := range ops {
		switch o.op {
		case 'M':
			x, y := rel(o.x, o.y)
			out = append(out, PathOp{Op: "M", X: x, Y: y})
		case 'L':
			x, y := rel(o.x, o.y)
			out = append(out, PathOp{Op: "L", X: x, Y: y})
		case 'C':
			x, y := rel(o.x, o.y)
			x1, y1 := rel(o.x1, o.y1)
			x2, y2 := rel(o.x2, o.y2)
			out = append(out, PathOp{Op: "C", X: x, Y: y, X1: x1, Y1: y1, X2: x2, Y2: y2})
		case 'Z':
			out = append(out, PathOp{Op: "Z"})
		}
	}
	return out
}

func (ip *interp) td(tx, ty float64) {
	m := ip.tlm
	ip.tlm[4] = tx*m[0] + ty*m[2] + m[4]
	ip.tlm[5] = tx*m[1] + ty*m[3] + m[5]
	ip.tm = ip.tlm
}

func (ip *interp) advance(tx float64) {
	ip.tm[4] += tx * ip.tm[0]
	ip.tm[5] += tx * ip.tm[1]
}

func (ip *interp) showTJ(arr []token) {
	if !ip.inText {
		return
	}
	var text string
	var total float64
	fontSize := ip.gs.fontSize
	for _, el := range arr {
		switch el.kind {
		case tkStr:
			s, adv := ip.decodeShow(el.raw)
			text += s
			total += adv
		case tkNum:
			adj := tjAdjust(el.num, fontSize, ip.gs.hScale)
			// A large positive gap is a word space encoded as a positioning
			// adjustment rather than a space character.
			if adj > fontSize*0.25 {
				text += " "
			}
			total += adj
		}
	}
	ip.emitText(text, total)
}

func (ip *interp) showString(raw []byte, extra float64) {
	if !ip.inText {
		return
	}
	// UTF-16BE literal strings (with BOM) are already Unicode, not char codes.
	if len(raw) >= 2 && raw[0] == 0xFE && raw[1] == 0xFF {
		u := make([]uint16, 0, (len(raw)-2)/2)
		for i := 2; i+1 < len(raw); i += 2 {
			u = append(u, uint16(raw[i])<<8|uint16(raw[i+1]))
		}
		s := string(utf16.Decode(u))
		adv := float64(len([]rune(s))) * ip.gs.fontSize * 0.5
		ip.emitText(s, adv+extra)
		return
	}
	s, adv := ip.decodeShow(raw)
	ip.emitText(s, adv+extra)
}

func (ip *interp) decodeShow(raw []byte) (string, float64) {
	f := ip.gs.font
	if f == nil && ip.gs.fontName != "" {
		f = ip.res.font(ip.gs.fontName)
		ip.gs.font = f
	}
	if f == nil {
		// Treat bytes as Latin-1 / ASCII.
		var b []rune
		var adv float64
		for _, c := range raw {
			if c == 0 {
				continue
			}
			b = append(b, rune(c))
			adv += glyphAdvance(500, ip.gs.fontSize, ip.gs.charSp, ip.gs.wordSp, ip.gs.hScale, rune(c))
		}
		return string(b), adv
	}
	return f.decodeBytes(raw, ip.gs.fontSize, ip.gs.charSp, ip.gs.wordSp, ip.gs.hScale)
}

func (ip *interp) emitText(text string, textAdv float64) {
	if text == "" || ip.gs.render == 3 {
		ip.advance(textAdv)
		return
	}
	eff := ip.effectiveSize()
	if eff < 0.5 {
		eff = ip.gs.fontSize
	}
	if eff < 0.5 {
		ip.advance(textAdv)
		return
	}
	desc := 0.20
	bold, italic := false, false
	family := "Arial"
	if ip.gs.font != nil {
		if ip.gs.font.descent > 0 {
			desc = ip.gs.font.descent
		}
		bold = ip.gs.font.bold
		italic = ip.gs.font.italic
		if ip.gs.font.family != "" {
			family = ip.gs.font.family
		}
	}
	if ip.gs.render == 2 {
		bold = true
	}
	bx, by := ip.gs.ctm.apply(ip.tm[4], ip.tm[5])
	// Rise is in unscaled text space.
	if ip.gs.rise != 0 {
		rx, ry := ip.tm.applyVec(0, ip.gs.rise)
		px, py := ip.gs.ctm.applyVec(rx, ry)
		bx += px
		by += py
	}
	width := ip.pageLen(textAdv)
	if width < eff*0.4 {
		width = float64(len([]rune(text))) * eff * 0.55
	}
	// Pad so the last glyph is not clipped, without a filled box.
	width += eff * 0.35
	// LibreOffice (and PowerPoint) put the baseline one em below the top of
	// a zero-inset text box. Using the font ascender here drops the glyphs
	// by (1-ascent)*size, which is several pixels on a typical slide.
	const baselineFromTop = 1.0
	height := (baselineFromTop + desc) * eff
	if height < eff*1.15 {
		height = eff * 1.15
	}
	yTop := (ip.originY + ip.pageH) - (by + baselineFromTop*eff)
	tb := &TextObject{
		Content:  text,
		FontName: family,
		FontSize: eff,
		Bold:     bold,
		Italic:   italic,
		X:        bx - ip.originX,
		Y:        yTop,
		Width:    width,
		Height:   height,
	}
	tb.ColorR, tb.ColorG, tb.ColorB = ip.gs.fill.bytes()
	ip.seenContent = true
	ip.elements = append(ip.elements, PageElement{Text: tb})
	ip.advance(textAdv)
}

func (ip *interp) effectiveSize() float64 {
	vx, vy := ip.tm.applyVec(0, ip.gs.fontSize)
	px, py := ip.gs.ctm.applyVec(vx, vy)
	return math.Hypot(px, py)
}

func (ip *interp) pageLen(textAdv float64) float64 {
	vx, vy := ip.tm.applyVec(textAdv, 0)
	px, py := ip.gs.ctm.applyVec(vx, vy)
	return math.Hypot(px, py)
}

func (ip *interp) doXObject(name string) {
	xo := ip.res.xobject(name)
	if xo == nil || ip.formDepth > 8 {
		return
	}
	switch xo.kind {
	case "image":
		ip.paintImage(xo)
	case "form":
		savedGS := ip.gs
		savedRes := ip.res
		savedOps := ip.ops
		savedIn := ip.inText
		savedTM, savedTLM := ip.tm, ip.tlm
		ip.formDepth++
		ip.ops = nil
		ip.inText = false
		ip.gs.ctm = concat(xo.matrix, ip.gs.ctm)
		if xo.res != nil {
			ip.res = xo.res
		}
		if len(xo.content) > 0 {
			ip.execContent(xo.content)
		}
		ip.formDepth--
		ip.gs = savedGS
		ip.res = savedRes
		ip.ops = savedOps
		ip.inText = savedIn
		ip.tm, ip.tlm = savedTM, savedTLM
	}
}

func (ip *interp) paintImage(xo *xobject) {
	if xo.stream == nil {
		return
	}
	// Corners of the unit square under the current CTM.
	c00x, c00y := ip.gs.ctm.apply(0, 0)
	c10x, c10y := ip.gs.ctm.apply(1, 0)
	c01x, c01y := ip.gs.ctm.apply(0, 1)
	c11x, c11y := ip.gs.ctm.apply(1, 1)
	minX := math.Min(math.Min(c00x, c10x), math.Min(c01x, c11x))
	maxX := math.Max(math.Max(c00x, c10x), math.Max(c01x, c11x))
	minY := math.Min(math.Min(c00y, c10y), math.Min(c01y, c11y))
	maxY := math.Max(math.Max(c00y, c10y), math.Max(c01y, c11y))
	w := maxX - minX
	h := maxY - minY
	if w < 0.5 || h < 0.5 {
		return
	}
	data, format, err := encodePlacedImage(ip.res.xrd, xo.stream, ip.gs.fillAlpha)
	if err != nil || len(data) == 0 {
		return
	}
	img := &ImageObject{
		Data:   data,
		Format: format,
		X:      minX - ip.originX,
		Y:      (ip.originY + ip.pageH) - maxY,
		Width:  w,
		Height: h,
	}
	ip.seenContent = true
	ip.elements = append(ip.elements, PageElement{Image: img})
}

// ---- tokenizer ----

type tokKind int

const (
	tkNum tokKind = iota
	tkName
	tkStr
	tkArray
	tkDict
	tkOp
)

type token struct {
	kind tokKind
	num  float64
	text string
	raw  []byte
	arr  []token
}

func tokenize(data []byte) []token {
	var out []token
	i := 0
	for i < len(data) {
		i = skipWS(data, i)
		if i >= len(data) {
			break
		}
		switch data[i] {
		case '%':
			for i < len(data) && data[i] != '\n' && data[i] != '\r' {
				i++
			}
		case '(':
			raw, ni := parseLiteral(data, i)
			out = append(out, token{kind: tkStr, raw: raw})
			i = ni
		case '<':
			if i+1 < len(data) && data[i+1] == '<' {
				ni := skipDict(data, i)
				out = append(out, token{kind: tkDict})
				i = ni
			} else {
				raw, ni := parseHex(data, i)
				out = append(out, token{kind: tkStr, raw: raw})
				i = ni
			}
		case '[':
			arr, ni := parseArray(data, i)
			out = append(out, token{kind: tkArray, arr: arr})
			i = ni
		case '/':
			name, ni := parseName(data, i)
			out = append(out, token{kind: tkName, text: name})
			i = ni
		default:
			j := i
			for j < len(data) && !isDelim(data[j]) {
				j++
			}
			word := string(data[i:j])
			if word == "" {
				i++
				continue
			}
			if n, ok := parseNum(word); ok {
				out = append(out, token{kind: tkNum, num: n})
			} else {
				out = append(out, token{kind: tkOp, text: word})
			}
			i = j
		}
	}
	return out
}

func skipWS(data []byte, i int) int {
	for i < len(data) && isWS(data[i]) {
		i++
	}
	return i
}

func isWS(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == 0
}

func isDelim(c byte) bool {
	return isWS(c) || c == '(' || c == ')' || c == '<' || c == '>' || c == '[' || c == ']' || c == '{' || c == '}' || c == '/' || c == '%'
}

func parseNum(s string) (float64, bool) {
	if s == "" || s == "+" || s == "-" || s == "." || s == "+." || s == "-." {
		return 0, false
	}
	n, err := strconv.ParseFloat(s, 64)
	return n, err == nil
}

func parseName(data []byte, i int) (string, int) {
	i++ // slash
	start := i
	for i < len(data) && !isDelim(data[i]) {
		i++
	}
	raw := string(data[start:i])
	return decodePDFName(raw), i
}

func decodePDFName(s string) string {
	if !containsHash(s) {
		return s
	}
	var b []byte
	for i := 0; i < len(s); i++ {
		if s[i] == '#' && i+2 < len(s) {
			h := s[i+1 : i+3]
			if v, err := strconv.ParseUint(h, 16, 8); err == nil {
				b = append(b, byte(v))
				i += 2
				continue
			}
		}
		b = append(b, s[i])
	}
	return string(b)
}

func containsHash(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '#' {
			return true
		}
	}
	return false
}

func parseLiteral(data []byte, i int) ([]byte, int) {
	i++ // (
	var b []byte
	depth := 1
	for i < len(data) && depth > 0 {
		c := data[i]
		if c == '\\' {
			i++
			if i >= len(data) {
				break
			}
			switch data[i] {
			case 'n':
				b = append(b, '\n')
			case 'r':
				b = append(b, '\r')
			case 't':
				b = append(b, '\t')
			case 'b':
				b = append(b, '\b')
			case 'f':
				b = append(b, '\f')
			case '(', ')', '\\':
				b = append(b, data[i])
			case '\n':
			case '\r':
				if i+1 < len(data) && data[i+1] == '\n' {
					i++
				}
			default:
				if data[i] >= '0' && data[i] <= '7' {
					oct := 0
					count := 0
					for count < 3 && i < len(data) && data[i] >= '0' && data[i] <= '7' {
						oct = oct*8 + int(data[i]-'0')
						i++
						count++
					}
					b = append(b, byte(oct))
					continue
				}
				b = append(b, data[i])
			}
			i++
			continue
		}
		if c == '(' {
			depth++
			b = append(b, c)
			i++
			continue
		}
		if c == ')' {
			depth--
			if depth == 0 {
				i++
				break
			}
			b = append(b, c)
			i++
			continue
		}
		b = append(b, c)
		i++
	}
	return b, i
}

func parseHex(data []byte, i int) ([]byte, int) {
	i++ // <
	var hexdigits []byte
	for i < len(data) && data[i] != '>' {
		c := data[i]
		if isHex(c) {
			hexdigits = append(hexdigits, c)
		}
		i++
	}
	if i < len(data) && data[i] == '>' {
		i++
	}
	if len(hexdigits)%2 == 1 {
		hexdigits = append(hexdigits, '0')
	}
	out := make([]byte, len(hexdigits)/2)
	for k := 0; k < len(out); k++ {
		hi := unhex(hexdigits[k*2])
		lo := unhex(hexdigits[k*2+1])
		out[k] = hi<<4 | lo
	}
	return out, i
}

func isHex(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func unhex(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10
	}
	return 0
}

func parseArray(data []byte, i int) ([]token, int) {
	i++ // [
	var arr []token
	for i < len(data) {
		i = skipWS(data, i)
		if i >= len(data) {
			break
		}
		if data[i] == ']' {
			i++
			break
		}
		if data[i] == '%' {
			for i < len(data) && data[i] != '\n' && data[i] != '\r' {
				i++
			}
			continue
		}
		switch data[i] {
		case '(':
			raw, ni := parseLiteral(data, i)
			arr = append(arr, token{kind: tkStr, raw: raw})
			i = ni
		case '<':
			if i+1 < len(data) && data[i+1] == '<' {
				ni := skipDict(data, i)
				arr = append(arr, token{kind: tkDict})
				i = ni
			} else {
				raw, ni := parseHex(data, i)
				arr = append(arr, token{kind: tkStr, raw: raw})
				i = ni
			}
		case '[':
			sub, ni := parseArray(data, i)
			arr = append(arr, token{kind: tkArray, arr: sub})
			i = ni
		case '/':
			name, ni := parseName(data, i)
			arr = append(arr, token{kind: tkName, text: name})
			i = ni
		default:
			j := i
			for j < len(data) && !isDelim(data[j]) {
				j++
			}
			word := string(data[i:j])
			if n, ok := parseNum(word); ok {
				arr = append(arr, token{kind: tkNum, num: n})
			} else if word != "" {
				arr = append(arr, token{kind: tkOp, text: word})
			}
			if j == i {
				i++
			} else {
				i = j
			}
		}
	}
	return arr, i
}

func skipDict(data []byte, i int) int {
	// i at '<' of '<<'
	if i+1 < len(data) && data[i] == '<' && data[i+1] == '<' {
		i += 2
	}
	depth := 1
	for i < len(data) && depth > 0 {
		if data[i] == '%' {
			for i < len(data) && data[i] != '\n' && data[i] != '\r' {
				i++
			}
			continue
		}
		if data[i] == '(' {
			_, ni := parseLiteral(data, i)
			i = ni
			continue
		}
		if data[i] == '<' && i+1 < len(data) && data[i+1] == '<' {
			depth++
			i += 2
			continue
		}
		if data[i] == '>' && i+1 < len(data) && data[i+1] == '>' {
			depth--
			i += 2
			continue
		}
		i++
	}
	return i
}

func pop1(st *[]token) float64 {
	if len(*st) == 0 {
		return 0
	}
	t := (*st)[len(*st)-1]
	*st = (*st)[:len(*st)-1]
	if t.kind == tkNum {
		return t.num
	}
	return 0
}

func popN2(st *[]token) (float64, float64) {
	b := pop1(st)
	a := pop1(st)
	return b, a
}

func popN3(st *[]token) (float64, float64, float64) {
	c := pop1(st)
	b := pop1(st)
	a := pop1(st)
	return c, b, a
}

func popN4(st *[]token) (float64, float64, float64, float64) {
	d := pop1(st)
	c := pop1(st)
	b := pop1(st)
	a := pop1(st)
	return d, c, b, a
}

func popN(st *[]token, n int) (a, b, c, d, e, f float64) {
	vals := make([]float64, n)
	for i := n - 1; i >= 0; i-- {
		vals[i] = pop1(st)
	}
	get := func(i int) float64 {
		if i < len(vals) {
			return vals[i]
		}
		return 0
	}
	return get(0), get(1), get(2), get(3), get(4), get(5)
}

func popName(st *[]token) string {
	if len(*st) == 0 {
		return ""
	}
	t := (*st)[len(*st)-1]
	*st = (*st)[:len(*st)-1]
	if t.kind == tkName {
		return t.text
	}
	return ""
}

func popStr(st *[]token) []byte {
	if len(*st) == 0 {
		return nil
	}
	t := (*st)[len(*st)-1]
	*st = (*st)[:len(*st)-1]
	if t.kind == tkStr {
		return t.raw
	}
	return nil
}

func popArray(st *[]token) []token {
	if len(*st) == 0 {
		return nil
	}
	t := (*st)[len(*st)-1]
	*st = (*st)[:len(*st)-1]
	if t.kind == tkArray {
		return t.arr
	}
	return nil
}
