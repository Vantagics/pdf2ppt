package goppt

import (
	"strings"
	"testing"
)

func TestTextOutline(t *testing.T) {
	xml := formatTextBox(&TextBox{
		Content:     "PPT",
		FontName:    "黑体",
		FontSize:    40,
		ColorR:      255,
		ColorG:      204,
		ColorB:      0,
		X:           100,
		Y:           100,
		Width:       1000,
		Height:      400,
		Stroke:      true,
		StrokeR:     255,
		StrokeG:     204,
		StrokeB:     0,
		StrokeWidth: 1.14,
	}, 2)
	ln := strings.Index(xml, `<a:ln w=`)
	fill := strings.Index(xml, `</a:ln><a:solidFill><a:srgbClr val="FFCC00"/>`)
	if ln < 0 || fill < 0 || ln > fill {
		t.Fatalf("text outline must precede the run fill: %s", xml)
	}
	bare := formatTextBox(&TextBox{Content: "Hi", FontName: "Arial", FontSize: 12, Width: 100, Height: 100}, 3)
	if strings.Contains(bare, "<a:ln ") {
		t.Fatal("plain text should not grow an outline")
	}
	vert := formatTextBox(&TextBox{Content: "Programs", FontName: "Noto Sans", FontSize: 12, X: 1, Y: 1, Width: 1000, Height: 400, Rotation: -90}, 4)
	if !strings.Contains(vert, `rot="16200000"`) {
		t.Fatalf("negative rotation was not normalized: %s", vert)
	}
}

func TestRTLParagraph(t *testing.T) {
	xml := formatTextBox(&TextBox{Content: "שלום", FontName: "Arial", FontSize: 18, Width: 200, Height: 40, RTL: true}, 5)
	if !strings.Contains(xml, `<a:pPr rtl="1"`) {
		t.Fatalf("rtl paragraph missing: %s", xml)
	}
}

func TestVerticalPDFWritesEaVert(t *testing.T) {
	xml := formatTextBox(&TextBox{
		Content: "あいうえお", FontName: "Noto Sans CJK JP", FontSize: 12,
		Width: 140000, Height: 700000, Vertical: true,
	}, 3)
	if !strings.Contains(xml, `vert="eaVert"`) {
		t.Fatalf("vertical body missing: %s", xml)
	}
	plain := formatTextBox(&TextBox{Content: "Hello", FontName: "Arial", FontSize: 18, Width: 200, Height: 40}, 4)
	if strings.Contains(plain, `vert=`) {
		t.Fatalf("horizontal box grew vert: %s", plain)
	}
}

func TestSubscriptBaseline(t *testing.T) {
	xml := formatTextRun(&TextBox{}, TextRun{
		Content: "2", FontName: "Tinos", FontSize: 17, Baseline: -25000,
	})
	if !strings.Contains(xml, `baseline="-25000"`) {
		t.Fatalf("subscript baseline missing: %s", xml)
	}
	plain := formatTextRun(&TextBox{}, TextRun{Content: "2024", FontName: "Tinos", FontSize: 32})
	if strings.Contains(plain, "baseline=") {
		t.Fatalf("a normal run grew a baseline: %s", plain)
	}
}

func TestHoleStaysOnePath(t *testing.T) {
	// DrawingML fills each a:path on its own. A frame, a letter O, and a
	// donut only stay open when both contours share one a:path, and an
	// even-odd hole that winds with the outer contour is reversed so
	// fill="norm" punches it.
	frameOuter := rectContour(0, 0, 1000, 1000)
	frameHole := rectContour(250, 250, 750, 750)
	t.Run("frame", func(t *testing.T) {
		xml := holeGeom(true, frameOuter, frameHole)
		assertOnePath(t, xml, 2)
		if !strings.Contains(xml, `<a:moveTo><a:pt x="250" y="750"/></a:moveTo>`) {
			t.Fatalf("same-direction hole was not reversed:\n%s", xml)
		}
	})
	t.Run("frame-opposite-winding", func(t *testing.T) {
		// Already opposite, so nonzero matches even-odd and the hole is left alone.
		opposite := rectContour(250, 750, 750, 250)
		xml := holeGeom(true, frameOuter, opposite)
		assertOnePath(t, xml, 2)
		if !strings.Contains(xml, `<a:moveTo><a:pt x="250" y="750"/></a:moveTo>`) {
			t.Fatalf("opposite hole was rewritten:\n%s", xml)
		}
		if strings.Contains(xml, `<a:moveTo><a:pt x="750" y="250"/></a:moveTo>`) {
			t.Fatalf("opposite hole was reversed:\n%s", xml)
		}
	})
	t.Run("letter-O", func(t *testing.T) {
		xml := holeGeom(true, circleContour(500, 500, 400), circleContour(500, 500, 180))
		assertOnePath(t, xml, 2)
		// Reversed inner cubic leads with the swapped controls of the last curve
		// (k = 180*5523/10000 = 99).
		if !strings.Contains(xml, `<a:pt x="680" y="401"/><a:pt x="599" y="320"/>`) {
			t.Fatalf("letter O hole was not reversed:\n%s", xml)
		}
	})
	t.Run("donut", func(t *testing.T) {
		xml := holeGeom(true, octagon(500, 500, 400), octagon(500, 500, 180))
		assertOnePath(t, xml, 2)
		// Same-direction inner octagon starts at the last vertex after reversal.
		if !strings.Contains(xml, `<a:moveTo><a:pt x="627" y="373"/></a:moveTo>`) {
			t.Fatalf("donut hole was not reversed:\n%s", xml)
		}
	})
}

