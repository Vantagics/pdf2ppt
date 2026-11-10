package gopdf2

import (
	"encoding/hex"
	"math"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf16"

	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
	"golang.org/x/text/encoding/charmap"
)

var subsetPrefix = regexp.MustCompile(`^[A-Z]{6}\+`)

// knownFamilies maps PDF BaseFont stems (no style suffix) to a family name
// that LibreOffice can resolve on a typical Linux install.
var knownFamilies = map[string]string{
	"LiberationSans":        "Liberation Sans",
	"LiberationSerif":       "Liberation Serif",
	"LiberationMono":        "Liberation Mono",
	"WenQuanYiMicroHei":     "WenQuanYi Micro Hei",
	"WenQuanYiMicroHeiMono": "WenQuanYi Micro Hei Mono",
	"WenQuanYiZenHei":       "WenQuanYi Zen Hei",
	"DroidSansFallback":     "Droid Sans Fallback",
	"NotoSans":              "Noto Sans",
	"NotoSerif":             "Noto Serif",
	"NotoSansCJK":           "Noto Sans CJK SC",
	"NotoSansCJKsc":         "Noto Sans CJK SC",
	"NotoSansCJKSC":         "Noto Sans CJK SC",
	"NotoSansMono":          "Noto Sans Mono",
	"Carlito":               "Carlito",
	"Caladea":               "Caladea",
	"DejaVuSans":            "DejaVu Sans",
	"DejaVuSerif":           "DejaVu Serif",
	"DejaVuSansMono":        "DejaVu Sans Mono",
	"TimesNewRoman":         "Times New Roman",
	"TimesNewRomanPSMT":     "Times New Roman",
	"TimesNewRomanPS":       "Times New Roman",
	"ArialMT":               "Arial",
	"Arial":                 "Arial",
	"CourierNew":            "Courier New",
	"CourierNewPSMT":        "Courier New",
	"Calibri":               "Calibri",
	"Cambria":               "Cambria",
	"Verdana":               "Verdana",
	"Tahoma":                "Tahoma",
	"Georgia":               "Georgia",
	"SimSun":                "SimSun",
	"SimHei":                "SimHei",
	"MicrosoftYaHei":        "Microsoft YaHei",
	"SourceHanSans":         "Source Han Sans SC",
	"SourceHanSansSC":       "Source Han Sans SC",
}

type pdfFont struct {
	family   string
	bold     bool
	italic   bool
	ascent   float64 // fraction of em, positive
	descent  float64 // fraction of em, positive
	codeLen  int
	toUni    map[uint64]string
	enc      map[byte]string // WinAnsi, MacRoman, or an encoding dictionary
	widths   map[int]float64 // glyph width in 1000ths of em, keyed by char code
	defWidth float64
}

func (f *pdfFont) decodeBytes(raw []byte, fontSize, charSp, wordSp, hScale float64) (string, float64) {
	if f == nil || len(raw) == 0 {
		return "", 0
	}
	step := f.codeLen
	if step < 1 {
		step = 1
	}
	if step > 1 && len(raw)%step != 0 {
		step = 1
	}
	var b strings.Builder
	var adv float64
	for i := 0; i+step <= len(raw); i += step {
		var code uint64
		for k := 0; k < step; k++ {
			code = code<<8 | uint64(raw[i+k])
		}
		s := f.unicode(code, raw[i])
		var ch rune
		if rs := []rune(s); len(rs) == 1 {
			ch = rs[0]
		}
		adv += glyphAdvance(f.widthOf(int(code)), fontSize, charSp, wordSp, hScale, ch)
		b.WriteString(s)
	}
	return b.String(), adv
}

func charmapTable(cm *charmap.Charmap) map[byte]string {
	out := make(map[byte]string, 256)
	for i := 0; i < 256; i++ {
		r := cm.DecodeByte(byte(i))
		if r == '\uFFFD' || r == 0 {
			continue
		}
		out[byte(i)] = string(r)
	}
	return out
}

