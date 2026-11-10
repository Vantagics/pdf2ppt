package gopdf2

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
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

func TestClipKeepsJPEGBytes(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 16, 16))
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, src, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatal(err)
	}
	raw := append([]byte(nil), buf.Bytes()...)
	ip := &interp{pageW: 100, pageH: 100}
	ip.elements = []PageElement{{Image: &ImageObject{
		Data: raw, Format: "jpeg", X: -10, Y: 10, Width: 50, Height: 40,
	}}}
	// Page clip: only the part that hangs off the slide is trimmed, and the
	// bytes stay the original JPEG.
	ip.clipLastImage(0, 0, 100, 100, false)
	img := ip.elements[0].Image
	if img.Format != "jpeg" || !bytes.Equal(img.Data, raw) {
		t.Fatalf("format %s len %d, jpeg was re-encoded", img.Format, len(img.Data))
	}
	if img.CropL < 0.15 || img.CropL > 0.25 {
		t.Fatalf("CropL = %v, want about 0.2", img.CropL)
	}
	if img.X < -0.1 || img.X > 0.1 {
		t.Fatalf("cropped X = %v, want 0", img.X)
	}
}

func TestShapeBoxHoldsHandlesClipBoxDoesNot(t *testing.T) {
	// Endpoints sit on y=0. The handles are at y=40. The curve itself peaks
	// at 30, which is what a clip should use. The shape box has to contain
	// the handles or DrawingML clamps them flat.
	ops := []pathOp{{op: 'M', x: 0, y: 0}, {op: 'C', x: 10, y: 0, x1: 0, y1: 40, x2: 10, y2: 40}}
	_, y0, _, y1, ok := pathBounds(ops)
	if !ok || y0 > 0.1 || y1 < 39 {
		t.Fatalf("shape bounds y %.2f..%.2f", y0, y1)
	}
	_, cy0, _, cy1, ok := clipBounds(ops)
	if !ok || cy1 > 31 || cy1 < 29 || cy0 < -0.1 {
		t.Fatalf("clip bounds y %.2f..%.2f", cy0, cy1)
	}
}

func TestOnePercentClipDoesNotMoveThePicture(t *testing.T) {
	ip := &interp{pageW: 200, pageH: 200}
	ip.elements = []PageElement{{Image: &ImageObject{
		Data: []byte{1}, Format: "png", X: 0, Y: 0, Width: 100, Height: 80,
	}}}
	ip.clipLastImage(1, 0, 100, 80, false)
	img := ip.elements[0].Image
	if img.CropL != 0 || math.Abs(img.Width-100) > 0.1 || math.Abs(img.X) > 0.1 {
		t.Fatalf("1%% clip moved the picture: x %.2f w %.2f cropL %.3f", img.X, img.Width, img.CropL)
	}
}

func TestDirectShadingBecomesCard(t *testing.T) {
	content := "10 10 80 40 re W n /Sh0 sh\n"
	p := openPDF(t, pdfBytes([]string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Count 1 /Kids [3 0 R] >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Resources << /Shading << /Sh0 5 0 R >> >> /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content),
		"<< /ShadingType 2 /ColorSpace /DeviceRGB /Coords [0 0 0 100] /Function << /FunctionType 2 /Domain [0 1] /C0 [0.22 0.22 0.22] /C1 [0 0 0] /N 1 >> >>",
	}))
	var card *ShapeObject
	for _, el := range p.Elements {
		if el.Shape != nil && el.Shape.Gradient != nil {
			card = el.Shape
		}
	}
	if card == nil {
		t.Fatal("shading was not painted")
	}
	if math.Abs(card.Width-80) > 1 || math.Abs(card.Height-40) > 1 {
		t.Fatalf("card box %.1fx%.1f", card.Width, card.Height)
	}
	if len(card.Gradient.Stops) < 2 || card.Gradient.Stops[0].R < 50 || card.Gradient.Stops[len(card.Gradient.Stops)-1].R > 5 {
		t.Fatalf("stops %+v", card.Gradient.Stops)
	}
	// t=0 is the bottom (y-up). DrawingML direction toward black is upward, 270°.
	if math.Abs(card.Gradient.Angle-270) > 1 {
		t.Fatalf("angle %v", card.Gradient.Angle)
	}
}

