package gopdf2

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
)

// bakeAlphaOverlays composites a translucent image onto the opaque image
// underneath it. PowerPoint composites PNG alpha against black, so a white
// wordmark (the Tencent logo) is drawn black even when the PNG pixels are white.
func bakeAlphaOverlays(p *Page) {
	drop := map[int]bool{}
	for i := range p.Elements {
		baseEl := p.Elements[i].Image
		if baseEl == nil || drop[i] {
			continue
		}
		base, err := decodeRGBA(baseEl.Data)
		if err != nil || base == nil {
			continue
		}
		changed := false
		for j := i + 1; j < len(p.Elements); j++ {
			overEl := p.Elements[j].Image
			if overEl == nil || drop[j] {
				continue
			}
			if !boxInside(overEl, baseEl, 2) {
				continue
			}
			over, err := decodeRGBA(overEl.Data)
			if err != nil || over == nil || !hasTransparency(over) {
				continue
			}
			blitOver(base, baseEl, over, overEl)
			drop[j] = true
			changed = true
		}
		if !changed {
			continue
		}
		var buf bytes.Buffer
		if err := png.Encode(&buf, base); err != nil {
			continue
		}
		baseEl.Data = buf.Bytes()
		baseEl.Format = "png"
	}
	if len(drop) == 0 {
		return
	}
	kept := p.Elements[:0]
	for i, el := range p.Elements {
		if drop[i] {
			continue
		}
		kept = append(kept, el)
	}
	p.Elements = kept
}

func decodeRGBA(data []byte) (*image.NRGBA, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	b := img.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			out.SetNRGBA(x-b.Min.X, y-b.Min.Y, c)
		}
	}
	return out, nil
}

func hasTransparency(img *image.NRGBA) bool {
	b := img.Bounds()
	step := 1
	if b.Dx()*b.Dy() > 40000 {
		step = 4
	}
	for y := 0; y < b.Dy(); y += step {
		for x := 0; x < b.Dx(); x += step {
			if img.NRGBAAt(x, y).A < 250 {
				return true
			}
		}
	}
	return false
}

func boxInside(inner, outer *ImageObject, slop float64) bool {
	if outer.Width < 1 || outer.Height < 1 || inner.Width < 1 || inner.Height < 1 {
		return false
	}
	return inner.X >= outer.X-slop && inner.Y >= outer.Y-slop &&
		inner.X+inner.Width <= outer.X+outer.Width+slop &&
		inner.Y+inner.Height <= outer.Y+outer.Height+slop
}

func blitOver(dst *image.NRGBA, dstBox *ImageObject, src *image.NRGBA, srcBox *ImageObject) {
	dw, dh := dst.Bounds().Dx(), dst.Bounds().Dy()
	sw, sh := src.Bounds().Dx(), src.Bounds().Dy()
	if dw < 1 || dh < 1 || sw < 1 || sh < 1 {
		return
	}
	x0 := int((srcBox.X - dstBox.X) / dstBox.Width * float64(dw))
	y0 := int((srcBox.Y - dstBox.Y) / dstBox.Height * float64(dh))
	x1 := int((srcBox.X + srcBox.Width - dstBox.X) / dstBox.Width * float64(dw))
	y1 := int((srcBox.Y + srcBox.Height - dstBox.Y) / dstBox.Height * float64(dh))
	if x1 <= x0 || y1 <= y0 {
		return
	}
	for y := y0; y < y1; y++ {
		if y < 0 || y >= dh {
			continue
		}
		sy := (y - y0) * sh / (y1 - y0)
		if sy < 0 || sy >= sh {
			continue
		}
		for x := x0; x < x1; x++ {
			if x < 0 || x >= dw {
				continue
			}
			sx := (x - x0) * sw / (x1 - x0)
			if sx < 0 || sx >= sw {
				continue
			}
			s := src.NRGBAAt(sx, sy)
			if s.A == 0 {
				continue
			}
			d := dst.NRGBAAt(x, y)
			af := float64(s.A) / 255
			dst.SetNRGBA(x, y, color.NRGBA{
				R: byte(float64(s.R)*af + float64(d.R)*(1-af)),
				G: byte(float64(s.G)*af + float64(d.G)*(1-af)),
				B: byte(float64(s.B)*af + float64(d.B)*(1-af)),
				A: 255,
			})
		}
	}
}
