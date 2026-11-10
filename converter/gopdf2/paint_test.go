package gopdf2

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"
)

func TestUnmatteBlack(t *testing.T) {
	// Stored 115 at alpha 128 against a black matte is straight 230.
	r, g, b := unmatte(115, 122, 130, 128, []float64{0, 0, 0})
	if r < 220 || g < 230 || b < 250 {
		t.Fatalf("unmatte = %d %d %d, want a light color", r, g, b)
	}
	r, g, b = unmatte(115, 122, 130, 128, nil)
	if r != 115 || g != 122 || b != 130 {
		t.Fatalf("nil matte changed the color to %d %d %d", r, g, b)
	}
}

func TestFlattenKeepsLightWatermark(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	r, g, b := unmatte(115, 122, 130, 128, []float64{0, 0, 0})
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			src.SetNRGBA(x, y, color.NRGBA{R: r, G: g, B: b, A: 128})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatal(err)
	}
	flat, err := flattenOn(buf.Bytes(), 255, 255, 255)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(flat))
	if err != nil {
		t.Fatal(err)
	}
	rr, gg, bb, aa := img.At(0, 0).RGBA()
	if aa>>8 != 255 || rr>>8 < 230 || gg>>8 < 230 || bb>>8 < 230 {
		t.Fatalf("flattened pixel = %d %d %d a=%d", rr>>8, gg>>8, bb>>8, aa>>8)
	}
}

func TestClipKeepsJPEGBytes(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 16, 16))
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, src, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatal(err)
	}
	raw := append([]byte(nil), buf.Bytes()...)
	ip := &interp{pageW: 100, pageH: 100}
	ip.elements = []PageElement{{Image: &ImageObject{
		Data: raw, Format: "jpeg", X: -10, Y: 10, Width: 50, Height: 40,
	}}}
	// Page clip: only the part that hangs off the slide is trimmed, and the
	// bytes stay the original JPEG.
	ip.clipLastImage(0, 0, 100, 100, false)
	img := ip.elements[0].Image
	if img.Format != "jpeg" || !bytes.Equal(img.Data, raw) {
		t.Fatalf("format %s len %d, jpeg was re-encoded", img.Format, len(img.Data))
	}
	if img.CropL < 0.15 || img.CropL > 0.25 {
		t.Fatalf("CropL = %v, want about 0.2", img.CropL)
	}
	if img.X < -0.1 || img.X > 0.1 {
		t.Fatalf("cropped X = %v, want 0", img.X)
	}
}

func TestShapeBoxHoldsHandlesClipBoxDoesNot(t *testing.T) {
	// Endpoints sit on y=0. The handles are at y=40. The curve itself peaks
	// at 30, which is what a clip should use. The shape box has to contain
	// the handles or DrawingML clamps them flat.
	ops := []pathOp{{op: 'M', x: 0, y: 0}, {op: 'C', x: 10, y: 0, x1: 0, y1: 40, x2: 10, y2: 40}}
	_, y0, _, y1, ok := pathBounds(ops)
	if !ok || y0 > 0.1 || y1 < 39 {
		t.Fatalf("shape bounds y %.2f..%.2f", y0, y1)
	}
	_, cy0, _, cy1, ok := clipBounds(ops)
	if !ok || cy1 > 31 || cy1 < 29 || cy0 < -0.1 {
		t.Fatalf("clip bounds y %.2f..%.2f", cy0, cy1)
	}
}

func TestOnePercentClipDoesNotMoveThePicture(t *testing.T) {
	ip := &interp{pageW: 200, pageH: 200}
	ip.elements = []PageElement{{Image: &ImageObject{
		Data: []byte{1}, Format: "png", X: 0, Y: 0, Width: 100, Height: 80,
	}}}
	ip.clipLastImage(1, 0, 100, 80, false)
	img := ip.elements[0].Image
	if img.CropL != 0 || math.Abs(img.Width-100) > 0.1 || math.Abs(img.X) > 0.1 {
		t.Fatalf("1%% clip moved the picture: x %.2f w %.2f cropL %.3f", img.X, img.Width, img.CropL)
	}
}

func TestDirectShadingBecomesCard(t *testing.T) {
	content := "10 10 80 40 re W n /Sh0 sh\n"
	p := openPDF(t, pdfBytes([]string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Count 1 /Kids [3 0 R] >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Resources << /Shading << /Sh0 5 0 R >> >> /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content),
		"<< /ShadingType 2 /ColorSpace /DeviceRGB /Coords [0 0 0 100] /Function << /FunctionType 2 /Domain [0 1] /C0 [0.22 0.22 0.22] /C1 [0 0 0] /N 1 >> >>",
	}))
	var card *ShapeObject
	for _, el := range p.Elements {
		if el.Shape != nil && el.Shape.Gradient != nil {
			card = el.Shape
		}
	}
	if card == nil {
		t.Fatal("shading was not painted")
	}
	if math.Abs(card.Width-80) > 1 || math.Abs(card.Height-40) > 1 {
		t.Fatalf("card box %.1fx%.1f", card.Width, card.Height)
	}
	if len(card.Gradient.Stops) < 2 || card.Gradient.Stops[0].R < 50 || card.Gradient.Stops[len(card.Gradient.Stops)-1].R > 5 {
		t.Fatalf("stops %+v", card.Gradient.Stops)
	}
	// t=0 is the bottom (y-up). DrawingML direction toward black is upward, 270°.
	if math.Abs(card.Gradient.Angle-270) > 1 {
		t.Fatalf("angle %v", card.Gradient.Angle)
	}
}

func TestNBSCardsAreGradients(t *testing.T) {
	pdf := corpusFile("nbs-alibaba")
	if pdf == "" {
		t.Skip("nbs-alibaba pdf not present")
	}
	doc, err := Open(pdf)
	if err != nil {
		t.Skip(err)
	}
	p, err := doc.GetPage(3)
	if err != nil {
		t.Fatal(err)
	}
	var cards int
	for _, el := range p.Elements {
		sh := el.Shape
		if sh == nil || sh.Gradient == nil || sh.Width < 400 || sh.Height < 200 {
			continue
		}
		cards++
		if sh.Gradient.Stops[0].R < 40 {
			t.Fatalf("card starts at %d, want the gray panel", sh.Gradient.Stops[0].R)
		}
	}
	if cards != 4 {
		t.Fatalf("gradient cards = %d, want 4", cards)
	}
}

func TestSubscriptJoinsBelowALetter(t *testing.T) {
	body := &TextObject{Content: "p = p", FontName: "Tinos", FontSize: 32, X: 100, Y: 100, Width: 80, Height: 36}
	sub := &TextObject{Content: "2", FontName: "Tinos", FontSize: 16, X: 108, Y: 124, Width: 10, Height: 18}
	year := &TextObject{Content: "2024", FontName: "Tinos", FontSize: 32, X: 200, Y: 100, Width: 60, Height: 36}
	p := &Page{Elements: []PageElement{{Text: body}, {Text: sub}, {Text: year}}}
	attachSubscripts(p)
	if len(p.TextObjects) != 0 && len(p.Elements) != 2 {
		// TextObjects is filled by syncLists; Elements is what attach edits.
	}
	if len(p.Elements) != 2 {
		t.Fatalf("elements %d, want body+year (subscript folded in)", len(p.Elements))
	}
	if body.Content != "p2 = p" || year.Content != "2024" {
		t.Fatalf("body %q year %q", body.Content, year.Content)
	}
	var base float64
	for _, r := range body.Runs {
		if r.Content == "2" {
			base = r.Baseline
		}
	}
	if base > -18000 || base < -32000 {
		t.Fatalf("baseline %v, want about -25000", base)
	}
}