// glyphName maps the Adobe names PowerPoint and other producers put in
// /Encoding /Differences arrays onto Unicode.
var glyphName = map[string]string{
	"space": " ", "exclam": "!", "quotedbl": "\"", "numbersign": "#", "dollar": "$",
	"percent": "%", "ampersand": "&", "quotesingle": "'", "parenleft": "(", "parenright": ")",
	"asterisk": "*", "plus": "+", "comma": ",", "hyphen": "-", "period": ".", "slash": "/",
	"zero": "0", "one": "1", "two": "2", "three": "3", "four": "4", "five": "5",
	"six": "6", "seven": "7", "eight": "8", "nine": "9", "colon": ":", "semicolon": ";",
	"less": "<", "equal": "=", "greater": ">", "question": "?", "at": "@",
	"bracketleft": "[", "backslash": "\\", "bracketright": "]", "asciicircum": "^", "underscore": "_",
	"grave": "`", "braceleft": "{", "bar": "|", "braceright": "}", "asciitilde": "~",
	"bullet": "•", "periodcentered": "·", "dotaccent": "˙", "dieresis": "¨", "ring": "˚",
	"cedilla": "¸", "hungarumlaut": "˝", "ogonek": "˛", "caron": "ˇ", "breve": "˘",
	"emdash": "—", "endash": "–", "quotedblleft": "“", "quotedblright": "”",
	"quoteleft": "‘", "quoteright": "’", "guilsinglleft": "‹", "guilsinglright": "›",
	"fi": "ﬁ", "fl": "ﬂ", "ellipsis": "…", "trademark": "™", "copyright": "©", "registered": "®",
	"degree": "°", "plusminus": "±", "multiply": "×", "divide": "÷", "minus": "−",
	"Agrave": "À", "Aacute": "Á", "Acircumflex": "Â", "Atilde": "Ã", "Adieresis": "Ä", "Aring": "Å",
	"AE": "Æ", "Ccedilla": "Ç", "Egrave": "È", "Eacute": "É", "Ecircumflex": "Ê", "Edieresis": "Ë",
	"Igrave": "Ì", "Iacute": "Í", "Icircumflex": "Î", "Idieresis": "Ï", "Eth": "Ð", "Ntilde": "Ñ",
	"Ograve": "Ò", "Oacute": "Ó", "Ocircumflex": "Ô", "Otilde": "Õ", "Odieresis": "Ö",
	"Oslash": "Ø", "Ugrave": "Ù", "Uacute": "Ú", "Ucircumflex": "Û", "Udieresis": "Ü", "Yacute": "Ý",
	"Thorn": "Þ", "germandbls": "ß",
	"agrave": "à", "aacute": "á", "acircumflex": "â", "atilde": "ã", "adieresis": "ä", "aring": "å",
	"ae": "æ", "ccedilla": "ç", "egrave": "è", "eacute": "é", "ecircumflex": "ê", "edieresis": "ë",
	"igrave": "ì", "iacute": "í", "icircumflex": "î", "idieresis": "ï", "eth": "ð", "ntilde": "ñ",
	"ograve": "ò", "oacute": "ó", "ocircumflex": "ô", "otilde": "õ", "odieresis": "ö",
	"oslash": "ø", "ugrave": "ù", "uacute": "ú", "ucircumflex": "û", "udieresis": "ü", "yacute": "ý",
	"thorn": "þ", "ydieresis": "ÿ",
	"lslash": "ł", "Lslash": "Ł", "oe": "œ", "OE": "Œ", "scaron": "š", "Scaron": "Š",
	"zcaron": "ž", "Zcaron": "Ž", "dotlessi": "ı",
}

func standardEncoding() map[byte]string {
	// PDF StandardEncoding is close to MacRoman for the letters we care about;
	// WinAnsi covers the same Latin-1 repertoire producers actually emit.
	return charmapTable(charmap.Windows1252)
}

func encodingFromDict(xrd *model.XRefTable, d types.Dict) map[byte]string {
	base := charmapTable(charmap.Windows1252)
	if n := d.NameEntry("BaseEncoding"); n != nil {
		switch *n {
		case "MacRomanEncoding":
			base = charmapTable(charmap.Macintosh)
		case "StandardEncoding":
			base = standardEncoding()
		case "WinAnsiEncoding":
			base = charmapTable(charmap.Windows1252)
		}
	}
	diffs := d.ArrayEntry("Differences")
	if diffs == nil {
		if o, ok := d.Find("Differences"); ok {
			if a, err := xrd.DereferenceArray(o); err == nil {
				diffs = a
			}
		}
	}
	code := 0
	for _, el := range diffs {
		el, _ = xrd.Dereference(el)
		switch v := el.(type) {
		case types.Integer:
			code = int(v)
		case types.Name:
			if s := glyphToUnicode(v.Value()); s != "" {
				base[byte(code)] = s
			}
			code++
		}
	}
	return base
}

func glyphToUnicode(name string) string {
	if s, ok := glyphName[name]; ok {
		return s
	}
	if len(name) == 1 {
		return name
	}
	if strings.HasPrefix(name, "uni") && len(name) == 7 {
		if n, err := hex.DecodeString(name[3:]); err == nil && len(n) == 2 {
			return string(rune(uint16(n[0])<<8 | uint16(n[1])))
		}
	}
	return ""
}

