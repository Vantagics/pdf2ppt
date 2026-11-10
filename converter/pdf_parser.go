package converter

import (
	"fmt"
	"github.com/Vantagics/pdf2ppt/converter/gopdf2"
	"strings"
)

// PDFParser PDF 文件解析器
type PDFParser struct {
	doc *gopdf2.Document
}

// NewPDFParser 创建 PDF 解析器
func NewPDFParser() *PDFParser {
	return &PDFParser{}
}

// Open 打开并验证 PDF 文件
// 验证文件是否存在、是否可读、是否为有效 PDF 格式
func (p *PDFParser) Open(filePath string) error {
	doc, err := gopdf2.Open(filePath)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "不是有效的 PDF 格式") {
			return &ConvertError{
				Code:    ErrCodeInvalidPDF,
				Message: msg,
			}
		}
		return &ConvertError{
			Code:    ErrCodeFileNotReadable,
			Message: msg,
		}
	}
	p.doc = doc
	return nil
}

// GetPageCount 获取 PDF 总页数
func (p *PDFParser) GetPageCount() int {
	if p.doc == nil {
		return 0
	}
	return p.doc.PageCount()
}

// ExtractPageContent 提取指定页面的所有可编辑元素
// pageNum 从 0 开始
func (p *PDFParser) ExtractPageContent(pageNum int) (*PageContent, error) {
	if p.doc == nil {
		return nil, &ConvertError{
			Code:    ErrCodePageExtract,
			Message: "PDF 文档未打开",
		}
	}
	page, err := p.doc.GetPage(pageNum)
	if err != nil {
		return nil, &ConvertError{
			Code:    ErrCodePageExtract,
			Message: fmt.Sprintf("提取第 %d 页内容时发生错误: %v", pageNum, err),
		}
	}

	content := &PageContent{
		PageWidth:     page.Width,
		PageHeight:    page.Height,
		HasBackground: page.HasBackground,
		BgR:           page.BgR,
		BgG:           page.BgG,
		BgB:           page.BgB,
		BgGradient:    gradientFromPDF(page.BgGradient),
		Elements:      make([]PageElement, 0, len(page.Elements)),
		TextElements:  make([]TextElement, 0, len(page.TextObjects)),
		ImageElements: make([]ImageElement, 0, len(page.Images)),
	}

	for _, el := range page.Elements {
		switch {
		case el.Text != nil:
			elem := textFromPDF(el.Text)
			content.Elements = append(content.Elements, PageElement{Text: &elem})
			content.TextElements = append(content.TextElements, elem)
		case el.Image != nil:
			elem := imageFromPDF(el.Image)
			content.Elements = append(content.Elements, PageElement{Image: &elem})
			content.ImageElements = append(content.ImageElements, elem)
		case el.Shape != nil:
			elem := shapeFromPDF(el.Shape)
			content.Elements = append(content.Elements, PageElement{Shape: &elem})
		}
	}
	// Pages produced by older callers that only fill the typed slices.
	if len(page.Elements) == 0 {
		for _, textObj := range page.TextObjects {
			elem := textFromPDF(textObj)
			content.TextElements = append(content.TextElements, elem)
			content.Elements = append(content.Elements, PageElement{Text: &elem})
		}
		for _, imgObj := range page.Images {
			elem := imageFromPDF(imgObj)
			content.ImageElements = append(content.ImageElements, elem)
			content.Elements = append(content.Elements, PageElement{Image: &elem})
		}
	}

	return content, nil
}

func textFromPDF(textObj *gopdf2.TextObject) TextElement {
	elem := TextElement{
		Content:     textObj.Content,
		ColorR:      textObj.ColorR,
		ColorG:      textObj.ColorG,
		ColorB:      textObj.ColorB,
		Bold:        textObj.Bold,
		Italic:      textObj.Italic,
		X:           textObj.X,
		Y:           textObj.Y,
		Width:       textObj.Width,
		Height:      textObj.Height,
		Rotation:    textObj.Rotation,
		FontName:    textObj.FontName,
		FontSize:    textObj.FontSize,
		Stroke:      textObj.Stroke,
		StrokeOnly:  textObj.StrokeOnly,
		StrokeR:     textObj.StrokeR,
		StrokeG:     textObj.StrokeG,
		StrokeB:     textObj.StrokeB,
		StrokeWidth: textObj.StrokeWidth,
		Tracking:    textObj.Tracking,
	}
	for _, r := range textObj.Runs {
		elem.Runs = append(elem.Runs, TextRun{
			Content: r.Content, FontName: r.FontName, FontSize: r.FontSize,
			Bold: r.Bold, Italic: r.Italic,
			ColorR: r.ColorR, ColorG: r.ColorG, ColorB: r.ColorB,
		})
	}
	if elem.FontName == "" {
		elem.FontName = DefaultFontName
	}
	if elem.FontSize == 0 {
		elem.FontSize = DefaultFontSize
	}
	if elem.Width < 1 {
		elem.Width = 1
	}
	if elem.Height < 1 {
		elem.Height = elem.FontSize
	}
	return elem
}

func imageFromPDF(imgObj *gopdf2.ImageObject) ImageElement {
	return ImageElement{
		Data:     imgObj.Data,
		Format:   imgObj.Format,
		X:        imgObj.X,
		Y:        imgObj.Y,
		Width:    imgObj.Width,
		Height:   imgObj.Height,
		Rotation: imgObj.Rotation,
	}
}

func shapeFromPDF(sh *gopdf2.ShapeObject) ShapeElement {
	elem := ShapeElement{
		X: sh.X, Y: sh.Y, Width: sh.Width, Height: sh.Height,
		HasFill: sh.HasFill, FillR: sh.FillR, FillG: sh.FillG, FillB: sh.FillB,
		FillAlpha: sh.FillAlpha,
		HasStroke: sh.HasStroke, StrokeR: sh.StrokeR, StrokeG: sh.StrokeG, StrokeB: sh.StrokeB,
		StrokeWidth: sh.StrokeWidth, EvenOdd: sh.EvenOdd, SimpleRect: sh.SimpleRect,
	}
	if elem.FillAlpha <= 0 {
		elem.FillAlpha = 1
	}
	for _, op := range sh.Path {
		elem.Path = append(elem.Path, PathOp{
			Op: op.Op, X: op.X, Y: op.Y, X1: op.X1, Y1: op.Y1, X2: op.X2, Y2: op.Y2,
		})
	}
	elem.Gradient = gradientFromPDF(sh.Gradient)
	return elem
}

func gradientFromPDF(g *gopdf2.Gradient) *GradientFill {
	if g == nil || len(g.Stops) == 0 {
		return nil
	}
	out := &GradientFill{Angle: g.Angle, Kind: g.Kind}
	for _, s := range g.Stops {
		out.Stops = append(out.Stops, GradientStop{Pos: s.Pos, R: s.R, G: s.G, B: s.B})
	}
	return out
}

// Close 关闭 PDF 文档，释放资源
func (p *PDFParser) Close() error {
	if p.doc != nil {
		return p.doc.Close()
	}
	return nil
}
