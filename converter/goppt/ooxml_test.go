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
	if !strings.Contains(xml, "<a:ln ") || !strings.Contains(xml, "FFCC00") {
		t.Fatalf("outline missing: %s", xml)
	}
	bare := formatTextBox(&TextBox{Content: "Hi", FontName: "Arial", FontSize: 12, Width: 100, Height: 100}, 3)
	if strings.Contains(bare, "<a:ln ") {
		t.Fatal("plain text should not grow an outline")
	}
}
