package converter

import "math"

// Default value constants for text elements with missing font information.
const (
	DefaultFontName = "Arial"
	DefaultFontSize = 12.0
	DefaultColorR   = 0
	DefaultColorG   = 0
	DefaultColorB   = 0
)

// Error code constants.
const (
	ErrCodeInvalidPDF      = "INVALID_PDF"
	ErrCodeEmptyPDF        = "EMPTY_PDF"
	ErrCodeFileNotReadable = "FILE_NOT_READABLE"
	ErrCodePageExtract     = "PAGE_EXTRACT_ERROR"
	ErrCodeWriteError      = "WRITE_ERROR"
	ErrCodeInternalError   = "INTERNAL_ERROR"
)

// TextElement 从 PDF 页面中提取的文本元素
type TextElement struct {
	Content     string    `json:"content"`
	FontName    string    `json:"fontName"`
	FontSize    float64   `json:"fontSize"`
	ColorR      uint8     `json:"colorR"`
	ColorG      uint8     `json:"colorG"`
	ColorB      uint8     `json:"colorB"`
	Bold        bool      `json:"bold"`
	Italic      bool      `json:"italic"`
	X           float64   `json:"x"`
	Y           float64   `json:"y"`
	Width       float64   `json:"width"`
	Height      float64   `json:"height"`
	Rotation    float64   `json:"rotation,omitempty"`
	Stroke      bool      `json:"stroke,omitempty"`
	StrokeOnly  bool      `json:"strokeOnly,omitempty"`
	StrokeR     uint8     `json:"strokeR,omitempty"`
	StrokeG     uint8     `json:"strokeG,omitempty"`
	StrokeB     uint8     `json:"strokeB,omitempty"`
	StrokeWidth float64   `json:"strokeWidth,omitempty"`
	Tracking    float64   `json:"tracking,omitempty"`
	Clip        bool      `json:"clip,omitempty"`
	NoFill      bool      `json:"noFill,omitempty"`
	RTL         bool      `json:"rtl,omitempty"`
	Vertical    bool      `json:"vertical,omitempty"`
	Runs        []TextRun `json:"runs,omitempty"`
}

// TextRun is one style span inside a line that was merged into one text box.
type TextRun struct {
	Content  string  `json:"content"`
	FontName string  `json:"fontName"`
	FontSize float64 `json:"fontSize"`
	Bold     bool    `json:"bold,omitempty"`
	Italic   bool    `json:"italic,omitempty"`
	ColorR   uint8   `json:"colorR"`
	ColorG   uint8   `json:"colorG"`
	ColorB   uint8   `json:"colorB"`
	Baseline float64 `json:"baseline,omitempty"`
}

// PathOp is one command of a vector outline. Coordinates are in points,
// relative to the shape box, with y growing downward.
type PathOp struct {
	Op string  `json:"op"` // M, L, C, Z
	X  float64 `json:"x"`
	Y  float64 `json:"y"`
	X1 float64 `json:"x1,omitempty"`
	Y1 float64 `json:"y1,omitempty"`
	X2 float64 `json:"x2,omitempty"`
	Y2 float64 `json:"y2,omitempty"`
}

// ShapeElement is a filled and/or stroked vector path (rectangles, rules,
// rounded rectangles, and other outlines recovered from the PDF).
type ShapeElement struct {
	X           float64       `json:"x"`
	Y           float64       `json:"y"`
	Width       float64       `json:"width"`
	Height      float64       `json:"height"`
	HasFill     bool          `json:"hasFill"`
	FillR       uint8         `json:"fillR"`
	FillG       uint8         `json:"fillG"`
	FillB       uint8         `json:"fillB"`
	FillAlpha   float64       `json:"fillAlpha"`
	HasStroke   bool          `json:"hasStroke"`
	StrokeR     uint8         `json:"strokeR"`
	StrokeG     uint8         `json:"strokeG"`
	StrokeB     uint8         `json:"strokeB"`
	StrokeWidth float64       `json:"strokeWidth"`
	EvenOdd     bool          `json:"evenOdd"`
	SimpleRect  bool          `json:"simpleRect"`
	Path        []PathOp      `json:"path,omitempty"`
	Gradient    *GradientFill `json:"gradient,omitempty"`
}

// GradientStop is one color along an axial blend. Pos runs from 0 to 1.
type GradientStop struct {
	Pos float64 `json:"pos"`
	R   uint8   `json:"r"`
	G   uint8   `json:"g"`
	B   uint8   `json:"b"`
}

// GradientFill is a linear gradient recovered from a PDF shading.
type GradientFill struct {
	Angle float64        `json:"angle"`
	Kind  string         `json:"kind,omitempty"`
	Stops []GradientStop `json:"stops"`
}

// PageElement is one drawable in paint order. Exactly one of the pointers is set.
type PageElement struct {
	Text  *TextElement  `json:"text,omitempty"`
	Image *ImageElement `json:"image,omitempty"`
	Shape *ShapeElement `json:"shape,omitempty"`
}

// ImageElement 从 PDF 页面中提取的图片元素
type ImageElement struct {
	Data     []byte  `json:"-"`
	Format   string  `json:"format"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Width    float64 `json:"width"`
	Height   float64 `json:"height"`
	Rotation float64 `json:"rotation,omitempty"`
	// Crop fractions of the source bitmap, matching DrawingML a:srcRect.
	CropL float64 `json:"cropL,omitempty"`
	CropT float64 `json:"cropT,omitempty"`
	CropR float64 `json:"cropR,omitempty"`
	CropB float64 `json:"cropB,omitempty"`
}

// PageContent 一个 PDF 页面中提取的所有元素集合.
// Coordinates are points, origin at the top-left, y growing downward.
type PageContent struct {
	PageWidth     float64        `json:"pageWidth"`
	PageHeight    float64        `json:"pageHeight"`
	HasBackground bool           `json:"hasBackground"`
	BgR           uint8          `json:"bgR"`
	BgG           uint8          `json:"bgG"`
	BgB           uint8          `json:"bgB"`
	BgGradient    *GradientFill  `json:"bgGradient,omitempty"`
	Elements      []PageElement  `json:"elements"`
	TextElements  []TextElement  `json:"textElements"`
	ImageElements []ImageElement `json:"imageElements"`
}

// ConversionProgress 转换进度信息
type ConversionProgress struct {
	Current int `json:"current"`
	Total   int `json:"total"`
}

// ConvertResult 转换结果
type ConvertResult struct {
	OutputPath string `json:"outputPath"`
	PageCount  int    `json:"pageCount"`
}

// ConvertError 转换错误
type ConvertError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Error implements the error interface for ConvertError.
func (e *ConvertError) Error() string {
	return e.Message
}

// PointsToEMU 将磅值转换为 EMU
// 1 磅 = 12700 EMU
func PointsToEMU(points float64) int64 {
	result := points * 12700
	if result > float64(math.MaxInt64) {
		return math.MaxInt64
	}
	if result < float64(math.MinInt64) {
		return math.MinInt64
	}
	return int64(result)
}
