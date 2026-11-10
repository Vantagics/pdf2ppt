package converter

import (
	"archive/zip"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestTextRecall compares words from pdftotext with the text stored in the
// PPTX. SSIM stays high when a background is right and the letters are gone.
func TestTextRecall(t *testing.T) {
	if _, err := exec.LookPath("pdftotext"); err != nil {
		t.Skip("pdftotext is not installed")
	}
	cases := []struct {
		pdf  string
		want []string
	}{
		{"/tmp/corpus/tencent.pdf", []string{"CORPORATE", "OVERVIEW", "Fourth", "Quarter"}},
		{"/tmp/corpus/saasopen.pdf", []string{"Subtle", "Business", "Yaroslav"}},
	}
	var ran bool
	for _, tc := range cases {
		if _, err := os.Stat(tc.pdf); err != nil {
			continue
		}
		ran = true
		out := filepath.Join(t.TempDir(), "out.pptx")
		if _, err := NewConverter(nil).Convert(tc.pdf, out); err != nil {
			t.Fatal(err)
		}
		pptxText := pptxText(t, out)
		pdfText := pdftotext(t, tc.pdf)
		for _, w := range tc.want {
			if !strings.Contains(pptxText, w) {
				t.Errorf("%s: PPTX text missing %q", tc.pdf, w)
			}
		}
		recall := wordRecall(pdfText, pptxText)
		if recall < 0.9 {
			t.Errorf("%s: word recall %.2f", tc.pdf, recall)
		}
	}
	if !ran {
		t.Skip("corpus PDFs are not on this machine")
	}
}

func pdftotext(t *testing.T, pdf string) string {
	t.Helper()
	cmd := exec.Command("pdftotext", "-q", pdf, "-")
	b, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

var textRun = regexp.MustCompile(`<a:t[^>]*>([^<]*)</a:t>`)

func pptxText(t *testing.T, path string) string {
	t.Helper()
	z, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var b strings.Builder
	for _, f := range z.File {
		if !strings.Contains(f.Name, "ppt/slides/slide") || !strings.HasSuffix(f.Name, ".xml") {
			continue
		}
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range textRun.FindAllSubmatch(data, -1) {
			b.Write(m[1])
			b.WriteByte(' ')
		}
	}
	return b.String()
}

func wordRecall(pdfText, pptxText string) float64 {
	pdfWords := words(pdfText)
	if len(pdfWords) == 0 {
		return 1
	}
	got := words(pptxText)
	have := map[string]int{}
	for _, w := range got {
		have[w]++
	}
	hit := 0
	for _, w := range pdfWords {
		if have[w] > 0 {
			hit++
			have[w]--
		}
	}
	return float64(hit) / float64(len(pdfWords))
}

func words(s string) []string {
	var out []string
	for _, f := range strings.Fields(s) {
		f = strings.Trim(f, ".,;:()[]\"'")
		if len([]rune(f)) < 4 {
			continue
		}
		out = append(out, strings.ToLower(f))
	}
	return out
}
