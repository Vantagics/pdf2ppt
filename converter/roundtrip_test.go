package converter

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestRoundTrip builds PPTX fixtures, exports them to PDF with LibreOffice,
// converts the PDF back with this package, and checks structure plus SSIM.
//
// Required tools: soffice, pdftoppm, python3 with python-pptx, pillow, numpy,
// scikit-image. See the repository README.
func TestRoundTrip(t *testing.T) {
	for _, bin := range []string{"soffice", "pdftoppm", "python3"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Fatalf("%s is required for the round-trip test: %v", bin, err)
		}
	}
	root := repoRoot(t)
	gen := filepath.Join(root, "tests", "roundtrip", "generate_samples.py")
	cmp := filepath.Join(root, "tests", "roundtrip", "compare.py")

	work := t.TempDir()
	samples := filepath.Join(work, "samples")
	pdfDir := filepath.Join(work, "pdf")
	outDir := filepath.Join(work, "out")
	rtPDF := filepath.Join(work, "rtpdf")
	origPNG := filepath.Join(work, "origpng")
	rtPNG := filepath.Join(work, "rtpng")
	for _, d := range []string{pdfDir, outDir, rtPDF, origPNG, rtPNG} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	cmd := exec.Command("python3", gen, samples)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("generate samples: %v", err)
	}
	matches, err := filepath.Glob(filepath.Join(samples, "*.pptx"))
	if err != nil || len(matches) == 0 {
		t.Fatalf("no samples: %v", err)
	}

	if err := runSoffice(t, filepath.Join(work, "lo1"), pdfDir, matches); err != nil {
		t.Fatal(err)
	}

	var produced []string
	for _, pptx := range matches {
		base := strings.TrimSuffix(filepath.Base(pptx), ".pptx")
		pdf := filepath.Join(pdfDir, base+".pdf")
		out := filepath.Join(outDir, base+".pptx")
		res, err := NewConverter(nil).Convert(pdf, out)
		if err != nil {
			t.Fatalf("%s: %v", base, err)
		}
		if res.PageCount < 1 {
			t.Fatalf("%s: no pages", base)
		}
		produced = append(produced, out)
		validateOOXML(t, out)
	}

	if err := runSoffice(t, filepath.Join(work, "lo2"), rtPDF, produced); err != nil {
		t.Fatal(err)
	}
	renderDir(t, pdfDir, origPNG)
	renderDir(t, rtPDF, rtPNG)

	for _, pptx := range matches {
		base := strings.TrimSuffix(filepath.Base(pptx), ".pptx")
		origSlides, _ := filepath.Glob(filepath.Join(origPNG, base+"-*.png"))
		rtSlides, _ := filepath.Glob(filepath.Join(rtPNG, base+"-*.png"))
		sort.Strings(origSlides)
		sort.Strings(rtSlides)
		if len(origSlides) == 0 || len(origSlides) != len(rtSlides) {
			t.Fatalf("%s: png count orig=%d roundtrip=%d", base, len(origSlides), len(rtSlides))
		}
		args := []string{cmp, pptx, filepath.Join(outDir, base+".pptx")}
		for i := range origSlides {
			args = append(args, origSlides[i], rtSlides[i])
		}
		c := exec.Command("python3", args...)
		out, err := c.CombinedOutput()
		t.Logf("%s %s", base, strings.TrimSpace(string(out)))
		if err != nil {
			t.Fatalf("%s failed: %v", base, err)
		}
	}
}

func runSoffice(t *testing.T, profile, outDir string, inputs []string) error {
	t.Helper()
	args := []string{
		"--headless", "--norestore", "--nolockcheck",
		"-env:UserInstallation=file://" + profile,
		"--convert-to", "pdf",
		"--outdir", outDir,
	}
	args = append(args, inputs...)
	cmd := exec.Command("soffice", args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

func renderDir(t *testing.T, pdfDir, pngDir string) {
	t.Helper()
	pdfs, err := filepath.Glob(filepath.Join(pdfDir, "*.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	for _, pdf := range pdfs {
		base := strings.TrimSuffix(filepath.Base(pdf), ".pdf")
		cmd := exec.Command("pdftoppm", "-png", "-r", "72", pdf, filepath.Join(pngDir, base))
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("pdftoppm %s: %v", base, err)
		}
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// This file lives in converter/, the module root is its parent.
	return filepath.Dir(dir)
}
