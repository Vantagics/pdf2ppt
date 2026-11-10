package goppt

import "testing"

func TestDominantSizeTieUsesFirstPage(t *testing.T) {
	p := NewPresentation()
	p.AddSlide(10, 20)
	p.AddSlide(30, 40)
	p.AddSlide(10, 20)
	p.AddSlide(30, 40)
	w, h := p.slideSize()
	if w != 10 || h != 20 {
		t.Fatalf("tie should keep the first page, got %d x %d", w, h)
	}
	q := NewPresentation()
	q.AddSlide(10, 20)
	q.AddSlide(30, 40)
	q.AddSlide(30, 40)
	w, h = q.slideSize()
	if w != 30 || h != 40 {
		t.Fatalf("majority should win, got %d x %d", w, h)
	}
}

func TestLetterboxScalesDownAndCenters(t *testing.T) {
	p := NewPresentation()
	// Counts tie, so the first page is the slide size. Both axes are
	// above the OOXML minimum (914400 EMU).
	p.AddSlide(2000000, 2000000)
	odd := p.AddSlide(1000000, 4000000)
	odd.HasBackground = true
	odd.BgR, odd.BgG, odd.BgB = 9, 8, 7
	if err := odd.AddTextBox(&TextBox{
		Content: "A", FontSize: 20, X: 10000, Y: 20000, Width: 30000, Height: 40000,
		StrokeWidth: 4, Tracking: 2,
	}); err != nil {
		t.Fatal(err)
	}
	if err := odd.AddShape(&Shape{
		X: 0, Y: 0, Width: 1000000, Height: 4000000,
		StrokeWidth: 100,
		Path:        []PathOp{{Op: "L", X: 1000000, Y: 4000000, X1: 10000, Y1: 20000, X2: 30000, Y2: 40000}},
	}); err != nil {
		t.Fatal(err)
	}
	p.fitSlides()

	s, err := p.GetSlide(1)
	if err != nil {
		t.Fatal(err)
	}
	if s.Width != 2000000 || s.Height != 2000000 {
		t.Fatalf("slide size = %d x %d", s.Width, s.Height)
	}
	if s.HasBackground {
		t.Fatal("page background should be the centered rectangle, not the whole slide")
	}
	bg, ok := s.Elements[0].(*Shape)
	if !ok || !bg.SimpleRect || bg.FillR != 9 {
		t.Fatalf("missing letterbox page rect: %#v", s.Elements[0])
	}
	// 1000000x4000000 into 2000000x2000000 scales by 0.5 to 500000x2000000, centered at x=750000.
	if bg.X != 750000 || bg.Y != 0 || bg.Width != 500000 || bg.Height != 2000000 {
		t.Fatalf("page rect = %d,%d %dx%d", bg.X, bg.Y, bg.Width, bg.Height)
	}
	tb := s.Elements[1].(*TextBox)
	if tb.X != 755000 || tb.Y != 10000 || tb.Width != 15000 || tb.Height != 20000 {
		t.Fatalf("text box = %d,%d %dx%d", tb.X, tb.Y, tb.Width, tb.Height)
	}
	if tb.FontSize != 10 || tb.StrokeWidth != 2 || tb.Tracking != 1 {
		t.Fatalf("text scale font=%v stroke=%v tracking=%v", tb.FontSize, tb.StrokeWidth, tb.Tracking)
	}
	sh := s.Elements[2].(*Shape)
	if sh.X != 750000 || sh.Y != 0 || sh.Width != 500000 || sh.Height != 2000000 || sh.StrokeWidth != 50 {
		t.Fatalf("shape = %d,%d %dx%d stroke %d", sh.X, sh.Y, sh.Width, sh.Height, sh.StrokeWidth)
	}
	op := sh.Path[0]
	if op.X != 500000 || op.Y != 2000000 || op.X1 != 5000 || op.Y1 != 10000 || op.X2 != 15000 || op.Y2 != 20000 {
		t.Fatalf("path not scaled: %+v", op)
	}

	// A page smaller in both axes is centered and not enlarged.
	small := NewPresentation()
	small.AddSlide(2000000, 2000000)
	page := small.AddSlide(1000000, 1000000)
	if err := page.AddShape(&Shape{X: 0, Y: 0, Width: 1000000, Height: 1000000, SimpleRect: true}); err != nil {
		t.Fatal(err)
	}
	small.fitSlides()
	got, _ := small.GetSlide(1)
	box := got.Elements[0].(*Shape)
	if box.Width != 1000000 || box.Height != 1000000 || box.X != 500000 || box.Y != 500000 {
		t.Fatalf("smaller page was not centered at native size: %d,%d %dx%d", box.X, box.Y, box.Width, box.Height)
	}
}
