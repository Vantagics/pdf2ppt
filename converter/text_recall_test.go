package converter

import (
	"archive/zip"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// TestTextRecall compares words from pdftotext with the text stored in the
// PPTX. SSIM stays high when a background is right and the letters are gone.
func TestTextRecall(t *testing.T) {
	if _, err := exec.LookPath("pdftotext"); err != nil {
		t.Skip("pdftotext is not installed")
	}
	cases := []struct {
		name string
		want []string
	}{
		{"tencent-4q24", []string{"CORPORATE", "OVERVIEW", "Fourth", "Quarter"}},
		{"saasopen-google", []string{"Subtle", "Business", "Yaroslav"}},
	}
	var ran bool
	for _, tc := range cases {
		pdf := corpusPDF(tc.name)
		if pdf == "" {
			continue
		}
		ran = true
		out := filepath.Join(t.TempDir(), "out.pptx")
		if _, err := NewConverter(nil).Convert(pdf, out); err != nil {
			t.Fatal(err)
		}
		pptxText := pptxText(t, out)
		pdfText := pdftotext(t, pdf)
		for _, w := range tc.want {
			if !strings.Contains(pptxText, w) {
				t.Errorf("%s: PPTX text missing %q", tc.name, w)
			}
		}
		recall := wordRecall(pdfText, pptxText)
		if recall < 0.9 {
			t.Errorf("%s: word recall %.2f", tc.name, recall)
		}
	}
	if !ran {
		t.Skip("corpus PDFs are not on this machine")
	}
}

// TestTextRecallBaseline fails when unique-token recall drops more than 0.3
// percentage points from the stored value. a:t is concatenated with no
// inserted spaces, matching the corpus score. Tokens are NFKC Latin/digit
// words of at least two characters, plus one token per CJK, kana, or hangul
// rune.
//
// The v1.4.8 KUST drop (ρg glued to ②, which NFKC turns into "ρg2") did not
// fail this test. The baseline was recorded from that same build, and
// pdftotext writes the source as "ρ g", so "ρg" is not a source token.
// TestKUSTOpeningParenMatchesZ checks that boundary directly.
func TestRecordRecallBaseline(t *testing.T) {
	if os.Getenv("PDF2PPT_RECORD_BASELINE") != "1" {
		t.Skip("set PDF2PPT_RECORD_BASELINE=1 to rewrite recall")
	}
	if _, err := exec.LookPath("pdftotext"); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("testdata/corpus_gate_v145.json")
	if err != nil {
		t.Fatal(err)
	}
	var gate struct {
		Decks map[string]json.RawMessage `json:"decks"`
	}
	if err := json.Unmarshal(raw, &gate); err != nil {
		t.Fatal(err)
	}
	recall := map[string]float64{}
	for name := range gate.Decks {
		pdf := corpusPDF(name)
		if pdf == "" {
			t.Fatalf("missing %s", name)
		}
		out := ensurePPTX(t, pdf, "recall-"+name)
		got := uniqueRecall(pdftotext(t, pdf), pptxConcatText(t, out))
		recall[name] = got
		t.Logf("%s %.4f", name, got)
	}
	doc := struct {
		Source string             `json:"source"`
		Note   string             `json:"note"`
		Recall map[string]float64 `json:"recall"`
	}{
		Source: "v1.4.9",
		Note:   "Unique-token recall of concatenated a:t against pdftotext. A later build fails if a deck drops more than 0.3 percentage points.",
		Recall: recall,
	}
	buf, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	buf = append(buf, '\n')
	if err := os.WriteFile("testdata/recall_baseline_v148.json", buf, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestTextRecallBaseline(t *testing.T) {
	if _, err := exec.LookPath("pdftotext"); err != nil {
		t.Skip("pdftotext is not installed")
	}
	raw, err := os.ReadFile("testdata/recall_baseline_v148.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Recall map[string]float64 `json:"recall"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Recall) == 0 {
		t.Fatal("empty recall baseline")
	}
	found := 0
	for name, base := range doc.Recall {
		pdf := corpusPDF(name)
		if pdf == "" {
			if corpusRequired() {
				t.Errorf("missing %s", name)
			}
			continue
		}
		found++
		name, base := name, base
		t.Run(name, func(t *testing.T) {
			out := ensurePPTX(t, pdf, name)
			got := uniqueRecall(pdftotext(t, pdf), pptxConcatText(t, out))
			if got < base-0.003 {
				t.Fatalf("unique recall %.4f, baseline %.4f (drop %.2f pp)", got, base, (base-got)*100)
			}
		})
	}
	if found == 0 && !corpusRequired() {
		t.Skip("no corpus PDFs")
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

func pptxConcatText(t *testing.T, path string) string {
	t.Helper()
	z, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var b strings.Builder
	for _, f := range z.File {
		if !strings.Contains(f.Name, "ppt/slides/slide") || !strings.HasSuffix(f.Name, ".xml") || strings.Contains(f.Name, "/_rels/") {
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
		}
	}
	return b.String()
}

func uniqueRecall(pdfText, pptxText string) float64 {
	src := uniqueTokens(pdfText)
	if len(src) == 0 {
		return 1
	}
	got := uniqueTokens(pptxText)
	hit := 0
	for w := range src {
		if _, ok := got[w]; ok {
			hit++
		}
	}
	return float64(hit) / float64(len(src))
}

func uniqueTokens(s string) map[string]struct{} {
	s = norm.NFKC.String(s)
	out := map[string]struct{}{}
	var b strings.Builder
	flush := func() {
		if b.Len() >= 2 {
			out[strings.ToLower(b.String())] = struct{}{}
		}
		b.Reset()
	}
	for _, r := range s {
		switch {
		case r == '\'' || unicode.IsLetter(r) && r < 0x2E80 || unicode.IsDigit(r):
			b.WriteRune(r)
		case r >= 0x2E80 && r <= 0x9FFF || r >= 0x3040 && r <= 0x30FF || r >= 0xAC00 && r <= 0xD7AF || r >= 0x0600 && r <= 0x06FF:
			flush()
			out[string(r)] = struct{}{}
		default:
			flush()
		}
	}
	flush()
	return out
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
