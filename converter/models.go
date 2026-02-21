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
	Content  string  `json:"content"`
	FontName string  `json:"fontName"`
	FontSize float64 `json:"fontSize"`
	ColorR   uint8   `json:"colorR"`
	ColorG   uint8   `json:"colorG"`
	ColorB   uint8   `json:"colorB"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Width    float64 `json:"width"`
	Height   float64 `json:"height"`
}

// ImageElement 从 PDF 页面中提取的图片元素
type ImageElement struct {
	Data   []byte  `json:"-"`
	Format string  `json:"format"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// PageContent 一个 PDF 页面中提取的所有元素集合
type PageContent struct {
	PageWidth     float64        `json:"pageWidth"`
	PageHeight    float64        `json:"pageHeight"`
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
