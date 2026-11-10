package gopdf2

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
	"strings"
	"testing"
)

func TestUnmatteBlack(t *testing.T) {
	// Stored 115 at alpha 128 against a black matte is straight 230.
	r, g, b := unmatte(115, 122, 130, 128, []float64{0, 0, 0})
	if r < 220 || g < 230 || b < 250 {
		t.Fatalf("unmatte = %d %d %d, want a light color", r, g, b)
	}
	r, g, b = unmatte(115, 122, 130, 128, nil)
	if r != 115 || g != 122 || b != 130 {
		t.Fatalf("nil matte changed the color to %d %d %d", r, g, b)
	}
}

func TestFlattenKeepsLightWatermark(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	r, g, b := unmatte(115, 122, 130, 128, []float64{0, 0, 0})
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			src.SetNRGBA(x, y, color.NRGBA{R: r, G: g, B: b, A: 128})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatal(err)
	}
	flat, err := flattenOn(buf.Bytes(), 255, 255, 255)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(flat))
	if err != nil {
		t.Fatal(err)
	}
	rr, gg, bb, aa := img.At(0, 0).RGBA()
	if aa>>8 != 255 || rr>>8 < 230 || gg>>8 < 230 || bb>>8 < 230 {
		t.Fatalf("flattened pixel = %d %d %d a=%d", rr>>8, gg>>8, bb>>8, aa>>8)
	}
}

func TestRotatePageKeepsTextUpright(t *testing.T) {
	p := &Page{Width: 612, Height: 792, Elements: []PageElement{{
		Text: &TextObject{
			Content: "MIT OpenCourseWare", X: 23.39, Y: 643.60,
			Width: 129.43, Height: 15.33, Rotation: -90,
		},
	}}}
	applyRotation(p, 90)
	if math.Abs(p.Width-792) > 0.1 || math.Abs(p.Height-612) > 0.1 {
		t.Fatalf("page size %.2f x %.2f", p.Width, p.Height)
	}
	tb := p.Elements[0].Text
	if tb.Rotation != 0 {
		t.Fatalf("rotation = %v", tb.Rotation)
	}
	if tb.Width < tb.Height {
		t.Fatalf("text box still tall: %.2f x %.2f", tb.Width, tb.Height)
	}
	if math.Abs(tb.X-76.02) > 0.3 || math.Abs(tb.Y-80.44) > 0.3 {
		t.Fatalf("text at %.2f,%.2f", tb.X, tb.Y)
	}
}

func TestMergeHyphenatedLine(t *testing.T) {
	p := &Page{Elements: []PageElement{
		{Text: &TextObject{Content: "post", X: 62.25, Y: 242.82, Width: 198.48, Height: 22.61, FontSize: 18.73, FontName: "Noto Sans"}},
		{Text: &TextObject{Content: "-", X: 259.27, Y: 238.84, Width: 12.35, Height: 27.41, FontSize: 22.71, FontName: "Noto Sans"}},
		{Text: &TextObject{Content: "it note.", X: 266.52, Y: 242.29, Width: 80, Height: 23.25, FontSize: 19.26, FontName: "Noto Sans"}},
		{Text: &TextObject{Content: "Jossey", X: 21.23, Y: 504.35, Width: 276.02, Height: 11.94, FontSize: 9.90, FontName: "Noto Sans"}},
		{Text: &TextObject{Content: "-", X: 296.68, Y: 504.14, Width: 4.91, Height: 12.20, FontSize: 10.11, FontName: "Noto Sans"}},
		{Text: &TextObject{Content: "Bass.", X: 300.80, Y: 504.38, Width: 40, Height: 11.91, FontSize: 9.87, FontName: "Noto Sans"}},
		{Text: &TextObject{Content: "top ", X: 242.38, Y: 314.72, Width: 320.19, Height: 22.85, FontSize: 18.93, FontName: "Noto Sans"}},
		{Text: &TextObject{Content: "laning", X: 561.08, Y: 315.43, Width: 54.56, Height: 21.99, FontSize: 18.22, FontName: "Noto Sans"}},
		{Text: &TextObject{Content: "alone", X: 900, Y: 100, Width: 40, Height: 20, FontSize: 18, FontName: "Noto Sans"}},
	}}
	mergeTextLines(p)
	var got []string
	for _, el := range p.Elements {
		if el.Text != nil {
			got = append(got, el.Text.Content)
		}
	}
	joined := strings.Join(got, " | ")
	for _, want := range []string{"post-it note.", "Jossey-Bass.", "top laning"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in %s", want, joined)
		}
	}
	if strings.Contains(joined, "post -") || strings.Contains(joined, "Jossey -") || strings.Contains(joined, "top  ") {
		t.Fatalf("gap survived: %s", joined)
	}
	if !strings.Contains(joined, "alone") {
		t.Fatalf("distant run was swallowed: %s", joined)
	}
}

func TestMITSlidePaint(t *testing.T) {
	doc, err := Open("/tmp/mit-ses1.pdf")
	if err != nil {
		t.Skip(err)
	}
	p, err := doc.GetPage(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Images) == 0 {
		t.Fatal("no background image")
	}
	img, err := png.Decode(bytes.NewReader(p.Images[0].Data))
	if err != nil {
		t.Fatal(err)
	}
	rr, gg, bb, aa := img.At(img.Bounds().Dx()/2, img.Bounds().Dy()/2).RGBA()
	if aa>>8 != 255 || rr>>8 < 220 || gg>>8 < 220 || bb>>8 < 220 {
		t.Fatalf("background pixel %d %d %d a=%d", rr>>8, gg>>8, bb>>8, aa>>8)
	}
	var text string
	for _, el := range p.Elements {
		if el.Text != nil {
			text += el.Text.Content + "\n"
		}
	}
	for _, want := range []string{"post-it", "top laning", "Jossey-Bass"} {
		if !strings.Contains(text, want) {
			t.Fatalf("page 1 missing %q\n%s", want, text)
		}
	}
	if strings.Contains(text, "post -it") || strings.Contains(text, "Jossey -Bass") || strings.Contains(text, "top  laning") {
		t.Fatalf("split words remain:\n%s", text)
	}
	last, err := doc.GetPage(22)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(last.Width-792) > 1 || math.Abs(last.Height-612) > 1 {
		t.Fatalf("page 23 size %.1f x %.1f", last.Width, last.Height)
	}
	var found bool
	for _, el := range last.Elements {
		if el.Text == nil || !strings.Contains(el.Text.Content, "MIT OpenCourseWare") {
			continue
		}
		found = true
		if el.Text.Rotation != 0 || el.Text.Width < el.Text.Height {
			t.Fatalf("courseware text rot=%.1f box %.1fx%.1f at %.1f,%.1f", el.Text.Rotation, el.Text.Width, el.Text.Height, el.Text.X, el.Text.Y)
		}
	}
	if !found {
		t.Fatal("page 23 lost the courseware line")
	}
}
