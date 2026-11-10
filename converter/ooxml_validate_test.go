package converter

import (
	"archive/zip"
	"bytes"
	"image"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"github.com/Vantagics/pdf2ppt/converter/goppt"
)

func TestOOXMLKitchenSink(t *testing.T) {
	p := goppt.NewPresentation()
	s := p.AddSlide(12192000, 6858000)
	s.HasBackground = true
	s.BgR, s.BgG, s.BgB = 255, 255, 255
	_ = s.AddTextBox(&goppt.TextBox{
		Content: "Hello", FontName: "Calibri", FontSize: 24,
		X: 100, Y: 100, Width: 2000000, Height: 400000,
		Stroke: true, StrokeWidth: 1.2, StrokeR: 10, StrokeG: 10, StrokeB: 10,
	})
	_ = s.AddShape(&goppt.Shape{
		X: 300, Y: 300, Width: 500000, Height: 400000,
		HasFill: true, FillR: 200, FillG: 10, FillB: 10,
		HasStroke: true, StrokeWidth: 12700, StrokeR: 0, StrokeG: 0, StrokeB: 0,
		EvenOdd: true,
		Path: []goppt.PathOp{
			{Op: "M", X: 0, Y: 0},
			{Op: "L", X: 500000, Y: 0},
			{Op: "L", X: 500000, Y: 400000},
			{Op: "Z"},
		},
	})
	_ = s.AddShape(&goppt.Shape{
		X: 0, Y: 0, Width: 12192000, Height: 6858000,
		HasFill: true,
		Gradient: &goppt.Gradient{Kind: "linear", Angle: 30, Stops: []goppt.GradientStop{
			{Pos: 0, R: 1, G: 2, B: 3},
			{Pos: 1, R: 250, G: 240, B: 10},
		}},
	})
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, image.NewNRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	_ = s.AddImage(&goppt.Image{Data: pngBuf.Bytes(), Format: "png", X: 1000, Y: 1000, Width: 100000, Height: 100000})
	out := filepath.Join(t.TempDir(), "sink.pptx")
	if err := p.Save(out); err != nil {
		t.Fatal(err)
	}
	assertZipDate(t, out)
	validateOOXML(t, out)
}

func TestOOXMLSampleAndMIT(t *testing.T) {
	sample := filepath.Join(moduleRoot(t), "skill", "pdf2ppt", "examples", "sample.pdf")
	out := filepath.Join(t.TempDir(), "sample.pptx")
	if _, err := NewConverter(nil).Convert(sample, out); err != nil {
		t.Fatal(err)
	}
	validateOOXML(t, out)
	if mit := "/tmp/mit-ses1.pdf"; fileExists(mit) {
		mitOut := filepath.Join(t.TempDir(), "mit.pptx")
		if _, err := NewConverter(nil).Convert(mit, mitOut); err != nil {
			t.Fatal(err)
		}
		validateOOXML(t, mitOut)
	}
}

func assertZipDate(t *testing.T, path string) {
	t.Helper()
	z, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	if z.File[0].Name != "[Content_Types].xml" {
		t.Fatalf("first zip entry is %s", z.File[0].Name)
	}
	d := z.File[0].Modified
	if d.Year() < 1980 || d.Month() == 0 || d.Day() == 0 {
		t.Fatalf("invalid zip timestamp %v", d)
	}
	body, err := z.File[0].Open()
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 64)
	_, _ = body.Read(buf)
	body.Close()
	if !bytes.Contains(mustRead(t, path, "ppt/presentation.xml"), []byte("<p:notesSz ")) {
		t.Fatal("presentation is missing notesSz")
	}
}

func mustRead(t *testing.T, pptx, name string) []byte {
	t.Helper()
	z, err := zip.OpenReader(pptx)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	for _, f := range z.File {
		if f.Name == name {
			r, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			b, err := io.ReadAll(r)
			if err != nil {
				t.Fatal(err)
			}
			return b
		}
	}
	t.Fatalf("missing %s", name)
	return nil
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

var (
	valOnce sync.Once
	valExe  string
	valErr  error
)

func validateOOXML(t *testing.T, pptx string) {
	t.Helper()
	if _, err := exec.LookPath("dotnet"); err != nil {
		t.Skip("dotnet SDK is required for OOXML schema validation")
	}
	valOnce.Do(func() {
		dir := filepath.Join(os.TempDir(), "pdf2ppt-ooxmlvalidate")
		cmd := exec.Command("dotnet", "build", filepath.Join(moduleRoot(t), "tools", "ooxmlvalidate"), "-c", "Release", "-o", dir)
		out, err := cmd.CombinedOutput()
		if err != nil {
			valErr = err
			valExe = string(out)
			return
		}
		valExe = filepath.Join(dir, "ooxmlvalidate")
	})
	if valErr != nil {
		t.Fatalf("build validator: %v\n%s", valErr, valExe)
	}
	cmd := exec.Command(valExe, pptx, "Office2016")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("OOXML invalid %s: %v\n%s", pptx, err, out)
	}
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}