func TestNBSCardsAreGradients(t *testing.T) {
	doc, err := Open("/tmp/corpus/nbs-alibaba.pdf")
	if err != nil {
		t.Skip(err)
	}
	p, err := doc.GetPage(3)
	if err != nil {
		t.Fatal(err)
	}
	var cards int
	for _, el := range p.Elements {
		sh := el.Shape
		if sh == nil || sh.Gradient == nil || sh.Width < 400 || sh.Height < 200 {
			continue
		}
		cards++
		if sh.Gradient.Stops[0].R < 40 {
			t.Fatalf("card starts at %d, want the gray panel", sh.Gradient.Stops[0].R)
		}
	}
	if cards != 4 {
		t.Fatalf("gradient cards = %d, want 4", cards)
	}
}

func TestSubscriptJoinsBelowALetter(t *testing.T) {
	body := &TextObject{Content: "p = p", FontName: "Tinos", FontSize: 32, X: 100, Y: 100, Width: 80, Height: 36}
	sub := &TextObject{Content: "2", FontName: "Tinos", FontSize: 16, X: 108, Y: 124, Width: 10, Height: 18}
	year := &TextObject{Content: "2024", FontName: "Tinos", FontSize: 32, X: 200, Y: 100, Width: 60, Height: 36}
	p := &Page{Elements: []PageElement{{Text: body}, {Text: sub}, {Text: year}}}
	attachSubscripts(p)
	if len(p.TextObjects) != 0 && len(p.Elements) != 2 {
		// TextObjects is filled by syncLists; Elements is what attach edits.
	}
	if len(p.Elements) != 2 {
		t.Fatalf("elements %d, want body+year (subscript folded in)", len(p.Elements))
	}
	if body.Content != "p2 = p" || year.Content != "2024" {
		t.Fatalf("body %q year %q", body.Content, year.Content)
	}
	var base float64
	for _, r := range body.Runs {
		if r.Content == "2" {
			base = r.Baseline
		}
	}
	if base > -18000 || base < -32000 {
		t.Fatalf("baseline %v, want about -25000", base)
	}
}

func TestPsumacLockIsOutline(t *testing.T) {
	doc, err := Open("/tmp/corpus/psumac.pdf")
	if err != nil {
		t.Skip(err)
	}
	p, err := doc.GetPage(29)
	if err != nil {
		t.Fatal(err)
	}
	var dollar bool
	for _, tObj := range p.TextObjects {
		if strings.TrimSpace(tObj.Content) == "$" && tObj.NoFill {
			dollar = true
		}
	}
	if !dollar {
		t.Fatal("lock character left a:t; it must stay as noFill text")
	}
	var shapes int
	for _, el := range p.Elements {
		if el.Shape != nil && el.Shape.Width > 200 && el.Shape.Height > 200 && len(el.Shape.Path) > 4 {
			shapes++
		}
	}
	if shapes == 0 {
		t.Fatal("no large outline for the lock")
	}
}

