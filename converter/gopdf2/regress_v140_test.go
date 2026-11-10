package gopdf2

import (
	"os"
	"testing"
)

func TestCanvaScriptWordmarkIsDrawn(t *testing.T) {
	if _, err := os.Stat("/tmp/corpus/canva.pdf"); err != nil {
		t.Skip("canva pdf not present")
	}
	doc, err := Open("/tmp/corpus/canva.pdf")
	if err != nil {
		t.Fatal(err)
	}
	p, err := doc.GetPage(0)
	if err != nil {
		t.Fatal(err)
	}
	var right float64
	grads := 0
	for _, el := range p.Elements {
		if el.Image != nil && el.Image.Y < 5 {
			if edge := el.Image.X + el.Image.Width; edge > right {
				right = edge
			}
		}
		if el.Shape != nil && el.Shape.Gradient != nil && len(el.Shape.Path) > 10 {
			grads++
		}
	}
	// The page clip may trim only what hangs off the slide. The diamonds
	// reach the right edge of the page.
	if right < p.Width-2 {
		t.Fatalf("background right edge %.0f stops short of the page %.0f", right, p.Width)
	}
	if grads < 1 {
		t.Fatal("script wordmark gradient was not painted")
	}
}

func TestUAIPicturesAreNotClippedAway(t *testing.T) {
	if _, err := os.Stat("/tmp/corpus/uai.pdf"); err != nil {
		t.Skip("uai pdf not present")
	}
	doc, err := Open("/tmp/corpus/uai.pdf")
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for i := 0; i < doc.PageCount(); i++ {
		p, err := doc.GetPage(i)
		if err != nil {
			t.Fatal(err)
		}
		for _, el := range p.Elements {
			if el.Image != nil {
				n++
			}
		}
	}
	// v1.3.7 had 253. v1.3.9's clip dropped the ones that missed the clip rect.
	if n < 253 {
		t.Fatalf("pictures %d, v1.3.7 had 253", n)
	}
}
