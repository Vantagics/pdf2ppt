// Package goppt provides PPTX file generation capabilities.
// This is a local implementation that creates valid OOXML PPTX files
// with editable text boxes and images.
package goppt

import (
	"archive/zip"
	"fmt"
	"os"
)

// Presentation represents a PPTX presentation.
type Presentation struct {
	slides []*Slide
}

// Slide represents a single slide in the presentation.
type Slide struct {
	Width         int64 // Width in EMU
	Height        int64 // Height in EMU
	HasBackground bool
	BgR, BgG, BgB uint8
	BgGradient    *Gradient
	Elements      []Element
}

// Element is an interface for slide elements.
type Element interface {
	elementType() string
}

// TextBox represents an editable text box on a slide.
type TextBox struct {
	Content     string
	FontName    string
	FontSize    float64
	ColorR      uint8
	ColorG      uint8
	ColorB      uint8
	Bold        bool
	Italic      bool
	X           int64   // Position X in EMU
	Y           int64   // Position Y in EMU
	Width       int64   // Width in EMU
	Height      int64   // Height in EMU
	Rotation    float64 // clockwise degrees
	Stroke      bool
	StrokeOnly  bool
	StrokeR     uint8
	StrokeG     uint8
	StrokeB     uint8
	StrokeWidth float64 // points
	Tracking    float64 // extra points between glyphs; negative tightens
	Clip        bool
	NoFill      bool
	RTL         bool
	Runs        []TextRun
}

// TextRun is one style span in a text box paragraph.
type TextRun struct {
	Content  string
	FontName string
	FontSize float64
	Bold     bool
	Italic   bool
	ColorR   uint8
	ColorG   uint8
	ColorB   uint8
	Tracking float64
	// Baseline is the OOXML baseline shift, in 1000ths of a percent.
	Baseline float64
}

// PathOp is a shape-local path command. Coordinates are EMU, y grows down.
type PathOp struct {
	Op     string
	X, Y   int64
	X1, Y1 int64
	X2, Y2 int64
}

// Shape is a vector outline on a slide.
type Shape struct {
	X, Y, Width, Height       int64
	HasFill                   bool
	FillR, FillG, FillB       uint8
	FillAlpha                 float64
	HasStroke                 bool
	StrokeR, StrokeG, StrokeB uint8
	StrokeWidth               int64
	EvenOdd                   bool
	SimpleRect                bool
	Path                      []PathOp
	Gradient                  *Gradient
}

// GradientStop is one color of a linear gradient. Pos is 0..1.
type GradientStop struct {
	Pos     float64
	R, G, B uint8
}

// Gradient is a linear fill.
type Gradient struct {
	Angle float64
	Kind  string // "linear" (default) or "radial"
	Stops []GradientStop
}

func (t *TextBox) elementType() string { return "textbox" }

// Image represents an image element on a slide.
type Image struct {
	Data     []byte
	Format   string  // "png", "jpeg", etc.
	X        int64   // Position X in EMU
	Y        int64   // Position Y in EMU
	Width    int64   // Width in EMU
	Height   int64   // Height in EMU
	Rotation float64 // clockwise degrees
	// CropL, CropT, CropR, CropB are fractions (0..1) of the bitmap removed
	// from the left, top, right, and bottom. Written as a:srcRect.
	CropL, CropT, CropR, CropB float64
}

func (i *Image) elementType() string { return "image" }

func (s *Shape) elementType() string { return "shape" }

// NewPresentation creates a new empty presentation.
func NewPresentation() *Presentation {
	return &Presentation{
		slides: make([]*Slide, 0),
	}
}

// AddSlide adds a new slide with the given dimensions (in EMU) and returns it.
func (p *Presentation) AddSlide(width, height int64) *Slide {
	slide := &Slide{
		Width:    width,
		Height:   height,
		Elements: make([]Element, 0),
	}
	p.slides = append(p.slides, slide)
	return slide
}

// SlideCount returns the number of slides in the presentation.
func (p *Presentation) SlideCount() int {
	return len(p.slides)
}

// GetSlide returns the slide at the given index (0-based).
func (p *Presentation) GetSlide(index int) (*Slide, error) {
	if index < 0 || index >= len(p.slides) {
		return nil, fmt.Errorf("slide index %d out of range (0-%d)", index, len(p.slides)-1)
	}
	return p.slides[index], nil
}