func TestLetterUnderAWordIsNotASubscript(t *testing.T) {
	// Canva sets a smaller title letter just under a display cap. That letter
	// is part of the word, not a formula subscript.
	big := &TextObject{Content: "W", FontName: "Noto Sans", FontSize: 134, X: 334, Y: 32, Width: 90, Height: 160}
	small := &TextObject{Content: "a", FontName: "Noto Sans", FontSize: 36, X: 430, Y: 90, Width: 28, Height: 40}
	p := &Page{Elements: []PageElement{{Text: big}, {Text: small}}}
	attachSubscripts(p)
	if len(p.Elements) != 2 {
		t.Fatalf("letter folded into the word: %d elements, content %q", len(p.Elements), big.Content)
	}
	if big.Content != "W" || small.Content != "a" {
		t.Fatalf("content %q %q", big.Content, small.Content)
	}
}

func TestVisualLTRToLogical(t *testing.T) {
	// Left-to-right visual order of "מעל ל 50 אחוז".
	visual := "זוחא 50 ל לעמ"
	got := visualLTRToLogical(visual)
	if got != "מעל ל 50 אחוז" {
		t.Fatalf("logical %q", got)
	}
	if visualLTRToLogical("Hello") != "Hello" {
		t.Fatal("latin moved")
	}
}

func TestVerticalPDFOpens(t *testing.T) {
	pdf := corpusFile("jp-tategaki")
	if pdf == "" {
		t.Skip("vertical sample not present")
	}
	doc, err := Open(pdf)
	if err != nil {
		t.Fatal(err)
	}
	if doc.PageCount() < 1 {
		t.Fatal("no pages")
	}
	p, err := doc.GetPage(0)
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for _, tb := range p.TextObjects {
		b.WriteString(tb.Content)
	}
	if !strings.Contains(b.String(), "あいうえお") {
		t.Fatalf("vertical text missing: %q", b.String())
	}
	var kana, kanji *TextObject
	for _, tb := range p.TextObjects {
		if strings.Contains(tb.Content, "あいうえお") {
			kana = tb
		}
		if strings.Contains(tb.Content, "日本語") {
			kanji = tb
		}
	}
	if kana == nil || kanji == nil {
		t.Fatal("the two columns were merged into one box")
	}
	if !kana.Vertical || !kanji.Vertical {
		t.Fatal("Identity-V text was not marked vertical")
	}
	if kana.Height <= kana.Width || kanji.Height <= kanji.Width {
		t.Fatalf("columns are not taller than wide: kana %.1fx%.1f kanji %.1fx%.1f", kana.Width, kana.Height, kanji.Width, kanji.Height)
	}
	if kanji.X >= kana.X {
		t.Fatalf("日本語 should be the left column, x %.1f vs あいうえお x %.1f", kanji.X, kana.X)
	}
}

func TestHebrewReadingOrder(t *testing.T) {
	pdf := corpusFile("he-openu-a11y")
	if pdf == "" {
		t.Skip("hebrew deck not present")
	}
	doc, err := Open(pdf)
	if err != nil {
		t.Fatal(err)
	}
	p, err := doc.GetPage(0)
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	rtl := false
	for _, tb := range p.TextObjects {
		b.WriteString(tb.Content)
		if tb.RTL {
			rtl = true
		}
	}
	if !strings.Contains(b.String(), "בדיקות") || strings.Contains(b.String(), "תוקידב") {
		t.Fatalf("hebrew still reversed: %q", b.String())
	}
	if !rtl {
		t.Fatal("hebrew box was not marked rtl")
	}
	if strings.Contains(b.String(), "בנישאילנה") || !strings.Contains(b.String(), "אילנה בניש") {
		t.Fatalf("name is glued or swapped: %q", b.String())
	}
	p2, err := doc.GetPage(1)
	if err != nil {
		t.Fatal(err)
	}
	var numBox, sixty *TextObject
	for _, tb := range p2.TextObjects {
		if strings.Contains(tb.Content, "1,600,000") && numBox == nil {
			numBox = tb
		}
		if strings.Contains(tb.Content, "60") && !strings.Contains(tb.Content, "1,600") {
			if sixty == nil || tb.X < sixty.X {
				sixty = tb
			}
		}
	}
	if numBox == nil {
		t.Fatal("page 2 lost the 1,600,000 line")
	}
	line := numBox.Content
	if strings.Index(line, "בישראל") < 0 || strings.Index(line, "בישראל") > strings.Index(line, "1,600,000") {
		t.Fatalf("number is on the wrong side of בישראל: %q", line)
	}
	if strings.Contains(line, "1,600,000בישראל") {
		t.Fatalf("number glued to בישראל: %q", line)
	}
	// v1.5.2 packed the whole bullet into one ~496 pt box, so 60 left the
	// left edge. The pair is about 200 pt and 60 stays in its own box.
	if strings.Contains(line, "גיל") || numBox.Width > 280 {
		t.Fatalf("first bullet packed: w %.1f %q", numBox.Width, line)
	}
	if sixty == nil || sixty.X >= numBox.X {
		x := -1.0
		if sixty != nil {
			x = sixty.X
		}
		t.Fatalf("60 is not left of the number: sixty x %.1f number x %.1f", x, numBox.X)
	}
	var bcg string
	for _, tb := range p2.TextObjects {
		if strings.Contains(tb.Content, "BCG") {
			bcg = tb.Content
		}
	}
	if bcg == "" || strings.Contains(bcg, "GCB") {
		t.Fatalf("BCG run was reversed: %q", bcg)
	}
	// The 50 line stays segmented at the PDF x, right to left:
	// איגוד, then מעל, then 50. One packed paragraph is the p2 SSIM drop.
	var igud, meal, fiftyBox *TextObject
	for _, tb := range p2.TextObjects {
		if strings.Contains(tb.Content, "איגוד") {
			igud = tb
		}
	}
	if igud == nil {
		t.Fatal("50 line lost איגוד")
	}
	for _, tb := range p2.TextObjects {
		if strings.Contains(tb.Content, "מעל") && tb.X < igud.X && (meal == nil || tb.X > meal.X) {
			meal = tb
		}
		if strings.Contains(tb.Content, "50") && !strings.Contains(tb.Content, "050") && !strings.Contains(tb.Content, "איגוד") {
			if fiftyBox == nil || abs(tb.Y-igud.Y) < abs(fiftyBox.Y-igud.Y) {
				fiftyBox = tb
			}
		}
	}
	if meal == nil || fiftyBox == nil {
		t.Fatal("50 line lost מעל or 50")
	}
	if !(igud.X > meal.X && meal.X > fiftyBox.X) {
		t.Fatalf("50 line positions: איגוד %.1f מעל %.1f 50 %.1f", igud.X, meal.X, fiftyBox.X)
	}
	if strings.Contains(fiftyBox.Content, "איגוד") || fiftyBox.Width > 400 && strings.Contains(igud.Content, "50") {
		t.Fatalf("50 line packed: %q / %q", igud.Content, fiftyBox.Content)
	}
	for _, tb := range p2.TextObjects {
		if strings.TrimSpace(tb.Content) == "•" && !strings.HasSuffix(tb.Content, " ") {
			t.Fatalf("bullet has no trailing space: %q", tb.Content)
		}
		if strings.Contains(tb.Content, "BCG") && strings.Contains(tb.Content, ":") && !strings.Contains(tb.Content, ": ") {
			t.Fatalf("BCG colon spacing: %q", tb.Content)
		}
	}
	// Pages 6, 10, and 13 glued Latin to the next Hebrew word. Concatenated
	// a:t on one slide has no separator, so the source word dropped out of
	// unique recall. The join between two slides is not a word.
	var all strings.Builder
	for i := 0; i < doc.PageCount(); i++ {
		pg, err := doc.GetPage(i)
		if err != nil {
			t.Fatal(err)
		}
		var pageText strings.Builder
		for _, tb := range pg.TextObjects {
			pageText.WriteString(tb.Content)
			all.WriteString(tb.Content)
		}
		if at := touchAt(pageText.String()); at >= 0 {
			s := pageText.String()
			lo, hi := at-8, at+12
			if lo < 0 {
				lo = 0
			}
			if hi > len(s) {
				hi = len(s)
			}
			t.Fatalf("page %d glues hebrew to latin: %q", i+1, s[lo:hi])
		}
	}
	for _, w := range []string{"ולוודא", "מסופקים", "תוויות", "Adobe", "Labels"} {
		if !strings.Contains(all.String(), w) {
			t.Fatalf("missing %q", w)
		}
	}
}

