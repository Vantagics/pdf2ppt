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
