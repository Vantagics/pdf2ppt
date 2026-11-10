package goppt

import "math"

// fitSlides picks one slide size and places every other page inside it.
// The size is the most common page. Ties keep the earlier page. A page
// that does not match is scaled down uniformly so both sides fit, then
// centered. A smaller page is not enlarged. Its background becomes the
// centered page rectangle, so the margin stays a letterbox. A taller
// scan page is fitted inside too: cropping it cut off the logo.
func (p *Presentation) fitSlides() {
	if len(p.slides) == 0 {
		return
	}
	w, h := p.slideSize()
	w, h = clampSlideSize(w, h)
	for _, s := range p.slides {
		s.fitInto(w, h)
	}
}

func (s *Slide) fitInto(dw, dh int64) {
	if s.Width == dw && s.Height == dh {
		return
	}
	pw, ph := s.Width, s.Height
	if pw < 1 {
		pw = 1
	}
	if ph < 1 {
		ph = 1
	}
	scale := 1.0
	// Fit the whole page inside the slide. Cropping a slightly taller
	// scan (OICW page 1) cut the logo and the banner.
	if pw > dw {
		scale = math.Min(scale, float64(dw)/float64(pw))
	}
	if ph > dh {
		scale = math.Min(scale, float64(dh)/float64(ph))
	}
	sw := float64(pw) * scale
	sh := float64(ph) * scale
	ox := (float64(dw) - sw) / 2
	oy := (float64(dh) - sh) / 2

	var bg *Shape
	if s.BgGradient != nil {
		bg = &Shape{HasFill: true, Gradient: s.BgGradient, SimpleRect: true}
		s.BgGradient = nil
	} else if s.HasBackground {
		bg = &Shape{
			HasFill: true, SimpleRect: true,
			FillR: s.BgR, FillG: s.BgG, FillB: s.BgB,
		}
		s.HasBackground = false
	}
	for _, el := range s.Elements {
		scaleElement(el, scale, ox, oy)
	}
	if bg != nil {
		bg.X = roundEMU(ox)
		bg.Y = roundEMU(oy)
		bg.Width = roundEMU(sw)
		bg.Height = roundEMU(sh)
		if bg.Width < 1 {
			bg.Width = 1
		}
		if bg.Height < 1 {
			bg.Height = 1
		}
		s.Elements = append([]Element{bg}, s.Elements...)
	}
	s.Width, s.Height = dw, dh
}

func scaleElement(el Element, scale, ox, oy float64) {
	switch e := el.(type) {
	case *TextBox:
		e.X, e.Y, e.Width, e.Height = mapBox(e.X, e.Y, e.Width, e.Height, scale, ox, oy)
		e.FontSize *= scale
		e.StrokeWidth *= scale
		e.Tracking *= scale
		for i := range e.Runs {
			e.Runs[i].FontSize *= scale
			e.Runs[i].Tracking *= scale
		}
	case *Image:
		e.X, e.Y, e.Width, e.Height = mapBox(e.X, e.Y, e.Width, e.Height, scale, ox, oy)
	case *Shape:
		e.X, e.Y, e.Width, e.Height = mapBox(e.X, e.Y, e.Width, e.Height, scale, ox, oy)
		e.StrokeWidth = roundEMU(float64(e.StrokeWidth) * scale)
		for i := range e.Path {
			e.Path[i].X = roundEMU(float64(e.Path[i].X) * scale)
			e.Path[i].Y = roundEMU(float64(e.Path[i].Y) * scale)
			e.Path[i].X1 = roundEMU(float64(e.Path[i].X1) * scale)
			e.Path[i].Y1 = roundEMU(float64(e.Path[i].Y1) * scale)
			e.Path[i].X2 = roundEMU(float64(e.Path[i].X2) * scale)
			e.Path[i].Y2 = roundEMU(float64(e.Path[i].Y2) * scale)
		}
	}
}

func mapBox(x, y, w, h int64, scale, ox, oy float64) (int64, int64, int64, int64) {
	nw := roundEMU(float64(w) * scale)
	nh := roundEMU(float64(h) * scale)
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	return roundEMU(ox + float64(x)*scale), roundEMU(oy + float64(y)*scale), nw, nh
}

func roundEMU(v float64) int64 {
	return int64(math.Round(v))
}