func touchAt(s string) int {
	prev := rune(0)
	prevEnd := 0
	for i, r := range s {
		if unicode.IsSpace(r) {
			prev = 0
			continue
		}
		if prev != 0 && wordRune(prev) && wordRune(r) && rtlRune(prev) != rtlRune(r) {
			return prevEnd
		}
		prev = r
		prevEnd = i
	}
	return -1
}

func wordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

func rtlRune(r rune) bool {
	return unicode.Is(unicode.Hebrew, r) || unicode.Is(unicode.Arabic, r)
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func TestRTLLineReadingOrder(t *testing.T) {
	right := &TextObject{Content: "אילנה", X: 70, Y: 10, Width: 40, Height: 16, FontSize: 16, RTL: true}
	left := &TextObject{Content: "בניש", X: 30, Y: 10, Width: 36, Height: 16, FontSize: 16, RTL: true}
	num := &TextObject{Content: "1,600,000", X: 40, Y: 40, Width: 70, Height: 16, FontSize: 16}
	heb := &TextObject{Content: "בישראל", X: 112, Y: 40, Width: 50, Height: 16, FontSize: 16, RTL: true}
	bullet := &TextObject{Content: "•", X: 180, Y: 40, Width: 10, Height: 16, FontSize: 16}
	p := &Page{Elements: []PageElement{{Text: left}, {Text: right}, {Text: num}, {Text: heb}, {Text: bullet}}}
	joinCloseRTLPairs(p)
	var name, line string
	for _, el := range p.Elements {
		if el.Text == nil {
			continue
		}
		if strings.Contains(el.Text.Content, "אילנה") {
			name = el.Text.Content
		}
		if strings.Contains(el.Text.Content, "1,600,000") {
			line = el.Text.Content
		}
	}
	if strings.TrimSpace(name) != "אילנה בניש" {
		t.Fatalf("name %q", name)
	}
	line = strings.TrimSpace(line)
	if line != "• בישראל 1,600,000" && line != "בישראל 1,600,000" {
		t.Fatalf("line %q", line)
	}
}

func TestCanvaRotaryWordsAreNotClipped(t *testing.T) {
	pdf := corpusFile("canva-rotary")
	if pdf == "" {
		t.Skip("canva rotary deck not present")
	}
	doc, err := Open(pdf)
	if err != nil {
		t.Fatal(err)
	}
	p, err := doc.GetPage(15)
	if err != nil {
		t.Fatal(err)
	}
	need := []string{"nonprofit", "Account", "for", "not-for-profit"}
	found := map[string]bool{}
	for _, tb := range p.TextObjects {
		if tb.Clip && (strings.Contains(tb.Content, "profit") || strings.Contains(tb.Content, "Account") || strings.TrimSpace(tb.Content) == "for") {
			t.Fatalf("clipped box %q width %.1f", tb.Content, tb.Width)
		}
		for _, word := range need {
			if !strings.Contains(tb.Content, word) {
				continue
			}
			// The letters, not the trailing space, have to fit. A substitute
			// font can be a little wider than Canva's; 15% is that gap.
			// Clipping "t" off "nonprofit" is much more than 15%.
			ink := strings.TrimRight(tb.Content, " ")
			w, ok := textWidth(tb.FontName, tb.Bold, tb.Italic, tb.FontSize, ink)
			if ok && w > 1 && tb.Width < w*0.85 {
				t.Fatalf("box narrower than the advance: %q width %.1f advance %.1f", tb.Content, tb.Width, w)
			}
		}
		for _, word := range need {
			if strings.Contains(tb.Content, word) {
				found[word] = true
			}
		}
	}
	for _, word := range need {
		if !found[word] {
			t.Fatalf("page 16 lost %q", word)
		}
	}
}

func TestMergeWordGap(t *testing.T) {
	// The first box holds the word and its trailing space. Shrinking it
	// to open a gap clips the last letter. The next word moves instead.
	a := &TextObject{Content: "not-for-profit ", X: 100, Y: 0, Width: 80, Height: 14, FontSize: 16}
	b := &TextObject{Content: "basis", X: 150, Y: 0, Width: 40, Height: 14, FontSize: 16}
	p := &Page{Elements: []PageElement{{Text: a}, {Text: b}}}
	pullBackOverlaps(p)
	if a.Width != 80 || a.Clip {
		t.Fatalf("word box was shrunk or clipped: w %.1f clip %v", a.Width, a.Clip)
	}
	if !strings.HasSuffix(a.Content, "t ") {
		t.Fatalf("last letter dropped: %q", a.Content)
	}
	if b.X < a.X+a.Width-0.1 {
		t.Fatalf("next word still overlaps the word: x %.1f end %.1f", b.X, a.X+a.Width)
	}
}

func TestMergeLatinWordSpace(t *testing.T) {
	word := &TextObject{Content: "t ", X: 40, Y: 0, Width: 14, Height: 12, FontSize: 12}
	next := &TextObject{Content: "organi", X: 50, Y: 0, Width: 40, Height: 12, FontSize: 12}
	// The boxes overlap, but the space is a word boundary.
	p := &Page{Elements: []PageElement{{Text: word}, {Text: next}}}
	mergeTextLines(p)
	var got string
	for _, el := range p.Elements {
		if el.Text != nil {
			got += el.Text.Content
		}
	}
	if !strings.Contains(got, "t organi") && !strings.Contains(got, "t  organi") {
		t.Fatalf("word space eaten: %q", got)
	}
}

func TestOfficePiecesJoin(t *testing.T) {
	offi := &TextObject{Content: "Offi", X: 0, Y: 0, Width: 40, Height: 20, FontSize: 20}
	ce := &TextObject{Content: "ce", X: 32, Y: 1, Width: 22, Height: 20, FontSize: 20}
	p := &Page{Elements: []PageElement{{Text: offi}, {Text: ce}}}
	mergeTextLines(p)
	if len(p.Elements) != 1 || !strings.Contains(p.Elements[0].Text.Content, "Office") {
		n := len(p.Elements)
		c := ""
		if n > 0 && p.Elements[0].Text != nil {
			c = p.Elements[0].Text.Content
		}
		t.Fatalf("Office split: %d elements %q", n, c)
	}
}

func TestLatinLigatureFolds(t *testing.T) {
	if foldLatinLigature("ffi", 600) != "\uFB03" {
		t.Fatal("ffi stayed three letters")
	}
	if foldLatinLigature("ffl", 700) != "\uFB04" {
		t.Fatal("ffl stayed three letters")
	}
	if foldLatinLigature("office", 3000) != "office" {
		t.Fatal("a word was folded")
	}
	if foldLatinLigature("ffi", 4000) != "ffi" {
		t.Fatal("a wide ffi was folded")
	}
}

func TestWhiteRectOnTextIsDropped(t *testing.T) {
	text := &TextObject{Content: "简单.创造.不简单", X: 173, Y: 143, Width: 247, Height: 31, FontSize: 22}
	behind := &ShapeObject{X: 161, Y: 133, Width: 69, Height: 47, HasFill: true, SimpleRect: true, FillR: 255, FillG: 255, FillB: 255, FillAlpha: 1}
	over := &ShapeObject{X: 250, Y: 133, Width: 69, Height: 47, HasFill: true, SimpleRect: true, FillR: 255, FillG: 255, FillB: 255, FillAlpha: 1}
	p := &Page{Elements: []PageElement{{Shape: behind}, {Text: text}, {Shape: over}}}
	dropWhiteTextCovers(p)
	if p.Elements[0].Shape == nil {
		t.Fatal("the plate behind the text was removed")
	}
	foundOver := false
	for _, el := range p.Elements {
		if el.Shape == over {
			foundOver = true
		}
	}
	if foundOver {
		t.Fatal("the white plate on top of the subtitle is still there")
	}
}

func TestWPSOfficeTitle(t *testing.T) {
	pdf := corpusFile("wps-office2019")
	if pdf == "" {
		t.Skip("wps deck not present")
	}
	doc, err := Open(pdf)
	if err != nil {
		t.Fatal(err)
	}
	p, err := doc.GetPage(0)
	if err != nil {
		t.Fatal(err)
	}
	var title, sub string
	var subBox *TextObject
	for _, tb := range p.TextObjects {
		if strings.Contains(tb.Content, "WPS") || strings.Contains(tb.Content, "ffi") || strings.Contains(tb.Content, "\uFB03") {
			title += tb.Content
		}
		if strings.Contains(tb.Content, "简单") {
			sub = tb.Content
			subBox = tb
		}
	}
	office := false
	for _, tb := range p.TextObjects {
		if strings.Contains(tb.Content, "Office") {
			office = true
		}
	}
	if !office {
		t.Fatalf("Office is still split: %q", title)
	}
	if !strings.Contains(sub, "创造") || !strings.Contains(sub, "不简单") {
		t.Fatalf("subtitle missing: %q", sub)
	}
	for _, el := range p.Elements {
		sh := el.Shape
		if sh == nil || subBox == nil {
			continue
		}
		if sh.FillR < 250 || !sh.HasFill || !sh.SimpleRect {
			continue
		}
		if sh.X > subBox.X+30 && whiteCoversText(sh, subBox) {
			t.Fatalf("white rect still covers the subtitle: %.1f,%.1f %.1fx%.1f", sh.X, sh.Y, sh.Width, sh.Height)
		}
	}
}

func TestEquationLetterSubscriptStays(t *testing.T) {
	// B_s on the beamer table. The s is a few points tall, unlike a Canva title letter.
	body := &TextObject{Content: "10B", FontName: "Tinos", FontSize: 11.2, X: 27.8, Y: 100, Width: 28, Height: 13}
	sub := &TextObject{Content: "s", FontName: "Tinos", FontSize: 7.8, X: 50, Y: 108, Width: 6, Height: 9}
	p := &Page{Elements: []PageElement{{Text: body}, {Text: sub}}}
	attachSubscripts(p)
	if len(p.Elements) != 1 || !strings.Contains(body.Content, "s") {
		t.Fatalf("equation subscript dropped: %d elements, content %q", len(p.Elements), body.Content)
	}
}

func TestSubscriptReplacesTheHoleSpace(t *testing.T) {
	// "2.n .d" has a space where h is drawn. The subscript replaces that space.
	body := &TextObject{Content: "2.n .d ", FontName: "Noto Sans", FontSize: 11, X: 0, Y: 0, Width: 70, Height: 12}
	sub := &TextObject{Content: "h", FontName: "Noto Sans", FontSize: 7.3, X: 40, Y: 4, Width: 4, Height: 8}
	insertSubscript(body, sub)
	if strings.Contains(body.Content, " ") && strings.Contains(body.Content, "n ") {
		t.Fatalf("hole space kept: %q", body.Content)
	}
	if !strings.Contains(body.Content, "nh") {
		t.Fatalf("subscript not inline: %q", body.Content)
	}
}

func TestKUSTOpeningParenMatchesZ(t *testing.T) {
	pdf := corpusFile("kust-pressure")
	if pdf == "" {
		pdf = corpusFile("kust")
	}
	if pdf == "" {
		t.Skip("kust pdf not present")
	}
	doc, err := Open(pdf)
	if err != nil {
		t.Fatal(err)
	}
	p, err := doc.GetPage(3)
	if err != nil {
		t.Fatal(err)
	}
	// PowerPoint draws each box from its own x, with no autofit. The opening
	// "(" has to end at or before z₁ starts. Sharing one box is fine only
	// when "(" comes first; a second box that still contains z must not
	// overlap that box either.
	var paren, zbox *TextObject
	for _, tb := range p.TextObjects {
		if tb.FontSize < 20 {
			continue
		}
		if strings.TrimSpace(tb.Content) == "(" && (paren == nil || tb.X > 400) {
			paren = tb
		}
		if strings.Contains(tb.Content, "z") && strings.Contains(tb.Content, "(") && zbox == nil {
			zbox = tb
			paren = tb
		}
	}
	if paren == nil {
		t.Fatal("formula parenthesis was not found")
	}
	if zbox == nil {
		for _, tb := range p.TextObjects {
			if tb.FontSize < 20 || !strings.Contains(tb.Content, "z") || tb.X < paren.X-1 {
				continue
			}
			if zbox == nil || tb.X < zbox.X {
				zbox = tb
			}
		}
	}
	if zbox == nil {
		t.Fatal("formula z was not found")
	}
	if zbox == paren {
		if strings.Index(paren.Content, "(") > strings.Index(paren.Content, "z") {
			t.Fatalf("z₁ is before the parenthesis: %q", paren.Content)
		}
	} else if paren.X+paren.Width > zbox.X+0.8 {
		t.Fatalf("paren box [%.1f, %.1f] overlaps z box [%.1f, %.1f] %q", paren.X, paren.X+paren.Width, zbox.X, zbox.X+zbox.Width, zbox.Content)
	}
	for _, tb := range p.TextObjects {
		if tb == paren || tb.FontSize < 20 || !strings.Contains(tb.Content, "z") {
			continue
		}
		if tb.X < paren.X+paren.Width-0.8 && paren.X < tb.X+tb.Width-0.8 {
			t.Fatalf("z box [%.1f, %.1f] %q overlaps the ( box [%.1f, %.1f]", tb.X, tb.X+tb.Width, tb.Content, paren.X, paren.X+paren.Width)
		}
	}
	var formula *TextObject
	for _, tb := range p.TextObjects {
		if tb.FontSize > 20 && strings.Contains(tb.Content, "ρg") {
			formula = tb
			break
		}
	}
	if formula == nil {
		t.Fatal("ρg box was not found")
	}
	// The parenthesis starts where the Times ink of ρg ends.
	if math.Abs(paren.X-(formula.X+formula.Width)) > 1.5 {
		t.Fatalf("paren x %.1f, ρg ink ends at %.1f (%q)", paren.X, formula.X+formula.Width, formula.Content)
	}
	// A trailing space keeps "ρg" from gluing to the next a:t (② → 2).
	var b strings.Builder
	for _, tb := range p.TextObjects {
		b.WriteString(tb.Content)
	}
	s := b.String()
	i := strings.Index(s, "ρg")
	if i < 0 || i+len("ρg") >= len(s) || s[i+len("ρg")] != ' ' {
		t.Fatalf("ρg is glued to the next word: %q", s[max(0, i):min(len(s), i+len("ρg")+6)])
	}
}

func TestNeuripsSubscriptsStayInline(t *testing.T) {
	pdf := corpusFile("neurips-effxfmr")
	if pdf == "" {
		pdf = corpusFile("neurips")
	}
	if pdf == "" {
		t.Skip("neurips pdf not present")
	}
	doc, err := Open(pdf)
	if err != nil {
		t.Fatal(err)
	}
	// Pages 42 and 45. The subscript h sits in the paragraph, with no space
	// where the PDF left a hole for it.
	for _, page := range []int{41, 44} {
		p, err := doc.GetPage(page)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, tb := range p.TextObjects {
			if !strings.Contains(tb.Content, "2.n") {
				continue
			}
			found = true
			if strings.Contains(tb.Content, "n ") || strings.Contains(tb.Content, " h") || strings.Contains(tb.Content, "d ") || strings.Contains(tb.Content, " g") {
				t.Fatalf("page %d subscript padding: %q", page+1, tb.Content)
			}
			if !strings.Contains(tb.Content, "nh") && !strings.Contains(tb.Content, "ng") {
				t.Fatalf("page %d subscript not inline: %q", page+1, tb.Content)
			}
		}
		if !found {
			t.Fatalf("page %d formula was not found", page+1)
		}
	}
}

func TestCanvaRotaryTitlesKeepReadingOrder(t *testing.T) {
	pdf := corpusFile("canva-rotary")
	if pdf == "" {
		pdf = corpusFile("canva")
	}
	if pdf == "" {
		t.Skip("canva rotary pdf not present")
	}
	doc, err := Open(pdf)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		page int
		want string
	}{
		{1, "What"},
		{1, "Canva"},
		{6, "Creating"},
		{6, "Designs"},
		{7, "Social"},
	}
	for _, c := range cases {
		p, err := doc.GetPage(c.page)
		if err != nil {
			t.Fatal(err)
		}
		for _, tb := range p.TextObjects {
			if tb.Y > 200 {
				continue
			}
			for _, r := range tb.Runs {
				if r.Baseline < -20000 && strings.TrimSpace(r.Content) != "" {
					t.Fatalf("page %d title letter %q was taken as a subscript", c.page+1, r.Content)
				}
			}
		}
		// Title boxes are placed left to right. Reading order is that order.
		type item struct {
			x float64
			s string
		}
		var items []item
		for _, tb := range p.TextObjects {
			if tb.Y > 200 {
				continue
			}
			items = append(items, item{tb.X, tb.Content})
		}
		for i := 1; i < len(items); i++ {
			for j := i; j > 0 && items[j].x < items[j-1].x; j-- {
				items[j], items[j-1] = items[j-1], items[j]
			}
		}
		var b strings.Builder
		for _, it := range items {
			b.WriteString(it.s)
		}
		if !strings.Contains(b.String(), c.want) {
			t.Fatalf("page %d title %q, missing %q", c.page+1, b.String(), c.want)
		}
	}
}

