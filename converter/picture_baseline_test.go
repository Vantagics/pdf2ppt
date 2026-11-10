package converter

import (
	"archive/zip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// picture_baseline_v137.json is the p:pic count and the sum of picture areas
// from a v1.3.7 conversion of the local corpus. v1.3.8 composited soft-masked
// pictures into the background and the counts fell. A deck may lose area only
// when areaSlack says clipping removed overflow (the NeurIPS tweet, images
// drawn outside their clip).
type pictureBaseline struct {
	Decks map[string]struct {
		Pics      int     `json:"pics"`
		Area      int64   `json:"area"`
		AreaSlack float64 `json:"areaSlack"`
		Tokens    int     `json:"tokens"`
	} `json:"decks"`
}

// requiredCorpusDecks is every manifest PDF except the generated chrome-skia
// fixture and the known-blocked URLs: tsmc (Cloudflare 403), korea-privacy
// (expired attachment link), and nrcs-cattle-distiller (the host accepts TLS
// and then sends no bytes).
const requiredCorpusDecks = 33

type corpusGate struct {
	Source string `json:"source"`
	Decks  map[string]struct {
		Pics         int     `json:"pics"`
		Area         int64   `json:"area"`
		AreaSlack    float64 `json:"areaSlack"`
		Tokens       int     `json:"tokens"`
		ConcatTokens int     `json:"concatTokens"`
		SizeV140     int64   `json:"sizeV140"`
	} `json:"decks"`
	Renders []renderSample `json:"renders"`
}

func loadCorpusGate(t *testing.T) corpusGate {
	t.Helper()
	raw, err := os.ReadFile("testdata/corpus_gate_v145.json")
	if err != nil {
		t.Fatal(err)
	}
	var gate corpusGate
	if err := json.Unmarshal(raw, &gate); err != nil {
		t.Fatal(err)
	}
	if len(gate.Decks) < requiredCorpusDecks {
		t.Fatalf("corpus gate lists %d decks, want at least %d", len(gate.Decks), requiredCorpusDecks)
	}
	return gate
}

func TestPictureCountBaseline(t *testing.T) {
	gate := loadCorpusGate(t)
	raw, err := os.ReadFile("testdata/picture_baseline_v137.json")
	if err != nil {
		t.Fatal(err)
	}
	var legacy pictureBaseline
	if err := json.Unmarshal(raw, &legacy); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	var missing []string
	found := 0
	for name, want := range gate.Decks {
		pdf := corpusPDF(name)
		if pdf == "" {
			missing = append(missing, name)
			continue
		}
		seen[pdf] = true
		found++
		name, want := name, want
		t.Run(name, func(t *testing.T) {
			out := ensurePPTX(t, pdf, name)
			pics, area := pptxPictures(t, out)
			if want.SizeV140 > 0 {
				st, err := os.Stat(out)
				if err != nil {
					t.Fatal(err)
				}
				if st.Size() > int64(float64(want.SizeV140)*1.10) {
					t.Fatalf("pptx is %d bytes, more than 10%% over the v1.4.0 size %d", st.Size(), want.SizeV140)
				}
			}
			if pics < want.Pics {
				t.Fatalf("p:pic %d, baseline %d", pics, want.Pics)
			}
			slack := want.AreaSlack
			if slack == 0 {
				slack = 0.02
			}
			if want.Area > 0 && float64(area) < float64(want.Area)*(1-slack) {
				t.Fatalf("picture area %d is below the baseline %d (slack %.0f%%)", area, want.Area, slack*100)
			}
			if want.Tokens > 0 {
				got := pptxTokens(t, out)
				if float64(got) < float64(want.Tokens)*0.997 {
					t.Fatalf("unique latin tokens %d, baseline %d", got, want.Tokens)
				}
			}
			if want.ConcatTokens > 0 {
				got := pptxConcatTokens(t, out)
				if float64(got) < float64(want.ConcatTokens)*0.997 {
					t.Fatalf("concatenated tokens %d, baseline %d", got, want.ConcatTokens)
				}
			}
		})
	}
	// Decks that predate the manifest keep the v1.3.7 numbers when their PDF
	// is on the machine and was not already checked under a manifest id.
	for name, want := range legacy.Decks {
		pdf := corpusPDF(name)
		if pdf == "" || seen[pdf] {
			continue
		}
		seen[pdf] = true
		found++
		name, want := name, want
		t.Run("legacy-"+name, func(t *testing.T) {
			out := ensurePPTX(t, pdf, name)
			pics, area := pptxPictures(t, out)
			if base, ok := sizeBaseline[name]; ok && base > 0 {
				st, err := os.Stat(out)
				if err != nil {
					t.Fatal(err)
				}
				if st.Size() > int64(float64(base)*1.10) {
					t.Fatalf("pptx is %d bytes, more than 10%% over the v1.4.0 size %d", st.Size(), base)
				}
			}
			if pics < want.Pics {
				t.Fatalf("p:pic %d, v1.3.7 baseline %d", pics, want.Pics)
			}
			slack := want.AreaSlack
			if slack == 0 {
				slack = 0.02
			}
			if want.Area > 0 && float64(area) < float64(want.Area)*(1-slack) {
				t.Fatalf("picture area %d is below the v1.3.7 baseline %d (slack %.0f%%)", area, want.Area, slack*100)
			}
			if want.Tokens > 0 {
				got := pptxTokens(t, out)
				if float64(got) < float64(want.Tokens)*0.997 {
					t.Fatalf("unique latin tokens %d, v1.3.7 baseline %d", got, want.Tokens)
				}
			}
			if base, ok := tokenBaseline[name]; ok && base > 0 {
				got := pptxConcatTokens(t, out)
				if float64(got) < float64(base)*0.997 {
					t.Fatalf("concatenated tokens %d, v1.4.2 baseline %d", got, base)
				}
			}
		})
	}
	if len(missing) > 0 && corpusRequired() {
		t.Fatalf("missing %d of %d gated PDFs in %s: %s", len(missing), len(gate.Decks), os.Getenv("PDF2PPT_CORPUS"), strings.Join(missing, ", "))
	}
	if found < requiredCorpusDecks && corpusRequired() {
		t.Fatalf("opened %d gated PDFs, want at least %d", found, requiredCorpusDecks)
	}
	if found == 0 {
		t.Skip("no corpus PDFs; set PDF2PPT_CORPUS")
	}
}

// ensurePPTX converts pdf once. PDF2PPT_PPTX_CACHE shares that file with the
// render gate so CI does not convert every deck twice.
func ensurePPTX(t *testing.T, pdf, name string) string {
	t.Helper()
	if dir := os.Getenv("PDF2PPT_PPTX_CACHE"); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		out := filepath.Join(dir, name+".pptx")
		if st, err := os.Stat(out); err == nil && st.Size() > 0 {
			return out
		}
		if _, err := NewConverter(nil).Convert(pdf, out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	out := filepath.Join(t.TempDir(), "out.pptx")
	if _, err := NewConverter(nil).Convert(pdf, out); err != nil {
		t.Fatal(err)
	}
	return out
}

// sizeBaseline is the v1.4.0 PPTX size for decks that predate the manifest
// gate. The 35 gated decks carry sizeV140 inside corpus_gate_v145.json and
// are checked from TestPictureCountBaseline. A deck may shrink. It may not
// grow by more than 10%.
var sizeBaseline map[string]int64

func init() {
	raw, err := os.ReadFile("testdata/pptx_size_baseline_v140.json")
	if err != nil {
		return
	}
	var doc struct {
		Bytes map[string]int64 `json:"bytes"`
	}
	if json.Unmarshal(raw, &doc) == nil {
		sizeBaseline = doc.Bytes
	}
}

func TestPPTXSizeBaseline(t *testing.T) {
	if len(sizeBaseline) == 0 {
		t.Skip("no size baseline")
	}
	raw, err := os.ReadFile("testdata/picture_baseline_v137.json")
	if err != nil {
		t.Fatal(err)
	}
	var pics pictureBaseline
	if err := json.Unmarshal(raw, &pics); err != nil {
		t.Fatal(err)
	}
	found := 0
	for name, base := range sizeBaseline {
		if _, ok := pics.Decks[name]; ok {
			continue // checked while counting pictures
		}
		pdf := corpusPDF(name)
		if pdf == "" {
			continue
		}
		found++
		t.Run(name, func(t *testing.T) {
			out := filepath.Join(t.TempDir(), "out.pptx")
			if _, err := NewConverter(nil).Convert(pdf, out); err != nil {
				t.Fatal(err)
			}
			st, err := os.Stat(out)
			if err != nil {
				t.Fatal(err)
			}
			if st.Size() > int64(float64(base)*1.10) {
				t.Fatalf("pptx is %d bytes, more than 10%% over the v1.4.0 size %d", st.Size(), base)
			}
		})
	}
	if found == 0 {
		if corpusRequired() {
			t.Log("gated deck sizes are checked in TestPictureCountBaseline")
			return
		}
		t.Skip("no extra corpus PDFs")
	}
}

var (
	picRe = regexp.MustCompile(`(?s)<p:pic>.*?</p:pic>`)
	extRe = regexp.MustCompile(`<a:ext cx="(\d+)" cy="(\d+)"`)
)

var tokenBaseline map[string]int

func init() {
	raw, err := os.ReadFile("testdata/token_baseline_v142.json")
	if err != nil {
		return
	}
	var doc struct {
		Tokens map[string]int `json:"tokens"`
	}
	if json.Unmarshal(raw, &doc) == nil {
		tokenBaseline = doc.Tokens
	}
}

// pptxConcatTokens counts unique Latin words after joining every a:t with
// no separator, which is how the corpus recall score sees a missing bullet.
func pptxConcatTokens(t *testing.T, path string) int {
	t.Helper()
	z, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var b strings.Builder
	for _, f := range z.File {
		name := f.Name
		if !strings.HasPrefix(name, "ppt/slides/slide") || !strings.HasSuffix(name, ".xml") || strings.Contains(name, "/_rels/") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range regexp.MustCompile(`<a:t>([^<]*)</a:t>`).FindAllStringSubmatch(string(body), -1) {
			b.WriteString(strings.NewReplacer("&amp;", "&", "&lt;", "<", "&gt;", ">").Replace(m[1]))
		}
	}
	seen := map[string]bool{}
	for _, w := range regexp.MustCompile(`[A-Za-z0-9']{2,}`).FindAllString(b.String(), -1) {
		seen[strings.ToLower(w)] = true
	}
	return len(seen)
}

func pptxTokens(t *testing.T, path string) int {
	t.Helper()
	z, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	seen := map[string]bool{}
	word := regexp.MustCompile(`[A-Za-z0-9']{2,}`)
	for _, f := range z.File {
		name := f.Name
		if !strings.HasPrefix(name, "ppt/slides/slide") || !strings.HasSuffix(name, ".xml") || strings.Contains(name, "/_rels/") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range regexp.MustCompile(`<a:t>([^<]*)</a:t>`).FindAllStringSubmatch(string(b), -1) {
			s := strings.NewReplacer("&amp;", "&", "&lt;", "<", "&gt;", ">").Replace(m[1])
			for _, w := range word.FindAllString(s, -1) {
				seen[strings.ToLower(w)] = true
			}
		}
	}
	return len(seen)
}

func pptxPictures(t *testing.T, path string) (int, int64) {
	t.Helper()
	z, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var pics int
	var area int64
	for _, f := range z.File {
		name := f.Name
		if !strings.HasPrefix(name, "ppt/slides/slide") || !strings.HasSuffix(name, ".xml") || strings.Contains(name, "/_rels/") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		for _, block := range picRe.FindAllString(string(b), -1) {
			pics++
			m := extRe.FindStringSubmatch(block)
			if m == nil {
				continue
			}
			cx, _ := strconv.ParseInt(m[1], 10, 64)
			cy, _ := strconv.ParseInt(m[2], 10, 64)
			area += cx * cy
		}
	}
	return pics, area
}
