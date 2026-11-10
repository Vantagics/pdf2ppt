package gopdf2

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
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

func TestTightWordSpaceIsKept(t *testing.T) {
	// 0.20 em is a real word space ("Getty" + "images"). A negative gap is
	// Canva's per-letter boxes and must not grow a space.
	p := &Page{Elements: []PageElement{
		{Text: &TextObject{Content: "Getty", X: 70, Y: 156, Width: 44, Height: 22, FontSize: 18, FontName: "Arimo"}},
		{Text: &TextObject{Content: "images", X: 118, Y: 156, Width: 59, Height: 22, FontSize: 18, FontName: "Arimo"}},
		{Text: &TextObject{Content: "Recogni", X: 10, Y: 10, Width: 80, Height: 30, FontSize: 30, FontName: "Noto Sans"}},
		{Text: &TextObject{Content: "zed", X: 88, Y: 12, Width: 40, Height: 30, FontSize: 30, FontName: "Noto Sans"}},
	}}
	mergeTextLines(p)
	var got []string
	for _, el := range p.Elements {
		if el.Text != nil {
			got = append(got, el.Text.Content)
		}
	}
	joined := strings.Join(got, "|")
	if !strings.Contains(joined, "Getty images") {
		t.Fatalf("missing word space: %s", joined)
	}
	if strings.Contains(joined, "Recogni zed") {
		t.Fatalf("split a word: %s", joined)
	}
	if !strings.Contains(joined, "Recognized") {
		t.Fatalf("letters not joined: %s", joined)
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

func TestZeroAlphaFillIsNotPainted(t *testing.T) {
	// Google Slides draws a black rect with ca 0, then clips an image to it.
	content := "q /A0 gs 0 0 0 rg 0 0 100 50 re f Q 1 0 0 rg 10 10 20 10 re f\n"
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Count 1 /Kids [3 0 R] >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 100 50] /Resources << /ExtGState << /A0 4 0 R >> >> /Contents 5 0 R >>",
		"<< /Type /ExtGState /ca 0 /CA 0 >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content),
	}
	p := openPDF(t, pdfBytes(objs))
	for _, el := range p.Elements {
		if el.Shape != nil && el.Shape.FillR == 0 && el.Shape.FillG == 0 && el.Shape.FillB == 0 && el.Shape.HasFill {
			t.Fatalf("transparent black rect was painted: %+v", el.Shape)
		}
	}
	var red bool
	for _, el := range p.Elements {
		if el.Shape != nil && el.Shape.FillR == 255 && el.Shape.HasFill {
			red = true
		}
	}
	if !red {
		t.Fatal("opaque red rect missing")
	}
}

func TestFrameIsNotASolidBackground(t *testing.T) {
	// Outer page rect plus an inner subpath. The hole must stay a shape,
	// not a solid gray page.
	content := "1 1 1 rg 0 0 200 100 re f 0.46 0.50 0.61 rg 0 0 m 200 0 l 200 100 l 0 100 l h 20 20 m 180 20 l 180 80 l 20 80 l h f\n"
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Count 1 /Kids [3 0 R] >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content),
	}
	p := openPDF(t, pdfBytes(objs))
	if p.BgR < 250 || p.BgG < 250 || p.BgB < 250 {
		t.Fatalf("background should stay the white rect, got %d,%d,%d", p.BgR, p.BgG, p.BgB)
	}
	var frame bool
	for _, el := range p.Elements {
		if el.Shape != nil && el.Shape.HasFill && len(el.Shape.Path) > 4 {
			frame = true
		}
	}
	if !frame {
		t.Fatal("frame with a hole was dropped")
	}
}