func (f *pdfFont) unicode(code uint64, first byte) string {
	if f != nil && f.toUni != nil {
		if s := f.toUni[code]; s != "" {
			return s
		}
	}
	if f != nil && f.enc != nil && code < 256 {
		if s := f.enc[byte(code)]; s != "" {
			return s
		}
	}
	if code >= 32 && code < 127 {
		return string(rune(code))
	}
	// Latin-1 is a safe last resort for unmarked single-byte fonts.
	if f != nil && f.codeLen <= 1 && code >= 160 && code <= 255 {
		return string(rune(code))
	}
	_ = first
	return ""
}

func (f *pdfFont) widthOf(code int) float64 {
	if f == nil {
		return 500
	}
	if w, ok := f.widths[code]; ok {
		return w
	}
	if f.defWidth > 0 {
		return f.defWidth
	}
	return 500
}

// glyphAdvance is the horizontal displacement in text space (points when the
// text matrix is unscaled). numAdjust is a TJ number (thousandths of em,
// negative moves right) and is not included here.
func glyphAdvance(width, fontSize, charSp, wordSp, hScale float64, ch rune) float64 {
	if hScale == 0 {
		hScale = 100
	}
	th := hScale / 100
	tx := ((width/1000)*fontSize + charSp) * th
	if ch == ' ' {
		tx += wordSp * th
	}
	return tx
}

// tjAdjust converts a TJ number into a text-space delta (positive moves right).
func tjAdjust(num, fontSize, hScale float64) float64 {
	if hScale == 0 {
		hScale = 100
	}
	return -num / 1000 * fontSize * (hScale / 100)
}

// gapIsWordSpace reports a TJ adjustment wide enough to be a missing space.
// Kerning stays under about 0.1 em. A real word space is 0.2–0.35 em, and
// xdvipdfmx uses exactly 0.25 em.
func gapIsWordSpace(adj, fontSize float64) bool {
	return fontSize > 0 && adj >= fontSize*0.17
}

func normalizeFontName(base string) (family string, bold, italic bool) {
	base = strings.TrimPrefix(base, "/")
	base = subsetPrefix.ReplaceAllString(base, "")
	base = strings.TrimSpace(base)
	switch {
	case strings.HasSuffix(base, "-BoldItalic"), strings.HasSuffix(base, ",BoldItalic"):
		bold, italic = true, true
		base = strings.TrimSuffix(strings.TrimSuffix(base, "-BoldItalic"), ",BoldItalic")
	case strings.HasSuffix(base, "-BoldOblique"), strings.HasSuffix(base, ",BoldOblique"):
		bold, italic = true, true
		base = strings.TrimSuffix(strings.TrimSuffix(base, "-BoldOblique"), ",BoldOblique")
	case strings.HasSuffix(base, "-Bold"), strings.HasSuffix(base, ",Bold"):
		bold = true
		base = strings.TrimSuffix(strings.TrimSuffix(base, "-Bold"), ",Bold")
	case strings.HasSuffix(base, "-Italic"), strings.HasSuffix(base, ",Italic"):
		italic = true
		base = strings.TrimSuffix(strings.TrimSuffix(base, "-Italic"), ",Italic")
	case strings.HasSuffix(base, "-Oblique"), strings.HasSuffix(base, ",Oblique"):
		italic = true
		base = strings.TrimSuffix(strings.TrimSuffix(base, "-Oblique"), ",Oblique")
	case strings.HasSuffix(base, "-Regular"), strings.HasSuffix(base, ",Regular"):
		base = strings.TrimSuffix(strings.TrimSuffix(base, "-Regular"), ",Regular")
	}
	if mapped, ok := knownFamilies[base]; ok {
		return mapped, bold, italic
	}
	if strings.Contains(base, " ") {
		return base, bold, italic
	}
	return splitCamel(base), bold, italic
}

func splitCamel(s string) string {
	rs := []rune(s)
	var b strings.Builder
	for i, r := range rs {
		if i > 0 && unicode.IsUpper(r) {
			prev := rs[i-1]
			nextLower := i+1 < len(rs) && unicode.IsLower(rs[i+1])
			if unicode.IsLower(prev) || (unicode.IsUpper(prev) && nextLower) {
				b.WriteByte(' ')
			}
		}
		b.WriteRune(r)
	}
	return b.String()
}