func TestNBSLogoKeepsJPEG(t *testing.T) {
	doc, err := Open("/tmp/corpus/nbs-alibaba.pdf")
	if err != nil {
		t.Skip(err)
	}
	p, err := doc.GetPage(3)
	if err != nil {
		t.Fatal(err)
	}
	var jpeg int
	for _, im := range p.Images {
		if im.Format == "jpeg" && im.Width > 40 && im.Width < 80 {
			jpeg++
		}
	}
	if jpeg > 0 && jpeg < 10 {
		t.Fatalf("logo jpegs %d", jpeg)
	}
	var pngLogos int
	for _, im := range p.Images {
		if im.Format == "png" && im.Width > 40 && im.Width < 80 && len(im.Data) > 2000 {
			pngLogos++
		}
	}
	if pngLogos < 10 {
		t.Fatalf("crisp logo pngs %d", pngLogos)
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
	img, _, err := image.Decode(bytes.NewReader(p.Images[0].Data))
	if err != nil {
		t.Fatal(err)
	}
	if p.Images[0].Format != "jpeg" {
		t.Fatalf("page photo format %s, want jpeg", p.Images[0].Format)
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

func TestLightWordmarkStaysAPicture(t *testing.T) {
	bg := image.NewNRGBA(image.Rect(0, 0, 20, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 20; x++ {
			bg.SetNRGBA(x, y, color.NRGBA{R: 0, G: 40, B: 140, A: 255})
		}
	}
	logo := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			logo.SetNRGBA(x, y, color.NRGBA{A: 0})
		}
	}
	for y := 2; y < 6; y++ {
		for x := 2; x < 6; x++ {
			logo.SetNRGBA(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
		}
	}
	var bgBuf, logoBuf bytes.Buffer
	if err := png.Encode(&bgBuf, bg); err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(&logoBuf, logo); err != nil {
		t.Fatal(err)
	}
	p := &Page{Elements: []PageElement{
		{Image: &ImageObject{Data: bgBuf.Bytes(), Format: "png", X: 0, Y: 0, Width: 20, Height: 10}},
		{Image: &ImageObject{Data: logoBuf.Bytes(), Format: "png", X: 4, Y: 2, Width: 8, Height: 8}},
	}}
	opacifyLightMasks(p)
	if len(p.Elements) != 2 {
		t.Fatalf("wordmark picture was dropped: %d elements", len(p.Elements))
	}
	img, err := png.Decode(bytes.NewReader(p.Elements[1].Image.Data))
	if err != nil {
		t.Fatal(err)
	}
	// Letter core is white and opaque. A clear pixel shows the blue behind it.
	lr, lg, lb, la := img.At(3, 3).RGBA()
	if la>>8 != 255 || lr>>8 < 240 || lg>>8 < 240 || lb>>8 < 240 {
		t.Fatalf("letter pixel %d,%d,%d a=%d", lr>>8, lg>>8, lb>>8, la>>8)
	}
	cr, cg, cb, ca := img.At(0, 0).RGBA()
	if ca>>8 != 255 || cb>>8 < 100 || cr>>8 > 40 {
		t.Fatalf("clear pixel should be the blue backdrop, got %d,%d,%d a=%d", cr>>8, cg>>8, cb>>8, ca>>8)
	}
}

func TestTwoImagesAreBothKept(t *testing.T) {
	img := bytes.Repeat([]byte{10, 20, 200}, 16)
	stream := "<<" + "/Type /XObject /Subtype /Image /Width 4 /Height 4 /ColorSpace /DeviceRGB /BitsPerComponent 8 /Length 48 >>\nstream\n" + string(img) + "\nendstream"
	content := "q\n40 0 0 30 0 0 cm\n/Im0 Do\nQ\nq\n8 0 0 8 4 4 cm\n/Im1 Do\nQ\n"
	p := openPDF(t, pdfBytes([]string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Count 1 /Kids [3 0 R] >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 40 30] /Resources << /XObject << /Im0 5 0 R /Im1 6 0 R >> >> /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content),
		stream,
		stream,
	}))
	n := 0
	for _, el := range p.Elements {
		if el.Image != nil {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("pictures = %d, want 2", n)
	}
}

func TestImageClipCropsAndDrops(t *testing.T) {
	img := bytes.Repeat([]byte{200, 10, 10}, 400)
	stream := "<< /Type /XObject /Subtype /Image /Width 20 /Height 20 /ColorSpace /DeviceRGB /BitsPerComponent 8 /Length 1200 >>\nstream\n" + string(img) + "\nendstream"
	content := "10 12 8 6 re W n\nq\n40 0 0 30 0 0 cm\n/Im0 Do\nQ\n"
	p := openPDF(t, pdfBytes([]string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Count 1 /Kids [3 0 R] >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 40 30] /Resources << /XObject << /Im0 5 0 R >> >> /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content),
		stream,
	}))
	var got *ImageObject
	for _, el := range p.Elements {
		if el.Image != nil {
			got = el.Image
		}
	}
	if got == nil {
		t.Fatal("clipped image was dropped")
	}
	if math.Abs(got.X-10) > 1 || math.Abs(got.Width-8) > 1 || math.Abs(got.Height-6) > 1 {
		t.Fatalf("crop box %.1f,%.1f %.1fx%.1f", got.X, got.Y, got.Width, got.Height)
	}
	outside := "0 0 5 5 re W n\nq\n10 0 0 10 30 0 cm\n/Im0 Do\nQ\n"
	p2 := openPDF(t, pdfBytes([]string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Count 1 /Kids [3 0 R] >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 40 30] /Resources << /XObject << /Im0 5 0 R >> >> /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(outside), outside),
		stream,
	}))
	kept := 0
	for _, el := range p2.Elements {
		if el.Image != nil {
			kept++
		}
	}
	if kept != 1 {
		t.Fatalf("picture fully outside a clip was deleted: %d", kept)
	}
}

