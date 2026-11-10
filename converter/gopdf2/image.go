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
	var indexed []byte
	var indexedN int
	if csName == "indexed" {
		indexedN, indexed = readIndexed(xrd, sd.Dict)
		if indexedN > 0 {
			ncomp = 1
		}
	}

	// Decode the stream. DCTDecode stays as JPEG bytes unless CMYK.
	if err := sd.Decode(); err != nil {
		return nil, "", err
	}
	raw := sd.Content
	if len(raw) == 0 {
		return nil, "", nil
	}

	smask, smW, smH, smDecode, matte := softMask(xrd, sd.Dict)
	// A soft mask that is solid white does not clip the photo. Keeping it
	// forces a JPEG logo (the NBS marks) through a PNG re-encode.
	if smask != nil && !decodeInverts(smDecode, 1) && maskSolid(smask, 255) {
		smask = nil
	}
	decode := decodeArray(xrd, sd.Dict)
	invert := decodeInverts(decode, ncomp)
	needPixels := smask != nil || opacity < 0.999 || invert || csName != "rgb" || bpc != 8

	if isJPEG(raw) && !needPixels {
		// A page photo stays the original JPEG. A small logo JPEG is stored
		// as PNG so PowerPoint does not soften the DCT when it draws the mark.
		if width <= 400 && height <= 400 {
			if img, err := jpeg.Decode(bytes.NewReader(raw)); err == nil {
				var buf bytes.Buffer
				if err := png.Encode(&buf, img); err == nil {
					return buf.Bytes(), "png", nil
				}
			}
		}
		return raw, "jpeg", nil
	}

	var img image.Image
	if isJPEG(raw) {
		img, err = jpeg.Decode(bytes.NewReader(raw))
		if err != nil {
			return nil, "", err
		}
	} else {
		img = samplesToImage(width, height, bpc, csName, ncomp, raw, decode, indexed, indexedN)
		if img == nil {
			return nil, "", nil
		}
	}

	out := image.NewNRGBA(img.Bounds())
	b := img.Bounds()
	anyAlpha := false
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
			if alpha != 255 {
				anyAlpha = true
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
	// A DCT photo that only needed a soft-mask composite, and came out opaque,
	// stays a JPEG. Logos and diagrams that were never JPEG stay PNG.
	if !anyAlpha && isJPEG(raw) && b.Dx()*b.Dy() >= 128*128 {
		if err := jpeg.Encode(&buf, out, &jpeg.Options{Quality: 92}); err != nil {
			return nil, "", err
		}
		return buf.Bytes(), "jpeg", nil
	}
	if err := png.Encode(&buf, out); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), "png", nil
}

func isJPEG(b []byte) bool {
	return len(b) > 3 && b[0] == 0xFF && b[1] == 0xD8
}

// cropEncodedImage trims a non-JPEG bitmap. DCT photos are not passed
// through here: a clip on those is a:srcRect, so the original bytes stay.
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

// pngHasAlpha reports whether a PNG can composite against the slide.
// JPEG bytes and opaque RGB PNGs cannot.
func pngHasAlpha(data []byte) bool {
	if len(data) < 26 || string(data[:8]) != "\x89PNG\r\n\x1a\n" {
		return false
	}
	switch data[25] {
	case 4, 6:
		return true
	}
	return bytes.Contains(data, []byte("tRNS"))
}

// flattenOn composites an image onto a solid color. A large result is a JPEG
// so a page photo is not stored as a PNG. A small result stays PNG (logos,
// the unit test's 2×2 watermark).
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
	if b.Dx()*b.Dy() >= 128*128 {
		if err := jpeg.Encode(&buf, out, &jpeg.Options{Quality: 92}); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	}
	if err := png.Encode(&buf, out); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func maskSolid(samples []byte, v byte) bool {
	if len(samples) == 0 {
		return false
	}
	for _, s := range samples {
		if s != v {
			return false
		}
	}
	return true
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

// readIndexed returns the base component count and the lookup table for an
// Indexed color space. Tencent's white wordmark is an indexed image whose
// opaque pixels are index 0 (white); ignoring the table paints them black.
func readIndexed(xrd *model.XRefTable, d types.Dict) (ncomp int, lut []byte) {
	o, ok := d.Find("ColorSpace")
	if !ok || o == nil {
		return 0, nil
	}
	o, err := xrd.Dereference(o)
	if err != nil {
		return 0, nil
	}
	arr, ok := o.(types.Array)
	if !ok || len(arr) < 4 {
		return 0, nil
	}
	kind := ""
	if nm, ok := arr[0].(types.Name); ok {
		kind = nm.Value()
	}
	if kind != "Indexed" {
		return 0, nil
	}
	ncomp = 3
	if nm, ok := arr[1].(types.Name); ok {
		_, ncomp = csName(nm.Value())
	} else if sd, _, err := xrd.DereferenceStreamDict(arr[1]); err == nil && sd != nil {
		if n := intEntry(xrd, sd.Dict, "N"); n == 1 || n == 3 || n == 4 {
			ncomp = n
		}
	}
	hival := 0
	if n, ok := objNum(xrd, arr[2]); ok {
		hival = int(n)
	}
	lut = lookupBytes(xrd, arr[3])
	need := (hival + 1) * ncomp
	if ncomp < 1 || len(lut) < need || need < ncomp {
		return 0, nil
	}
	return ncomp, lut[:need]
}

func lookupBytes(xrd *model.XRefTable, o types.Object) []byte {
	if o == nil {
		return nil
	}
	o, err := xrd.Dereference(o)
	if err != nil || o == nil {
		return nil
	}
	switch v := o.(type) {
	case types.StringLiteral:
		b, err := types.Unescape(v.Value())
		if err != nil {
			return []byte(v.Value())
		}
		return b
	case types.HexLiteral:
		b, err := v.Bytes()
		if err != nil {
			return nil
		}
		return b
	case types.StreamDict:
		sd := v
		if err := sd.Decode(); err != nil {
			return nil
		}
		return sd.Content
	default:
		if sd, _, err := xrd.DereferenceStreamDict(o); err == nil && sd != nil {
			if err := sd.Decode(); err != nil {
				return nil
			}
			return sd.Content
		}
		return nil
	}
}

func samplesToImage(width, height, bpc int, cs string, ncomp int, raw []byte, dec []float64, indexed []byte, indexedN int) image.Image {
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
			case "indexed":
				if indexedN < 1 || len(indexed) < indexedN {
					continue
				}
				idx := int(pix[0])
				off := idx * indexedN
				if off < 0 || off+indexedN > len(indexed) {
					continue
				}
				if indexedN == 1 {
					v := indexed[off]
					r, g, b = v, v, v
				} else if indexedN >= 3 {
					r, g, b = indexed[off], indexed[off+1], indexed[off+2]
				}
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
