package gopdf2

import "math"

// dropWhiteTextCovers removes a near-white rectangle painted on top of a
// text line. The WPS subtitle is one complete a:t run, and five white
// rectangles the size of "创造" and "不简单" sit on top of it, so PowerPoint
// shows "简单" and clips the rest. A white plate painted before the text
// is a background and stays.
func dropWhiteTextCovers(p *Page) {
	var texts []*TextObject
	for i := range p.Elements {
		el := &p.Elements[i]
		if el.Text != nil && stringsTrim(el.Text.Content) != "" {
			texts = append(texts, el.Text)
			continue
		}
		sh := el.Shape
		if sh == nil || !sh.SimpleRect || !sh.HasFill || sh.HasStroke {
			continue
		}
		if sh.FillR < 250 || sh.FillG < 250 || sh.FillB < 250 {
			continue
		}
		if sh.FillAlpha > 0 && sh.FillAlpha < 0.95 {
			continue
		}
		for _, t := range texts {
			if whiteCoversText(sh, t) {
				el.Shape = nil
				break
			}
		}
	}
	kept := p.Elements[:0]
	for _, el := range p.Elements {
		if el.Shape == nil && el.Text == nil && el.Image == nil {
			continue
		}
		kept = append(kept, el)
	}
	p.Elements = kept
}

func stringsTrim(s string) string {
	i := 0
	j := len(s)
	for i < j && (s[i] == ' ' || s[i] == '\n' || s[i] == '\t') {
		i++
	}
	for j > i && (s[j-1] == ' ' || s[j-1] == '\n' || s[j-1] == '\t') {
		j--
	}
	return s[i:j]
}

func whiteCoversText(sh *ShapeObject, t *TextObject) bool {
	ix := math.Min(sh.X+sh.Width, t.X+t.Width) - math.Max(sh.X, t.X)
	iy := math.Min(sh.Y+sh.Height, t.Y+t.Height) - math.Max(sh.Y, t.Y)
	if ix < 4 || iy < 4 {
		return false
	}
	// A card behind a short label is wider than the text. These plates
	// cover part of a longer run.
	if t.Width < sh.Width*0.98 {
		return false
	}
	return iy > math.Min(sh.Height, t.Height)*0.45
}