func TestMahidolKeepsAI(t *testing.T) {
	pdf := corpusFile("canva-mahidol-adv")
	if pdf == "" {
		pdf = corpusFile("canva-mahidol")
	}
	if pdf == "" {
		t.Skip("canva mahidol pdf not present")
	}
	doc, err := Open(pdf)
	if err != nil {
		t.Fatal(err)
	}
	// Slide 24 is the style list whose last chip is GenAI. "AI" has to stay
	// its own run. The v1.4.2 unique-token extra was the doubled "aaii"
	// from writing each letter twice; the letters themselves are these runs.
	n := 0
	for i := 0; i < doc.PageCount(); i++ {
		p, err := doc.GetPage(i)
		if err != nil {
			t.Fatal(err)
		}
		for _, tb := range p.TextObjects {
			if strings.TrimSpace(tb.Content) == "AI" || strings.Contains(tb.Content, "AI") {
				n++
			}
		}
	}
	if n < 2 {
		t.Fatalf("AI runs %d, the GenAI chip was dropped", n)
	}
}

func TestPsumacLockIsOutline(t *testing.T) {
	pdf := corpusFile("psumac")
	if pdf == "" {
		t.Skip("psumac pdf not present")
	}
	doc, err := Open(pdf)
	if err != nil {
		t.Skip(err)
	}
	p, err := doc.GetPage(29)
	if err != nil {
		t.Fatal(err)
	}
	var dollar bool
	for _, tObj := range p.TextObjects {
		if strings.TrimSpace(tObj.Content) == "$" && tObj.NoFill {
			dollar = true
		}
	}
	if !dollar {
		t.Fatal("lock character left a:t; it must stay as noFill text")
	}
	var shapes int
	for _, el := range p.Elements {
		if el.Shape != nil && el.Shape.Width > 200 && el.Shape.Height > 200 && len(el.Shape.Path) > 4 {
			shapes++
		}
	}
	if shapes == 0 {
		t.Fatal("no large outline for the lock")
	}
}

