package converter

import (
	"os"
	"path/filepath"
)

// corpusRoots is where gated PDFs live. CI sets PDF2PPT_CORPUS to the
// directory fetch.py just filled. Without the variable, local checkouts
// still find a corpus that was already on the machine.
func corpusRoots() []string {
	if d := os.Getenv("PDF2PPT_CORPUS"); d != "" {
		return []string{d}
	}
	return []string{"/tmp/corpus", "/tmp/corpus2"}
}

// corpusRequired is true when CI (or a local run) pointed the tests at a
// corpus directory. A missing gated PDF is a failure in that mode. A laptop
// without the corpus still skips.
func corpusRequired() bool {
	return os.Getenv("PDF2PPT_CORPUS") != ""
}

// corpusAlias lists filenames for one deck. The manifest id is the name CI
// downloads. Shorter names are the files already sitting in /tmp/corpus.
var corpusAlias = map[string][]string{
	"wps":                      {"wps-tutorial", "wps"},
	"wps-tutorial":             {"wps-tutorial", "wps"},
	"psumac":                   {"psumac-keynote", "psumac"},
	"psumac-keynote":           {"psumac-keynote", "psumac"},
	"canva-mahidol":            {"canva-mahidol-adv", "canva-mahidol"},
	"canva-mahidol-adv":        {"canva-mahidol-adv", "canva-mahidol"},
	"nbs":                      {"nbs-alibaba", "nbs"},
	"nbs-alibaba":              {"nbs-alibaba", "nbs"},
	"kust":                     {"kust-pressure", "kust"},
	"kust-pressure":            {"kust-pressure", "kust"},
	"tencent":                  {"tencent-4q24", "tencent"},
	"tencent-4q24":             {"tencent-4q24", "tencent"},
	"saasopen":                 {"saasopen-google", "saasopen"},
	"saasopen-google":          {"saasopen-google", "saasopen"},
	"uai":                      {"uai2025-keynote", "uai"},
	"uai2025-keynote":          {"uai2025-keynote", "uai"},
	"undp":                     {"undp-gender-ar", "undp"},
	"undp-gender-ar":           {"undp-gender-ar", "undp"},
	"mine":                     {"mine-safety", "mine"},
	"mine-safety":              {"mine-safety", "mine"},
	"neurips":                  {"neurips-effxfmr", "neurips"},
	"neurips-effxfmr":          {"neurips-effxfmr", "neurips"},
	"kaist":                    {"kaist-xai", "kaist"},
	"kaist-xai":                {"kaist-xai", "kaist"},
	"cdc":                      {"cdc-nbs-m365", "cdc"},
	"cdc-nbs-m365":             {"cdc-nbs-m365", "cdc"},
	"uab-isc":                  {"uab-isc2-2016", "uab-isc"},
	"uab-isc2-2016":            {"uab-isc2-2016", "uab-isc"},
	"uab-reo":                  {"uab-reo-m365", "uab-reo"},
	"uab-reo-m365":             {"uab-reo-m365", "uab-reo"},
	"tx-hhs":                   {"tx-hhs-word", "tx-hhs"},
	"tx-hhs-word":              {"tx-hhs-word", "tx-hhs"},
	"gzmtu":                    {"gzmtu-ppt-2013", "gzmtu"},
	"gzmtu-ppt-2013":           {"gzmtu-ppt-2013", "gzmtu"},
	"canva":                    {"canva-rotary", "canva"},
	"canva-rotary":             {"canva-rotary", "canva"},
	"beamer-mcgill":            {"beamer-mcgill"},
	"beamer-flip":              {"beamer-flip"},
	"cesar2025-ai":             {"cesar2025-ai", "cesar"},
	"libo-cjk":                 {"libo-cjk", "libo"},
	"mit-ocw-ses1":             {"mit-ocw-ses1", "mit-ses1"},
	"nrcs-cattle-distiller":    {"nrcs-cattle-distiller", "nrcs-cattle"},
	"macadmins-autopkg-google": {"macadmins-autopkg-google", "macadmins-autopkg"},
}

// corpusPDF returns the PDF for a manifest id or a legacy short name.
func corpusPDF(name string) string {
	names := corpusAlias[name]
	if len(names) == 0 {
		names = []string{name}
	}
	for _, root := range corpusRoots() {
		for _, n := range names {
			p := filepath.Join(root, n+".pdf")
			st, err := os.Stat(p)
			if err == nil && st.Size() > 0 {
				return p
			}
		}
	}
	return ""
}
