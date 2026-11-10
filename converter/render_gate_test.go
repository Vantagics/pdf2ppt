package converter

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

// renderSample is one slide whose LibreOffice render is stored under
// testdata/render. A hole that fills solid moves the mean pixel error well
// past the tolerance; token and byte-size checks do not see that.
type renderSample struct {
	name string
	pdf  []string
	page int // 1-based
}

var renderSamples = []renderSample{
	{name: "wps-p1", pdf: []string{"/tmp/corpus/wps.pdf"}, page: 1},
	{name: "wps-p2", pdf: []string{"/tmp/corpus/wps.pdf"}, page: 2},
	{name: "wps-p4", pdf: []string{"/tmp/corpus/wps.pdf"}, page: 4},
	{name: "psumac-p30", pdf: []string{"/tmp/corpus/psumac.pdf"}, page: 30},
	{name: "beamer-mcgill-p1", pdf: []string{"/tmp/corpus2/beamer-mcgill.pdf"}, page: 1},
	{name: "canva-mahidol-p1", pdf: []string{
		"/home/ubuntu/.cursor/projects/workspace/uploads/canva-mahidol-adv_104f.pdf",
		"/tmp/corpus/canva-mahidol.pdf",
	}, page: 1},
	{name: "bjedu-2026-p1", pdf: []string{
		"/tmp/corpus/bjedu-2026.pdf",
		"/tmp/bjedu-2026.pdf",
	}, page: 1},
}

func TestRenderedSlidesMatchBaseline(t *testing.T) {
	if _, err := exec.LookPath("soffice"); err != nil {
		t.Skip("soffice not installed")
	}
	if _, err := exec.LookPath("pdftoppm"); err != nil {
		t.Skip("pdftoppm not installed")
	}
	found := 0
	for _, s := range renderSamples {
		pdf := firstExisting(s.pdf)
		if pdf == "" {
			t.Logf("skip %s: pdf not on this machine", s.name)
			continue
		}
		basePath := filepath.Join("testdata", "render", s.name+".png")
		base, err := readPNG(basePath)
		if err != nil {
			t.Fatalf("%s: baseline %s: %v", s.name, basePath, err)
		}
		found++
		t.Run(s.name, func(t *testing.T) {
			got := renderSlide(t, pdf, s.page)
			if got.Bounds() != base.Bounds() {
				t.Fatalf("size %v, baseline %v", got.Bounds().Size(), base.Bounds().Size())
			}
			errv := meanAbsErr(base, got)
			// A filled hole (the WPS title card, the lock cutout) is tens of
			// levels. Antialias noise on a matching slide stays under 2.
			if errv > 6 {
				t.Fatalf("mean abs error %.2f against the stored render (hole or layout changed)", errv)
			}
		})
	}
	if found == 0 {
		t.Skip("no sampled PDFs on this machine")
	}
}

func firstExisting(paths []string) string {
	for _, p := range paths {
		if st, err := os.Stat(p); err == nil && st.Size() > 0 {
			return p
		}
	}
	return ""
}