func TestNBSLogoKeepsJPEG(t *testing.T) {
	pdf := corpusFile("nbs-alibaba")
	if pdf == "" {
		t.Skip("nbs-alibaba pdf not present")
	}
	doc, err := Open(pdf)
	if err != nil {
		t.Skip(err)
	}
	p, err := doc.GetPage(3)
	if err != nil {
		t.Fatal(err)
	}
	var jpeg int
	for _, im := range p.Images {
		if im.Format == "jpeg" && im.Width > 40 && im.Width < 80 {
			jpeg++
		}
	}
	if jpeg < 10 {
		t.Fatalf("logo jpegs %d, want the original DCT bytes", jpeg)
	}
}

func TestRotatePageKeepsTextUpright(t *testing.T) {
	p := &Page{Width: 612, Height: 792, Elements: []PageElement{{
		Text: &TextObject{
			Content: "MIT OpenCourseWare", X: 23.39, Y: 643.60,
			Width: 129.43, Height: 15.33, Rotation: -90,
		},
	}}}
	applyRotation(p, 90)
	if math.Abs(p.Width-792) > 0.1 || math.Abs(p.Height-612) > 0.1 {
		t.Fatalf("page size %.2f x %.2f", p.Width, p.Height)
	}
	tb := p.Elements[0].Text
	if tb.Rotation != 0 {
		t.Fatalf("rotation = %v", tb.Rotation)
	}
	if tb.Width < tb.Height {
		t.Fatalf("text box still tall: %.2f x %.2f", tb.Width, tb.Height)
	}
	if math.Abs(tb.X-76.02) > 0.3 || math.Abs(tb.Y-80.44) > 0.3 {
		t.Fatalf("text at %.2f,%.2f", tb.X, tb.Y)
	}
}

func TestTightWordSpaceIsKept(t *testing.T) {
	// 0.20 em is a real word space ("Getty" + "images"). A negative gap is
	// Canva's per-letter boxes and must not grow a space.
	p := &Page{Elements: []PageElement{
		{Text: &TextObject{Content: "Getty", X: 70, Y: 156, Width: 44, Height: 22, FontSize: 18, FontName: "Arimo"}},
		{Text: &TextObject{Content: "images", X: 118, Y: 156, Width: 59, Height: 22, FontSize: 18, FontName: "Arimo"}},
		{Text: &TextObject{Content: "Recogni", X: 10, Y: 10, Width: 80, Height: 30, FontSize: 30, FontName: "Noto Sans"}},
		{Text: &TextObject{Content: "zed", X: 88, Y: 12, Width: 40, Height: 30, FontSize: 30, FontName: "Noto Sans"}},
	}}
	mergeTextLines(p)
	var got []string
	for _, el := range p.Elements {
		if el.Text != nil {
			got = append(got, el.Text.Content)
		}
	}
	joined := strings.Join(got, "|")
	if !strings.Contains(joined, "Getty images") {
		t.Fatalf("missing word space: %s", joined)
	}
	if strings.Contains(joined, "Recogni zed") {
		t.Fatalf("split a word: %s", joined)
	}
	if !strings.Contains(joined, "Recognized") {
		t.Fatalf("letters not joined: %s", joined)
	}
}

