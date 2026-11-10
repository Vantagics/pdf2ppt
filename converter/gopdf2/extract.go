package gopdf2

import (
	"math"

	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

func (d *Document) extractPages() error {
	n := d.ctx.PageCount
	if n < 0 {
		n = 0
	}
	d.pages = make([]*Page, 0, n)
	for i := 1; i <= n; i++ {
		page, err := d.extractPage(i)
		if err != nil {
			return err
		}
		d.pages = append(d.pages, page)
	}
	return nil
}

func (d *Document) extractPage(pageNr int) (*Page, error) {
	xrd := d.ctx.XRefTable
	dict, _, inh, err := xrd.PageDict(pageNr, false)
	if err != nil {
		return nil, err
	}
	box := (*types.Rectangle)(nil)
	if inh != nil {
		box = inh.CropBox
		if box == nil {
			box = inh.MediaBox
		}
	}
	originX, originY := 0.0, 0.0
	width, height := 612.0, 792.0
	if box != nil {
		originX = box.LL.X
		originY = box.LL.Y
		width = box.Width()
		height = box.Height()
		if width < 1 {
			width = 612
		}
		if height < 1 {
			height = 792
		}
	}
	var resDict types.Dict
	if inh != nil && inh.Resources != nil {
		resDict = inh.Resources
	} else if dict != nil {
		if o, ok := dict.Find("Resources"); ok {
			if rd, err := xrd.DereferenceDict(o); err == nil {
				resDict = rd
			}
		}
	}
	var content []byte
	if dict != nil {
		bb, err := xrd.PageContent(dict)
		if err != nil && err != model.ErrNoContent {
			return nil, err
		}
		content = bb
	}
	res := newResources(xrd, resDict, nil)
	page := interpretPage(content, res, originX, originY, width, height)
	if inh != nil && inh.Rotate != 0 {
		applyRotation(page, inh.Rotate)
	}
	mergeTextLines(page)
	page.syncLists()
	return page, nil
}

// applyRotation turns a rotated page upright. 90/270 swap the axes.
func applyRotation(p *Page, deg int) {
	deg = ((deg % 360) + 360) % 360
	if deg == 0 {
		return
	}
	w, h := p.Width, p.Height
	switch deg {
	case 90:
		p.Width, p.Height = h, w
	case 270:
		p.Width, p.Height = h, w
	case 180:
	default:
		return
	}
	for i := range p.Elements {
		el := &p.Elements[i]
		switch {
		case el.Text != nil:
			// The box is the unrotated glyph box; rot spins it around its
			// center. Move the center and add the page angle. Swapping the
			// box and leaving rot in place draws the text on its side.
			el.Text.X, el.Text.Y, el.Text.Width, el.Text.Height, el.Text.Rotation = rotatePlaced(deg, w, h, el.Text.X, el.Text.Y, el.Text.Width, el.Text.Height, el.Text.Rotation)
		case el.Image != nil:
			el.Image.X, el.Image.Y, el.Image.Width, el.Image.Height, el.Image.Rotation = rotatePlaced(deg, w, h, el.Image.X, el.Image.Y, el.Image.Width, el.Image.Height, el.Image.Rotation)
		case el.Shape != nil:
			rotateShape(deg, w, h, el.Shape)
		}
	}
}

// rotatePlaced turns an object that DrawingML rotates around its center.
// Width and height stay in the object's own axes.
func rotatePlaced(deg int, pageW, pageH, x, y, w, h, rot float64) (float64, float64, float64, float64, float64) {
	cx, cy := x+w/2, y+h/2
	nx, ny := mapPoint(deg, pageW, pageH, cx, cy)
	return nx - w/2, ny - h/2, w, h, normRot(rot + float64(deg))
}

// mapPoint rotates a y-down page point clockwise into the displayed page.
func mapPoint(deg int, pageW, pageH, x, y float64) (float64, float64) {
	switch deg {
	case 90:
		return pageH - y, x
	case 270:
		return y, pageW - x
	case 180:
		return pageW - x, pageH - y
	default:
		return x, y
	}
}

func normRot(d float64) float64 {
	d = math.Mod(d, 360)
	if d < 0 {
		d += 360
	}
	if d < 0.05 || d > 359.95 {
		return 0
	}
	return d
}

func rotateShape(deg int, pageW, pageH float64, sh *ShapeObject) {
	if len(sh.Path) == 0 || sh.SimpleRect {
		sh.X, sh.Y, sh.Width, sh.Height = rotateBox(deg, pageW, pageH, sh.X, sh.Y, sh.Width, sh.Height)
		return
	}
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	consider := func(lx, ly float64) {
		x, y := mapPoint(deg, pageW, pageH, sh.X+lx, sh.Y+ly)
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
	for _, op := range sh.Path {
		switch op.Op {
		case "M", "L":
			consider(op.X, op.Y)
		case "C":
			consider(op.X, op.Y)
			consider(op.X1, op.Y1)
			consider(op.X2, op.Y2)
		}
	}
	if minX > maxX {
		sh.X, sh.Y, sh.Width, sh.Height = rotateBox(deg, pageW, pageH, sh.X, sh.Y, sh.Width, sh.Height)
		sh.SimpleRect = true
		sh.Path = nil
		return
	}
	rel := func(lx, ly float64) (float64, float64) {
		x, y := mapPoint(deg, pageW, pageH, sh.X+lx, sh.Y+ly)
		return x - minX, y - minY
	}
	for i, op := range sh.Path {
		switch op.Op {
		case "M", "L":
			sh.Path[i].X, sh.Path[i].Y = rel(op.X, op.Y)
		case "C":
			sh.Path[i].X, sh.Path[i].Y = rel(op.X, op.Y)
			sh.Path[i].X1, sh.Path[i].Y1 = rel(op.X1, op.Y1)
			sh.Path[i].X2, sh.Path[i].Y2 = rel(op.X2, op.Y2)
		}
	}
	sh.X, sh.Y = minX, minY
	sh.Width, sh.Height = maxX-minX, maxY-minY
	if sh.Width < 0.4 {
		sh.Width = 0.4
	}
	if sh.Height < 0.4 {
		sh.Height = 0.4
	}
}

func (p *Page) syncLists() {
	p.TextObjects = nil
	p.Images = nil
	for i := range p.Elements {
		el := &p.Elements[i]
		if el.Text != nil {
			p.TextObjects = append(p.TextObjects, el.Text)
		}
		if el.Image != nil {
			p.Images = append(p.Images, el.Image)
		}
	}
}

func rotateBox(deg int, pageW, pageH, x, y, w, h float64) (float64, float64, float64, float64) {
	switch deg {
	case 90:
		// Clockwise: (x, y) -> (pageH - (y+h), x), size swaps.
		return pageH - (y + h), x, h, w
	case 270:
		return y, pageW - (x + w), h, w
	case 180:
		return pageW - (x + w), pageH - (y + h), w, h
	default:
		return x, y, w, h
	}
}
