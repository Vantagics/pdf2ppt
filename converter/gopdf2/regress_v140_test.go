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
	var wide float64
	grads := 0
	for _, el := range p.Elements {
		if el.Image != nil && el.Image.Y < 5 && el.Image.Width > wide {
			wide = el.Image.Width
		}
		if el.Shape != nil && el.Shape.Gradient != nil && len(el.Shape.Path) > 10 {
			grads++
		}
	}
	// v1.3.9 cropped this photo from 1579pt down to the page and cut the diamonds.
	if wide < 1500 {
		t.Fatalf("background width %.0f was cropped", wide)
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