func parseFont(xrd *model.XRefTable, obj types.Object) *pdfFont {
	d, err := xrd.DereferenceDict(obj)
	if err != nil || d == nil {
		return nil
	}
	f := &pdfFont{
		ascent:   0.80,
		descent:  0.20,
		codeLen:  1,
		toUni:    map[uint64]string{},
		widths:   map[int]float64{},
		defWidth: 500,
	}
	if bf := d.NameEntry("BaseFont"); bf != nil {
		f.family, f.bold, f.italic = normalizeFontName(*bf)
	}
	if enc := d.NameEntry("Encoding"); enc != nil {
		switch *enc {
		case "Identity-H", "Identity-V":
			f.codeLen = 2
		case "WinAnsiEncoding":
			f.enc = charmapTable(charmap.Windows1252)
		case "MacRomanEncoding":
			f.enc = charmapTable(charmap.Macintosh)
		case "StandardEncoding":
			f.enc = standardEncoding()
		}
	} else if o, ok := d.Find("Encoding"); ok {
		if ed, err := xrd.DereferenceDict(o); err == nil && ed != nil {
			f.enc = encodingFromDict(xrd, ed)
		}
	}
	if o, ok := d.Find("ToUnicode"); ok {
		applyToUnicode(xrd, o, f)
	}
	applyWidths(xrd, d, f)
	applyDescriptor(xrd, d, f)

	if dfs := d.ArrayEntry("DescendantFonts"); len(dfs) > 0 {
		dd, err := xrd.DereferenceDict(dfs[0])
		if err == nil && dd != nil {
			applyCIDWidths(xrd, dd, f)
			applyDescriptor(xrd, dd, f)
			if dw := numEntry(xrd, dd, "DW"); dw > 0 {
				f.defWidth = dw
			}
		}
	}
	if f.family == "" {
		f.family = "Arial"
	}
	return f
}

func applyDescriptor(xrd *model.XRefTable, d types.Dict, f *pdfFont) {
	o, ok := d.Find("FontDescriptor")
	if !ok {
		return
	}
	fd, err := xrd.DereferenceDict(o)
	if err != nil || fd == nil {
		return
	}
	if a := numEntry(xrd, fd, "Ascent"); a != 0 {
		f.ascent = emRatio(a)
	}
	if ds := numEntry(xrd, fd, "Descent"); ds != 0 {
		f.descent = math.Abs(emRatio(ds))
	}
	if mw := numEntry(xrd, fd, "MissingWidth"); mw > 0 {
		f.defWidth = mw
	}
	if fl := intEntry(xrd, fd, "Flags"); fl != 0 {
		if fl&(1<<6) != 0 {
			f.italic = true
		}
		if fl&(1<<18) != 0 {
			f.bold = true
		}
	}
	if f.family == "" || f.family == "Arial" {
		if n := fd.NameEntry("FontName"); n != nil {
			fam, b, i := normalizeFontName(*n)
			if fam != "" {
				f.family = fam
				f.bold = f.bold || b
				f.italic = f.italic || i
			}
		}
	}
}

func emRatio(v float64) float64 {
	if math.Abs(v) > 3 {
		return v / 1000
	}
	return v
}

func applyWidths(xrd *model.XRefTable, d types.Dict, f *pdfFont) {
	o, ok := d.Find("Widths")
	if !ok {
		return
	}
	arr, err := xrd.DereferenceArray(o)
	if err != nil || arr == nil {
		return
	}
	first := intEntry(xrd, d, "FirstChar")
	for i, el := range arr {
		if w, ok := objNum(xrd, el); ok {
			f.widths[first+i] = w
		}
	}
}

func applyCIDWidths(xrd *model.XRefTable, d types.Dict, f *pdfFont) {
	o, ok := d.Find("W")
	if !ok {
		return
	}
	arr, err := xrd.DereferenceArray(o)
	if err != nil || arr == nil {
		return
	}
	for i := 0; i < len(arr); {
		c0, ok := objNum(xrd, arr[i])
		if !ok {
			i++
			continue
		}
		i++
		if i >= len(arr) {
			break
		}
		if sub, err := xrd.DereferenceArray(arr[i]); err == nil && sub != nil && isArrayObj(arr[i], sub) {
			// Could be a real array element. DereferenceArray on a number fails.
			i++
			for j, wobj := range sub {
				if w, ok := objNum(xrd, wobj); ok {
					f.widths[int(c0)+j] = w
				}
			}
			continue
		}
		if i+1 >= len(arr) {
			break
		}
		c1, ok1 := objNum(xrd, arr[i])
		w, ok2 := objNum(xrd, arr[i+1])
		if !ok1 || !ok2 {
			i++
			continue
		}
		i += 2
		for c := int(c0); c <= int(c1); c++ {
			f.widths[c] = w
		}
	}
}

