package gopdf2

import (
	"os"
	"path/filepath"
)

// corpusFile finds a deck PDF. CI sets PDF2PPT_CORPUS to the directory
// fetch.py filled (manifest ids). A local machine without that variable
// still sees /tmp/corpus and /tmp/corpus2.
func corpusFile(name string) string {
	roots := []string{"/tmp/corpus", "/tmp/corpus2"}
	if d := os.Getenv("PDF2PPT_CORPUS"); d != "" {
		roots = []string{d}
	}
	names := []string{name}
	switch name {
	case "psumac":
		names = []string{"psumac-keynote", "psumac"}
	case "canva":
		names = []string{"canva-rotary", "canva"}
	case "uai":
		names = []string{"uai2025-keynote", "uai"}
	case "nbs-alibaba":
		names = []string{"nbs-alibaba", "nbs"}
	}
	for _, root := range roots {
		for _, n := range names {
			p := filepath.Join(root, n+".pdf")
			if st, err := os.Stat(p); err == nil && st.Size() > 0 {
				return p
			}
		}
	}
	return ""
}
