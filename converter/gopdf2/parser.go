package gopdf2

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// Pre-compiled regex patterns (compiled once at package init).
var (
	pagePattern     = regexp.MustCompile(`(?s)/Type\s*/Page[^s]`)
	mediaBoxPattern = regexp.MustCompile(`/MediaBox\s*\[\s*([\d.]+)\s+([\d.]+)\s+([\d.]+)\s+([\d.]+)\s*\]`)
	streamPattern   = regexp.MustCompile(`(?s)stream\r?\n(.*?)endstream`)
	textBlockPattern = regexp.MustCompile(`(?s)BT\s*(.*?)\s*ET`)
	tdPattern       = regexp.MustCompile(`([\d.-]+)\s+([\d.-]+)\s+Td`)
	tmPattern       = regexp.MustCompile(`([\d.-]+)\s+[\d.-]+\s+[\d.-]+\s+([\d.-]+)\s+([\d.-]+)\s+([\d.-]+)\s+Tm`)
	tfPattern       = regexp.MustCompile(`/(\S+)\s+([\d.]+)\s+Tf`)
	tjPattern       = regexp.MustCompile(`\((.*?)\)\s*Tj`)
	tjArrayPattern  = regexp.MustCompile(`\[(.*?)\]\s*TJ`)
	imagePattern    = regexp.MustCompile(`(?s)/Subtype\s*/Image.*?/Width\s+(\d+).*?/Height\s+(\d+)`)
)

const maxPDFSize = 500 * 1024 * 1024 // 500MB

// parsePDF performs basic PDF parsing to extract pages, text, and images.
// This is a simplified parser that handles common PDF structures.
func parsePDF(f *os.File) (*Document, error) {
	data, err := io.ReadAll(io.LimitReader(f, maxPDFSize+1))
	if err != nil {
		return nil, fmt.Errorf("无法读取文件: %w", err)
	}
	if len(data) > maxPDFSize {
		return nil, fmt.Errorf("PDF 文件过大，最大支持 500MB")
	}

	doc := &Document{
		pages: make([]*Page, 0),
	}

	// Find all page objects and extract content
	pages, err := extractPages(data)
	if err != nil {
		return nil, err
	}
	doc.pages = pages

	return doc, nil
}

// extractPages finds page definitions in the PDF and extracts their content.
func extractPages(data []byte) ([]*Page, error) {
	pages := make([]*Page, 0)

	// Find page objects by looking for /Type /Page entries
	// This is a simplified approach for common PDF structures
	matches := pagePattern.FindAllIndex(data, -1)

	for _, match := range matches {
		page := &Page{
			Width:       612, // Default US Letter width in points
			Height:      792, // Default US Letter height in points
			TextObjects: make([]*TextObject, 0),
			Images:      make([]*ImageObject, 0),
		}

		// Look for MediaBox near this page definition
		// Search in a window around the match
		searchStart := match[0]
		searchEnd := match[0] + 2000
		if searchEnd > len(data) {
			searchEnd = len(data)
		}
		// Also look backwards
		if searchStart > 500 {
			searchStart = match[0] - 500
		} else {
			searchStart = 0
		}

		if mb := mediaBoxPattern.FindSubmatch(data[searchStart:searchEnd]); mb != nil {
			if w, err := strconv.ParseFloat(string(mb[3]), 64); err == nil {
				page.Width = w
			}
			if h, err := strconv.ParseFloat(string(mb[4]), 64); err == nil {
				page.Height = h
			}
		}

		pages = append(pages, page)
	}

	// If we found pages, try to extract text content from streams
	if len(pages) > 0 {
		extractTextFromStreams(data, pages)
		extractImagesFromStreams(data, pages)
	}

	return pages, nil
}

// extractTextFromStreams attempts to extract text objects from PDF content streams.
func extractTextFromStreams(data []byte, pages []*Page) {
	// Find content streams between "stream" and "endstream" markers
	streams := streamPattern.FindAllSubmatch(data, -1)

	// Simple text extraction from PDF operators
	// BT ... ET blocks contain text operations

	pageIdx := 0
	for _, stream := range streams {
		if pageIdx >= len(pages) {
			break
		}

		content := stream[1]
		textBlocks := textBlockPattern.FindAllSubmatch(content, -1)

		for _, block := range textBlocks {
			textObjs := parseTextBlock(block[1])
			if len(textObjs) > 0 {
				pages[pageIdx].TextObjects = append(pages[pageIdx].TextObjects, textObjs...)
			}
		}

		// Move to next page if this stream had content
		if len(textBlocks) > 0 {
			pageIdx++
		}
	}
}