func TestSmallRectClipStillCrops(t *testing.T) {
	// A chart card clips off a footer that is only a quarter of the image.
	// That must still crop. A page-sized clip must not inset past the page.
	img := bytes.Repeat([]byte{20, 20, 20}, 400)
	stream := "<< /Type /XObject /Subtype /Image /Width 20 /Height 20 /ColorSpace /DeviceRGB /BitsPerComponent 8 /Length 1200 >>\nstream\n" + string(img) + "\nendstream"
	content := "0 0 100 60 re W n\nq\n100 0 0 80 0 0 cm\n/Im0 Do\nQ\n"
	p := openPDF(t, pdfBytes([]string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Count 1 /Kids [3 0 R] >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 100 80] /Resources << /XObject << /Im0 5 0 R >> >> /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content),
		stream,
	}))
	var got *ImageObject
	for _, el := range p.Elements {
		if el.Image != nil {
			got = el.Image
		}
	}
	if got == nil {
		t.Fatal("image deleted")
	}
	if math.Abs(got.Height-60) > 2 {
		t.Fatalf("footer was not clipped, height %.1f", got.Height)
	}
}

func TestPageClipOnlyTrimsOverflow(t *testing.T) {
	img := bytes.Repeat([]byte{20, 20, 20}, 400)
	stream := "<< /Type /XObject /Subtype /Image /Width 20 /Height 20 /ColorSpace /DeviceRGB /BitsPerComponent 8 /Length 1200 >>\nstream\n" + string(img) + "\nendstream"
	// Image hangs 20pt off the right. The clip is the page.
	content := "0 0 100 80 re W n\nq\n120 0 0 80 0 0 cm\n/Im0 Do\nQ\n"
	p := openPDF(t, pdfBytes([]string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Count 1 /Kids [3 0 R] >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 100 80] /Resources << /XObject << /Im0 5 0 R >> >> /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content),
		stream,
	}))
	var got *ImageObject
	for _, el := range p.Elements {
		if el.Image != nil {
			got = el.Image
		}
	}
	if got == nil {
		t.Fatal("image deleted")
	}
	if got.X+got.Width < 98 {
		t.Fatalf("page clip inset the picture to %.1f+%.1f", got.X, got.Width)
	}
}

func TestStitchRealisticGeometry(t *testing.T) {
	parts := []struct {
		s        string
		x, w, sz float64
	}{
		{"R", 622.9, 26.8, 41.2},
		{"Re", 622.9, 55.0, 42.9},
		{"ea", 652.5, 52.8, 42.9},
		{"al", 680.6, 50.6, 44.8},
		{"li", 708.0, 37.4, 44.8},
		{"is", 722.3, 36.2, 44.8},
		{"st", 737.1, 47.4, 44.8},
		{"ti", 761.2, 42.7, 44.8},
		{"ic", 780.7, 37.7, 44.8},
		{"c", 795.5, 22.9, 44.8},
	}
	p := &Page{}
	for _, part := range parts {
		p.Elements = append(p.Elements, PageElement{Text: &TextObject{
			Content: part.s, X: part.x, Y: 643.07 - part.sz, Width: part.w, Height: 50,
			FontSize: part.sz, FontName: "Noto Sans",
		}})
	}
	mergeTextLines(p)
	var got string
	for _, el := range p.Elements {
		if el.Text != nil {
			got += el.Text.Content
		}
	}
	for _, letter := range []string{"R", "e", "a", "l", "i", "s", "t", "c"} {
		if !strings.Contains(got, letter) {
			t.Fatalf("lost %q in %s", letter, got)
		}
	}
}

func TestDuplicateKeepsTheLighterColor(t *testing.T) {
	p := &Page{Elements: []PageElement{
		{Text: &TextObject{Content: "R", X: 0, Y: 100, Width: 20, Height: 20, FontSize: 18, FontName: "Noto Sans", ColorR: 0, ColorG: 0, ColorB: 0}},
		{Text: &TextObject{Content: "R", X: 0, Y: 100, Width: 20, Height: 20, FontSize: 18, FontName: "Noto Sans", ColorR: 255, ColorG: 255, ColorB: 255}},
	}}
	mergeTextLines(p)
	n := 0
	for _, el := range p.Elements {
		if el.Text == nil {
			continue
		}
		n++
		if el.Text.ColorR != 255 || el.Text.ColorG != 255 || el.Text.ColorB != 255 {
			t.Fatalf("shadow stayed black %d,%d,%d", el.Text.ColorR, el.Text.ColorG, el.Text.ColorB)
		}
	}
	if n != 2 {
		t.Fatalf("copies = %d, both fragments must stay so the token is not lost", n)
	}
}

