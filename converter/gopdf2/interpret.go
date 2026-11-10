package gopdf2

import (
	"math"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

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
	fillCS, strokeCS       string
	lineWidth              float64
	fillAlpha, strokeAlpha float64
	fillGrad               *Gradient
	fillTile               *tileFill
	fillIsPat              bool // color space is a pattern we could not resolve
	fontName               string
	fontSize               float64
	charSp, wordSp, hScale float64
	leading, rise          float64
	render                 int
	font                   *pdfFont
	hasClip                bool
	clipX0, clipY0         float64
	clipX1, clipY1         float64
}

// tileFill is a PatternType 1 cell whose content is one or more images.
// PowerPoint exports a slide background this way: the cell is one photo,
// the pattern matrix pins the cell origin to the top of the page, and the
// tile that actually covers the page is the one below the origin.
type tileFill struct {
	base         matrix
	pm           matrix
	xStep, yStep float64
	images       []patImage
}

type patImage struct {
	res  *resources
	name string
	m    matrix
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
	// baseCTM is the transform in effect at the start of the current content
	// stream. Shading pattern matrices map into that space; the cm operators
	// that follow must not be applied again (PDF viewers cancel them).
	baseCTM     matrix
	seenContent bool
	hasBg       bool
	bg          color3
	bgGrad      *Gradient
	elements    []PageElement
	formDepth   int
	// ocSkip counts marked-content groups whose optional content is off.
	ocSkip int
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
		baseCTM: ident(),
	}
	if len(content) > 0 {
		ip.execContent(content)
	}
	page := &Page{
		Width:         width,
		Height:        height,
		HasBackground: ip.hasBg,
		Elements:      suppressDuplicateGlyphOutlines(ip.elements),
	}
	if ip.hasBg {
		page.BgR, page.BgG, page.BgB = ip.bg.bytes()
		page.BgGradient = ip.bgGrad
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
		ip.clearFillPattern()
	case "G":
		gray := pop1(st)
		ip.gs.stroke = color3{gray, gray, gray}
		ip.gs.strokeN = 1
	case "rg":
		b, g, r := popN3(st)
		ip.gs.fill = color3{r, g, b}
		ip.gs.fillN = 3
		ip.clearFillPattern()
	case "RG":
		b, g, r := popN3(st)
		ip.gs.stroke = color3{r, g, b}
		ip.gs.strokeN = 3
	case "k":
		k, y, m, c := popN4(st)
		ip.gs.fill = cmyk(c, m, y, k)
		ip.gs.fillN = 4
		ip.clearFillPattern()
	case "K":
		k, y, m, c := popN4(st)
		ip.gs.stroke = cmyk(c, m, y, k)
		ip.gs.strokeN = 4
	case "cs":
		name := popName(st)
		ip.gs.fillCS = name
		ip.gs.fillN = ip.res.numComponents(name)
		ip.gs.fillIsPat = name == "Pattern" || ip.res.isPatternSpace(name)
		if ip.gs.fillIsPat {
			ip.gs.fillGrad = nil
			ip.gs.fillTile = nil
		}
	case "CS":
		name := popName(st)
		ip.gs.strokeCS = name
		ip.gs.strokeN = ip.res.numComponents(name)
	case "sc":
		ip.popColor(st, ip.gs.fillN, true)
	case "scn":
		if len(*st) > 0 && (*st)[len(*st)-1].kind == tkName {
			name := popName(st)
			ip.gs.fillGrad = nil
			ip.gs.fillTile = nil
			if g := ip.res.axialPattern(name, ip.baseCTM); g != nil {
				ip.gs.fillGrad = g
				ip.gs.fillIsPat = false
			} else if g := ip.res.shadingInsideTile(name, ip.baseCTM); g != nil {
				// Canva's script wordmark is a letter outline filled with a
				// tiling pattern whose only paint is an axial shading.
				ip.gs.fillGrad = g
				ip.gs.fillIsPat = false
			} else if t := ip.res.tilingPattern(name, ip.baseCTM); t != nil {
				ip.gs.fillTile = t
				ip.gs.fillIsPat = false
			} else {
				ip.gs.fillIsPat = true
			}
		} else {
			ip.popColor(st, ip.gs.fillN, true)
		}
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
		// The path becomes the clip. A later image is cropped to this rectangle,
		// which drops the NeurIPS tweet that is drawn outside its card.
		if x0, y0, x1, y1, ok := pathBounds(ip.ops); ok {
			if ip.gs.hasClip {
				if x0 < ip.gs.clipX0 {
					x0 = ip.gs.clipX0
				}
				if y0 < ip.gs.clipY0 {
					y0 = ip.gs.clipY0
				}
				if x1 > ip.gs.clipX1 {
					x1 = ip.gs.clipX1
				}
				if y1 > ip.gs.clipY1 {
					y1 = ip.gs.clipY1
				}
			}
			ip.gs.hasClip = true
			ip.gs.clipX0, ip.gs.clipY0, ip.gs.clipX1, ip.gs.clipY1 = x0, y0, x1, y1
		}
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
	case "BMC":
		popName(st)
		if ip.ocSkip > 0 {
			ip.ocSkip++
		}
	case "MP":
		popName(st)
	case "BDC", "DP":
		var prop string
		if len(*st) > 0 {
			last := (*st)[len(*st)-1]
			*st = (*st)[:len(*st)-1]
			if last.kind == tkName {
				prop = last.text
			}
		}
		tag := popName(st)
		if ip.ocSkip > 0 || (tag == "OC" && ip.res.ocHidden(prop)) {
			ip.ocSkip++
		}
	case "EMC":
		if ip.ocSkip > 0 {
			ip.ocSkip--
		}
	default:
		// drop a trailing operand if this was a no-op we don't model
	}
}

