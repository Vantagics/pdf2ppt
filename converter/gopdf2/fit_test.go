package gopdf2

import (
	"math"
	"os"
	"strings"
	"testing"
)

func TestTrackingPullsWideFont(t *testing.T) {
	sz, tr := trackingFor("HELLO", 20, 40, 60, func(s float64) float64 { return 60 * s / 20 })
	if sz >= 20 {
		t.Fatalf("size %v should shrink a font that is 50%% too wide", sz)
	}
	if math.Abs(tr) > sz*0.05 {
		t.Fatalf("tracking %v condenses glyphs (size %v)", tr, sz)
	}
	ink := 60*sz/20 + tr*4
	if ink >= 55 {
		t.Fatalf("fitted ink %v did not shrink (size %v tracking %v)", ink, sz, tr)
	}
}

func TestModerateMismatchDoesNotCondense(t *testing.T) {
	// About 16% too wide, the "Gotham University" case. Scale the em;
	// do not pull the letters together.
	sz, tr := trackingFor("Gotham University", 12, 84, 100, func(s float64) float64 { return 100 * s / 12 })
	if math.Abs(tr) > 0.15 {
		t.Fatalf("tracking %v is large enough to squeeze the word", tr)
	}
	if sz < 9.5 || sz > 10.5 {
		t.Fatalf("size %v, want about 10", sz)
	}
}

func TestLatoKeepsItsName(t *testing.T) {
	tb := &TextObject{Content: "nonprofit ", FontName: "Lato-Black", FontSize: 34.7, X: 573.7, Y: 231, Width: 149.6, Height: 40}
	p := &Page{Elements: []PageElement{{Text: tb}}}
	fitCloudFontSpacing(p)
	if tb.FontName != "Lato-Black" {
		t.Fatalf("font rewritten to %q", tb.FontName)
	}
	if tb.Tracking != 0 {
		t.Fatalf("lato spacing %v, want 0 when the face is not installed", tb.Tracking)
	}
	if tb.X != 573.7 || math.Abs(tb.Width-149.6) > 0.1 {
		t.Fatalf("box moved: x %.1f w %.1f", tb.X, tb.Width)
	}
	if !isOfficeCloudFont("Anton") || !isOfficeCloudFont("Open Sans") || isOfficeCloudFont("Gill Sans") {
		t.Fatal("cloud font list")
	}
}

func TestTrackingLeavesMatchingFont(t *testing.T) {
	sz, tr := trackingFor("HELLO", 12, 40, 40.5, func(float64) float64 { return 0 })
	if sz != 12 || tr != 0 {
		t.Fatalf("got size %v tracking %v", sz, tr)
	}
}

func TestSuppressesGlyphOutlinesInsideText(t *testing.T) {
	text := &TextObject{Content: "THIS IS A", FontName: "Noto Sans", FontSize: 48, X: 80, Y: 4, Width: 210, Height: 60}
	letter := &ShapeObject{
		X: 83, Y: 16, Width: 28, Height: 36,
		HasStroke: true, StrokeWidth: 1.9,
		StrokeR: 48, StrokeG: 48, StrokeB: 48,
		Path: make([]PathOp, 8),
	}
	stem := &ShapeObject{
		X: 148, Y: 16, Width: 6, Height: 36,
		HasStroke: true, StrokeWidth: 2, SimpleRect: true,
	}
	underline := &ShapeObject{
		X: 80, Y: 58, Width: 200, Height: 1.2,
		HasStroke: true, SimpleRect: true,
	}
	logo := &ShapeObject{
		X: 10, Y: 10, Width: 40, Height: 40,
		HasFill: true, HasStroke: true, Path: make([]PathOp, 10),
	}
	els := suppressDuplicateGlyphOutlines([]PageElement{
		{Text: text},
		{Shape: letter},
		{Shape: stem},
		{Shape: underline},
		{Shape: logo},
	})
	if len(els) != 3 {
		t.Fatalf("kept %d elements, want text + underline + logo", len(els))
	}
	for _, el := range els {
		if el.Shape == letter {
			t.Fatal("letter outline was kept")
		}
	}
	if !text.Stroke || text.StrokeWidth < 1.9 || !text.Bold {
		t.Fatalf("outline weight not kept on the run: stroke=%v sw=%v bold=%v", text.Stroke, text.StrokeWidth, text.Bold)
	}
}

func TestBeamerTitleIsSingleAndFitted(t *testing.T) {
	path := "/tmp/score/slices/beamer-flip.pdf"
	if _, err := os.Stat(path); err != nil {
		t.Skip(path)
	}
	doc, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	page, err := doc.GetPage(0)
	if err != nil {
		t.Fatal(err)
	}
	var title *TextObject
	for _, el := range page.Elements {
		if el.Text != nil && strings.Contains(el.Text.Content, "THIS IS A") {
			title = el.Text
		}
		if el.Shape != nil && glyphOutlineOfText(el.Shape, page.TextObjects) {
			t.Fatalf("glyph outline survived at x=%.1f y=%.1f", el.Shape.X, el.Shape.Y)
		}
	}
	if title == nil {
		t.Fatal("title text missing")
	}
	ink := InkWidth(title)
	if ink > title.Width+2 {
		t.Fatalf("title ink %.1f exceeds box %.1f", ink, title.Width)
	}
	if !title.Stroke || title.StrokeWidth < 0.5 {
		t.Fatalf("title lost the contour weight: stroke=%v sw=%.2f", title.Stroke, title.StrokeWidth)
	}
	if math.Abs(title.Tracking) > title.FontSize*0.05 {
		t.Fatalf("title tracking %.2f condenses the line", title.Tracking)
	}
	// The following run on the same baseline must not be crossed by this ink.
	for _, other := range page.TextObjects {
		if other == title || !sameBaseline(title, other) {
			continue
		}
		if title.X+ink > other.X+1.0 && other.X > title.X {
			t.Fatalf("%q ink reaches %q at x %.1f", title.Content, other.Content, other.X)
		}
	}
	for _, tx := range page.TextObjects {
		if math.Abs(tx.Tracking) > tx.FontSize*0.05 {
			t.Fatalf("%q tracking %.2f condenses glyphs", tx.Content, tx.Tracking)
		}
	}
	overlaps := TextOverlaps(page.TextObjects)
	if len(overlaps) > 0 {
		t.Fatalf("overlapping runs: %v", overlaps)
	}
	p2, err := doc.GetPage(1)
	if err != nil {
		t.Fatal(err)
	}
	var joined string
	for _, tx := range p2.TextObjects {
		joined += tx.Content + "\n"
	}
	if !strings.Contains(joined, "You can now abuse fonts") {
		t.Fatalf("word spaces dropped: %q", joined)
	}
}