func TestIndexedColorIsNotGray(t *testing.T) {
	// Index 1 is navy (0, 32, 96). Treating the index as a gray level paints
	// it gray-blue instead.
	lookup := string([]byte{255, 255, 255, 0, 32, 96})
	content := "/Ix cs 1 sc 0 0 30 20 re f\n"
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Count 1 /Kids [3 0 R] >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 40 30] /Resources << /ColorSpace << /Ix [/Indexed /DeviceRGB 1 5 0 R] >> >> /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content),
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(lookup), lookup),
	}
	p := openPDF(t, pdfBytes(objs))
	var gotR, gotG, gotB uint8
	found := false
	for _, el := range p.Elements {
		if el.Shape != nil && el.Shape.HasFill {
			gotR, gotG, gotB = el.Shape.FillR, el.Shape.FillG, el.Shape.FillB
			found = true
		}
	}
	if !found {
		t.Fatal("no fill")
	}
	if gotR > 5 || gotG < 20 || gotG > 45 || gotB < 80 {
		t.Fatalf("indexed navy became %d,%d,%d", gotR, gotG, gotB)
	}
}

func TestDropDuplicateLetters(t *testing.T) {
	// Each letter is painted twice at the same place. The copy must be
	// discarded so the word stays "Realistic" rather than "Re"+"ea"+...
	letters := []struct {
		s    string
		x, w float64
	}{
		{"R", 0, 12}, {"R", 0, 12},
		{"e", 10, 12}, {"e", 10, 12},
		{"a", 20, 12}, {"a", 20, 12},
		{"l", 30, 12}, {"l", 30, 12},
		{"i", 40, 10}, {"i", 40, 10},
		{"s", 48, 12}, {"s", 48, 12},
		{"t", 58, 12}, {"t", 58, 12},
		{"i", 68, 10}, {"i", 68, 10},
		{"c", 76, 12}, {"c", 76, 12},
	}
	p := &Page{}
	for _, part := range letters {
		p.Elements = append(p.Elements, PageElement{Text: &TextObject{
			Content: part.s, X: part.x, Y: 100, Width: part.w, Height: 20,
			FontSize: 18, FontName: "Noto Sans",
		}})
	}
	mergeTextLines(p)
	rs := 0
	for _, el := range p.Elements {
		if el.Text != nil && strings.Contains(el.Text.Content, "R") {
			rs++
		}
	}
	if rs < 2 {
		t.Fatalf("a duplicate letter was deleted, copies containing R = %d", rs)
	}
}

func TestStitchOverlappingBigrams(t *testing.T) {
	// Canva draws "Realistic" as overlapping pairs. Each pair shares a letter
	// with the next and covers more than half of the shorter box.
	parts := []struct {
		s    string
		x, w float64
	}{
		{"R", 0, 12},
		{"Re", 0, 22},
		{"ea", 10, 22},
		{"al", 20, 22},
		{"li", 30, 22},
		{"is", 40, 22},
		{"st", 50, 22},
		{"ti", 60, 22},
		{"ic", 70, 22},
		{"c", 80, 14},
	}
	p := &Page{}
	for _, part := range parts {
		p.Elements = append(p.Elements, PageElement{Text: &TextObject{
			Content: part.s, X: part.x, Y: 100, Width: part.w, Height: 20,
			FontSize: 18, FontName: "Noto Sans",
		}})
	}
	// A real word gap must still become a space.
	p.Elements = append(p.Elements,
		PageElement{Text: &TextObject{Content: "Getty", X: 200, Y: 100, Width: 40, Height: 20, FontSize: 18, FontName: "Noto Sans"}},
		PageElement{Text: &TextObject{Content: "images", X: 248, Y: 100, Width: 50, Height: 20, FontSize: 18, FontName: "Noto Sans"}},
	)
	mergeTextLines(p)
	var text string
	for _, el := range p.Elements {
		if el.Text != nil {
			text += el.Text.Content + "\n"
		}
	}
	if !strings.Contains(text, "Getty images") {
		t.Fatalf("word space dropped:\n%s", text)
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