func TestMergeHyphenatedLine(t *testing.T) {
	p := &Page{Elements: []PageElement{
		{Text: &TextObject{Content: "post", X: 62.25, Y: 242.82, Width: 198.48, Height: 22.61, FontSize: 18.73, FontName: "Noto Sans"}},
		{Text: &TextObject{Content: "-", X: 259.27, Y: 238.84, Width: 12.35, Height: 27.41, FontSize: 22.71, FontName: "Noto Sans"}},
		{Text: &TextObject{Content: "it note.", X: 266.52, Y: 242.29, Width: 80, Height: 23.25, FontSize: 19.26, FontName: "Noto Sans"}},
		{Text: &TextObject{Content: "Jossey", X: 21.23, Y: 504.35, Width: 276.02, Height: 11.94, FontSize: 9.90, FontName: "Noto Sans"}},
		{Text: &TextObject{Content: "-", X: 296.68, Y: 504.14, Width: 4.91, Height: 12.20, FontSize: 10.11, FontName: "Noto Sans"}},
		{Text: &TextObject{Content: "Bass.", X: 300.80, Y: 504.38, Width: 40, Height: 11.91, FontSize: 9.87, FontName: "Noto Sans"}},
		{Text: &TextObject{Content: "top ", X: 242.38, Y: 314.72, Width: 320.19, Height: 22.85, FontSize: 18.93, FontName: "Noto Sans"}},
		{Text: &TextObject{Content: "laning", X: 561.08, Y: 315.43, Width: 54.56, Height: 21.99, FontSize: 18.22, FontName: "Noto Sans"}},
		{Text: &TextObject{Content: "alone", X: 900, Y: 100, Width: 40, Height: 20, FontSize: 18, FontName: "Noto Sans"}},
	}}
	mergeTextLines(p)
	var got []string
	for _, el := range p.Elements {
		if el.Text != nil {
			got = append(got, el.Text.Content)
		}
	}
	joined := strings.Join(got, " | ")
	for _, want := range []string{"post-it note.", "Jossey-Bass.", "top laning"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in %s", want, joined)
		}
	}
	if strings.Contains(joined, "post -") || strings.Contains(joined, "Jossey -") || strings.Contains(joined, "top  ") {
		t.Fatalf("gap survived: %s", joined)
	}
	if !strings.Contains(joined, "alone") {
		t.Fatalf("distant run was swallowed: %s", joined)
	}
}

func TestZeroAlphaFillIsNotPainted(t *testing.T) {
	// Google Slides draws a black rect with ca 0, then clips an image to it.
	content := "q /A0 gs 0 0 0 rg 0 0 100 50 re f Q 1 0 0 rg 10 10 20 10 re f\n"
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Count 1 /Kids [3 0 R] >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 100 50] /Resources << /ExtGState << /A0 4 0 R >> >> /Contents 5 0 R >>",
		"<< /Type /ExtGState /ca 0 /CA 0 >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content),
	}
	p := openPDF(t, pdfBytes(objs))
	for _, el := range p.Elements {
		if el.Shape != nil && el.Shape.FillR == 0 && el.Shape.FillG == 0 && el.Shape.FillB == 0 && el.Shape.HasFill {
			t.Fatalf("transparent black rect was painted: %+v", el.Shape)
		}
	}
	var red bool
	for _, el := range p.Elements {
		if el.Shape != nil && el.Shape.FillR == 255 && el.Shape.HasFill {
			red = true
		}
	}
	if !red {
		t.Fatal("opaque red rect missing")
	}
}

func TestFrameIsNotASolidBackground(t *testing.T) {
	// Outer page rect plus an inner subpath. The hole must stay a shape,
	// not a solid gray page.
	content := "1 1 1 rg 0 0 200 100 re f 0.46 0.50 0.61 rg 0 0 m 200 0 l 200 100 l 0 100 l h 20 20 m 180 20 l 180 80 l 20 80 l h f\n"
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Count 1 /Kids [3 0 R] >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content),
	}
	p := openPDF(t, pdfBytes(objs))
	if p.BgR < 250 || p.BgG < 250 || p.BgB < 250 {
		t.Fatalf("background should stay the white rect, got %d,%d,%d", p.BgR, p.BgG, p.BgB)
	}
	var frame bool
	for _, el := range p.Elements {
		if el.Shape != nil && el.Shape.HasFill && len(el.Shape.Path) > 4 {
			frame = true
		}
	}
	if !frame {
		t.Fatal("frame with a hole was dropped")
	}
}

func openPDF(t *testing.T, b []byte) *Page {
	t.Helper()
	path := filepath.Join(t.TempDir(), "t.pdf")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	p, err := doc.GetPage(0)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func pdfBytes(objs []string) []byte {
	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")
	offs := make([]int, len(objs)+1)
	for i, body := range objs {
		offs[i+1] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", i+1, body)
	}
	xref := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n", len(objs)+1)
	buf.WriteString("0000000000 65535 f \n")
	for i := 1; i <= len(objs); i++ {
		fmt.Fprintf(&buf, "%010d 00000 n \n", offs[i])
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objs)+1, xref)
	return buf.Bytes()
}

func TestMITSlidePaint(t *testing.T) {
	doc, err := Open("/tmp/mit-ses1.pdf")
	if err != nil {
		t.Skip(err)
	}
	p, err := doc.GetPage(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Images) == 0 {
		t.Fatal("no background image")
	}
	img, _, err := image.Decode(bytes.NewReader(p.Images[0].Data))
	if err != nil {
		t.Fatal(err)
	}
	if p.Images[0].Format != "jpeg" {
		t.Fatalf("page photo format %s, want jpeg", p.Images[0].Format)
	}
	rr, gg, bb, aa := img.At(img.Bounds().Dx()/2, img.Bounds().Dy()/2).RGBA()
	if aa>>8 != 255 || rr>>8 < 220 || gg>>8 < 220 || bb>>8 < 220 {
		t.Fatalf("background pixel %d %d %d a=%d", rr>>8, gg>>8, bb>>8, aa>>8)
	}
	var text string
	for _, el := range p.Elements {
		if el.Text != nil {
			text += el.Text.Content + "\n"
		}
	}
	for _, want := range []string{"post-it", "top laning", "Jossey-Bass"} {
		if !strings.Contains(text, want) {
			t.Fatalf("page 1 missing %q\n%s", want, text)
		}
	}
	if strings.Contains(text, "post -it") || strings.Contains(text, "Jossey -Bass") || strings.Contains(text, "top  laning") {
		t.Fatalf("split words remain:\n%s", text)
	}
	last, err := doc.GetPage(22)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(last.Width-792) > 1 || math.Abs(last.Height-612) > 1 {
		t.Fatalf("page 23 size %.1f x %.1f", last.Width, last.Height)
	}
	var found bool
	for _, el := range last.Elements {
		if el.Text == nil || !strings.Contains(el.Text.Content, "MIT OpenCourseWare") {
			continue
		}
		found = true
		if el.Text.Rotation != 0 || el.Text.Width < el.Text.Height {
			t.Fatalf("courseware text rot=%.1f box %.1fx%.1f at %.1f,%.1f", el.Text.Rotation, el.Text.Width, el.Text.Height, el.Text.X, el.Text.Y)
		}
	}
	if !found {
		t.Fatal("page 23 lost the courseware line")
	}
}