func isArrayObj(o types.Object, dereferenced types.Array) bool {
	switch o.(type) {
	case types.Array, types.IndirectRef:
		return dereferenced != nil
	default:
		return false
	}
}

func applyToUnicode(xrd *model.XRefTable, obj types.Object, f *pdfFont) {
	sd, _, err := xrd.DereferenceStreamDict(obj)
	if err != nil || sd == nil {
		return
	}
	if err := sd.Decode(); err != nil {
		return
	}
	parseCMap(string(sd.Content), f)
}

var (
	bfCharRe  = regexp.MustCompile(`(?s)beginbfchar\s*(.*?)\s*endbfchar`)
	bfRangeRe = regexp.MustCompile(`(?s)beginbfrange\s*(.*?)\s*endbfrange`)
	hexTokRe  = regexp.MustCompile(`<([0-9A-Fa-f]+)>`)
	codeSpace = regexp.MustCompile(`<([0-9A-Fa-f]+)>\s*<([0-9A-Fa-f]+)>`)
)

func parseCMap(s string, f *pdfFont) {
	if m := regexp.MustCompile(`(?s)begincodespacerange\s*(.*?)\s*endcodespacerange`).FindStringSubmatch(s); m != nil {
		if cs := codeSpace.FindStringSubmatch(m[1]); cs != nil {
			n := (len(cs[1]) + 1) / 2
			if n > f.codeLen {
				f.codeLen = n
			}
		}
	}
	for _, m := range bfCharRe.FindAllStringSubmatch(s, -1) {
		pairs := hexTokRe.FindAllStringSubmatch(m[1], -1)
		for i := 0; i+1 < len(pairs); i += 2 {
			code := hexToCode(pairs[i][1])
			f.toUni[code] = utf16Hex(pairs[i+1][1])
		}
	}
	for _, m := range bfRangeRe.FindAllStringSubmatch(s, -1) {
		parseBFRange(m[1], f)
	}
}

func parseBFRange(body string, f *pdfFont) {
	lines := strings.FieldsFunc(body, func(r rune) bool { return r == '\n' || r == '\r' })
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		hexes := hexTokRe.FindAllStringSubmatch(line, -1)
		if len(hexes) < 2 {
			continue
		}
		start := hexToCode(hexes[0][1])
		end := hexToCode(hexes[1][1])
		if len(hexes) >= 3 && !strings.Contains(line, "[") {
			dst := utf16Hex(hexes[2][1])
			rs := []rune(dst)
			if len(rs) == 0 {
				continue
			}
			for c, i := start, 0; c <= end; c, i = c+1, i+1 {
				f.toUni[c] = string(rs[0] + rune(i))
			}
			continue
		}
		// <start> <end> [<u1> <u2> ...]
		if len(hexes) > 3 || strings.Contains(line, "[") {
			dsts := hexes[2:]
			for i := 0; i < len(dsts) && start+uint64(i) <= end; i++ {
				f.toUni[start+uint64(i)] = utf16Hex(dsts[i][1])
			}
		}
	}
}

func hexToCode(h string) uint64 {
	if len(h)%2 == 1 {
		h = "0" + h
	}
	b, err := hex.DecodeString(h)
	if err != nil {
		return 0
	}
	var c uint64
	for _, by := range b {
		c = c<<8 | uint64(by)
	}
	return c
}

func utf16Hex(h string) string {
	if len(h)%2 == 1 {
		h = "0" + h
	}
	b, err := hex.DecodeString(h)
	if err != nil || len(b) == 0 {
		return ""
	}
	if len(b)%2 == 1 {
		b = append([]byte{0}, b...)
	}
	u := make([]uint16, len(b)/2)
	for i := range u {
		u[i] = uint16(b[i*2])<<8 | uint16(b[i*2+1])
	}
	return string(utf16.Decode(u))
}

func objNum(xrd *model.XRefTable, o types.Object) (float64, bool) {
	if xrd != nil && o != nil {
		if d, err := xrd.Dereference(o); err == nil && d != nil {
			o = d
		}
	}
	switch n := o.(type) {
	case types.Integer:
		return float64(n), true
	case types.Float:
		return float64(n), true
	case nil:
		return 0, false
	default:
		return 0, false
	}
}

func numEntry(xrd *model.XRefTable, d types.Dict, key string) float64 {
	o, ok := d.Find(key)
	if !ok {
		return 0
	}
	n, ok := objNum(xrd, o)
	if !ok {
		return 0
	}
	return n
}

func intEntry(xrd *model.XRefTable, d types.Dict, key string) int {
	return int(numEntry(xrd, d, key))
}
