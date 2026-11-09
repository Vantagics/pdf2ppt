package goppt

import (
	"archive/zip"
	"fmt"
	"math"
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
	if w, h := p.slideSize(); w > 0 && h > 0 {
		sb.WriteString(fmt.Sprintf(`<p:sldSz cx="%d" cy="%d" type="custom"/>`, w, h))
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
	if slide.BgGradient != nil {
		sb.WriteString(`<p:bg><p:bgPr>`)
		sb.WriteString(formatGradFill(slide.BgGradient))
		sb.WriteString(`<a:effectLst/></p:bgPr></p:bg>`)
	} else if slide.HasBackground {
		sb.WriteString(`<p:bg><p:bgPr>`)
		sb.WriteString(fmt.Sprintf(`<a:solidFill><a:srgbClr val="%02X%02X%02X"/></a:solidFill>`, slide.BgR, slide.BgG, slide.BgB))
		sb.WriteString(`<a:effectLst/></p:bgPr></p:bg>`)
	}
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
		case *Shape:
			sb.WriteString(formatShape(e, shapeID))
			shapeID++
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
	fontSize := int(math.Round(tb.FontSize * 100)) // hundredths of a point
	if fontSize < 100 {
		fontSize = 100
	}
	lang := "en-US"
	if containsCJK(tb.Content) {
		lang = "zh-CN"
	}
	b := "0"
	if tb.Bold {
		b = "1"
	}
	i := "0"
	if tb.Italic {
		i = "1"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`<p:sp><p:nvSpPr><p:cNvPr id="%d" name="TextBox %d"/><p:cNvSpPr txBox="1"/><p:nvPr/></p:nvSpPr>`, id, id))
	rot := ""
	if math.Abs(tb.Rotation) > 0.4 {
		rot = fmt.Sprintf(` rot="%d"`, int64(math.Round(tb.Rotation*60000)))
	}
	sb.WriteString(fmt.Sprintf(`<p:spPr><a:xfrm%s><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm>`, rot, tb.X, tb.Y, tb.Width, tb.Height))
	sb.WriteString(`<a:prstGeom prst="rect"><a:avLst/></a:prstGeom><a:noFill/><a:ln><a:noFill/></a:ln></p:spPr>`)
	sb.WriteString(`<p:txBody><a:bodyPr wrap="none" lIns="0" tIns="0" rIns="0" bIns="0" rtlCol="0" anchor="t" horzOverflow="overflow" vertOverflow="overflow"/>`)
	sb.WriteString(`<a:lstStyle/>`)
	// 100% line spacing keeps the first baseline on the font's em box. The
	// default (omitted) spacing adds external leading and drops the glyphs.
	sb.WriteString(`<a:p><a:pPr marL="0" indent="0"><a:lnSpc><a:spcPct val="100000"/></a:lnSpc><a:spcBef><a:spcPts val="0"/></a:spcBef><a:spcAft><a:spcPts val="0"/></a:spcAft><a:buNone/></a:pPr>`)
	sb.WriteString(fmt.Sprintf(`<a:r><a:rPr lang="%s" sz="%d" b="%s" i="%s" dirty="0"><a:solidFill><a:srgbClr val="%s"/></a:solidFill>`, lang, fontSize, b, i, colorHex))
	if tb.FontName != "" {
		name := escapeXML(tb.FontName)
		sb.WriteString(fmt.Sprintf(`<a:latin typeface="%s"/><a:ea typeface="%s"/><a:cs typeface="%s"/>`, name, name, name))
	}
	sb.WriteString(fmt.Sprintf(`</a:rPr><a:t>%s</a:t></a:r></a:p>`, escapeXML(tb.Content)))
	sb.WriteString(`</p:txBody></p:sp>`)
	return sb.String()
}

func formatGradFill(g *Gradient) string {
	if g == nil || len(g.Stops) == 0 {
		return `<a:solidFill><a:srgbClr val="FFFFFF"/></a:solidFill>`
	}
	ang := int64(math.Round(g.Angle * 60000))
	var sb strings.Builder
	sb.WriteString(`<a:gradFill flip="none" rotWithShape="1"><a:gsLst>`)
	for _, s := range g.Stops {
		pos := int(math.Round(s.Pos * 100000))
		if pos < 0 {
			pos = 0
		}
		if pos > 100000 {
			pos = 100000
		}
		sb.WriteString(fmt.Sprintf(`<a:gs pos="%d"><a:srgbClr val="%02X%02X%02X"/></a:gs>`, pos, s.R, s.G, s.B))
	}
	sb.WriteString(fmt.Sprintf(`</a:gsLst><a:lin ang="%d" scaled="1"/></a:gradFill>`, ang))
	return sb.String()
}

