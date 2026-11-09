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
	if content.HasBackground {
		slide.SetBackground(content.BgR, content.BgG, content.BgB)
	}

	elements := content.Elements
	if len(elements) == 0 {
		for i := range content.TextElements {
			te := content.TextElements[i]
			elements = append(elements, PageElement{Text: &te})
		}
		for i := range content.ImageElements {
			ie := content.ImageElements[i]
			elements = append(elements, PageElement{Image: &ie})
		}
	}

	for i, el := range elements {
		switch {
		case el.Text != nil:
			te := el.Text
			tb := &goppt.TextBox{
				Content:  te.Content,
				FontName: te.FontName,
				FontSize: te.FontSize,
				ColorR:   te.ColorR,
				ColorG:   te.ColorG,
				ColorB:   te.ColorB,
				Bold:     te.Bold,
				Italic:   te.Italic,
				X:        PointsToEMU(te.X),
				Y:        PointsToEMU(te.Y),
				Width:    PointsToEMU(te.Width),
				Height:   PointsToEMU(te.Height),
			}
			if err := slide.AddTextBox(tb); err != nil {
				log.Printf("warning: failed to add text box %d: %v", i, err)
			}
		case el.Image != nil:
			ie := el.Image
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
			}
		case el.Shape != nil:
			sh := el.Shape
			out := &goppt.Shape{
				X: PointsToEMU(sh.X), Y: PointsToEMU(sh.Y),
				Width: PointsToEMU(sh.Width), Height: PointsToEMU(sh.Height),
				HasFill: sh.HasFill, FillR: sh.FillR, FillG: sh.FillG, FillB: sh.FillB,
				FillAlpha: sh.FillAlpha,
				HasStroke: sh.HasStroke, StrokeR: sh.StrokeR, StrokeG: sh.StrokeG, StrokeB: sh.StrokeB,
				StrokeWidth: PointsToEMU(sh.StrokeWidth),
				EvenOdd:     sh.EvenOdd,
				SimpleRect:  sh.SimpleRect,
			}
			for _, op := range sh.Path {
				out.Path = append(out.Path, goppt.PathOp{
					Op: op.Op,
					X:  PointsToEMU(op.X), Y: PointsToEMU(op.Y),
					X1: PointsToEMU(op.X1), Y1: PointsToEMU(op.Y1),
					X2: PointsToEMU(op.X2), Y2: PointsToEMU(op.Y2),
				})
			}
			if err := slide.AddShape(out); err != nil {
				log.Printf("warning: failed to add shape %d: %v", i, err)
			}
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