// parseTextBlock parses a BT...ET text block and extracts text objects.
func parseTextBlock(block []byte) []*TextObject {
	objects := make([]*TextObject, 0)
	scanner := bufio.NewScanner(bytes.NewReader(block))

	var currentX, currentY float64
	var currentFontName string
	var currentFontSize float64

	for scanner.Scan() {
		line := scanner.Text()

		// Font setting: /FontName size Tf
		if m := tfPattern.FindStringSubmatch(line); m != nil {
			currentFontName = m[1]
			if size, err := strconv.ParseFloat(m[2], 64); err == nil {
				currentFontSize = size
			}
		}

		// Text position: x y Td
		if m := tdPattern.FindStringSubmatch(line); m != nil {
			if x, err := strconv.ParseFloat(m[1], 64); err == nil {
				currentX = x
			}
			if y, err := strconv.ParseFloat(m[2], 64); err == nil {
				currentY = y
			}
		}

		// Text matrix: a b c d x y Tm
		if m := tmPattern.FindStringSubmatch(line); m != nil {
			if x, err := strconv.ParseFloat(m[3], 64); err == nil {
				currentX = x
			}
			if y, err := strconv.ParseFloat(m[4], 64); err == nil {
				currentY = y
			}
		}

		// Simple text show: (text) Tj
		if m := tjPattern.FindStringSubmatch(line); m != nil {
			text := m[1]
			if text != "" {
				obj := &TextObject{
					Content:  text,
					FontName: currentFontName,
					FontSize: currentFontSize,
					X:        currentX,
					Y:        currentY,
					Width:    float64(len(text)) * currentFontSize * 0.5, // Approximate
					Height:   currentFontSize * 1.2,                      // Approximate
				}
				objects = append(objects, obj)
			}
		}

		// Array text show: [(text) kern (text)] TJ
		if m := tjArrayPattern.FindStringSubmatch(line); m != nil {
			text := extractTextFromTJArray(m[1])
			if text != "" {
				obj := &TextObject{
					Content:  text,
					FontName: currentFontName,
					FontSize: currentFontSize,
					X:        currentX,
					Y:        currentY,
					Width:    float64(len(text)) * currentFontSize * 0.5,
					Height:   currentFontSize * 1.2,
				}
				objects = append(objects, obj)
			}
		}
	}

	return objects
}

// extractTextFromTJArray extracts text content from a TJ array string.
func extractTextFromTJArray(arrayContent string) string {
	var result strings.Builder
	inParens := false
	for i := 0; i < len(arrayContent); i++ {
		if arrayContent[i] == '(' && !inParens {
			inParens = true
			continue
		}
		if arrayContent[i] == ')' && inParens {
			inParens = false
			continue
		}
		if inParens {
			result.WriteByte(arrayContent[i])
		}
	}
	return result.String()
}

// extractImagesFromStreams attempts to extract image objects from PDF.
func extractImagesFromStreams(data []byte, pages []*Page) {
	// Look for image XObjects
	// This is a simplified approach - real PDF image extraction is complex
	matches := imagePattern.FindAllSubmatch(data, -1)

	// Distribute images across pages evenly; if more images than pages,
	// remaining images go to the last page.
	for i, match := range matches {
		pageIdx := i
		if pageIdx >= len(pages) {
			pageIdx = len(pages) - 1
		}

		width, _ := strconv.ParseFloat(string(match[1]), 64)
		height, _ := strconv.ParseFloat(string(match[2]), 64)

		img := &ImageObject{
			Data:   []byte{}, // Actual image data extraction is complex
			Format: "png",
			X:      0,
			Y:      0,
			Width:  width,
			Height: height,
		}
		pages[pageIdx].Images = append(pages[pageIdx].Images, img)
	}
}
