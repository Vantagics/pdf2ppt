package gopdf2

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"

	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

func encodePlacedImage(xrd *model.XRefTable, sd *types.StreamDict, opacity float64) (data []byte, format string, err error) {
	if sd == nil {
		return nil, "", nil
	}
	if opacity <= 0 {
		opacity = 1
	}
	width := intEntry(xrd, sd.Dict, "Width")
	height := intEntry(xrd, sd.Dict, "Height")
	if width <= 0 || height <= 0 {
		return nil, "", nil
	}
	bpc := intEntry(xrd, sd.Dict, "BitsPerComponent")
	if bpc == 0 {
		bpc = 8
	}
	csName, ncomp := colorInfo(xrd, sd.Dict)

	// Decode the stream. DCTDecode stays as JPEG bytes unless CMYK.
	if err := sd.Decode(); err != nil {
		return nil, "", err
	}
	raw := sd.Content
	if len(raw) == 0 {
		return nil, "", nil
	}

	smask, smW, smH, smDecode, matte := softMask(xrd, sd.Dict)
	decode := decodeArray(xrd, sd.Dict)
	invert := decodeInverts(decode, ncomp)
	needPixels := smask != nil || opacity < 0.999 || invert || csName != "rgb" || bpc != 8

	if isJPEG(raw) && !needPixels {
		return raw, "jpeg", nil
	}

	var img image.Image
	if isJPEG(raw) {
		img, err = jpeg.Decode(bytes.NewReader(raw))
		if err != nil {
			return nil, "", err
		}
	} else {
		img = samplesToImage(width, height, bpc, csName, ncomp, raw, decode)
		if img == nil {
			return nil, "", nil
		}
	}

	out := image.NewNRGBA(img.Bounds())
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bv, a := img.At(x, y).RGBA()
			alpha := byte(a >> 8)
			if smask != nil {
				alpha = maskAlpha(smask, smW, smH, smDecode, x-b.Min.X, y-b.Min.Y, b.Dx(), b.Dy())
			}
			if opacity < 0.999 {
				alpha = byte(float64(alpha) * opacity)
			}
			rr, gg, bb := byte(r>>8), byte(g>>8), byte(bv>>8)
			// An SMask Matte means the color was premultiplied with that
			// color. Leaving it premultiplied paints the matte (black, for
			// these slides) a second time, so a light watermark becomes gray.
			if len(matte) > 0 {
				rr, gg, bb = unmatte(rr, gg, bb, alpha, matte)
			}
			out.SetNRGBA(x, y, color.NRGBA{R: rr, G: gg, B: bb, A: alpha})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, out); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), "png", nil
}

func isJPEG(b []byte) bool {
	return len(b) > 3 && b[0] == 0xFF && b[1] == 0xD8
}

// cropEncodedImage trims an encoded image by fractions of its width and height
// measured from each edge. left/right/top/bottom are 0..1. Top is the first
// row of the bitmap.
func cropEncodedImage(data []byte, left, top, right, bottom float64) ([]byte, string, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", err
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	x0 := b.Min.X + int(math.Round(left*float64(w)))
	x1 := b.Max.X - int(math.Round(right*float64(w)))
	y0 := b.Min.Y + int(math.Round(top*float64(h)))
	y1 := b.Max.Y - int(math.Round(bottom*float64(h)))
	if x1-x0 < 1 || y1-y0 < 1 {
		return nil, "", nil
	}
	type subImager interface {
		SubImage(r image.Rectangle) image.Image
	}
	var cropped image.Image
	if s, ok := img.(subImager); ok {
		cropped = s.SubImage(image.Rect(x0, y0, x1, y1))
	} else {
		dst := image.NewNRGBA(image.Rect(0, 0, x1-x0, y1-y0))
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				dst.Set(x-x0, y-y0, img.At(x, y))
			}
		}
		cropped = dst
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, cropped); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), "png", nil
}

