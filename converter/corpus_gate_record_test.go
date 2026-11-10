package converter

import (
	"bytes"
	"encoding/json"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"
)

// TestRecordCorpusGate rewrites testdata/corpus_gate_v145.json and the
// LibreOffice PNGs under testdata/render. It does not run in CI.
//
//	PDF2PPT_CORPUS=/path/to/pdfs PDF2PPT_V140=/path/to/pdf2ppt-1.4.0 \
//	  PDF2PPT_RECORD_BASELINE=1 go test ./converter -run TestRecordCorpusGate -timeout 90m
func TestRecordCorpusGate(t *testing.T) {
	if os.Getenv("PDF2PPT_RECORD_BASELINE") != "1" {
		t.Skip("set PDF2PPT_RECORD_BASELINE=1 to rewrite the corpus gate")
	}
	if _, err := exec.LookPath("soffice"); err != nil {
		t.Fatal(err)
	}
	ids := manifestDeckIDs(t)
	if len(ids) < requiredCorpusDecks {
		t.Fatalf("manifest has %d gated decks, want %d", len(ids), requiredCorpusDecks)
	}
	v140 := os.Getenv("PDF2PPT_V140")
	if v140 == "" {
		t.Fatal("set PDF2PPT_V140 to the v1.4.0 binary")
	}
	extraPage := map[string][]int{
		"wps-tutorial":   {2, 4},
		"psumac-keynote": {30},
	}
	type deck struct {
		Pics         int     `json:"pics"`
		Area         int64   `json:"area"`
		AreaSlack    float64 `json:"areaSlack"`
		Tokens       int     `json:"tokens"`
		ConcatTokens int     `json:"concatTokens"`
		SizeV140     int64   `json:"sizeV140"`
	}
	decks := map[string]deck{}
	var renders []renderSample
	for _, id := range ids {
		pdf := corpusPDF(id)
		if pdf == "" {
			t.Fatalf("missing %s", id)
		}
		t.Logf("measure %s", id)
		out := ensurePPTX(t, pdf, id)
		pics, area := pptxPictures(t, out)
		decks[id] = deck{
			Pics:         pics,
			Area:         area,
			AreaSlack:    0.02,
			Tokens:       pptxTokens(t, out),
			ConcatTokens: pptxConcatTokens(t, out),
			SizeV140:     v140Size(t, v140, pdf, id),
		}
		pages := []int{1}
		pages = append(pages, extraPage[id]...)
		for _, page := range pages {
			name := id + "-p" + itoa(page)
			t.Logf("render %s", name)
			img := renderSlideNamed(t, pdf, id, page)
			f, err := os.Create(filepath.Join("testdata", "render", name+".png"))
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(f, img); err != nil {
				f.Close()
				t.Fatal(err)
			}
			f.Close()
			renders = append(renders, renderSample{Name: name, ID: id, Page: page})
		}
	}
	doc := struct {
		Source  string          `json:"source"`
		Note    string          `json:"note"`
		Decks   map[string]deck `json:"decks"`
		Renders []renderSample  `json:"renders"`
	}{
		Source:  "v1.4.5",
		Note:    "Picture count, picture area, unique Latin tokens, concatenated a:t tokens, and v1.4.0 PPTX bytes for every manifest deck except the known-blocked TSMC file. Renders are LibreOffice page images. A later build fails if pictures or tokens drop more than 0.3%, area drops more than the slack, the PPTX exceeds 110% of the v1.4.0 size, or a rendered page drifts.",
		Decks:   decks,
		Renders: renders,
	}
	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, '\n')
	if err := os.WriteFile("testdata/corpus_gate_v145.json", raw, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote %d decks, %d renders", len(decks), len(renders))
}

func manifestDeckIDs(t *testing.T) []string {
	t.Helper()
	raw, err := os.ReadFile("../tests/corpus/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Files []struct {
			ID      string `json:"id"`
			URL     string `json:"url"`
			Blocked string `json:"blocked"`
		} `json:"files"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, f := range doc.Files {
		if f.URL == "" || f.Blocked != "" {
			continue
		}
		ids = append(ids, f.ID)
	}
	sort.Strings(ids)
	return ids
}

func v140Size(t *testing.T, bin, pdf, id string) int64 {
	t.Helper()
	dir := t.TempDir()
	out := filepath.Join(dir, id+".pptx")
	cmd := exec.Command(bin, pdf, out)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("v1.4.0 %s: %v\n%s", id, err, stderr.String())
	}
	st, err := os.Stat(out)
	if err != nil {
		t.Fatal(err)
	}
	return st.Size()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [16]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