func csComponents(name string) int {
	switch name {
	case "DeviceGray", "G", "CalGray":
		return 1
	case "DeviceCMYK", "CMYK":
		return 4
	case "DeviceRGB", "RGB", "CalRGB", "Lab":
		return 3
	default:
		return 3
	}
}

// numComponents is how many numbers sc/SC consume. A named DeviceGray or an
// ICC profile with N=1 is one number. Treating that number as RGB stores it
// in the blue channel, which is why PDFMaker pages came out #0000FF.
func (r *resources) numComponents(name string) int {
	switch name {
	case "DeviceGray", "G", "CalGray":
		return 1
	case "DeviceRGB", "RGB", "CalRGB", "Lab":
		return 3
	case "DeviceCMYK", "CMYK":
		return 4
	case "Pattern":
		return 0
	}
	if r == nil {
		return 3
	}
	if n, ok := componentsOf(r.xrd, r.colorSpaceObj(name)); ok {
		return n
	}
	if r.parent != nil {
		return r.parent.numComponents(name)
	}
	return 3
}

func (r *resources) indexedColor(name string, idx float64) (color3, bool) {
	if r == nil || name == "" {
		return color3{}, false
	}
	o := r.colorSpaceObj(name)
	if o == nil {
		if r.parent != nil {
			return r.parent.indexedColor(name, idx)
		}
		return color3{}, false
	}
	o, err := r.xrd.Dereference(o)
	if err != nil || o == nil {
		return color3{}, false
	}
	arr, ok := o.(types.Array)
	if !ok || len(arr) < 4 {
		return color3{}, false
	}
	kind := ""
	if nm, ok := arr[0].(types.Name); ok {
		kind = nm.Value()
	}
	if kind != "Indexed" {
		return color3{}, false
	}
	baseN := 3
	if nm, ok := arr[1].(types.Name); ok {
		_, baseN = csName(nm.Value())
	}
	if baseN != 1 && baseN != 3 {
		baseN = 3
	}
	hival := 0
	if n, ok := objNum(r.xrd, arr[2]); ok {
		hival = int(n)
	}
	lut := lookupBytes(r.xrd, arr[3])
	i := int(math.Round(idx))
	if i < 0 {
		i = 0
	}
	if hival > 0 && i > hival {
		i = hival
	}
	off := i * baseN
	if off < 0 || off+baseN > len(lut) {
		return color3{}, false
	}
	if baseN == 1 {
		v := float64(lut[off]) / 255
		return color3{v, v, v}, true
	}
	return color3{float64(lut[off]) / 255, float64(lut[off+1]) / 255, float64(lut[off+2]) / 255}, true
}

func (r *resources) separationColor(name string, tint float64) (color3, bool) {
	if r == nil || name == "" {
		return color3{}, false
	}
	o := r.colorSpaceObj(name)
	if o == nil {
		if r.parent != nil {
			return r.parent.separationColor(name, tint)
		}
		return color3{}, false
	}
	o, err := r.xrd.Dereference(o)
	if err != nil || o == nil {
		return color3{}, false
	}
	arr, ok := o.(types.Array)
	if !ok || len(arr) < 2 {
		return color3{}, false
	}
	kind := ""
	if nm, ok := arr[0].(types.Name); ok {
		kind = nm.Value()
	}
	if kind != "Separation" && kind != "DeviceN" {
		return color3{}, false
	}
	// A spot tint is not a gray level. Without a tint transform we refuse
	// the gray fallback; callers keep the previous color.
	return color3{}, true
}

func (r *resources) isPatternSpace(name string) bool {
	if name == "Pattern" || r == nil {
		return name == "Pattern"
	}
	o := r.colorSpaceObj(name)
	if o == nil {
		if r.parent != nil {
			return r.parent.isPatternSpace(name)
		}
		return false
	}
	o, err := r.xrd.Dereference(o)
	if err != nil || o == nil {
		return false
	}
	arr, ok := o.(types.Array)
	if !ok || len(arr) == 0 {
		return false
	}
	nm, ok := arr[0].(types.Name)
	return ok && nm.Value() == "Pattern"
}

func (r *resources) ocHidden(name string) bool {
	if r == nil || name == "" || r.xrd == nil {
		return false
	}
	props := r.sub("Properties")
	if props == nil {
		if r.parent != nil {
			return r.parent.ocHidden(name)
		}
		return false
	}
	o, ok := props.Find(name)
	if !ok {
		return false
	}
	off := ocOffSet(r.xrd)
	if ir, ok := o.(types.IndirectRef); ok {
		return off[int(ir.ObjectNumber)]
	}
	d, err := r.xrd.DereferenceDict(o)
	if err != nil || d == nil {
		return false
	}
	if oc, ok := d.Find("OC"); ok {
		if ir, ok := oc.(types.IndirectRef); ok {
			return off[int(ir.ObjectNumber)]
		}
	}
	return false
}

func ocOffSet(xrd *model.XRefTable) map[int]bool {
	out := map[int]bool{}
	if xrd == nil {
		return out
	}
	cat, err := xrd.Catalog()
	if err != nil || cat == nil {
		return out
	}
	oc, err := xrd.DereferenceDict(cat["OCProperties"])
	if err != nil || oc == nil {
		return out
	}
	d, err := xrd.DereferenceDict(oc["D"])
	if err != nil || d == nil {
		return out
	}
	arr, err := xrd.DereferenceArray(d["OFF"])
	if err != nil || arr == nil {
		return out
	}
	for _, el := range arr {
		if ir, ok := el.(types.IndirectRef); ok {
			out[int(ir.ObjectNumber)] = true
		}
	}
	return out
}