func colorInfo(xrd *model.XRefTable, d types.Dict) (name string, n int) {
	o, ok := d.Find("ColorSpace")
	if !ok || o == nil {
		if im := d.BooleanEntry("ImageMask"); im != nil && *im {
			return "gray", 1
		}
		return "rgb", 3
	}
	o, err := xrd.Dereference(o)
	if err != nil || o == nil {
		return "rgb", 3
	}
	switch cs := o.(type) {
	case types.Name:
		return csName(cs.Value())
	case types.Array:
		if len(cs) == 0 {
			return "rgb", 3
		}
		kind := ""
		if nm, ok := cs[0].(types.Name); ok {
			kind = nm.Value()
		}
		switch kind {
		case "ICCBased":
			if len(cs) > 1 {
				if sd, _, err := xrd.DereferenceStreamDict(cs[1]); err == nil && sd != nil {
					if n := intEntry(xrd, sd.Dict, "N"); n == 1 || n == 3 || n == 4 {
						if n == 1 {
							return "gray", 1
						}
						if n == 4 {
							return "cmyk", 4
						}
						return "rgb", 3
					}
				}
			}
			return "rgb", 3
		case "Indexed":
			return "indexed", 1
		case "CalRGB", "Lab":
			return "rgb", 3
		case "CalGray":
			return "gray", 1
		case "DeviceN", "Separation":
			return "gray", 1
		default:
			return csName(kind)
		}
	default:
		return "rgb", 3
	}
}

func csName(v string) (string, int) {
	switch v {
	case "DeviceGray", "G":
		return "gray", 1
	case "DeviceCMYK", "CMYK":
		return "cmyk", 4
	case "DeviceRGB", "RGB", "":
		return "rgb", 3
	default:
		return "rgb", 3
	}
}

func decodeArray(xrd *model.XRefTable, d types.Dict) []float64 {
	o, ok := d.Find("Decode")
	if !ok {
		return nil
	}
	arr, err := xrd.DereferenceArray(o)
	if err != nil {
		return nil
	}
	out := make([]float64, 0, len(arr))
	for _, el := range arr {
		if n, ok := objNum(xrd, el); ok {
			out = append(out, n)
		}
	}
	return out
}

func decodeInverts(dec []float64, ncomp int) bool {
	if len(dec) < 2 {
		return false
	}
	// Any component mapped high-to-low needs a pixel rewrite.
	pairs := ncomp
	if pairs < 1 {
		pairs = 1
	}
	for i := 0; i < pairs && i*2+1 < len(dec); i++ {
		if dec[i*2] > dec[i*2+1] {
			return true
		}
	}
	return false
}

func softMask(xrd *model.XRefTable, d types.Dict) (samples []byte, w, h int, dec, matte []float64) {
	o, ok := d.Find("SMask")
	if !ok || o == nil {
		return nil, 0, 0, nil, nil
	}
	sd, _, err := xrd.DereferenceStreamDict(o)
	if err != nil || sd == nil {
		return nil, 0, 0, nil, nil
	}
	w = intEntry(xrd, sd.Dict, "Width")
	h = intEntry(xrd, sd.Dict, "Height")
	matte = numberList(xrd, sd.Dict["Matte"])
	if err := sd.Decode(); err != nil {
		return nil, 0, 0, nil, nil
	}
	data := sd.Content
	if isJPEG(data) {
		img, err := jpeg.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, 0, 0, nil, nil
		}
		b := img.Bounds()
		samples = make([]byte, b.Dx()*b.Dy())
		i := 0
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				r, g, bv, _ := img.At(x, y).RGBA()
				samples[i] = byte(((r + g + bv) / 3) >> 8)
				i++
			}
		}
		w, h = b.Dx(), b.Dy()
	} else {
		samples = data
	}
	return samples, w, h, decodeArray(xrd, sd.Dict), matte
}

// unmatte reverses PDF soft-mask premultiplication:
// stored = matte + alpha*(straight - matte).
func unmatte(r, g, b, alpha byte, matte []float64) (byte, byte, byte) {
	if alpha == 0 || len(matte) == 0 {
		return r, g, b
	}
	a := float64(alpha) / 255
	comp := func(sample byte, m float64) byte {
		v := (float64(sample)/255 - (1-a)*m) / a
		if v < 0 {
			v = 0
		}
		if v > 1 {
			v = 1
		}
		return byte(math.Round(v * 255))
	}
	if len(matte) == 1 {
		v := comp(r, matte[0])
		return v, v, v
	}
	mr, mg, mb := matte[0], 0.0, 0.0
	if len(matte) > 1 {
		mg = matte[1]
	}
	if len(matte) > 2 {
		mb = matte[2]
	}
	return comp(r, mr), comp(g, mg), comp(b, mb)
}