// AddTextBox adds an editable text box to the slide.
func (s *Slide) AddTextBox(tb *TextBox) error {
	if tb == nil {
		return fmt.Errorf("text box cannot be nil")
	}
	if tb.Width <= 0 || tb.Height <= 0 {
		return fmt.Errorf("text box dimensions must be positive, got width=%d height=%d", tb.Width, tb.Height)
	}
	s.Elements = append(s.Elements, tb)
	return nil
}

// SetBackground sets a solid slide background.
func (s *Slide) SetBackground(r, g, b uint8) {
	s.HasBackground = true
	s.BgR, s.BgG, s.BgB = r, g, b
}

// SetGradientBackground sets a linear slide background.
func (s *Slide) SetGradientBackground(g *Gradient) {
	s.HasBackground = true
	s.BgGradient = g
}

// AddShape adds a vector shape to the slide.
func (s *Slide) AddShape(sh *Shape) error {
	if sh == nil {
		return fmt.Errorf("shape cannot be nil")
	}
	if sh.Width <= 0 || sh.Height <= 0 {
		return fmt.Errorf("shape dimensions must be positive, got width=%d height=%d", sh.Width, sh.Height)
	}
	s.Elements = append(s.Elements, sh)
	return nil
}

// AddImage adds an image element to the slide.
func (s *Slide) AddImage(img *Image) error {
	if img == nil {
		return fmt.Errorf("image cannot be nil")
	}
	if img.Width <= 0 || img.Height <= 0 {
		return fmt.Errorf("image dimensions must be positive, got width=%d height=%d", img.Width, img.Height)
	}
	s.Elements = append(s.Elements, img)
	return nil
}

// TextBoxes returns all text box elements on the slide.
func (s *Slide) TextBoxes() []*TextBox {
	var result []*TextBox
	for _, e := range s.Elements {
		if tb, ok := e.(*TextBox); ok {
			result = append(result, tb)
		}
	}
	return result
}

// Images returns all image elements on the slide.
func (s *Slide) Images() []*Image {
	var result []*Image
	for _, e := range s.Elements {
		if img, ok := e.(*Image); ok {
			result = append(result, img)
		}
	}
	return result
}

// slideSize is the most common page size. A tie keeps the size that
// appears first, which is the first page when every size is tied.
func (p *Presentation) slideSize() (int64, int64) {
	if len(p.slides) == 0 {
		return 0, 0
	}
	type key struct{ w, h int64 }
	count := map[key]int{}
	best := key{p.slides[0].Width, p.slides[0].Height}
	bestN := 0
	for _, s := range p.slides {
		k := key{s.Width, s.Height}
		count[k]++
		if count[k] > bestN {
			best = k
			bestN = count[k]
		}
	}
	return best.w, best.h
}

// Save writes the presentation to a PPTX file at the given path.
func (p *Presentation) Save(outputPath string) error {
	f, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("无法写入目标路径: %w", err)
	}

	w := zip.NewWriter(f)

	writeErr := func() error {
		// One sldSz for the deck: the most common page, with other pages letterboxed.
		p.fitSlides()
		// Write OOXML structure
		if err := writeContentTypes(w, p); err != nil {
			return err
		}
		if err := writeRels(w); err != nil {
			return err
		}
		if err := writePresentationXML(w, p); err != nil {
			return err
		}
		if err := writePresentationRels(w, p); err != nil {
			return err
		}
		if err := writePackageParts(w, p); err != nil {
			return err
		}
		for i, slide := range p.slides {
			if err := writeSlideXML(w, slide, i+1); err != nil {
				return err
			}
			if err := writeSlideRels(w, slide, i+1); err != nil {
				return err
			}
		}
		return nil
	}()

	// Close zip writer first, then file
	if closeErr := w.Close(); closeErr != nil && writeErr == nil {
		writeErr = closeErr
	}
	if closeErr := f.Close(); closeErr != nil && writeErr == nil {
		writeErr = closeErr
	}

	// Clean up partial file on error
	if writeErr != nil {
		os.Remove(outputPath)
		return writeErr
	}

	return nil
}

// Close releases any resources held by the presentation.
func (p *Presentation) Close() error {
	p.slides = nil
	return nil
}