func holeGeom(evenOdd bool, contours ...[]PathOp) string {
	var path []PathOp
	for _, c := range contours {
		path = append(path, c...)
	}
	return formatCustGeom(&Shape{Width: 1000, Height: 1000, HasFill: true, EvenOdd: evenOdd, Path: path})
}

func assertOnePath(t *testing.T, xml string, contours int) {
	t.Helper()
	if strings.Count(xml, "<a:path ") != 1 {
		t.Fatalf("hole split across %d a:path elements:\n%s", strings.Count(xml, "<a:path "), xml)
	}
	if strings.Count(xml, `<a:moveTo>`) != contours {
		t.Fatalf("want %d contours, xml:\n%s", contours, xml)
	}
}

func rectContour(x0, y0, x1, y1 int64) []PathOp {
	return []PathOp{
		{Op: "M", X: x0, Y: y0},
		{Op: "L", X: x1, Y: y0},
		{Op: "L", X: x1, Y: y1},
		{Op: "L", X: x0, Y: y1},
		{Op: "Z"},
	}
}

func circleContour(cx, cy, r int64) []PathOp {
	// Four cubics. k/r ≈ 0.5523, rounded so the hole's reversed controls stay integers.
	k := r * 5523 / 10000
	return []PathOp{
		{Op: "M", X: cx + r, Y: cy},
		{Op: "C", X: cx, Y: cy + r, X1: cx + r, Y1: cy + k, X2: cx + k, Y2: cy + r},
		{Op: "C", X: cx - r, Y: cy, X1: cx - k, Y1: cy + r, X2: cx - r, Y2: cy + k},
		{Op: "C", X: cx, Y: cy - r, X1: cx - r, Y1: cy - k, X2: cx - k, Y2: cy - r},
		{Op: "C", X: cx + r, Y: cy, X1: cx + k, Y1: cy - r, X2: cx + r, Y2: cy - k},
		{Op: "Z"},
	}
}

func octagon(cx, cy, r int64) []PathOp {
	// r*0.7071, the diagonal of a regular octagon.
	d := r * 7071 / 10000
	pts := [][2]int64{
		{cx + r, cy}, {cx + d, cy + d}, {cx, cy + r}, {cx - d, cy + d},
		{cx - r, cy}, {cx - d, cy - d}, {cx, cy - r}, {cx + d, cy - d},
	}
	ops := []PathOp{{Op: "M", X: pts[0][0], Y: pts[0][1]}}
	for _, p := range pts[1:] {
		ops = append(ops, PathOp{Op: "L", X: p[0], Y: p[1]})
	}
	ops = append(ops, PathOp{Op: "Z"})
	return ops
}

func TestImageSrcRect(t *testing.T) {
	xml := formatImage(&Image{
		X: 1, Y: 2, Width: 100, Height: 80,
		CropL: 0.1, CropT: 0.2, CropR: 0.3, CropB: 0.4,
	}, 1, 1)
	want := `<a:srcRect l="10000" t="20000" r="30000" b="40000"/>`
	if !strings.Contains(xml, want) {
		t.Fatalf("srcRect missing: %s", xml)
	}
	plain := formatImage(&Image{X: 1, Y: 2, Width: 100, Height: 80}, 2, 1)
	if strings.Contains(plain, "srcRect") {
		t.Fatalf("uncropped picture grew a srcRect: %s", plain)
	}
}