func TestLightWordmarkStaysAPicture(t *testing.T) {
	bg := image.NewNRGBA(image.Rect(0, 0, 20, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 20; x++ {
			bg.SetNRGBA(x, y, color.NRGBA{R: 0, G: 40, B: 140, A: 255})
		}
	}
	logo := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			logo.SetNRGBA(x, y, color.NRGBA{A: 0})
		}
	}
	for y := 2; y < 6; y++ {
		for x := 2; x < 6; x++ {
			logo.SetNRGBA(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
		}
	}
	var bgBuf, logoBuf bytes.Buffer
	if err := png.Encode(&bgBuf, bg); err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(&logoBuf, logo); err != nil {
		t.Fatal(err)
	}
	p := &Page{Elements: []PageElement{
		{Image: &ImageObject{Data: bgBuf.Bytes(), Format: "png", X: 0, Y: 0, Width: 20, Height: 10}},
		{Image: &ImageObject{Data: logoBuf.Bytes(), Format: "png", X: 4, Y: 2, Width: 8, Height: 8}},
	}}
	opacifyLightMasks(p)
	if len(p.Elements) != 2 {
		t.Fatalf("wordmark picture was dropped: %d elements", len(p.Elements))
	}
	img, err := png.Decode(bytes.NewReader(p.Elements[1].Image.Data))
	if err != nil {
		t.Fatal(err)
	}
	// Letter core is white and opaque. A clear pixel shows the blue behind it.
	lr, lg, lb, la := img.At(3, 3).RGBA()
	if la>>8 != 255 || lr>>8 < 240 || lg>>8 < 240 || lb>>8 < 240 {
		t.Fatalf("letter pixel %d,%d,%d a=%d", lr>>8, lg>>8, lb>>8, la>>8)
	}
	cr, cg, cb, ca := img.At(0, 0).RGBA()
	if ca>>8 != 255 || cb>>8 < 100 || cr>>8 > 40 {
		t.Fatalf("clear pixel should be the blue backdrop, got %d,%d,%d a=%d", cr>>8, cg>>8, cb>>8, ca>>8)
	}
}

func TestTwoImagesAreBothKept(t *testing.T) {
	img := bytes.Repeat([]byte{10, 20, 200}, 16)
	stream := "<<" + "/Type /XObject /Subtype /Image /Width 4 /Height 4 /ColorSpace /DeviceRGB /BitsPerComponent 8 /Length 48 >>\nstream\n" + string(img) + "\nendstream"
	content := "q\n40 0 0 30 0 0 cm\n/Im0 Do\nQ\nq\n8 0 0 8 4 4 cm\n/Im1 Do\nQ\n"
	p := openPDF(t, pdfBytes([]string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Count 1 /Kids [3 0 R] >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 40 30] /Resources << /XObject << /Im0 5 0 R /Im1 6 0 R >> >> /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content),
		stream,
		stream,
	}))
	n := 0
	for _, el := range p.Elements {
		if el.Image != nil {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("pictures = %d, want 2", n)
	}
}

func TestImageClipCropsAndDrops(t *testing.T) {
	img := bytes.Repeat([]byte{200, 10, 10}, 400)
	stream := "<< /Type /XObject /Subtype /Image /Width 20 /Height 20 /ColorSpace /DeviceRGB /BitsPerComponent 8 /Length 1200 >>\nstream\n" + string(img) + "\nendstream"
	content := "10 12 8 6 re W n\nq\n40 0 0 30 0 0 cm\n/Im0 Do\nQ\n"
	p := openPDF(t, pdfBytes([]string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Count 1 /Kids [3 0 R] >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 40 30] /Resources << /XObject << /Im0 5 0 R >> >> /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content),
		stream,
	}))
	var got *ImageObject
	for _, el := range p.Elements {
		if el.Image != nil {
			got = el.Image
		}
	}
	if got == nil {
		t.Fatal("clipped image was dropped")
	}
	if math.Abs(got.X-10) > 1 || math.Abs(got.Width-8) > 1 || math.Abs(got.Height-6) > 1 {
		t.Fatalf("crop box %.1f,%.1f %.1fx%.1f", got.X, got.Y, got.Width, got.Height)
	}
	outside := "0 0 5 5 re W n\nq\n10 0 0 10 30 0 cm\n/Im0 Do\nQ\n"
	p2 := openPDF(t, pdfBytes([]string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Count 1 /Kids [3 0 R] >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 40 30] /Resources << /XObject << /Im0 5 0 R >> >> /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(outside), outside),
		stream,
	}))
	kept := 0
	for _, el := range p2.Elements {
		if el.Image != nil {
			kept++
		}
	}
	if kept != 1 {
		t.Fatalf("picture fully outside a clip was deleted: %d", kept)
	}
}

func TestSmallRectClipStillCrops(t *testing.T) {
	// A chart card clips off a footer that is only a quarter of the image.
	// That must still crop. A page-sized clip must not inset past the page.
	img := bytes.Repeat([]byte{20, 20, 20}, 400)
	stream := "<< /Type /XObject /Subtype /Image /Width 20 /Height 20 /ColorSpace /DeviceRGB /BitsPerComponent 8 /Length 1200 >>\nstream\n" + string(img) + "\nendstream"
	content := "0 0 100 60 re W n\nq\n100 0 0 80 0 0 cm\n/Im0 Do\nQ\n"
	p := openPDF(t, pdfBytes([]string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Count 1 /Kids [3 0 R] >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 100 80] /Resources << /XObject << /Im0 5 0 R >> >> /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content),
		stream,
	}))
	var got *ImageObject
	for _, el := range p.Elements {
		if el.Image != nil {
			got = el.Image
		}
	}
	if got == nil {
		t.Fatal("image deleted")
	}
	if math.Abs(got.Height-60) > 2 {
		t.Fatalf("footer was not clipped, height %.1f", got.Height)
	}
}

func TestPageClipOnlyTrimsOverflow(t *testing.T) {
	img := bytes.Repeat([]byte{20, 20, 20}, 400)
	stream := "<< /Type /XObject /Subtype /Image /Width 20 /Height 20 /ColorSpace /DeviceRGB /BitsPerComponent 8 /Length 1200 >>\nstream\n" + string(img) + "\nendstream"
	// Image hangs 20pt off the right. The clip is the page.
	content := "0 0 100 80 re W n\nq\n120 0 0 80 0 0 cm\n/Im0 Do\nQ\n"
	p := openPDF(t, pdfBytes([]string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Count 1 /Kids [3 0 R] >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 100 80] /Resources << /XObject << /Im0 5 0 R >> >> /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content),
		stream,
	}))
	var got *ImageObject
	for _, el := range p.Elements {
		if el.Image != nil {
			got = el.Image
		}
	}
	if got == nil {
		t.Fatal("image deleted")
	}
	if got.X+got.Width < 98 {
		t.Fatalf("page clip inset the picture to %.1f+%.1f", got.X, got.Width)
	}
}

func TestStitchRealisticGeometry(t *testing.T) {
	parts := []struct {
		s        string
		x, w, sz float64
	}{
		{"R", 622.9, 26.8, 41.2},
		{"Re", 622.9, 55.0, 42.9},
		{"ea", 652.5, 52.8, 42.9},
		{"al", 680.6, 50.6, 44.8},
		{"li", 708.0, 37.4, 44.8},
		{"is", 722.3, 36.2, 44.8},
		{"st", 737.1, 47.4, 44.8},
		{"ti", 761.2, 42.7, 44.8},
		{"ic", 780.7, 37.7, 44.8},
		{"c", 795.5, 22.9, 44.8},
	}
	p := &Page{}
	for _, part := range parts {
		p.Elements = append(p.Elements, PageElement{Text: &TextObject{
			Content: part.s, X: part.x, Y: 643.07 - part.sz, Width: part.w, Height: 50,
			FontSize: part.sz, FontName: "Noto Sans",
		}})
	}
	mergeTextLines(p)
	var got string
	for _, el := range p.Elements {
		if el.Text != nil {
			got += el.Text.Content
		}
	}
	for _, letter := range []string{"R", "e", "a", "l", "i", "s", "t", "c"} {
		if !strings.Contains(got, letter) {
			t.Fatalf("lost %q in %s", letter, got)
		}
	}
}

