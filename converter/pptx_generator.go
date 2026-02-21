package converter

import (
	"fmt"
	"log"

	"pdf2ppt/converter/goppt"
)

// PPTXGenerator PPTX 文件生成器
type PPTXGenerator struct {
	ppt *goppt.Presentation
}

// NewPPTXGenerator 创建 PPTX 生成器
func NewPPTXGenerator() *PPTXGenerator {
	return &PPTXGenerator{
		ppt: goppt.NewPresentation(),
	}
}

// Presentation returns the underlying GoPPT presentation for testing.
func (g *PPTXGenerator) Presentation() *goppt.Presentation {
	return g.ppt
}

// AddSlideFromContent 根据 PageContent 添加一张包含可编辑元素的幻灯片。
// 为每个 TextElement 创建可编辑文本框，为每个 ImageElement 插入独立图片。
// 单个元素创建失败时跳过该元素并记录警告，继续处理剩余元素。
func (g *PPTXGenerator) AddSlideFromContent(content *PageContent) error {
	widthEMU := PointsToEMU(content.PageWidth)
	heightEMU := PointsToEMU(content.PageHeight)

	slide := g.ppt.AddSlide(widthEMU, heightEMU)

	for i, te := range content.TextElements {
		tb := &goppt.TextBox{
			Content:  te.Content,
			FontName: te.FontName,
			FontSize: te.FontSize,
			ColorR:   te.ColorR,
			ColorG:   te.ColorG,
			ColorB:   te.ColorB,
			X:        PointsToEMU(te.X),
			Y:        PointsToEMU(te.Y),
			Width:    PointsToEMU(te.Width),
			Height:   PointsToEMU(te.Height),
		}
		if err := slide.AddTextBox(tb); err != nil {
			log.Printf("warning: failed to add text box %d: %v", i, err)
			continue
		}
	}

	for i, ie := range content.ImageElements {
		img := &goppt.Image{
			Data:   ie.Data,
			Format: ie.Format,
			X:      PointsToEMU(ie.X),
			Y:      PointsToEMU(ie.Y),
			Width:  PointsToEMU(ie.Width),
			Height: PointsToEMU(ie.Height),
		}
		if err := slide.AddImage(img); err != nil {
			log.Printf("warning: failed to add image %d: %v", i, err)
			continue
		}
	}

	return nil
}

// Save 将 PPTX 文件保存到指定路径
func (g *PPTXGenerator) Save(outputPath string) error {
	if err := g.ppt.Save(outputPath); err != nil {
		return &ConvertError{
			Code:    ErrCodeWriteError,
			Message: fmt.Sprintf("无法写入目标路径: %v", err),
		}
	}
	return nil
}

// Close 关闭并清理资源
func (g *PPTXGenerator) Close() error {
	if g.ppt != nil {
		return g.ppt.Close()
	}
	return nil
}