func (r *resources) colorSpaceObj(name string) types.Object {
	if r == nil {
		return nil
	}
	cs := r.sub("ColorSpace")
	if cs == nil {
		return nil
	}
	o, ok := cs.Find(name)
	if !ok {
		return nil
	}
	return o
}

func componentsOf(xrd *model.XRefTable, o types.Object) (int, bool) {
	if xrd == nil || o == nil {
		return 0, false
	}
	o, err := xrd.Dereference(o)
	if err != nil || o == nil {
		return 0, false
	}
	switch v := o.(type) {
	case types.Name:
		switch v.Value() {
		case "DeviceGray", "G", "CalGray":
			return 1, true
		case "DeviceRGB", "RGB", "CalRGB", "Lab":
			return 3, true
		case "DeviceCMYK", "CMYK":
			return 4, true
		default:
			return 0, false
		}
	case types.Array:
		if len(v) == 0 {
			return 0, false
		}
		kind := ""
		if nm, ok := v[0].(types.Name); ok {
			kind = nm.Value()
		}
		switch kind {
		case "CalGray":
			return 1, true
		case "CalRGB", "Lab":
			return 3, true
		case "Indexed", "Separation":
			return 1, true
		case "ICCBased":
			if len(v) < 2 {
				return 0, false
			}
			sd, _, err := xrd.DereferenceStreamDict(v[1])
			if err != nil || sd == nil {
				return 0, false
			}
			if n := intEntry(xrd, sd.Dict, "N"); n > 0 {
				return n, true
			}
			return 0, false
		case "DeviceN":
			if len(v) < 2 {
				return 0, false
			}
			names, err := xrd.DereferenceArray(v[1])
			if err != nil {
				return 0, false
			}
			return len(names), true
		default:
			return 0, false
		}
	default:
		return 0, false
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
		cs := ip.gs.fillCS
		if !fill {
			cs = ip.gs.strokeCS
		}
		if looked, ok := ip.res.indexedColor(cs, vals[0]); ok {
			c = looked
		} else if _, spot := ip.res.separationColor(cs, vals[0]); spot {
			// An index or a spot tint is not a gray level. Leaving it black
			// beats painting navy body text as cyan.
		} else {
			// DeviceGray, CalGray, and a 1-channel ICC profile.
			c = color3{vals[0], vals[0], vals[0]}
		}
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
		ip.clearFillPattern()
	} else {
		ip.gs.stroke = c
	}
}

