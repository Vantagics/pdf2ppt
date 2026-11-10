package converter

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
)

// A deck of one repeated size plus a different page must use the common
// size. The odd page keeps its aspect ratio, stays inside the slide, and
// is centered. A tie uses the first page.
func TestMixedPageSizeLetterbox(t *testing.T) {
	wide, tall := 960.0, 540.0
	pages := make([]pdfPage, 0, 23)
	for i := 0; i < 22; i++ {
		pages = append(pages, pdfPage{w: wide, h: tall})
	}
	pages = append(pages, pdfPage{w: 612, h: 792})
	assertLetterbox(t, pages, wide, tall, 22, 612, 792)

	// /Rotate 90 on a portrait letter box displays as 792 x 612. That is
	// the last page of the MIT deck. The slide size stays 16:9.
	rotated := append([]pdfPage{}, pages[:22]...)
	rotated = append(rotated, pdfPage{w: 612, h: 792, rotate: 90})
	assertLetterbox(t, rotated, wide, tall, 22, 792, 612)

	// Equal counts: the first page wins, and the portrait pages fit inside it.
	tie := []pdfPage{
		{w: 720, h: 405},
		{w: 720, h: 405},
		{w: 405, h: 720},
		{w: 405, h: 720},
	}
	assertLetterbox(t, tie, 720, 405, 2, 405, 720)

	// A smaller page is centered at its own size.
	small := []pdfPage{
		{w: wide, h: tall},
		{w: wide, h: tall},
		{w: 200, h: 100},
	}
	assertLetterbox(t, small, wide, tall, 2, 200, 100)
}

func TestMITSlideSize(t *testing.T) {
	mit := "/tmp/mit-ses1.pdf"
	if !fileExists(mit) {
		t.Skip("MIT PDF is not on this machine")
	}
	out := filepath.Join(t.TempDir(), "mit.pptx")
	if _, err := NewConverter(nil).Convert(mit, out); err != nil {
		t.Fatal(err)
	}
	cx, cy := readSldSz(t, out)
	if cx != 12192000 || cy != 6858000 {
		t.Fatalf("MIT sldSz = %d x %d, want 12192000 x 6858000 (960 x 540 pt)", cx, cy)
	}
	validateOOXML(t, out)
}

type pdfPage struct {
	w, h   float64
	rotate int
}