func numberList(xrd *model.XRefTable, o types.Object) []float64 {
	if o == nil {
		return nil
	}
	arr, err := xrd.DereferenceArray(o)
	if err != nil || len(arr) == 0 {
		return nil
	}
	out := make([]float64, 0, len(arr))
	for _, el := range arr {
		if n, ok := objNum(xrd, el); ok {
			out = append(out, n)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// flattenOn composites a PNG onto a solid color and returns an opaque PNG.
// PowerPoint has composited PNG alpha against black, which turns a
// translucent light watermark into a gray page.
func flattenOn(data []byte, bgR, bgG, bgB byte) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	b := img.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			// NRGBAModel yields straight alpha. color.Color.RGBA is premultiplied.
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			af := float64(c.A) / 255
			out.SetNRGBA(x-b.Min.X, y-b.Min.Y, color.NRGBA{
				R: byte(math.Round(float64(c.R)*af + float64(bgR)*(1-af))),
				G: byte(math.Round(float64(c.G)*af + float64(bgG)*(1-af))),
				B: byte(math.Round(float64(c.B)*af + float64(bgB)*(1-af))),
				A: 255,
			})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, out); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func maskAlpha(samples []byte, w, h int, dec []float64, x, y, dstW, dstH int) byte {
	if w <= 0 || h <= 0 || len(samples) == 0 {
		return 255
	}
	sx, sy := x, y
	if w != dstW || h != dstH {
		sx = x * w / max(dstW, 1)
		sy = y * h / max(dstH, 1)
	}
	if sx < 0 {
		sx = 0
	}
	if sy < 0 {
		sy = 0
	}
	if sx >= w {
		sx = w - 1
	}
	if sy >= h {
		sy = h - 1
	}
	idx := sy*w + sx
	if idx < 0 || idx >= len(samples) {
		return 255
	}
	norm := float64(samples[idx]) / 255
	if len(dec) >= 2 {
		norm = dec[0] + norm*(dec[1]-dec[0])
	}
	if norm < 0 {
		norm = 0
	}
	if norm > 1 {
		norm = 1
	}
	return byte(math.Round(norm * 255))
}

func samplesToImage(width, height, bpc int, cs string, ncomp int, raw []byte, dec []float64) image.Image {
	if bpc != 8 || width <= 0 || height <= 0 || ncomp <= 0 {
		return nil
	}
	stride := width * ncomp
	if len(raw) < stride*height {
		return nil
	}
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		row := raw[y*stride : (y+1)*stride]
		for x := 0; x < width; x++ {
			pix := row[x*ncomp : (x+1)*ncomp]
			r, g, b := byte(0), byte(0), byte(0)
			switch cs {
			case "gray":
				v := applyDec(pix[0], dec, 0)
				r, g, b = v, v, v
			case "cmyk":
				if len(pix) < 4 {
					continue
				}
				c := float64(applyDec(pix[0], dec, 0)) / 255
				m := float64(applyDec(pix[1], dec, 1)) / 255
				yy := float64(applyDec(pix[2], dec, 2)) / 255
				k := float64(applyDec(pix[3], dec, 3)) / 255
				r = byte(math.Round(255 * (1 - math.Min(1, c+k))))
				g = byte(math.Round(255 * (1 - math.Min(1, m+k))))
				b = byte(math.Round(255 * (1 - math.Min(1, yy+k))))
			default:
				if len(pix) < 3 {
					continue
				}
				r = applyDec(pix[0], dec, 0)
				g = applyDec(pix[1], dec, 1)
				b = applyDec(pix[2], dec, 2)
			}
			img.SetNRGBA(x, y, color.NRGBA{R: r, G: g, B: b, A: 255})
		}
	}
	return img
}

func applyDec(sample byte, dec []float64, comp int) byte {
	if len(dec) < (comp+1)*2 {
		return sample
	}
	d0 := dec[comp*2]
	d1 := dec[comp*2+1]
	if d0 == 0 && d1 == 1 {
		return sample
	}
	norm := float64(sample) / 255
	norm = d0 + norm*(d1-d0)
	if norm < 0 {
		norm = 0
	}
	if norm > 1 {
		norm = 1
	}
	return byte(math.Round(norm * 255))
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
