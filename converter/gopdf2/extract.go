package gopdf2

import (
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
			el.Text.X, el.Text.Y, el.Text.Width, el.Text.Height = rotateBox(deg, w, h, el.Text.X, el.Text.Y, el.Text.Width, el.Text.Height)
		case el.Image != nil:
			el.Image.X, el.Image.Y, el.Image.Width, el.Image.Height = rotateBox(deg, w, h, el.Image.X, el.Image.Y, el.Image.Width, el.Image.Height)
		case el.Shape != nil:
			el.Shape.X, el.Shape.Y, el.Shape.Width, el.Shape.Height = rotateBox(deg, w, h, el.Shape.X, el.Shape.Y, el.Shape.Width, el.Shape.Height)
			// Path commands are in the pre-rotation local space. A swapped
			// box without rewriting the path would distort curves, so rotated
			// non-rect shapes fall back to their axis-aligned bounds as rects.
			if deg != 180 && !el.Shape.SimpleRect {
				el.Shape.SimpleRect = true
				el.Shape.Path = nil
			}
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