func assertLetterbox(t *testing.T, pages []pdfPage, slideW, slideH float64, odd int, pageW, pageH float64) {
	t.Helper()
	dir := t.TempDir()
	pdfPath := filepath.Join(dir, "in.pdf")
	pptxPath := filepath.Join(dir, "out.pptx")
	if err := os.WriteFile(pdfPath, buildMixedPDF(pages), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewConverter(nil).Convert(pdfPath, pptxPath); err != nil {
		t.Fatal(err)
	}
	cx, cy := readSldSz(t, pptxPath)
	wantCX, wantCY := PointsToEMU(slideW), PointsToEMU(slideH)
	if cx != wantCX || cy != wantCY {
		t.Fatalf("sldSz = %d x %d, want %d x %d", cx, cy, wantCX, wantCY)
	}
	boxes := readShapeBoxes(t, pptxPath, odd+1)
	if len(boxes) == 0 {
		t.Fatal("odd page has no shape")
	}
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, b := range boxes {
		if b.x < minX {
			minX = b.x
		}
		if b.y < minY {
			minY = b.y
		}
		if b.x+b.w > maxX {
			maxX = b.x + b.w
		}
		if b.y+b.h > maxY {
			maxY = b.y + b.h
		}
	}
	const tol = 12700.0 * 2 // two points, for EMU truncation
	if minX < -tol || minY < -tol || maxX > float64(cx)+tol || maxY > float64(cy)+tol {
		t.Fatalf("content outside slide: (%.0f,%.0f)-(%.0f,%.0f) slide %d x %d", minX, minY, maxX, maxY, cx, cy)
	}
	midX := (minX + maxX) / 2
	midY := (minY + maxY) / 2
	if math.Abs(midX-float64(cx)/2) > tol || math.Abs(midY-float64(cy)/2) > tol {
		t.Fatalf("content center (%.0f,%.0f), slide center (%.0f,%.0f)", midX, midY, float64(cx)/2, float64(cy)/2)
	}
	scale := 1.0
	if pageW > slideW {
		scale = math.Min(scale, slideW/pageW)
	}
	if pageH > slideH {
		scale = math.Min(scale, slideH/pageH)
	}
	wantW := 0.9 * pageW * scale * 12700
	wantH := 0.9 * pageH * scale * 12700
	if math.Abs((maxX-minX)-wantW) > tol || math.Abs((maxY-minY)-wantH) > tol {
		t.Fatalf("content size %.0f x %.0f, want %.0f x %.0f (scale %.4f)", maxX-minX, maxY-minY, wantW, wantH, scale)
	}
	validateOOXML(t, pptxPath)
}

type box struct{ x, y, w, h float64 }

var xfrmRe = regexp.MustCompile(`<a:off x="(-?\d+)" y="(-?\d+)"/><a:ext cx="(\d+)" cy="(\d+)"/>`)

func readSldSz(t *testing.T, path string) (int64, int64) {
	t.Helper()
	s := readZipPart(t, path, "ppt/presentation.xml")
	re := regexp.MustCompile(`<p:sldSz cx="(\d+)" cy="(\d+)"`)
	m := re.FindStringSubmatch(s)
	if m == nil {
		t.Fatalf("no sldSz in %s", s)
	}
	return mustInt(t, m[1]), mustInt(t, m[2])
}

func readShapeBoxes(t *testing.T, path string, slideNum int) []box {
	t.Helper()
	s := readZipPart(t, path, fmt.Sprintf("ppt/slides/slide%d.xml", slideNum))
	all := xfrmRe.FindAllStringSubmatch(s, -1)
	if len(all) == 0 {
		t.Fatalf("no transforms in slide %d", slideNum)
	}
	// The first transform is the slide group, not content.
	var out []box
	for _, m := range all[1:] {
		out = append(out, box{
			x: float64(mustInt(t, m[1])),
			y: float64(mustInt(t, m[2])),
			w: float64(mustInt(t, m[3])),
			h: float64(mustInt(t, m[4])),
		})
	}
	return out
}

func readZipPart(t *testing.T, path, name string) string {
	t.Helper()
	z, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	for _, f := range z.File {
		if f.Name != name {
			continue
		}
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	t.Fatalf("missing %s", name)
	return ""
}

func mustInt(t *testing.T, s string) int64 {
	t.Helper()
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func buildMixedPDF(pages []pdfPage) []byte {
	n := len(pages)
	objs := make([]string, 0, 2+2*n)
	kids := ""
	for i := 0; i < n; i++ {
		if i > 0 {
			kids += " "
		}
		kids += fmt.Sprintf("%d 0 R", 3+i)
	}
	objs = append(objs,
		"<< /Type /Catalog /Pages 2 0 R >>",
		fmt.Sprintf("<< /Type /Pages /Count %d /Kids [%s] >>", n, kids),
	)
	for i, p := range pages {
		contentID := 3 + n + i
		rot := ""
		if p.rotate != 0 {
			rot = fmt.Sprintf(" /Rotate %d", p.rotate)
		}
		objs = append(objs, fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.4f %.4f]%s /Contents %d 0 R >>", p.w, p.h, rot, contentID))
	}
	for _, p := range pages {
		x, y := 0.05*p.w, 0.05*p.h
		w, h := 0.9*p.w, 0.9*p.h
		stream := fmt.Sprintf("1 0 0 rg\n%.4f %.4f %.4f %.4f re\nf\n", x, y, w, h)
		objs = append(objs, fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream), stream))
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
