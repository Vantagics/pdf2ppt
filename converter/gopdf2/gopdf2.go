// Package gopdf2 reads PDF pages into editable text, images, and vector shapes.
// Coordinates on a Page are in points with the origin at the top-left (y grows
// downward), matching PowerPoint. The package understands compressed content
// streams, ToUnicode maps (including CJK), images, and path fills.
package gopdf2

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/validate"
)

// Document is an opened PDF.
type Document struct {
	pages []*Page
	ctx   *model.Context
}

// Page is one PDF page worth of drawing commands.
type Page struct {
	Width         float64
	Height        float64
	HasBackground bool
	BgR, BgG, BgB uint8
	BgGradient    *Gradient
	Elements      []PageElement
	// TextObjects and Images mirror Elements for callers that scan by type.
	TextObjects []*TextObject
	Images      []*ImageObject
}

// PageElement is one painted object in content-stream order.
type PageElement struct {
	Text  *TextObject
	Image *ImageObject
	Shape *ShapeObject
}

// TextObject is a single text run (one Tj/TJ showing).
type TextObject struct {
	Content  string
	FontName string
	FontSize float64
	Bold     bool
	Italic   bool
	ColorR   uint8
	ColorG   uint8
	ColorB   uint8
	X        float64
	Y        float64
	Width    float64
	Height   float64
	Rotation float64 // clockwise degrees, matching DrawingML
	// Stroke is set for PDF text rendering modes that outline the glyphs
	// (modes 1, 2, 5, and 6). StrokeOnly is mode 1 or 5 (no fill).
	Stroke                    bool
	StrokeOnly                bool
	StrokeR, StrokeG, StrokeB uint8
	StrokeWidth               float64
	// Tracking is extra space between glyphs, in points. Negative tightens
	// a fallback font so the run stays inside the PDF advance.
	Tracking float64
	// Clip stops the run from painting outside its box. Set when the PDF
	// advance runs into the next word.
	Clip bool
	// NoFill keeps the characters in the file for recall but does not paint
	// them. A Type3 outline is the visible glyph.
	NoFill bool
	// RTL is set when the text is Hebrew or Arabic in logical order.
	RTL bool
	// Vertical is East Asian top-to-bottom text (Identity-V / WMode 1).
	// The box is one column: narrow, and taller than it is wide.
	Vertical bool
	// symbolProg is the embedded font for a one-character symbolic run.
	// replaceIsolatedSymbols draws it when the run stays isolated.
	symbolProg []byte
	// Runs is set when several PDF runs on one line were merged into this
	// box. Content is the concatenation. Empty means a single run.
	Runs []TextRun
	// holdX keeps this glyph from being merged into the run beside it.
	// PowerPoint lays a merged run out from the box origin, which put the
	// KUST "(" on top of z₁.
	holdX bool
}

// TextRun is one style span inside a merged line.
type TextRun struct {
	Content  string
	FontName string
	FontSize float64
	Bold     bool
	Italic   bool
	ColorR   uint8
	ColorG   uint8
	ColorB   uint8
	// Baseline is the DrawingML baseline shift in 1000ths of a percent.
	// Negative drops a subscript (a typical subscript is -25000).
	Baseline float64
}

// ImageObject is a raster image placed on the page.
type ImageObject struct {
	Data     []byte
	Format   string // "png" or "jpeg"
	X        float64
	Y        float64
	Width    float64
	Height   float64
	Rotation float64 // clockwise degrees, matching DrawingML
	// CropL, CropT, CropR, CropB are fractions of the source bitmap removed
	// from each edge (DrawingML a:srcRect). The placement box is the remainder.
	CropL, CropT, CropR, CropB float64
	// Stencil is a 1-bit soft mask painted through a tiny color sample.
	// The glyphs are the mask. Nearby stencils are composited into one picture.
	Stencil bool
}

// PathOp is one command of a shape outline in shape-local, y-down points.
type PathOp struct {
	Op     string // "M", "L", "C", "Z"
	X, Y   float64
	X1, Y1 float64
	X2, Y2 float64
}

// GradStop is one color stop. Pos is 0..1 along the axis.
type GradStop struct {
	Pos     float64
	R, G, B uint8
}

// Gradient is an axial blend recovered from a PDF shading pattern.
type Gradient struct {
	Angle float64 // clockwise degrees, for a linear DrawingML fill
	Kind  string  // "linear" or "radial"
	Stops []GradStop
}

// ShapeObject is a filled and/or stroked path.
type ShapeObject struct {
	X, Y, Width, Height       float64
	HasFill                   bool
	FillR, FillG, FillB       uint8
	FillAlpha                 float64
	HasStroke                 bool
	StrokeR, StrokeG, StrokeB uint8
	StrokeWidth               float64
	EvenOdd                   bool
	SimpleRect                bool
	Path                      []PathOp
	Gradient                  *Gradient
	// Glyph marks a Type 3 outline. Adjacent glyphs on one line become one shape.
	Glyph bool
}

// Open reads and validates a PDF file.
func Open(filePath string) (*Document, error) {
	f, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New("无法读取文件")
		}
		return nil, fmt.Errorf("无法读取文件: %w", err)
	}
	defer f.Close()

	header := make([]byte, 5)
	n, err := f.Read(header)
	if err != nil || n < 5 || string(header) != "%PDF-" {
		return nil, errors.New("文件不是有效的 PDF 格式")
	}

	if _, err := f.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("无法读取文件: %w", err)
	}
	ctx, err := api.ReadContext(f, model.NewDefaultConfiguration())
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "no such file") || strings.Contains(msg, "permission") {
			return nil, fmt.Errorf("无法读取文件: %w", err)
		}
		return nil, fmt.Errorf("文件不是有效的 PDF 格式: %w", err)
	}
	// Some TeX files name a destination with an indirect object where a name
	// is required. That does not affect the page content, so keep the file.
	if err := validate.XRefTable(ctx); err != nil && !strings.Contains(err.Error(), "validateDestinationArray") {
		return nil, fmt.Errorf("文件不是有效的 PDF 格式: %w", err)
	}

	doc := &Document{ctx: ctx}
	if err := doc.extractPages(); err != nil {
		return nil, err
	}
	return doc, nil
}

// PageCount returns the number of pages.
func (d *Document) PageCount() int {
	if d == nil {
		return 0
	}
	return len(d.pages)
}

// GetPage returns the page at index (0-based).
func (d *Document) GetPage(index int) (*Page, error) {
	if d == nil || index < 0 || index >= len(d.pages) {
		return nil, fmt.Errorf("页码 %d 超出范围", index)
	}
	return d.pages[index], nil
}

// Close releases the document.
func (d *Document) Close() error {
	d.ctx = nil
	return nil
}