func renderSlide(t *testing.T, pdf string, page int) image.Image {
	t.Helper()
	dir := t.TempDir()
	pptx := filepath.Join(dir, "out.pptx")
	if _, err := NewConverter(nil).Convert(pdf, pptx); err != nil {
		t.Fatal(err)
	}
	profile := filepath.Join(dir, "lo")
	pdfOut := filepath.Join(dir, "lo-pdf")
	if err := os.MkdirAll(pdfOut, 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("soffice",
		"--headless", "--norestore", "--nolockcheck",
		"-env:UserInstallation=file://"+profile,
		"--convert-to", "pdf", "--outdir", pdfOut, pptx)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	pngDir := filepath.Join(dir, "png")
	if err := os.MkdirAll(pngDir, 0o755); err != nil {
		t.Fatal(err)
	}
	prefix := filepath.Join(pngDir, "page")
	cmd = exec.Command("pdftoppm", "-png", "-r", "40", "-f", strconv.Itoa(page), "-l", strconv.Itoa(page),
		filepath.Join(pdfOut, "out.pdf"), prefix)
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	matches, err := filepath.Glob(prefix + "*.png")
	if err != nil || len(matches) != 1 {
		t.Fatalf("render pngs %v err %v", matches, err)
	}
	img, err := readPNG(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func readPNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}

func meanAbsErr(a, b image.Image) float64 {
	ab, bb := a.Bounds(), b.Bounds()
	n := 0
	var sum float64
	for y := ab.Min.Y; y < ab.Max.Y; y++ {
		for x := ab.Min.X; x < ab.Max.X; x++ {
			ar, ag, abv, _ := a.At(x, y).RGBA()
			br, bg, bbv, _ := b.At(x-ab.Min.X+bb.Min.X, y-ab.Min.Y+bb.Min.Y).RGBA()
			sum += absDiff(ar, br) + absDiff(ag, bg) + absDiff(abv, bbv)
			n += 3
		}
	}
	if n == 0 {
		return 0
	}
	return sum / float64(n) / 256 // RGBA is 16-bit; scale back toward 0..255
}

func absDiff(a, b uint32) float64 {
	if a > b {
		return float64(a - b)
	}
	return float64(b - a)
}

// TestHoleRendersOpen draws a frame, a letter O, and a donut and checks the
// center of each hole. LibreOffice fills a second a:path solid, the same way
// PowerPoint did on the WPS title card, so this fails without the corpus PDFs.
func TestHoleRendersOpen(t *testing.T) {
	if _, err := exec.LookPath("soffice"); err != nil {
		t.Skip("soffice not installed")
	}
	if _, err := exec.LookPath("pdftoppm"); err != nil {
		t.Skip("pdftoppm not installed")
	}
	var content bytes.Buffer
	content.WriteString("0.15 0.20 0.45 rg\n0 0 300 100 re f\n")
	content.WriteString("0.75 0.10 0.10 rg\n")
	content.WriteString("10 20 70 60 re\n25 35 40 30 re\nf*\n")
	content.WriteString(circlePath(150, 50, 36))
	content.WriteString(circlePath(150, 50, 16))
	content.WriteString("f*\n")
	content.WriteString(octagonPath(250, 50, 36))
	content.WriteString(octagonPath(250, 50, 16))
	content.WriteString("f*\n")
	pdf := filepath.Join(t.TempDir(), "holes.pdf")
	if err := os.WriteFile(pdf, pdfBytes([]string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Count 1 /Kids [3 0 R] >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 300 100] /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content.String()), content.String()),
	}), 0o644); err != nil {
		t.Fatal(err)
	}
	img := renderSlide(t, pdf, 1)
	b := img.Bounds()
	// 72 dpi would be 300x100. pdftoppm -r 40 scales by 40/72.
	at := func(px, py float64) (r, g, bl uint32) {
		x := b.Min.X + int(float64(b.Dx())*px)
		y := b.Min.Y + int(float64(b.Dy())*py)
		if x >= b.Max.X {
			x = b.Max.X - 1
		}
		if y >= b.Max.Y {
			y = b.Max.Y - 1
		}
		rr, gg, bb, _ := img.At(x, y).RGBA()
		return rr >> 8, gg >> 8, bb >> 8
	}
	// PDF y is up. Image y is down. Hole centers sit on the vertical middle.
	holes := []struct {
		name string
		px   float64
	}{
		{"frame", 45.0 / 300},
		{"letter O", 150.0 / 300},
		{"donut", 250.0 / 300},
	}
	for _, h := range holes {
		r, g, bl := at(h.px, 0.5)
		if r > 160 && g < 100 && bl < 100 {
			t.Errorf("%s hole filled solid (rgb %d,%d,%d)", h.name, r, g, bl)
		}
	}
	// The frame's left bar is the red fill. A missing shape would also leave
	// the center dark and hide the bug.
	r, g, bl := at(18.0/300, 0.5)
	if r < 160 || g > 100 {
		t.Fatalf("frame bar rgb %d,%d,%d, want the red fill", r, g, bl)
	}
}

func circlePath(cx, cy, r float64) string {
	k := r * 0.5522847498
	return fmt.Sprintf("%g %g m\n%g %g %g %g %g %g c\n%g %g %g %g %g %g c\n%g %g %g %g %g %g c\n%g %g %g %g %g %g c\nh\n",
		cx+r, cy,
		cx+r, cy+k, cx+k, cy+r, cx, cy+r,
		cx-k, cy+r, cx-r, cy+k, cx-r, cy,
		cx-r, cy-k, cx-k, cy-r, cx, cy-r,
		cx+k, cy-r, cx+r, cy-k, cx+r, cy)
}

func octagonPath(cx, cy, r float64) string {
	d := r * 0.7071067812
	pts := [][2]float64{
		{cx + r, cy}, {cx + d, cy + d}, {cx, cy + r}, {cx - d, cy + d},
		{cx - r, cy}, {cx - d, cy - d}, {cx, cy - r}, {cx + d, cy - d},
	}
	s := fmt.Sprintf("%g %g m\n", pts[0][0], pts[0][1])
	for _, p := range pts[1:] {
		s += fmt.Sprintf("%g %g l\n", p[0], p[1])
	}
	return s + "h\n"
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