func containsCJK(s string) bool {
	for _, r := range s {
		if r >= 0x2E80 && r <= 0x9FFF {
			return true
		}
	}
	return false
}

func formatShape(sh *Shape, id int) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`<p:sp><p:nvSpPr><p:cNvPr id="%d" name="Shape %d"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr>`, id, id))
	sb.WriteString(`<p:spPr>`)
	sb.WriteString(fmt.Sprintf(`<a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm>`, sh.X, sh.Y, sh.Width, sh.Height))
	if sh.SimpleRect || len(sh.Path) == 0 {
		sb.WriteString(`<a:prstGeom prst="rect"><a:avLst/></a:prstGeom>`)
	} else {
		sb.WriteString(formatCustGeom(sh))
	}
	if sh.Gradient != nil {
		sb.WriteString(formatGradFill(sh.Gradient))
	} else if sh.HasFill {
		sb.WriteString(`<a:solidFill><a:srgbClr val="` + fmt.Sprintf("%02X%02X%02X", sh.FillR, sh.FillG, sh.FillB) + `"`)
		if sh.FillAlpha > 0 && sh.FillAlpha < 0.999 {
			alpha := int(math.Round(sh.FillAlpha * 100000))
			sb.WriteString(fmt.Sprintf(`><a:alpha val="%d"/></a:srgbClr></a:solidFill>`, alpha))
		} else {
			sb.WriteString(`/></a:solidFill>`)
		}
	} else {
		sb.WriteString(`<a:noFill/>`)
	}
	if sh.HasStroke && sh.StrokeWidth > 0 {
		sb.WriteString(fmt.Sprintf(`<a:ln w="%d" cap="flat" cmpd="sng"><a:solidFill><a:srgbClr val="%02X%02X%02X"/></a:solidFill><a:prstDash val="solid"/></a:ln>`,
			sh.StrokeWidth, sh.StrokeR, sh.StrokeG, sh.StrokeB))
	} else {
		sb.WriteString(`<a:ln><a:noFill/></a:ln>`)
	}
	sb.WriteString(`</p:spPr></p:sp>`)
	return sb.String()
}

func formatCustGeom(sh *Shape) string {
	fill := "norm"
	if sh.EvenOdd {
		fill = "evenOdd"
	}
	w, h := sh.Width, sh.Height
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	clamp := func(v, max int64) int64 {
		if v < 0 {
			return 0
		}
		if v > max {
			return max
		}
		return v
	}
	var sb strings.Builder
	sb.WriteString(`<a:custGeom><a:avLst/><a:gdLst/><a:ahLst/><a:cxnLst/><a:rect l="l" t="t" r="r" b="b"/><a:pathLst>`)
	sb.WriteString(fmt.Sprintf(`<a:path w="%d" h="%d" fill="%s">`, w, h, fill))
	for _, op := range sh.Path {
		switch op.Op {
		case "M":
			sb.WriteString(fmt.Sprintf(`<a:moveTo><a:pt x="%d" y="%d"/></a:moveTo>`, clamp(op.X, w), clamp(op.Y, h)))
		case "L":
			sb.WriteString(fmt.Sprintf(`<a:lnTo><a:pt x="%d" y="%d"/></a:lnTo>`, clamp(op.X, w), clamp(op.Y, h)))
		case "C":
			sb.WriteString(`<a:cubicBezTo>`)
			sb.WriteString(fmt.Sprintf(`<a:pt x="%d" y="%d"/>`, clamp(op.X1, w), clamp(op.Y1, h)))
			sb.WriteString(fmt.Sprintf(`<a:pt x="%d" y="%d"/>`, clamp(op.X2, w), clamp(op.Y2, h)))
			sb.WriteString(fmt.Sprintf(`<a:pt x="%d" y="%d"/>`, clamp(op.X, w), clamp(op.Y, h)))
			sb.WriteString(`</a:cubicBezTo>`)
		case "Z":
			sb.WriteString(`<a:close/>`)
		}
	}
	sb.WriteString(`</a:path></a:pathLst></a:custGeom>`)
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