func openPDF(t *testing.T, b []byte) *Page {
	t.Helper()
	path := filepath.Join(t.TempDir(), "t.pdf")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	p, err := doc.GetPage(0)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func pdfBytes(objs []string) []byte {
	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")
	offs := make([]int, len(objs)+1)
	for i, body := range objs {
		offs[i+1] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", i+1, body)
	}
	xref := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n", len(objs)+1)
	buf.WriteString("0000000000 65535 f \n")
	for i := 1; i <= len(objs); i++ {
		fmt.Fprintf(&buf, "%010d 00000 n \n", offs[i])
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objs)+1, xref)
	return buf.Bytes()
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

func TestBakeWhiteLogoOntoBackground(t *testing.T) {
	bg := image.NewNRGBA(image.Rect(0, 0, 20, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 20; x++ {
			bg.SetNRGBA(x, y, color.NRGBA{R: 0, G: 40, B: 140, A: 255})
		}
	}
	logo := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			logo.SetNRGBA(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
		}
	}
	logo.SetNRGBA(0, 0, color.NRGBA{A: 0})
	var bgBuf, logoBuf bytes.Buffer
	if err := png.Encode(&bgBuf, bg); err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(&logoBuf, logo); err != nil {
		t.Fatal(err)
	}
	p := &Page{Elements: []PageElement{
		{Image: &ImageObject{Data: bgBuf.Bytes(), Format: "png", X: 0, Y: 0, Width: 20, Height: 10}},
		{Image: &ImageObject{Data: logoBuf.Bytes(), Format: "png", X: 4, Y: 2, Width: 4, Height: 4}},
	}}
	bakeAlphaOverlays(p)
	if len(p.Elements) != 1 {
		t.Fatalf("overlay still separate: %d elements", len(p.Elements))
	}
	img, err := png.Decode(bytes.NewReader(p.Elements[0].Image.Data))
	if err != nil {
		t.Fatal(err)
	}
	// Logo covers x 4..8 of 20 px, so pixel x=5 should be white.
	r, g, b, _ := img.At(5, 3).RGBA()
	if r>>8 < 240 || g>>8 < 240 || b>>8 < 240 {
		t.Fatalf("baked pixel %d,%d,%d", r>>8, g>>8, b>>8)
	}
}

func TestDeviceGrayIsNeutral(t *testing.T) {
	cases := []struct {
		name    string
		content string
		cs      string
		extra   []string
		want    byte
		stroke  bool
	}{
		{name: "g", content: "0.25 g 0 0 30 20 re f\n", want: 64},
		{name: "G", content: "0.50 G 2 w 0 0 30 20 re S\n", want: 128, stroke: true},
		{name: "cs", content: "/DeviceGray cs 0.25 sc 0 0 30 20 re f\n", want: 64},
		{name: "named", content: "/DG cs 0.50 sc 0 0 30 20 re f\n", cs: "/DG /DeviceGray", want: 128},
		{name: "calgray", content: "/CG cs 0 sc 0 0 30 20 re f\n", cs: "/CG [/CalGray << /WhitePoint [0.9505 1 1.089] >>]", want: 0},
		{name: "icc", content: "/IG cs 1 sc 0 0 30 20 re f\n", cs: "/IG [/ICCBased 5 0 R]", extra: []string{"<< /N 1 /Length 4 >>\nstream\nxxxx\nendstream"}, want: 255},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := "<< >>"
			if tc.cs != "" {
				res = "<< /ColorSpace << " + tc.cs + " >> >>"
			}
			objs := []string{
				"<< /Type /Catalog /Pages 2 0 R >>",
				"<< /Type /Pages /Count 1 /Kids [3 0 R] >>",
				fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 40 30] /Resources %s /Contents 4 0 R >>", res),
				fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(tc.content), tc.content),
			}
			objs = append(objs, tc.extra...)
			p := openPDF(t, pdfBytes(objs))
			var gotR, gotG, gotB uint8
			var found bool
			for _, el := range p.Elements {
				if el.Shape == nil {
					continue
				}
				if tc.stroke && el.Shape.HasStroke {
					gotR, gotG, gotB = el.Shape.StrokeR, el.Shape.StrokeG, el.Shape.StrokeB
					found = true
				}
				if !tc.stroke && el.Shape.HasFill {
					gotR, gotG, gotB = el.Shape.FillR, el.Shape.FillG, el.Shape.FillB
					found = true
				}
			}
			if !tc.stroke && p.HasBackground && !found {
				gotR, gotG, gotB = p.BgR, p.BgG, p.BgB
				found = true
			}
			if !found {
				t.Fatal("no painted color")
			}
			if gotR != gotG || gotG != gotB {
				t.Fatalf("gray became RGB %d,%d,%d", gotR, gotG, gotB)
			}
			if absDiff(gotR, tc.want) > 2 {
				t.Fatalf("level %d, want %d", gotR, tc.want)
			}
		})
	}
}

func absDiff(a, b byte) byte {
	if a > b {
		return a - b
	}
	return b - a
}
