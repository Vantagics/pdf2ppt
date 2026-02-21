// Package gopdf2 provides PDF parsing capabilities.
// This is a local implementation providing the interfaces needed
// for PDF text and image extraction.
package gopdf2

import (
	"errors"
	"fmt"
	"io"
	"os"
)

// Document represents an opened PDF document.
type Document struct {
	pages []*Page
	file  *os.File
}

// Page represents a single page in a PDF document.
type Page struct {
	Width       float64
	Height      float64
	TextObjects []*TextObject
	Images      []*ImageObject
}

// TextObject represents a text element on a PDF page.
type TextObject struct {
	Content  string
	FontName string  // Empty string means font info is missing
	FontSize float64 // 0 means font size info is missing
	ColorR   uint8
	ColorG   uint8
	ColorB   uint8
	X        float64
	Y        float64
	Width    float64
	Height   float64
}

// ImageObject represents an image element on a PDF page.
type ImageObject struct {
	Data   []byte
	Format string  // "png", "jpeg", etc.
	X      float64
	Y      float64
	Width  float64
	Height float64
}

// Open opens and validates a PDF file, returning a Document.
// Returns an error if the file doesn't exist, can't be read, or is not a valid PDF.
func Open(filePath string) (*Document, error) {
	f, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New("无法读取文件")
		}
		return nil, fmt.Errorf("无法读取文件: %w", err)
	}

	// Read the first bytes to validate PDF magic number
	header := make([]byte, 5)
	n, err := f.Read(header)
	if err != nil || n < 5 {
		f.Close()
		return nil, errors.New("文件不是有效的 PDF 格式")
	}
	if string(header[:5]) != "%PDF-" {
		f.Close()
		return nil, errors.New("文件不是有效的 PDF 格式")
	}

	// Reset to beginning for full parsing
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		f.Close()
		return nil, fmt.Errorf("无法读取文件: %w", err)
	}

	// Parse the PDF content
	doc, err := parsePDF(f)
	if err != nil {
		f.Close()
		return nil, err
	}
	doc.file = f
	return doc, nil
}

// PageCount returns the number of pages in the document.
func (d *Document) PageCount() int {
	return len(d.pages)
}

// GetPage returns the page at the given index (0-based).
func (d *Document) GetPage(index int) (*Page, error) {
	if index < 0 || index >= len(d.pages) {
		return nil, fmt.Errorf("页码 %d 超出范围 (0-%d)", index, len(d.pages)-1)
	}
	return d.pages[index], nil
}

// Close closes the document and releases resources.
func (d *Document) Close() error {
	if d.file != nil {
		return d.file.Close()
	}
	return nil
}
