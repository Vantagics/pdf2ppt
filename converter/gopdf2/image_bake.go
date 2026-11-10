package gopdf2

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
)

// opacifyLightMasks flattens a mostly-transparent light wordmark onto the
// picture behind it and stores that as an opaque PNG. The picture stays in
// the slide. PowerPoint has drawn this file's white RGBA wordmark as black
// (it treats the four channels like a mask); an opaque RGB picture has no
// mask to misread. Other translucent pictures are left alone: folding every
// soft image into the background is what deleted logos in v1.3.8.
func opacifyLightMasks(p *Page) {
	for i := range p.Elements {
		overEl := p.Elements[i].Image
		if overEl == nil {
			continue
		}
		over, err := decodeRGBA(overEl.Data)
		if err != nil || over == nil || !lightMask(over) {
			continue
		}
		baseEl, base := backdrop(p, i, overEl)
		if base == nil {
			continue
		}
		flat := image.NewNRGBA(over.Bounds())
		sampleBackdrop(flat, over, overEl, base, baseEl)
		var buf bytes.Buffer
		if err := png.Encode(&buf, flat); err != nil {
			continue
		}
		overEl.Data = buf.Bytes()
		overEl.Format = "png"
	}
}

func backdrop(p *Page, i int, over *ImageObject) (*ImageObject, *image.NRGBA) {
	for j := i - 1; j >= 0; j-- {
		baseEl := p.Elements[j].Image
		if baseEl == nil || !boxInside(over, baseEl, 2) {
			continue
		}
		base, err := decodeRGBA(baseEl.Data)
		if err != nil || base == nil {
			continue
		}
		return baseEl, base
	}
	return nil, nil
}

// lightMask is a wordmark: most pixels are clear and the visible pixels are
// near-white. A photograph with a soft edge is mostly opaque and is kept.
func lightMask(img *image.NRGBA) bool {
	b := img.Bounds()
	n := b.Dx() * b.Dy()
	if n < 16 {
		return false
	}
	var clear, opaque int
	var r, g, bl int64
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			c := img.NRGBAAt(x, y)
			if c.A < 16 {
				clear++
				continue
			}
			if c.A < 200 {
				continue
			}
			opaque++
			r += int64(c.R)
			g += int64(c.G)
			bl += int64(c.B)
		}
	}
	if opaque < 8 || clear*2 < n {
		return false
	}
	return r/int64(opaque) > 200 && g/int64(opaque) > 200 && bl/int64(opaque) > 200
}

func sampleBackdrop(dst, src *image.NRGBA, srcBox *ImageObject, base *image.NRGBA, baseBox *ImageObject) {
	sw, sh := src.Bounds().Dx(), src.Bounds().Dy()
	bw, bh := base.Bounds().Dx(), base.Bounds().Dy()
	for y := 0; y < sh; y++ {
		py := srcBox.Y + (float64(y)+0.5)/float64(sh)*srcBox.Height
		by := int((py - baseBox.Y) / baseBox.Height * float64(bh))
		if by < 0 {
			by = 0
		}
		if by >= bh {
			by = bh - 1
		}
		for x := 0; x < sw; x++ {
			s := src.NRGBAAt(x, y)
			px := srcBox.X + (float64(x)+0.5)/float64(sw)*srcBox.Width
			bx := int((px - baseBox.X) / baseBox.Width * float64(bw))
			if bx < 0 {
				bx = 0
			}
			if bx >= bw {
				bx = bw - 1
			}
			d := base.NRGBAAt(bx, by)
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

func boxInside(inner, outer *ImageObject, slop float64) bool {
	if outer.Width < 1 || outer.Height < 1 || inner.Width < 1 || inner.Height < 1 {
		return false
	}
	return inner.X >= outer.X-slop && inner.Y >= outer.Y-slop &&
		inner.X+inner.Width <= outer.X+outer.Width+slop &&
		inner.Y+inner.Height <= outer.Y+outer.Height+slop
}