func (ip *interp) clearFillPattern() {
	ip.gs.fillGrad = nil
	ip.gs.fillTile = nil
	ip.gs.fillIsPat = false
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
	if ip.ocSkip > 0 {
		ip.ops = nil
		ip.haveCur = false
		return
	}
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
	// ca/CA of 0 is how Google Slides hides the black rect under a clipped image.
	// Painting it opaque covers the real background and the text.
	if fill && ip.gs.fillAlpha <= 0.001 {
		fill = false
	}
	if stroke && ip.gs.strokeAlpha <= 0.001 {
		stroke = false
	}
	if !fill && !stroke {
		return
	}
	if fill && ip.gs.fillTile != nil {
		ip.emitTile(minX, minY, maxX, maxY)
		fill = false
		if !stroke {
			return
		}
	}
	// An unresolved pattern must not fall back to the default black fill.
	if fill && ip.gs.fillIsPat {
		fill = false
		if !stroke {
			return
		}
	}
	if fill && ip.gs.fillGrad != nil && !ip.seenContent && ip.coversPage(minX, minY, maxX, maxY) && !hasCurve(ops) {
		ip.hasBg = true
		ip.bgGrad = ip.gs.fillGrad
		return
	}
	// A second subpath is a hole (a frame around a white panel). Painting the
	// bounds as a solid page color fills that hole.
	if fill && ip.gs.fillGrad == nil && !ip.seenContent && ip.coversPage(minX, minY, maxX, maxY) && !hasCurve(ops) && subpathCount(ops) <= 1 {
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
		sh.Gradient = ip.gs.fillGrad
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

func subpathCount(ops []pathOp) int {
	n := 0
	for _, o := range ops {
		if o.op == 'M' {
			n++
		}
	}
	return n
}

func endsWithCJK(s string) bool {
	if s == "" {
		return false
	}
	r, _ := utf8.DecodeLastRuneInString(s)
	return r >= 0x2E80 && r <= 0x9FFF || r >= 0x3000 && r <= 0x303F || r >= 0xFF00 && r <= 0xFFEF
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
	var cx, cy float64
	have := false
	for _, o := range ops {
		switch o.op {
		case 'M', 'L':
			consider(o.x, o.y)
			cx, cy = o.x, o.y
			have = true
		case 'C':
			if !have {
				cx, cy = o.x1, o.y1
			}
			// Control points sit outside a rounded corner. The box of the
			// controls is taller than the curve and keeps the footer under
			// the NeurIPS chart. Use the curve's own extremes.
			xLo, xHi := bezExtrema(cx, o.x1, o.x2, o.x)
			yLo, yHi := bezExtrema(cy, o.y1, o.y2, o.y)
			consider(xLo, yLo)
			consider(xHi, yHi)
			cx, cy = o.x, o.y
			have = true
		}
	}
	return
}

func bezExtrema(p0, p1, p2, p3 float64) (float64, float64) {
	lo, hi := p0, p0
	if p3 < lo {
		lo = p3
	}
	if p3 > hi {
		hi = p3
	}
	sample := func(t float64) {
		if t <= 0 || t >= 1 {
			return
		}
		u := 1 - t
		v := u*u*u*p0 + 3*u*u*t*p1 + 3*u*t*t*p2 + t*t*t*p3
		if v < lo {
			lo = v
		}
		if v > hi {
			hi = v
		}
	}
	// B'(t) = 0 is a quadratic a t^2 + b t + c = 0.
	a := -p0 + 3*p1 - 3*p2 + p3
	b := 2*p0 - 4*p1 + 2*p2
	c := -p0 + p1
	if math.Abs(a) < 1e-9 {
		if math.Abs(b) > 1e-9 {
			sample(-c / b)
		}
		return lo, hi
	}
	disc := b*b - 4*a*c
	if disc < 0 {
		return lo, hi
	}
	s := math.Sqrt(disc)
	sample((-b + s) / (2 * a))
	sample((-b - s) / (2 * a))
	return lo, hi
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
	if !ip.inText || ip.ocSkip > 0 {
		return
	}
	var text string
	var total float64
	fontSize := ip.gs.fontSize
	for _, el := range arr {
		switch el.kind {
		case tkStr:
			s, adv := ip.decodeShow(el.raw)
			// A large Tc (character spacing) is how PowerPoint spreads the
			// parentheses of P=P+ρg(z−z) across the formula. One run would
			// draw "()" as two adjacent glyphs.
			if ip.gs.charSp > ip.gs.fontSize*0.35 && len([]rune(s)) > 1 {
				if text != "" {
					ip.emitText(text, total)
					text, total = "", 0
				}
				rs := []rune(s)
				each := adv / float64(len(rs))
				for _, r := range rs {
					ip.emitText(string(r), each)
					if n := len(ip.elements); n > 0 && ip.elements[n-1].Text != nil {
						tb := ip.elements[n-1].Text
						// The advance includes Tc. The glyph itself is about one em.
						if tb.Width > tb.FontSize*1.2 {
							tb.Width = tb.FontSize * 0.6
						}
					}
				}
				continue
			}
			text += s
			total += adv
		case tkNum:
			adj := tjAdjust(el.num, fontSize, ip.gs.hScale)
			// xdvipdfmx writes a word space as TJ -250, which is exactly a
			// quarter of an em. A strict greater-than missed that gap and
			// glued "You can now abuse fonts" into one word.
			// A much wider gap is a separate run (the "155" and "8" on the
			// NRCS slide), not one space stretched across every letter.
			if adj > fontSize*0.8 && text != "" {
				ip.emitText(text, total)
				ip.advance(adj)
				text = ""
				total = 0
				continue
			}
			if gapIsWordSpace(adj, fontSize) && !strings.HasSuffix(text, " ") && !endsWithCJK(text) {
				text += " "
			}
			total += adj
		}
	}
	ip.emitText(text, total)
}

func (ip *interp) showString(raw []byte, extra float64) {
	if !ip.inText || ip.ocSkip > 0 {
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
	if mapped, fam, ok := rewriteWingdings(family, text); ok {
		text = mapped
		family = fam
	}
	if mapped, fam, ok := rewriteSymbol(family, text); ok {
		text = mapped
		family = fam
	}
	stroke := ip.gs.render == 1 || ip.gs.render == 2 || ip.gs.render == 5 || ip.gs.render == 6
	strokeOnly := ip.gs.render == 1 || ip.gs.render == 5
	sw := 0.0
	var sr, sg, sb uint8
	if stroke {
		sx := math.Hypot(ip.gs.ctm[0], ip.gs.ctm[1])
		sy := math.Hypot(ip.gs.ctm[2], ip.gs.ctm[3])
		scale := (sx + sy) / 2
		if scale == 0 {
			scale = 1
		}
		sw = ip.gs.lineWidth * scale
		sr, sg, sb = ip.gs.stroke.bytes()
	}
	bx, by := ip.gs.ctm.apply(ip.tm[4], ip.tm[5])
	// Rise is in unscaled text space.
	if ip.gs.rise != 0 {
		rx, ry := ip.tm.applyVec(0, ip.gs.rise)
		px, py := ip.gs.ctm.applyVec(rx, ry)
		bx += px
		by += py
	}
	pdfWidth := ip.pageLen(textAdv)
	if pdfWidth < eff*0.4 {
		pdfWidth = float64(len([]rune(text))) * eff * 0.55
	}
	// Fit the fallback font to the PDF advance before the box is padded,
	// so a wider substitute cannot paint into the next run.
	family, eff, tracking := fitRun(family, bold, italic, eff, pdfWidth, text)
	// A small right-bearing pad keeps the last glyph inside the box.
	// It is not extra ink: tracking already pulled the glyphs to pdfWidth.
	width := pdfWidth + math.Min(eff*0.08, 1.2)
	// LibreOffice (and PowerPoint) put the baseline one em below the top of
	// a zero-inset text box. Using the font ascender here drops the glyphs
	// by (1-ascent)*size, which is several pixels on a typical slide.
	const baselineFromTop = 1.0
	height := (baselineFromTop + desc) * eff
	if height < eff*1.15 {
		height = eff * 1.15
	}
	yTop := (ip.originY + ip.pageH) - (by + baselineFromTop*eff)
	xLeft := bx - ip.originX
	rot := textRotation(ip.tm, ip.gs.ctm)
	if math.Abs(rot) > 0.4 {
		// DrawingML rotates the box around its center. Shift the unrotated
		// top-left so the baseline origin stays on the PDF point.
		base := baselineFromTop * eff
		vx, vy := -width/2, base-height/2
		// The offset uses the same clockwise angle DrawingML applies.
		// Empirically the y-down clockwise matrix matches OOXML when the
		// angle is the value written to rot.
		a := rot * math.Pi / 180
		cos, sin := math.Cos(a), math.Sin(a)
		// y-down, clockwise: x' = x cos - y sin, y' = x sin + y cos.
		ox := vx*cos - vy*sin
		oy := vx*sin + vy*cos
		baselineY := (ip.originY + ip.pageH) - by
		cx := xLeft - ox
		cy := baselineY - oy
		xLeft = cx - width/2
		yTop = cy - height/2
	}
	tb := &TextObject{
		Content:     text,
		FontName:    family,
		FontSize:    eff,
		Bold:        bold,
		Italic:      italic,
		X:           xLeft,
		Y:           yTop,
		Width:       width,
		Height:      height,
		Rotation:    rot,
		Stroke:      stroke && sw > 0.05,
		StrokeOnly:  strokeOnly,
		StrokeR:     sr,
		StrokeG:     sg,
		StrokeB:     sb,
		StrokeWidth: sw,
		Tracking:    tracking,
	}
	tb.ColorR, tb.ColorG, tb.ColorB = ip.gs.fill.bytes()
	ip.seenContent = true
	ip.elements = append(ip.elements, PageElement{Text: tb})
	ip.advance(textAdv)
}

func (r *resources) tilingPattern(name string, base matrix) *tileFill {
	d, sd := r.patternDict(name)
	if d == nil || intEntry(r.xrd, d, "PatternType") != 1 || sd == nil {
		return nil
	}
	if err := sd.Decode(); err != nil || len(sd.Content) == 0 {
		return nil
	}
	placed := patternImagePlacements(sd.Content)
	if len(placed) == 0 {
		return nil
	}
	res := r
	if o, ok := sd.Find("Resources"); ok {
		if rd, err := r.xrd.DereferenceDict(o); err == nil && rd != nil {
			res = newResources(r.xrd, rd, r)
		}
	}
	pm := ident()
	if m := readMatrix(r.xrd, d.ArrayEntry("Matrix")); m != nil {
		pm = *m
	}
	xs := numEntry(r.xrd, d, "XStep")
	ys := numEntry(r.xrd, d, "YStep")
	if bb := d.ArrayEntry("BBox"); len(bb) == 4 {
		x0, ok0 := objNum(r.xrd, bb[0])
		y0, ok1 := objNum(r.xrd, bb[1])
		x1, ok2 := objNum(r.xrd, bb[2])
		y1, ok3 := objNum(r.xrd, bb[3])
		if ok0 && ok1 && ok2 && ok3 {
			if xs == 0 {
				xs = x1 - x0
			}
			if ys == 0 {
				ys = y1 - y0
			}
		}
	}
	tf := &tileFill{base: base, pm: pm, xStep: xs, yStep: ys}
	for _, p := range placed {
		tf.images = append(tf.images, patImage{res: res, name: p.name, m: p.m})
	}
	return tf
}

type placedPatImage struct {
	name string
	m    matrix
}

// patternImagePlacements walks a tiling-pattern content stream and records
// each image Do with the CTM in effect in pattern space.
func patternImagePlacements(content []byte) []placedPatImage {
	toks := tokenize(content)
	var st []token
	ctm := ident()
	var saved []matrix
	var out []placedPatImage
	for _, t := range toks {
		if t.kind != tkOp {
			st = append(st, t)
			continue
		}
		switch t.text {
		case "q":
			saved = append(saved, ctm)
			st = nil
		case "Q":
			if n := len(saved); n > 0 {
				ctm = saved[n-1]
				saved = saved[:n-1]
			}
			st = nil
		case "cm":
			if len(st) >= 6 {
				nums := make([]float64, 6)
				okm := true
				for i := 0; i < 6; i++ {
					if st[len(st)-6+i].kind != tkNum {
						okm = false
						break
					}
					nums[i] = st[len(st)-6+i].num
				}
				if okm {
					ctm = concat(matrix{nums[0], nums[1], nums[2], nums[3], nums[4], nums[5]}, ctm)
				}
			}
			st = nil
		case "Do":
			if len(st) > 0 && st[len(st)-1].kind == tkName {
				out = append(out, placedPatImage{name: st[len(st)-1].text, m: ctm})
			}
			st = nil
		default:
			st = nil
		}
	}
	return out
}

func (r *resources) patternDict(name string) (types.Dict, *types.StreamDict) {
	if r == nil || name == "" {
		return nil, nil
	}
	pd := r.sub("Pattern")
	if pd == nil {
		if r.parent != nil {
			return r.parent.patternDict(name)
		}
		return nil, nil
	}
	o, ok := pd.Find(name)
	if !ok {
		if r.parent != nil {
			return r.parent.patternDict(name)
		}
		return nil, nil
	}
	sd, _, err := r.xrd.DereferenceStreamDict(o)
	if err == nil && sd != nil {
		return sd.Dict, sd
	}
	d, err := r.xrd.DereferenceDict(o)
	if err != nil {
		return nil, nil
	}
	return d, nil
}

func readMatrix(xrd *model.XRefTable, m types.Array) *matrix {
	if len(m) != 6 {
		return nil
	}
	var mm [6]float64
	for i := 0; i < 6; i++ {
		n, ok := objNum(xrd, m[i])
		if !ok {
			return nil
		}
		mm[i] = n
	}
	out := matrix{mm[0], mm[1], mm[2], mm[3], mm[4], mm[5]}
	return &out
}

func invertMatrix(m matrix) (matrix, bool) {
	det := m[0]*m[3] - m[1]*m[2]
	if math.Abs(det) < 1e-12 {
		return ident(), false
	}
	inv := 1 / det
	a, b, c, d, e, f := m[0], m[1], m[2], m[3], m[4], m[5]
	return matrix{
		d * inv, -b * inv, -c * inv, a * inv,
		(c*f - d*e) * inv,
		(b*e - a*f) * inv,
	}, true
}

func (ip *interp) emitTile(minX, minY, maxX, maxY float64) {
	t := ip.gs.fillTile
	if t == nil {
		return
	}
	// User = base × patternMatrix × patternPoint (pattern matrix first).
	toPat, ok := invertMatrix(concat(t.pm, t.base))
	if !ok {
		return
	}
	px0, py0, px1, py1 := userBoundsToPattern(toPat, minX, minY, maxX, maxY)
	for _, im := range t.images {
		ix0, iy0, ix1, iy1 := matrixBounds(im.m)
		xs, ys := t.xStep, t.yStep
		if xs < 0.5 {
			xs = ix1 - ix0
		}
		if ys < 0.5 {
			ys = iy1 - iy0
		}
		if xs < 0.5 || ys < 0.5 {
			continue
		}
		xi0 := int(math.Floor((px0-ix1)/xs)) - 1
		xi1 := int(math.Ceil((px1-ix0)/xs)) + 1
		yi0 := int(math.Floor((py0-iy1)/ys)) - 1
		yi1 := int(math.Ceil((py1-iy0)/ys)) + 1
		if xi1-xi0 > 6 {
			xi0, xi1 = -1, 1
		}
		if yi1-yi0 > 6 {
			yi0, yi1 = -2, 1
		}
		for xi := xi0; xi <= xi1; xi++ {
			for yi := yi0; yi <= yi1; yi++ {
				tr := matrix{1, 0, 0, 1, float64(xi) * xs, float64(yi) * ys}
				// Apply image CTM, then the tile offset, then pattern matrix, then base.
				user := concat(im.m, concat(tr, concat(t.pm, t.base)))
				ub0, vb0, ub1, vb1 := matrixBounds(user)
				if ub1 < minX-1 || ub0 > maxX+1 || vb1 < minY-1 || vb0 > maxY+1 {
					continue
				}
				savedCTM := ip.gs.ctm
				savedRes := ip.res
				ip.gs.ctm = user
				ip.res = im.res
				if xo := im.res.xobject(im.name); xo != nil && xo.kind == "image" {
					ip.paintImage(xo)
					// A tile that falls outside its cell is a duplicate. Drop it.
					ip.clipLastImage(minX, minY, maxX, maxY, true)
				}
				ip.gs.ctm = savedCTM
				ip.res = savedRes
			}
		}
	}
}

func userBoundsToPattern(toPat matrix, minX, minY, maxX, maxY float64) (x0, y0, x1, y1 float64) {
	pts := [][2]float64{{minX, minY}, {maxX, minY}, {minX, maxY}, {maxX, maxY}}
	x0, y0 = math.MaxFloat64, math.MaxFloat64
	x1, y1 = -math.MaxFloat64, -math.MaxFloat64
	for _, p := range pts {
		x, y := toPat.apply(p[0], p[1])
		if x < x0 {
			x0 = x
		}
		if y < y0 {
			y0 = y
		}
		if x > x1 {
			x1 = x
		}
		if y > y1 {
			y1 = y
		}
	}
	return
}

func matrixBounds(m matrix) (minX, minY, maxX, maxY float64) {
	pts := [][2]float64{{0, 0}, {1, 0}, {0, 1}, {1, 1}}
	minX, minY = math.MaxFloat64, math.MaxFloat64
	maxX, maxY = -math.MaxFloat64, -math.MaxFloat64
	for _, p := range pts {
		x, y := m.apply(p[0], p[1])
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
	return
}

func (ip *interp) clipIsPage(minX, minY, maxX, maxY float64) bool {
	if ip.pageW < 1 || ip.pageH < 1 {
		return false
	}
	px0, py0 := ip.originX, ip.originY
	px1, py1 := px0+ip.pageW, py0+ip.pageH
	tolX, tolY := ip.pageW*0.04, ip.pageH*0.04
	return minX <= px0+tolX && minY <= py0+tolY && maxX >= px1-tolX && maxY >= py1-tolY
}

func (ip *interp) clipLastImage(minX, minY, maxX, maxY float64, dropOutside bool) {
	if len(ip.elements) == 0 {
		return
	}
	el := &ip.elements[len(ip.elements)-1]
	img := el.Image
	if img == nil {
		return
	}
	// A clip that is the page only trims what hangs off the slide. Cropping
	// to a slightly inset page path is what cut the Canva diamonds.
	if !dropOutside && ip.clipIsPage(minX, minY, maxX, maxY) {
		minX, minY = ip.originX, ip.originY
		maxX, maxY = ip.originX+ip.pageW, ip.originY+ip.pageH
	}
	imgMinX := img.X + ip.originX
	imgMaxX := imgMinX + img.Width
	imgMaxY := (ip.originY + ip.pageH) - img.Y
	imgMinY := imgMaxY - img.Height
	cx0 := math.Max(minX, imgMinX)
	cy0 := math.Max(minY, imgMinY)
	cx1 := math.Min(maxX, imgMaxX)
	cy1 := math.Min(maxY, imgMaxY)
	if cx1-cx0 < 0.5 || cy1-cy0 < 0.5 {
		// Pattern cells drop a tile that missed the cell. A content clip
		// does not delete the picture (that removed 13 UAI images).
		if dropOutside {
			ip.elements = ip.elements[:len(ip.elements)-1]
		}
		return
	}
	// Only axis-aligned placements can be cropped by a rectangle.
	if img.Width < 0.5 || img.Height < 0.5 || math.Abs(img.Rotation) > 0.4 {
		return
	}
	left := (cx0 - imgMinX) / img.Width
	right := (imgMaxX - cx1) / img.Width
	top := (imgMaxY - cy1) / img.Height
	bottom := (cy0 - imgMinY) / img.Height
	cropped, format, err := cropEncodedImage(img.Data, left, top, right, bottom)
	if err != nil || len(cropped) == 0 {
		return
	}
	img.Data = cropped
	img.Format = format
	img.X = cx0 - ip.originX
	img.Y = (ip.originY + ip.pageH) - cy1
	img.Width = cx1 - cx0
	img.Height = cy1 - cy0
}

// shadingInsideTile resolves a tiling pattern that only paints one shading
// pattern. Canva fills each script letter this way.
func (r *resources) shadingInsideTile(name string, ctm matrix) *Gradient {
	d, sd := r.patternDict(name)
	if d == nil || sd == nil || intEntry(r.xrd, d, "PatternType") != 1 {
		return nil
	}
	if err := sd.Decode(); err != nil || len(sd.Content) == 0 {
		return nil
	}
	inner := lastPatternName(sd.Content)
	if inner == "" {
		return nil
	}
	res := r
	if o, ok := sd.Find("Resources"); ok {
		if rd, err := r.xrd.DereferenceDict(o); err == nil && rd != nil {
			res = newResources(r.xrd, rd, r)
		}
	}
	if g := res.axialPattern(inner, ctm); g != nil {
		return g
	}
	return res.shadingInsideTile(inner, ctm)
}

func lastPatternName(content []byte) string {
	toks := tokenize(content)
	name := ""
	for _, t := range toks {
		if t.kind == tkName {
			name = t.text
			continue
		}
		if t.kind == tkOp && (t.text == "scn" || t.text == "SCN") && name != "" && name != "Pattern" {
			return name
		}
	}
	return ""
}

func (r *resources) axialPattern(name string, ctm matrix) *Gradient {
	if r == nil || name == "" {
		return nil
	}
	pd := r.sub("Pattern")
	if pd == nil {
		if r.parent != nil {
			return r.parent.axialPattern(name, ctm)
		}
		return nil
	}
	o, ok := pd.Find(name)
	if !ok {
		if r.parent != nil {
			return r.parent.axialPattern(name, ctm)
		}
		return nil
	}
	d, err := r.xrd.DereferenceDict(o)
	if err != nil || d == nil {
		return nil
	}
	// PatternType 2 is a shading pattern.
	if intEntry(r.xrd, d, "PatternType") != 2 {
		return nil
	}
	shObj, ok := d.Find("Shading")
	if !ok {
		return nil
	}
	sh, err := r.xrd.DereferenceDict(shObj)
	if err != nil || sh == nil {
		return nil
	}
	shadeType := intEntry(r.xrd, sh, "ShadingType")
	coords := sh.ArrayEntry("Coords")
	if shadeType == 3 && len(coords) >= 6 {
		stops := shadingStops(r.xrd, sh)
		if len(stops) < 2 {
			return nil
		}
		// A type-3 shading whose two circles share a center is a plain radial blend.
		return &Gradient{Kind: "radial", Stops: stops}
	}
	if shadeType != 2 || len(coords) < 4 {
		return nil
	}
	nums := make([]float64, 4)
	for i := 0; i < 4; i++ {
		n, ok := objNum(r.xrd, coords[i])
		if !ok {
			return nil
		}
		nums[i] = n
	}
	pm := ident()
	if m := d.ArrayEntry("Matrix"); len(m) == 6 {
		okm := true
		var mm [6]float64
		for i := 0; i < 6; i++ {
			n, ok := objNum(r.xrd, m[i])
			if !ok {
				okm = false
				break
			}
			mm[i] = n
		}
		if okm {
			pm = matrix{mm[0], mm[1], mm[2], mm[3], mm[4], mm[5]}
		}
	}
	// ctm here is the content stream's base transform, not the CTM at the
	// paint operator. Chrome stores the full page mapping in the pattern
	// matrix; multiplying by the paint-time CTM flips the axis.
	x0, y0 := ctm.apply(pm.apply(nums[0], nums[1]))
	x1, y1 := ctm.apply(pm.apply(nums[2], nums[3]))
	ang := math.Atan2(y1-y0, x1-x0) * 180 / math.Pi
	// DrawingML angles are clockwise; PDF is counterclockwise, y-up.
	clockwise := -ang
	for clockwise < 0 {
		clockwise += 360
	}
	stops := shadingStops(r.xrd, sh)
	if len(stops) < 2 {
		return nil
	}
	return &Gradient{Angle: clockwise, Kind: "linear", Stops: stops}
}

func shadingStops(xrd *model.XRefTable, sh types.Dict) []GradStop {
	o, ok := sh.Find("Function")
	if !ok {
		return nil
	}
	if sd, _, err := xrd.DereferenceStreamDict(o); err == nil && sd != nil && intEntry(xrd, sd.Dict, "FunctionType") == 0 {
		return sampledGradient(sd)
	}
	fn, err := xrd.DereferenceDict(o)
	if err != nil || fn == nil {
		return nil
	}
	if intEntry(xrd, fn, "FunctionType") == 2 {
		c0 := rgbArray(xrd, fn, "C0")
		c1 := rgbArray(xrd, fn, "C1")
		if c0 == nil || c1 == nil {
			return nil
		}
		return []GradStop{{0, c0[0], c0[1], c0[2]}, {1, c1[0], c1[1], c1[2]}}
	}
	if intEntry(xrd, fn, "FunctionType") != 3 {
		return nil
	}
	subs := fn.ArrayEntry("Functions")
	bounds := fn.ArrayEntry("Bounds")
	if len(subs) == 0 {
		return nil
	}
	var stops []GradStop
	pos := 0.0
	for i, sub := range subs {
		sd, err := xrd.DereferenceDict(sub)
		if err != nil || sd == nil {
			continue
		}
		c0 := rgbArray(xrd, sd, "C0")
		c1 := rgbArray(xrd, sd, "C1")
		if c0 == nil || c1 == nil {
			continue
		}
		if len(stops) == 0 {
			stops = append(stops, GradStop{0, c0[0], c0[1], c0[2]})
		}
		end := 1.0
		if i < len(bounds) {
			if n, ok := objNum(xrd, bounds[i]); ok {
				end = n
			}
		}
		stops = append(stops, GradStop{end, c1[0], c1[1], c1[2]})
		pos = end
		_ = pos
	}
	return stops
}

// sampledGradient reads a type-0 function (a table of RGB samples) as a
// two-stop blend from the first sample to the last. PowerPoint badges such
// as the mine-safety “四” disc are stored this way.
func sampledGradient(sd *types.StreamDict) []GradStop {
	if err := sd.Decode(); err != nil {
		return nil
	}
	ncomp := 3
	if rg := sd.Dict.ArrayEntry("Range"); len(rg) >= 2 {
		ncomp = len(rg) / 2
	}
	if ncomp < 3 {
		ncomp = 3
	}
	size := 0
	if arr := sd.Dict.ArrayEntry("Size"); len(arr) > 0 {
		if n, ok := objNum(nil, arr[0]); ok {
			size = int(n)
		}
	}
	if size < 2 || intEntry(nil, sd.Dict, "BitsPerSample") != 8 {
		return nil
	}
	raw := sd.Content
	if len(raw) < size*ncomp {
		return nil
	}
	c0 := raw[0:ncomp]
	c1 := raw[(size-1)*ncomp : (size-1)*ncomp+ncomp]
	return []GradStop{
		{0, c0[0], c0[1], c0[2]},
		{1, c1[0], c1[1], c1[2]},
	}
}

func rgbArray(xrd *model.XRefTable, d types.Dict, key string) *[3]uint8 {
	o, ok := d.Find(key)
	if !ok {
		return nil
	}
	arr, err := xrd.DereferenceArray(o)
	if err != nil || len(arr) < 3 {
		return nil
	}
	var out [3]uint8
	for i := 0; i < 3; i++ {
		n, ok := objNum(xrd, arr[i])
		if !ok {
			return nil
		}
		out[i] = toByte(n)
	}
	return &out
}

func textRotation(tm, ctm matrix) float64 {
	dx, dy := tm.applyVec(1, 0)
	px, py := ctm.applyVec(dx, dy)
	if math.Hypot(px, py) < 1e-6 {
		return 0
	}
	deg := math.Atan2(py, px) * 180 / math.Pi
	for deg > 180 {
		deg -= 360
	}
	for deg < -180 {
		deg += 360
	}
	if math.Abs(deg) < 0.5 {
		return 0
	}
	return -deg
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
	if ip.ocSkip > 0 {
		return
	}
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
		savedBase := ip.baseCTM
		ip.formDepth++
		ip.ops = nil
		ip.inText = false
		ip.gs.ctm = concat(xo.matrix, ip.gs.ctm)
		ip.baseCTM = ip.gs.ctm
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
		ip.baseCTM = savedBase
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
	vx, vy := c10x-c00x, c10y-c00y
	ux, uy := c01x-c00x, c01y-c00y
	wAxis, hAxis := math.Hypot(vx, vy), math.Hypot(ux, uy)
	rot := -math.Atan2(vy, vx) * 180 / math.Pi
	for rot < 0 {
		rot += 360
	}
	for rot >= 360 {
		rot -= 360
	}
	// Axis-aligned CTMs, including the y-flip Google Slides uses, already
	// match the bitmap when placed in the bounding box. A fanned photo does not.
	diagonal := wAxis > 0.5 && math.Min(math.Abs(vx), math.Abs(vy)) > 0.2*wAxis
	rotated := diagonal && hAxis > 0.5
	var x, y, w, h float64
	if rotated {
		cx := (c00x + c10x + c01x + c11x) / 4
		cy := (c00y + c10y + c01y + c11y) / 4
		w, h = wAxis, hAxis
		x = cx - w/2 - ip.originX
		y = (ip.originY + ip.pageH) - cy - h/2
	} else {
		w = maxX - minX
		h = maxY - minY
		x = minX - ip.originX
		y = (ip.originY + ip.pageH) - maxY
		rot = 0
	}
	if w < 0.5 || h < 0.5 {
		return
	}
	data, format, err := encodePlacedImage(ip.res.xrd, xo.stream, ip.gs.fillAlpha)
	if err != nil || len(data) == 0 {
		return
	}
	// A page-sized translucent image is the slide background. Bake it onto
	// the page color so PowerPoint cannot composite the PNG alpha against black.
	if ip.coversPage(minX, minY, maxX, maxY) {
		br, bgc, bb := byte(255), byte(255), byte(255)
		if ip.hasBg && ip.bgGrad == nil {
			br, bgc, bb = ip.bg.bytes()
		}
		if flat, ferr := flattenOn(data, br, bgc, bb); ferr == nil && len(flat) > 0 {
			data, format = flat, "png"
		}
	}
	img := &ImageObject{
		Data:     data,
		Format:   format,
		X:        x,
		Y:        y,
		Width:    w,
		Height:   h,
		Rotation: rot,
	}
	ip.seenContent = true
	ip.elements = append(ip.elements, PageElement{Image: img})
	if ip.gs.hasClip {
		ip.clipLastImage(ip.gs.clipX0, ip.gs.clipY0, ip.gs.clipX1, ip.gs.clipY1, false)
	}
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
