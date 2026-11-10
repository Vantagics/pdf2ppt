package gopdf2

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
)

// renderStencil paints a soft-mask glyph image. The color comes from the
// tiny sample; the mask is the ink. A 2×2 image stretched over the line
// is a blank rectangle.
func renderStencil(src image.Image, mask []byte, mw, mh int, dec, matte []float64, opacity float64) []byte {
	if src == nil || mw < 1 || mh < 1 || len(mask) < mw*mh {
		return nil
	}
	out := image.NewNRGBA(image.Rect(0, 0, mw, mh))
	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	if sw < 1 {
		sw = 1
	}
	if sh < 1 {
		sh = 1
	}
	ink := 0
	for y := 0; y < mh; y++ {
		sy := sb.Min.Y + y*sh/mh
		if sy >= sb.Max.Y {
			sy = sb.Max.Y - 1
		}
		for x := 0; x < mw; x++ {
			sx := sb.Min.X + x*sw/mw
			if sx >= sb.Max.X {
				sx = sb.Max.X - 1
			}
			r, g, b, _ := src.At(sx, sy).RGBA()
			alpha := maskAlpha(mask, mw, mh, dec, x, y, mw, mh)
			if opacity < 0.999 {
				alpha = byte(float64(alpha) * opacity)
			}
			rr, gg, bb := byte(r>>8), byte(g>>8), byte(b>>8)
			if len(matte) > 0 {
				rr, gg, bb = unmatte(rr, gg, bb, alpha, matte)
			}
			if alpha > 16 {
				ink++
			}
			out.SetNRGBA(x, y, color.NRGBA{R: rr, G: gg, B: bb, A: alpha})
		}
	}
	if ink < 8 {
		return nil
	}
	var buf bytes.Buffer
	if err := (&png.Encoder{CompressionLevel: png.BestCompression}).Encode(&buf, out); err != nil {
		return nil
	}
	return buf.Bytes()
}

// compositeStencils joins neighboring glyph masks into one PNG per text
// block. One picture per line would add a thousand drawingML parts.
type stencilItem struct {
	i  int
	im *ImageObject
}

func compositeStencils(p *Page) {
	var items []stencilItem
	for i := range p.Elements {
		im := p.Elements[i].Image
		if im == nil || !im.Stencil || math.Abs(im.Rotation) > 0.4 {
			continue
		}
		items = append(items, stencilItem{i, im})
	}
	if len(items) < 2 {
		return
	}
	parent := make([]int, len(items))
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(a int) int {
		if parent[a] != a {
			parent[a] = find(parent[a])
		}
		return parent[a]
	}
	union := func(a, b int) {
		pa, pb := find(a), find(b)
		if pa != pb {
			parent[pa] = pb
		}
	}
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if stencilNear(items[i].im, items[j].im) {
				union(i, j)
			}
		}
	}
	groups := map[int][]int{}
	for i := range items {
		groups[find(i)] = append(groups[find(i)], i)
	}
	remove := map[int]bool{}
	for _, g := range groups {
		if len(g) < 2 {
			continue
		}
		comp, ok := compositeGroup(items, g)
		if !ok {
			continue
		}
		p.Elements[items[g[0]].i].Image = comp
		for _, k := range g[1:] {
			remove[items[k].i] = true
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

func stencilNear(a, b *ImageObject) bool {
	mx := math.Max(12, math.Max(a.Height, b.Height)*1.2)
	my := math.Max(4, math.Max(a.Height, b.Height)*0.85)
	return a.X < b.X+b.Width+mx && b.X < a.X+a.Width+mx &&
		a.Y < b.Y+b.Height+my && b.Y < a.Y+a.Height+my
}

func compositeGroup(items []stencilItem, g []int) (*ImageObject, bool) {
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	scale := 2.0
	decoded := make([]image.Image, len(g))
	for n, k := range g {
		im := items[k].im
		if im.X < minX {
			minX = im.X
		}
		if im.Y < minY {
			minY = im.Y
		}
		if im.X+im.Width > maxX {
			maxX = im.X + im.Width
		}
		if im.Y+im.Height > maxY {
			maxY = im.Y + im.Height
		}
		src, err := png.Decode(bytes.NewReader(im.Data))
		if err != nil || src == nil {
			return nil, false
		}
		decoded[n] = src
		if im.Width > 1 {
			s := float64(src.Bounds().Dx()) / im.Width
			if s > scale {
				scale = s
			}
		}
	}
	if scale > 8 {
		scale = 8
	}
	pw := int(math.Round((maxX - minX) * scale))
	ph := int(math.Round((maxY - minY) * scale))
	if pw < 1 || ph < 1 || pw > 6000 || ph > 6000 || pw*ph > 6_000_000 {
		return nil, false
	}
	dst := image.NewNRGBA(image.Rect(0, 0, pw, ph))
	for n, k := range g {
		im := items[k].im
		blitOver(dst, decoded[n],
			int(math.Round((im.X-minX)*scale)),
			int(math.Round((im.Y-minY)*scale)),
			int(math.Round(im.Width*scale)),
			int(math.Round(im.Height*scale)),
		)
	}
	var buf bytes.Buffer
	if err := (&png.Encoder{CompressionLevel: png.BestCompression}).Encode(&buf, dst); err != nil {
		return nil, false
	}
	return &ImageObject{
		Data: buf.Bytes(), Format: "png",
		X: minX, Y: minY, Width: maxX - minX, Height: maxY - minY,
	}, true
}

func blitOver(dst *image.NRGBA, src image.Image, dx, dy, dw, dh int) {
	if dw < 1 || dh < 1 {
		return
	}
	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	if sw < 1 || sh < 1 {
		return
	}
	for y := 0; y < dh; y++ {
		yy := dy + y
		if yy < 0 || yy >= dst.Bounds().Dy() {
			continue
		}
		sy := sb.Min.Y + y*sh/dh
		if sy >= sb.Max.Y {
			sy = sb.Max.Y - 1
		}
		for x := 0; x < dw; x++ {
			xx := dx + x
			if xx < 0 || xx >= dst.Bounds().Dx() {
				continue
			}
			sx := sb.Min.X + x*sw/dw
			if sx >= sb.Max.X {
				sx = sb.Max.X - 1
			}
			c := color.NRGBAModel.Convert(src.At(sx, sy)).(color.NRGBA)
			if c.A == 0 {
				continue
			}
			d := dst.NRGBAAt(xx, yy)
			if d.A == 0 || c.A == 255 {
				dst.SetNRGBA(xx, yy, c)
				continue
			}
			a := float64(c.A) / 255
			da := float64(d.A) / 255
			outA := a + da*(1-a)
			dst.SetNRGBA(xx, yy, color.NRGBA{
				R: byte((float64(c.R)*a + float64(d.R)*da*(1-a)) / outA),
				G: byte((float64(c.G)*a + float64(d.G)*da*(1-a)) / outA),
				B: byte((float64(c.B)*a + float64(d.B)*da*(1-a)) / outA),
				A: byte(outA * 255),
			})
		}
	}
}
