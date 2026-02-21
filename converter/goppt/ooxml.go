package goppt

import (
	"archive/zip"
	"fmt"
	"strings"
)

func writeContentTypes(w *zip.Writer, p *Presentation) error {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	sb.WriteString(`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">`)
	sb.WriteString(`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>`)
	sb.WriteString(`<Default Extension="xml" ContentType="application/xml"/>`)
	sb.WriteString(`<Default Extension="png" ContentType="image/png"/>`)
	sb.WriteString(`<Default Extension="jpeg" ContentType="image/jpeg"/>`)
	sb.WriteString(`<Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>`)
	for i := range p.slides {
		sb.WriteString(fmt.Sprintf(`<Override PartName="/ppt/slides/slide%d.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>`, i+1))
	}
	sb.WriteString(`</Types>`)

	f, err := w.Create("[Content_Types].xml")
	if err != nil {
		return err
	}
	_, err = f.Write([]byte(sb.String()))
	return err
}

func writeRels(w *zip.Writer) error {
	content := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/>
</Relationships>`

	f, err := w.Create("_rels/.rels")
	if err != nil {
		return err
	}
	_, err = f.Write([]byte(content))
	return err
}

func writePresentationXML(w *zip.Writer, p *Presentation) error {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	sb.WriteString(`<p:presentation xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">`)
	sb.WriteString(`<p:sldMasterIdLst/>`)
	sb.WriteString(`<p:sldIdLst>`)
	for i := range p.slides {
		sb.WriteString(fmt.Sprintf(`<p:sldId id="%d" r:id="rId%d"/>`, 256+i, i+1))
	}
	sb.WriteString(`</p:sldIdLst>`)
	if len(p.slides) > 0 {
		sb.WriteString(fmt.Sprintf(`<p:sldSz cx="%d" cy="%d"/>`, p.slides[0].Width, p.slides[0].Height))
	}
	sb.WriteString(`</p:presentation>`)

	f, err := w.Create("ppt/presentation.xml")
	if err != nil {
		return err
	}
	_, err = f.Write([]byte(sb.String()))
	return err
}

func writePresentationRels(w *zip.Writer, p *Presentation) error {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	sb.WriteString(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	for i := range p.slides {
		sb.WriteString(fmt.Sprintf(`<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide%d.xml"/>`, i+1, i+1))
	}
	sb.WriteString(`</Relationships>`)

	f, err := w.Create("ppt/_rels/presentation.xml.rels")
	if err != nil {
		return err
	}
	_, err = f.Write([]byte(sb.String()))
	return err
}

func writeSlideXML(w *zip.Writer, slide *Slide, slideNum int) error {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	sb.WriteString(`<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">`)
	sb.WriteString(`<p:cSld>`)
	sb.WriteString(`<p:spTree>`)
	sb.WriteString(`<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>`)
	sb.WriteString(`<p:grpSpPr/>`)

	shapeID := 2
	imageIdx := 1

	for _, elem := range slide.Elements {
		switch e := elem.(type) {
		case *TextBox:
			sb.WriteString(formatTextBox(e, shapeID))
			shapeID++
		case *Image:
			sb.WriteString(formatImage(e, shapeID, imageIdx))
			shapeID++
			imageIdx++
		}
	}

	sb.WriteString(`</p:spTree>`)
	sb.WriteString(`</p:cSld>`)
	sb.WriteString(`</p:sld>`)

	f, err := w.Create(fmt.Sprintf("ppt/slides/slide%d.xml", slideNum))
	if err != nil {
		return err
	}
	_, err = f.Write([]byte(sb.String()))
	return err
}

func formatTextBox(tb *TextBox, id int) string {
	colorHex := fmt.Sprintf("%02X%02X%02X", tb.ColorR, tb.ColorG, tb.ColorB)
	fontSize := int(tb.FontSize * 100) // OOXML font size is in hundredths of a point

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`<p:sp><p:nvSpPr><p:cNvPr id="%d" name="TextBox %d"/><p:cNvSpPr txBox="1"/><p:nvPr/></p:nvSpPr>`, id, id))
	sb.WriteString(fmt.Sprintf(`<p:spPr><a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm>`, tb.X, tb.Y, tb.Width, tb.Height))
	sb.WriteString(`<a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr>`)
	sb.WriteString(`<p:txBody><a:bodyPr wrap="square" rtlCol="0"/>`)
	sb.WriteString(fmt.Sprintf(`<a:p><a:r><a:rPr lang="zh-CN" sz="%d"><a:solidFill><a:srgbClr val="%s"/></a:solidFill>`, fontSize, colorHex))
	if tb.FontName != "" {
		sb.WriteString(fmt.Sprintf(`<a:latin typeface="%s"/><a:ea typeface="%s"/>`, tb.FontName, tb.FontName))
	}
	sb.WriteString(fmt.Sprintf(`</a:rPr><a:t>%s</a:t></a:r></a:p>`, escapeXML(tb.Content)))
	sb.WriteString(`</p:txBody></p:sp>`)

	return sb.String()
}

func formatImage(img *Image, id int, imageIdx int) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`<p:pic><p:nvPicPr><p:cNvPr id="%d" name="Image %d"/><p:cNvPicPr/><p:nvPr/></p:nvPicPr>`, id, id))
	sb.WriteString(fmt.Sprintf(`<p:blipFill><a:blip r:embed="rImg%d"/>`, imageIdx))
	sb.WriteString(`<a:stretch><a:fillRect/></a:stretch></p:blipFill>`)
	sb.WriteString(fmt.Sprintf(`<p:spPr><a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm>`, img.X, img.Y, img.Width, img.Height))
	sb.WriteString(`<a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr></p:pic>`)
	return sb.String()
}

func writeSlideRels(w *zip.Writer, slide *Slide, slideNum int) error {
	images := slide.Images()
	if len(images) == 0 {
		// No rels needed if no images
		return nil
	}

	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	sb.WriteString(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)

	for i, img := range images {
		ext := img.Format
		if ext == "" {
			ext = "png"
		}
		imgPath := fmt.Sprintf("../media/image_s%d_%d.%s", slideNum, i+1, ext)
		sb.WriteString(fmt.Sprintf(`<Relationship Id="rImg%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="%s"/>`, i+1, imgPath))

		// Also write the image data
		if len(img.Data) > 0 {
			imgFile, err := w.Create(fmt.Sprintf("ppt/media/image_s%d_%d.%s", slideNum, i+1, ext))
			if err != nil {
				return fmt.Errorf("failed to create image file: %w", err)
			}
			if _, err := imgFile.Write(img.Data); err != nil {
				return fmt.Errorf("failed to write image data: %w", err)
			}
		}
	}

	sb.WriteString(`</Relationships>`)

	f, err := w.Create(fmt.Sprintf("ppt/slides/_rels/slide%d.xml.rels", slideNum))
	if err != nil {
		return err
	}
	_, err = f.Write([]byte(sb.String()))
	return err
}

var xmlReplacer = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
	"'", "&apos;",
	`"`, "&quot;",
)

func escapeXML(s string) string {
	return xmlReplacer.Replace(s)
}