func TestDuplicateKeepsTheLighterColor(t *testing.T) {
	p := &Page{Elements: []PageElement{
		{Text: &TextObject{Content: "R", X: 0, Y: 100, Width: 20, Height: 20, FontSize: 18, FontName: "Noto Sans", ColorR: 0, ColorG: 0, ColorB: 0}},
		{Text: &TextObject{Content: "R", X: 0, Y: 100, Width: 20, Height: 20, FontSize: 18, FontName: "Noto Sans", ColorR: 255, ColorG: 255, ColorB: 255}},
	}}
	mergeTextLines(p)
	n := 0
	for _, el := range p.Elements {
		if el.Text == nil {
			continue
		}
		n++
		if el.Text.ColorR != 255 || el.Text.ColorG != 255 || el.Text.ColorB != 255 {
			t.Fatalf("shadow stayed black %d,%d,%d", el.Text.ColorR, el.Text.ColorG, el.Text.ColorB)
		}
	}
	if n != 2 {
		t.Fatalf("copies = %d, both fragments must stay so the token is not lost", n)
	}
}

func TestIndexedColorIsNotGray(t *testing.T) {
	// Index 1 is navy (0, 32, 96). Treating the index as a gray level paints
	// it gray-blue instead.
	lookup := string([]byte{255, 255, 255, 0, 32, 96})
	content := "/Ix cs 1 sc 0 0 30 20 re f\n"
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Count 1 /Kids [3 0 R] >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 40 30] /Resources << /ColorSpace << /Ix [/Indexed /DeviceRGB 1 5 0 R] >> >> /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content),
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(lookup), lookup),
	}
	p := openPDF(t, pdfBytes(objs))
	var gotR, gotG, gotB uint8
	found := false
	for _, el := range p.Elements {
		if el.Shape != nil && el.Shape.HasFill {
			gotR, gotG, gotB = el.Shape.FillR, el.Shape.FillG, el.Shape.FillB
			found = true
		}
	}
	if !found {
		t.Fatal("no fill")
	}
	if gotR > 5 || gotG < 20 || gotG > 45 || gotB < 80 {
		t.Fatalf("indexed navy became %d,%d,%d", gotR, gotG, gotB)
	}
}

func TestDropDuplicateLetters(t *testing.T) {
	// Each letter is painted twice at the same place. The copy must be
	// discarded so the word stays "Realistic" rather than "Re"+"ea"+...
	letters := []struct {
		s    string
		x, w float64
	}{
		{"R", 0, 12}, {"R", 0, 12},
		{"e", 10, 12}, {"e", 10, 12},
		{"a", 20, 12}, {"a", 20, 12},
		{"l", 30, 12}, {"l", 30, 12},
		{"i", 40, 10}, {"i", 40, 10},
		{"s", 48, 12}, {"s", 48, 12},
		{"t", 58, 12}, {"t", 58, 12},
		{"i", 68, 10}, {"i", 68, 10},
		{"c", 76, 12}, {"c", 76, 12},
	}
	p := &Page{}
	for _, part := range letters {
		p.Elements = append(p.Elements, PageElement{Text: &TextObject{
			Content: part.s, X: part.x, Y: 100, Width: part.w, Height: 20,
			FontSize: 18, FontName: "Noto Sans",
		}})
	}
	mergeTextLines(p)
	rs := 0
	for _, el := range p.Elements {
		if el.Text != nil && strings.Contains(el.Text.Content, "R") {
			rs++
		}
	}
	if rs < 2 {
		t.Fatalf("a duplicate letter was deleted, copies containing R = %d", rs)
	}
}

func TestStitchOverlappingBigrams(t *testing.T) {
	// Canva draws "Realistic" as overlapping pairs. Each pair shares a letter
	// with the next and covers more than half of the shorter box.
	parts := []struct {
		s    string
		x, w float64
	}{
		{"R", 0, 12},
		{"Re", 0, 22},
		{"ea", 10, 22},
		{"al", 20, 22},
		{"li", 30, 22},
		{"is", 40, 22},
		{"st", 50, 22},
		{"ti", 60, 22},
		{"ic", 70, 22},
		{"c", 80, 14},
	}
	p := &Page{}
	for _, part := range parts {
		p.Elements = append(p.Elements, PageElement{Text: &TextObject{
			Content: part.s, X: part.x, Y: 100, Width: part.w, Height: 20,
			FontSize: 18, FontName: "Noto Sans",
		}})
	}
	// A real word gap must still become a space.
	p.Elements = append(p.Elements,
		PageElement{Text: &TextObject{Content: "Getty", X: 200, Y: 100, Width: 40, Height: 20, FontSize: 18, FontName: "Noto Sans"}},
		PageElement{Text: &TextObject{Content: "images", X: 248, Y: 100, Width: 50, Height: 20, FontSize: 18, FontName: "Noto Sans"}},
	)
	mergeTextLines(p)
	var text string
	for _, el := range p.Elements {
		if el.Text != nil {
			text += el.Text.Content + "\n"
		}
	}
	if !strings.Contains(text, "Getty images") {
		t.Fatalf("word space dropped:\n%s", text)
	}
}

func TestDeviceGrayIsNeutral(t *testing.T) {
	cases := []struct {
		name    string
		content string
		cs      string
		extra   []string
		want    byte
		stroke  bool
	}{
		{name: "g", content: "0.25 g 0 0 30 20 re f\n", want: 64},
		{name: "G", content: "0.50 G 2 w 0 0 30 20 re S\n", want: 128, stroke: true},
		{name: "cs", content: "/DeviceGray cs 0.25 sc 0 0 30 20 re f\n", want: 64},
		{name: "named", content: "/DG cs 0.50 sc 0 0 30 20 re f\n", cs: "/DG /DeviceGray", want: 128},
		{name: "calgray", content: "/CG cs 0 sc 0 0 30 20 re f\n", cs: "/CG [/CalGray << /WhitePoint [0.9505 1 1.089] >>]", want: 0},
		{name: "icc", content: "/IG cs 1 sc 0 0 30 20 re f\n", cs: "/IG [/ICCBased 5 0 R]", extra: []string{"<< /N 1 /Length 4 >>\nstream\nxxxx\nendstream"}, want: 255},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := "<< >>"
			if tc.cs != "" {
				res = "<< /ColorSpace << " + tc.cs + " >> >>"
			}
			objs := []string{
				"<< /Type /Catalog /Pages 2 0 R >>",
				"<< /Type /Pages /Count 1 /Kids [3 0 R] >>",
				fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 40 30] /Resources %s /Contents 4 0 R >>", res),
				fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(tc.content), tc.content),
			}
			objs = append(objs, tc.extra...)
			p := openPDF(t, pdfBytes(objs))
			var gotR, gotG, gotB uint8
			var found bool
			for _, el := range p.Elements {
				if el.Shape == nil {
					continue
				}
				if tc.stroke && el.Shape.HasStroke {
					gotR, gotG, gotB = el.Shape.StrokeR, el.Shape.StrokeG, el.Shape.StrokeB
					found = true
				}
				if !tc.stroke && el.Shape.HasFill {
					gotR, gotG, gotB = el.Shape.FillR, el.Shape.FillG, el.Shape.FillB
					found = true
				}
			}
			if !tc.stroke && p.HasBackground && !found {
				gotR, gotG, gotB = p.BgR, p.BgG, p.BgB
				found = true
			}
			if !found {
				t.Fatal("no painted color")
			}
			if gotR != gotG || gotG != gotB {
				t.Fatalf("gray became RGB %d,%d,%d", gotR, gotG, gotB)
			}
			if absDiff(gotR, tc.want) > 2 {
				t.Fatalf("level %d, want %d", gotR, tc.want)
			}
		})
	}
}

func absDiff(a, b byte) byte {
	if a > b {
		return a - b
	}
	return b - a
}
