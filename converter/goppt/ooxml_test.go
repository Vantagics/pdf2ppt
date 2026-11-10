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
