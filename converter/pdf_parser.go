package converter

import (
	"fmt"
	"pdf2ppt/converter/gopdf2"
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
		TextElements:  make([]TextElement, 0, len(page.TextObjects)),
		ImageElements: make([]ImageElement, 0, len(page.Images)),
	}

	// 提取文本元素
	for _, textObj := range page.TextObjects {
		elem := TextElement{
			Content: textObj.Content,
			ColorR:  textObj.ColorR,
			ColorG:  textObj.ColorG,
			ColorB:  textObj.ColorB,
			X:       textObj.X,
			Y:       textObj.Y,
			Width:   textObj.Width,
			Height:  textObj.Height,
		}

		// 字体缺失时使用默认值
		if textObj.FontName == "" {
			elem.FontName = DefaultFontName
		} else {
			elem.FontName = textObj.FontName
		}

		if textObj.FontSize == 0 {
			elem.FontSize = DefaultFontSize
		} else {
			elem.FontSize = textObj.FontSize
		}

		content.TextElements = append(content.TextElements, elem)
	}

	// 提取图片元素
	for _, imgObj := range page.Images {
		elem := ImageElement{
			Data:   imgObj.Data,
			Format: imgObj.Format,
			X:      imgObj.X,
			Y:      imgObj.Y,
			Width:  imgObj.Width,
			Height: imgObj.Height,
		}
		content.ImageElements = append(content.ImageElements, elem)
	}

	return content, nil
}

// Close 关闭 PDF 文档，释放资源
func (p *PDFParser) Close() error {
	if p.doc != nil {
		return p.doc.Close()
	}
	return nil
}
