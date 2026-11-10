package gopdf2

import (
	"math"
	"unicode"
	"unicode/utf8"

	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// symbolShape draws one isolated character from an embedded symbolic font
// as a filled outline. Keynote's lock is a TrueType glyph whose cmap entry
// is "$"; painting that code in Noto Sans is the giant dollar sign.
// A normal letter or digit stays text. The outline is nil when the font
// program does not parse or the glyph is empty.
// replaceIsolatedSymbols turns a one-character run that never joined a word
// into the embedded glyph outline. The Keynote lock stays "$" until here,
// then becomes the lock path. A comma inside "你好，世界" was already merged.
func replaceIsolatedSymbols(p *Page) {
	for i := range p.Elements {
		t := p.Elements[i].Text
		if t == nil || len(t.symbolProg) == 0 || len(t.Runs) > 0 {
			continue
		}
		sh := symbolShape(t.symbolProg, t.Content, t.X, t.Y+t.FontSize, t.FontSize, t.ColorR, t.ColorG, t.ColorB)
		if sh == nil {
			continue
		}
		p.Elements[i] = PageElement{Shape: sh}
	}
}

func symbolShape(prog []byte, text string, originX, baselineY, size float64, r, g, b uint8) *ShapeObject {
	if len(prog) == 0 || size < 1 {
		return nil
	}
	if utf8.RuneCountInString(text) != 1 {
		return nil
	}
	ch, _ := utf8.DecodeRuneInString(text)
	if ch == utf8.RuneError || unicode.IsLetter(ch) || unicode.IsDigit(ch) {
		return nil
	}
	font, err := sfnt.Parse(prog)
	if err != nil {
		return nil
	}
	var buf sfnt.Buffer
	gid, err := font.GlyphIndex(&buf, ch)
	if err != nil || gid == 0 {
		// A one-glyph subset often stores the icon at index 1 and the PDF
		// encoding, not the cmap, is what selects it.
		if font.NumGlyphs() < 2 {
			return nil
		}
		gid = 1
	}
	ppem := fixed.I(64)
	segs, err := font.LoadGlyph(&buf, gid, ppem, nil)
	if err != nil || len(segs) == 0 {
		return nil
	}
	// 64 px per em. Segment coords are 26.6 pixels, y growing down.
	scale := size / 64
	toPt := func(v fixed.Int26_6) float64 {
		return float64(v) / 64 * scale
	}
	var ops []PathOp
	minX, minY := math.MaxFloat64, math.MaxFloat64
	maxX, maxY := -math.MaxFloat64, -math.MaxFloat64
	consider := func(x, y float64) {
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
	for _, seg := range segs {
		switch seg.Op {
		case sfnt.SegmentOpMoveTo:
			x := originX + toPt(seg.Args[0].X)
			y := baselineY + toPt(seg.Args[0].Y)
			consider(x, y)
			ops = append(ops, PathOp{Op: "M", X: x, Y: y})
		case sfnt.SegmentOpLineTo:
			x := originX + toPt(seg.Args[0].X)
			y := baselineY + toPt(seg.Args[0].Y)
			consider(x, y)
			ops = append(ops, PathOp{Op: "L", X: x, Y: y})
		case sfnt.SegmentOpQuadTo:
			// One quadratic becomes a cubic so DrawingML can draw it.
			qx := originX + toPt(seg.Args[0].X)
			qy := baselineY + toPt(seg.Args[0].Y)
			x := originX + toPt(seg.Args[1].X)
			y := baselineY + toPt(seg.Args[1].Y)
			consider(qx, qy)
			consider(x, y)
			px, py := x, y
			if n := len(ops); n > 0 {
				px, py = ops[n-1].X, ops[n-1].Y
			}
			ops = append(ops, PathOp{
				Op: "C",
				X:  x, Y: y,
				X1: px + 2.0/3.0*(qx-px), Y1: py + 2.0/3.0*(qy-py),
				X2: x + 2.0/3.0*(qx-x), Y2: y + 2.0/3.0*(qy-y),
			})
		case sfnt.SegmentOpCubeTo:
			x1 := originX + toPt(seg.Args[0].X)
			y1 := baselineY + toPt(seg.Args[0].Y)
			x2 := originX + toPt(seg.Args[1].X)
			y2 := baselineY + toPt(seg.Args[1].Y)
			x := originX + toPt(seg.Args[2].X)
			y := baselineY + toPt(seg.Args[2].Y)
			consider(x1, y1)
			consider(x2, y2)
			consider(x, y)
			ops = append(ops, PathOp{Op: "C", X: x, Y: y, X1: x1, Y1: y1, X2: x2, Y2: y2})
		}
	}
	if len(ops) == 0 || maxX-minX < 0.5 || maxY-minY < 0.5 {
		return nil
	}
	local := make([]PathOp, len(ops))
	for i, op := range ops {
		op.X -= minX
		op.Y -= minY
		op.X1 -= minX
		op.Y1 -= minY
		op.X2 -= minX
		op.Y2 -= minY
		local[i] = op
	}
	return &ShapeObject{
		X: minX, Y: minY, Width: maxX - minX, Height: maxY - minY,
		HasFill: true, FillR: r, FillG: g, FillB: b, FillAlpha: 1,
		Path: local,
	}
}
