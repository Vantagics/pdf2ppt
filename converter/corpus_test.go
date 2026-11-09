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

// TestChromeSkiaSlide prints tests/corpus/slides.html with headless Chrome.
// The producer is Skia/PDF, the same backend Google Slides uses. The page has
// a gradient, a rounded card, rotated text, and Chinese.
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
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
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
	score := ssimFile(t, filepath.Join(srcPNG, "page-1.png"), filepath.Join(rtPNG, "page-1.png"))
	t.Logf("chrome/skia slide 1 SSIM %.4f", score)
	if score < 0.90 {
		t.Fatalf("SSIM %.4f, want >= 0.90", score)
	}
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
