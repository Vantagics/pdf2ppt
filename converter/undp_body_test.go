package converter

import (
	"archive/zip"
	"bytes"
	"image/png"
	"regexp"
	"testing"
)

// The page 10 headings are soft-mask glyphs. A blank body is a fidelity
// failure even when the portrait underneath is present. The gate checks
// that a wide picture on that slide actually contains ink.
func TestUNDPPage10Body(t *testing.T) {
	pdf := corpusPDF("undp-gender-ar")
	if pdf == "" {
		t.Skip("undp deck not present")
	}
	out := ensurePPTX(t, pdf, "undp-gender-ar")
	z, err := zip.OpenReader(out)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var slide string
	for _, f := range z.File {
		if f.Name == "ppt/slides/slide10.xml" {
			b, err := readZip(f)
			if err != nil {
				t.Fatal(err)
			}
			slide = string(b)
		}
	}
	if slide == "" {
		t.Fatal("slide 10 missing")
	}
	relsName := "ppt/slides/_rels/slide10.xml.rels"
	var rels string
	for _, f := range z.File {
		if f.Name == relsName {
			b, err := readZip(f)
			if err != nil {
				t.Fatal(err)
			}
			rels = string(b)
		}
	}
	targets := regexp.MustCompile(`Target="\.\./(media/[^"]+)"`).FindAllStringSubmatch(rels, -1)
	inkLines := 0
	for _, m := range targets {
		name := "ppt/" + m[1]
		var data []byte
		for _, f := range z.File {
			if f.Name != name {
				continue
			}
			data, err = readZip(f)
			if err != nil {
				t.Fatal(err)
			}
		}
		if len(data) == 0 || !bytes.HasPrefix(data, []byte("\x89PNG")) {
			continue
		}
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			continue
		}
		b := img.Bounds()
		if b.Dx() < b.Dy()*2 {
			continue // the portrait, not a text line
		}
		opaque := 0
		for y := b.Min.Y; y < b.Max.Y; y += 2 {
			for x := b.Min.X; x < b.Max.X; x += 2 {
				_, _, _, a := img.At(x, y).RGBA()
				if a > 32*256 {
					opaque++
				}
			}
		}
		if opaque > 2000 {
			inkLines++
		}
	}
	if inkLines < 1 {
		t.Fatalf("page 10 body text is blank (%d wide ink pictures)", inkLines)
	}
}

func readZip(f *zip.File) ([]byte, error) {
	r, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	var buf bytes.Buffer
	_, err = buf.ReadFrom(r)
	return buf.Bytes(), err
}
