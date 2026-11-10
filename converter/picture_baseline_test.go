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

func TestPictureCountBaseline(t *testing.T) {
	raw, err := os.ReadFile("testdata/picture_baseline_v137.json")
	if err != nil {
		t.Fatal(err)
	}
	var base pictureBaseline
	if err := json.Unmarshal(raw, &base); err != nil {
		t.Fatal(err)
	}
	found := 0
	for name, want := range base.Decks {
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
			pics, area := pptxPictures(t, out)
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
				// One percent: a fix must not trade away another deck's text.
				if float64(got) < float64(want.Tokens)*0.99 {
					t.Fatalf("unique latin tokens %d, v1.3.7 baseline %d", got, want.Tokens)
				}
			}
		})
	}
	if found == 0 {
		t.Skip("no corpus PDFs in /tmp/corpus")
	}
}

func corpusPDF(name string) string {
	candidates := []string{filepath.Join("/tmp/corpus", name+".pdf")}
	if name == "canva-mahidol" {
		candidates = append(candidates,
			"/home/ubuntu/.cursor/projects/workspace/uploads/canva-mahidol-adv_104f.pdf",
			filepath.Join("/tmp/corpus", "canva-mahidol-adv.pdf"),
		)
	}
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && st.Size() > 0 {
			return p
		}
	}
	return ""
}

var (
	picRe = regexp.MustCompile(`(?s)<p:pic>.*?</p:pic>`)
	extRe = regexp.MustCompile(`<a:ext cx="(\d+)" cy="(\d+)"`)
)

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
