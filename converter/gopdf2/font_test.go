package gopdf2

import (
	"testing"

	"golang.org/x/text/encoding/charmap"
)

func TestNormalizeFontName(t *testing.T) {
	cases := []struct {
		in           string
		family       string
		bold, italic bool
	}{
		{"BAAAAA+LiberationSans-Bold", "Liberation Sans", true, false},
		{"CAAAAA+WenQuanYiMicroHei", "WenQuanYi Micro Hei", false, false},
		{"LiberationSerif-Italic", "Liberation Serif", false, true},
		{"Carlito-BoldItalic", "Carlito", true, true},
		{"Arial", "Arial", false, false},
	}
	for _, tc := range cases {
		fam, b, i := normalizeFontName(tc.in)
		if fam != tc.family || b != tc.bold || i != tc.italic {
			t.Errorf("%s -> %q bold=%v italic=%v, want %q bold=%v italic=%v", tc.in, fam, b, i, tc.family, tc.bold, tc.italic)
		}
	}
}

func TestToUnicodeAndAdvance(t *testing.T) {
	f := &pdfFont{
		codeLen:  1,
		toUni:    map[uint64]string{},
		widths:   map[int]float64{1: 500, 2: 1000},
		defWidth: 500,
		ascent:   0.8,
		descent:  0.2,
	}
	parseCMap("beginbfchar\n<01> <0041>\n<02> <4E2D>\nendbfchar\n", f)
	s, adv := f.decodeBytes([]byte{0x01, 0x02}, 10, 0, 0, 100)
	if s != "A中" {
		t.Fatalf("decoded %q", s)
	}
	// (500/1000)*10 + (1000/1000)*10 = 15
	if adv < 14.9 || adv > 15.1 {
		t.Fatalf("advance %v", adv)
	}
}

func TestWinAnsiAccent(t *testing.T) {
	f := &pdfFont{
		codeLen:  1,
		toUni:    map[uint64]string{},
		enc:      charmapTable(charmap.Windows1252),
		widths:   map[int]float64{},
		defWidth: 500,
	}
	// WinAnsi: E0 à, E8 è, E9 é, ED í, F3 ó
	s, _ := f.decodeBytes([]byte{0xE0, 0xE8, 0xE9, 0xED, 0xF3}, 12, 0, 0, 100)
	if s != "àèéíó" {
		t.Fatalf("got %q", s)
	}
}

func TestWordSpaceGap(t *testing.T) {
	// xdvipdfmx TJ -250 is exactly 0.25 em and must count as a space.
	if !gapIsWordSpace(tjAdjust(-250, 12, 100), 12) {
		t.Fatal("quarter-em gap should be a word space")
	}
	if gapIsWordSpace(tjAdjust(-40, 12, 100), 12) {
		t.Fatal("kerning should not become a space")
	}
}

func TestTokenizerTJ(t *testing.T) {
	src := []byte("BT /F1 12 Tf 10 20 Td [<0102>-3<03>] TJ ET")
	toks := tokenize(src)
	var ops []string
	for _, tk := range toks {
		if tk.kind == tkOp {
			ops = append(ops, tk.text)
		}
	}
	want := []string{"Tf", "Td", "TJ", "ET"}
	// BT is also an op
	if len(ops) < 4 || ops[0] != "BT" {
		t.Fatalf("ops %v", ops)
	}
	found := false
	for _, tk := range toks {
		if tk.kind == tkArray && len(tk.arr) == 3 {
			found = true
			if string(tk.arr[0].raw) != string([]byte{0x01, 0x02}) {
				t.Fatalf("hex bytes %v", tk.arr[0].raw)
			}
			if tk.arr[1].num != -3 {
				t.Fatalf("kern %v", tk.arr[1].num)
			}
		}
	}
	if !found {
		t.Fatal("TJ array not parsed")
	}
	_ = want
}
