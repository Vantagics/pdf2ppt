package converter

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestChromeSkiaSlide prints tests/corpus/slides.html with headless Chrome
// (Skia/PDF). Page 1 is the 120° gradient whose axis used to be flipped.
// Later pages cover other linear angles, a radial shading, and body text.
func TestChromeSkiaSlide(t *testing.T) {
	for _, bin := range []string{"google-chrome", "soffice", "pdftoppm", "python3"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s not installed", bin)
		}
	}
	root := repoRoot(t)
	html := filepath.Join(root, "tests", "corpus", "slides.html")
	work := t.TempDir()
	pdf := filepath.Join(work, "slides.pdf")
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	chrome := exec.CommandContext(ctx, "google-chrome", "--headless", "--disable-gpu", "--no-sandbox",
		"--no-pdf-header-footer",
		"--user-data-dir="+filepath.Join(work, "profile"),
		"--print-to-pdf="+pdf, "file://"+html)
	chrome.Stdout, chrome.Stderr = os.Stdout, os.Stderr
	err := chrome.Run()
	st, statErr := os.Stat(pdf)
	if statErr != nil || st.Size() < 1000 {
		t.Fatalf("chrome print failed: %v", err)
	}
	pptx := filepath.Join(work, "slides.pptx")
	if _, err := NewConverter(nil).Convert(pdf, pptx); err != nil {
		t.Fatal(err)
	}
	validateOOXML(t, pptx)
	rtDir := filepath.Join(work, "rt")
	if err := os.MkdirAll(rtDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runSoffice(t, filepath.Join(work, "lo"), rtDir, []string{pptx}); err != nil {
		t.Fatal(err)
	}
	srcPNG := filepath.Join(work, "src")
	rtPNG := filepath.Join(work, "outpng")
	os.MkdirAll(srcPNG, 0o755)
	os.MkdirAll(rtPNG, 0o755)
	if err := exec.Command("pdftoppm", "-png", "-r", "72", pdf, filepath.Join(srcPNG, "page")).Run(); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("pdftoppm", "-png", "-r", "72", filepath.Join(rtDir, "slides.pdf"), filepath.Join(rtPNG, "page")).Run(); err != nil {
		t.Fatal(err)
	}
	// Page 1 is the 120° multi-stop gradient plus the rotated label. The other
	// linear pages are different angles and stop counts. The radial page is a
	// centered circle. The last page is columns/table text, where antialiasing
	// dominates and is not the gradient regression.
	mins := []float64{0.99, 0.98, 0.98, 0.98, 0.97}
	for i, min := range mins {
		a := filepath.Join(srcPNG, fmtPage(i+1))
		b := filepath.Join(rtPNG, fmtPage(i+1))
		score := ssimFile(t, a, b)
		t.Logf("chrome/skia slide %d SSIM %.4f", i+1, score)
		if score < min {
			t.Fatalf("slide %d SSIM %.4f, want >= %.2f", i+1, score, min)
		}
	}
}

func fmtPage(n int) string {
	return "page-" + strconv.Itoa(n) + ".png"
}

func ssimFile(t *testing.T, a, b string) float64 {
	t.Helper()
	cmd := exec.Command("python3", "-c", `
import sys
from PIL import Image
import numpy as np
from skimage.metrics import structural_similarity as ssim
a=np.array(Image.open(sys.argv[1]).convert("RGB"))
b=np.array(Image.open(sys.argv[2]).convert("RGB"))
if a.shape!=b.shape:
    b=np.array(Image.fromarray(b).resize((a.shape[1], a.shape[0])))
print(float(ssim(a,b,channel_axis=2)))
`, a, b)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ssim: %v %s", err, out)
	}
	score, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		t.Fatal(err)
	}
	return score
}
