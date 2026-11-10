package gopdf2

import (
	"bytes"
	"fmt"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestRewriteWingdingsArrow(t *testing.T) {
	s, fam, ok := rewriteWingdings("Wingdings", "\uf0e0")
	if !ok || s != "→" || fam != "DejaVu Sans" {
		t.Fatalf("got %q %q %v", s, fam, ok)
	}
	if _, _, ok := rewriteWingdings("黑体", "\uf0e0"); ok {
		t.Fatal("non-wingdings run should stay private-use")
	}
}

func TestPatternImagePlacement(t *testing.T) {
	got := patternImagePlacements([]byte("q 961 0 0 722 0 0 cm/Image7 Do Q"))
	if len(got) != 1 || got[0].name != "Image7" {
		t.Fatalf("placement = %+v", got)
	}
	if math.Abs(got[0].m[0]-961) > 0.01 || math.Abs(got[0].m[3]-722) > 0.01 {
		t.Fatalf("matrix = %v", got[0].m)
	}
}

func TestTilingPatternBackground(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tile.pdf")
	if err := os.WriteFile(path, tilingPatternPDF(), 0o644); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	page, err := doc.GetPage(0)
	if err != nil || page == nil {
		t.Fatal(err)
	}
	if len(page.Images) != 1 {
		t.Fatalf("images = %d, elements = %d", len(page.Images), len(page.Elements))
	}
	img := page.Images[0]
	if math.Abs(img.X) > 1 || math.Abs(img.Y) > 1 || math.Abs(img.Width-100) > 1 || math.Abs(img.Height-50) > 1 {
		t.Fatalf("image box = %.2f,%.2f %.2fx%.2f", img.X, img.Y, img.Width, img.Height)
	}
	for _, el := range page.Elements {
		if el.Shape != nil && el.Shape.HasFill && el.Shape.FillR == 0 && el.Shape.FillG == 0 && el.Shape.FillB == 0 && el.Shape.Width > 50 {
			t.Fatal("unresolved pattern painted as a black shape")
		}
	}
	decoded, err := png.Decode(bytes.NewReader(img.Data))
	if err != nil {
		t.Fatal(err)
	}
	r, g, b, _ := decoded.At(0, 0).RGBA()
	if r>>8 < 200 || g>>8 > 30 || b>>8 > 30 {
		t.Fatalf("top pixel = %d,%d,%d, want red (tile above the origin must be the one on the page)", r>>8, g>>8, b>>8)
	}
}

// tilingPatternPDF is a 100x50 page filled with a colored tiling pattern.
// The pattern cell sits above the page (matrix ty = page height, positive y
// scale), which is how PowerPoint places a slide-sized background photo.
// The tile at yi=-1 covers the page. The cell image is red on top, blue below.
func tilingPatternPDF() []byte {
	img := []byte{
		255, 0, 0, 255, 0, 0,
		0, 0, 255, 0, 0, 255,
	}
	pat := []byte("q 10 0 0 10 0 0 cm /Im Do Q\n")
	content := []byte("/Pattern cs /P1 scn\n0 0 100 50 re f\n")
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Count 1 /Kids [3 0 R] >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 100 50] /Resources << /Pattern << /P1 4 0 R >> >> /Contents 6 0 R >>",
		fmt.Sprintf("<< /Type /Pattern /PatternType 1 /PaintType 1 /TilingType 1 /BBox [0 0 10 10] /XStep 10 /YStep 10 /Matrix [10 0 0 5 0 50] /Resources << /XObject << /Im 5 0 R >> >> /Length %d >>\nstream\n%s\nendstream", len(pat), pat),
		fmt.Sprintf("<< /Type /XObject /Subtype /Image /Width 2 /Height 2 /ColorSpace /DeviceRGB /BitsPerComponent 8 /Length %d >>\nstream\n%s\nendstream", len(img), img),
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content),
	}
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
