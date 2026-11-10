package goppt

import (
	"archive/zip"
	"fmt"
	"io"
	"math"
	"strings"
	"time"
)

// zipTime is a legal DOS timestamp. The zero time encodes as 1980-00-00,
// which PowerPoint rejects and repairs.
var zipTime = time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC)

func createPart(w *zip.Writer, name string) (io.Writer, error) {
	h := &zip.FileHeader{Name: name, Method: zip.Deflate}
	h.Modified = zipTime
	return w.CreateHeader(h)
}

func writeContentTypes(w *zip.Writer, p *Presentation) error {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	sb.WriteString(`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">`)
	sb.WriteString(`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>`)
	sb.WriteString(`<Default Extension="xml" ContentType="application/xml"/>`)
	sb.WriteString(`<Default Extension="png" ContentType="image/png"/>`)
	sb.WriteString(`<Default Extension="jpeg" ContentType="image/jpeg"/>`)
	sb.WriteString(`<Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>`)
	sb.WriteString(`<Override PartName="/ppt/slideMasters/slideMaster1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideMaster+xml"/>`)
	sb.WriteString(`<Override PartName="/ppt/slideLayouts/slideLayout1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideLayout+xml"/>`)
	sb.WriteString(`<Override PartName="/ppt/theme/theme1.xml" ContentType="application/vnd.openxmlformats-officedocument.theme+xml"/>`)
	sb.WriteString(`<Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/>`)
	sb.WriteString(`<Override PartName="/docProps/app.xml" ContentType="application/vnd.openxmlformats-officedocument.extended-properties+xml"/>`)
	for i := range p.slides {
		sb.WriteString(fmt.Sprintf(`<Override PartName="/ppt/slides/slide%d.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>`, i+1))
	}
	sb.WriteString(`</Types>`)

	f, err := createPart(w, "[Content_Types].xml")
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
<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/>
<Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/extended-properties" Target="docProps/app.xml"/>
</Relationships>`

	f, err := createPart(w, "_rels/.rels")
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
	sb.WriteString(`<p:sldMasterIdLst><p:sldMasterId id="2147483648" r:id="rId1"/></p:sldMasterIdLst>`)
	sb.WriteString(`<p:sldIdLst>`)
	for i := range p.slides {
		sb.WriteString(fmt.Sprintf(`<p:sldId id="%d" r:id="rId%d"/>`, 256+i, i+2))
	}
	sb.WriteString(`</p:sldIdLst>`)
	cx, cy := p.slideSize()
	cx, cy = clampSlideSize(cx, cy)
	sb.WriteString(fmt.Sprintf(`<p:sldSz cx="%d" cy="%d" type="custom"/>`, cx, cy))
	// notesSz is required by the presentation schema. PowerPoint repairs files that omit it.
	sb.WriteString(`<p:notesSz cx="6858000" cy="9144000"/>`)
	sb.WriteString(`</p:presentation>`)

	f, err := createPart(w, "ppt/presentation.xml")
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
	sb.WriteString(`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="slideMasters/slideMaster1.xml"/>`)
	for i := range p.slides {
		sb.WriteString(fmt.Sprintf(`<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide%d.xml"/>`, i+2, i+1))
	}
	sb.WriteString(`</Relationships>`)

	f, err := createPart(w, "ppt/_rels/presentation.xml.rels")
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
	cx, cy := clampExtent(slide.Width), clampExtent(slide.Height)
	sb.WriteString(`<p:nvGrpSpPr><p:cNvPr id="1" name="Slide"/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>`)
	sb.WriteString(fmt.Sprintf(`<p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="%d" cy="%d"/><a:chOff x="0" y="0"/><a:chExt cx="%d" cy="%d"/></a:xfrm></p:grpSpPr>`, cx, cy, cx, cy))

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

	f, err := createPart(w, fmt.Sprintf("ppt/slides/slide%d.xml", slideNum))
	if err != nil {
		return err
	}
	_, err = f.Write([]byte(sb.String()))
	return err
}

func formatTextBox(tb *TextBox, id int) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`<p:sp><p:nvSpPr><p:cNvPr id="%d" name="TextBox %d"/><p:cNvSpPr txBox="1"/><p:nvPr/></p:nvSpPr>`, id, id))
	rot := ""
	if math.Abs(tb.Rotation) > 0.4 {
		// DrawingML rot is clockwise and LibreOffice ignores a negative value,
		// which left vertical labels horizontal on top of the numbers.
		deg := tb.Rotation
		for deg < 0 {
			deg += 360
		}
		for deg >= 360 {
			deg -= 360
		}
		rot = fmt.Sprintf(` rot="%d"`, int64(math.Round(deg*60000)))
	}
	// wrap=none and no autofit: a single line is not reflowed. The box stays
	// the PDF advance. Padding it made neighboring columns collide.
	sb.WriteString(fmt.Sprintf(`<p:spPr><a:xfrm%s><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm>`, rot, tb.X, tb.Y, clampExtent(tb.Width), clampExtent(tb.Height)))
	sb.WriteString(`<a:prstGeom prst="rect"><a:avLst/></a:prstGeom><a:noFill/><a:ln><a:noFill/></a:ln></p:spPr>`)
	sb.WriteString(`<p:txBody><a:bodyPr wrap="none" lIns="0" tIns="0" rIns="0" bIns="0" rtlCol="0" anchor="t" horzOverflow="overflow" vertOverflow="overflow"/>`)
	sb.WriteString(`<a:lstStyle/>`)
	// 100% line spacing keeps the first baseline on the font's em box. The
	// default (omitted) spacing adds external leading and drops the glyphs.
	sb.WriteString(`<a:p><a:pPr marL="0" indent="0"><a:lnSpc><a:spcPct val="100000"/></a:lnSpc><a:spcBef><a:spcPts val="0"/></a:spcBef><a:spcAft><a:spcPts val="0"/></a:spcAft><a:buNone/></a:pPr>`)
	for _, run := range textRuns(tb) {
		sb.WriteString(formatTextRun(tb, run))
	}
	sb.WriteString(`</a:p></p:txBody></p:sp>`)
	return sb.String()
}

func textRuns(tb *TextBox) []TextRun {
	if len(tb.Runs) > 0 {
		return tb.Runs
	}
	return []TextRun{{
		Content: tb.Content, FontName: tb.FontName, FontSize: tb.FontSize,
		Bold: tb.Bold, Italic: tb.Italic,
		ColorR: tb.ColorR, ColorG: tb.ColorG, ColorB: tb.ColorB,
		Tracking: tb.Tracking,
	}}
}

func formatTextRun(tb *TextBox, run TextRun) string {
	fontSize := int(math.Round(run.FontSize * 100))
	if fontSize < 100 {
		fontSize = 100
	}
	if fontSize > 400000 {
		fontSize = 400000
	}
	lang := "en-US"
	if containsCJK(run.Content) {
		lang = "zh-CN"
	}
	b, i := "0", "0"
	if run.Bold {
		b = "1"
	}
	if run.Italic {
		i = "1"
	}
	var line string
	if tb.Stroke && tb.StrokeWidth > 0.05 {
		lw := int(math.Round(tb.StrokeWidth * 12700))
		if lw < 635 {
			lw = 635
		}
		if lw > 20116800 {
			lw = 20116800
		}
		line = fmt.Sprintf(`<a:ln w="%d" cap="rnd" cmpd="sng"><a:solidFill><a:srgbClr val="%02X%02X%02X"/></a:solidFill><a:prstDash val="solid"/><a:round/></a:ln>`, lw, tb.StrokeR, tb.StrokeG, tb.StrokeB)
	}
	fill := fmt.Sprintf(`<a:solidFill><a:srgbClr val="%02X%02X%02X"/></a:solidFill>`, run.ColorR, run.ColorG, run.ColorB)
	if tb.StrokeOnly {
		fill = `<a:noFill/>`
	}
	spc := ""
	if math.Abs(run.Tracking) > 0.01 {
		sp := int(math.Round(run.Tracking * 100))
		if sp < -400000 {
			sp = -400000
		}
		if sp > 400000 {
			sp = 400000
		}
		spc = fmt.Sprintf(` spc="%d"`, sp)
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`<a:r><a:rPr lang="%s" sz="%d" b="%s" i="%s"%s dirty="0">%s%s`, lang, fontSize, b, i, spc, line, fill))
	if run.FontName != "" {
		name := escapeXML(run.FontName)
		sb.WriteString(fmt.Sprintf(`<a:latin typeface="%s"/><a:ea typeface="%s"/><a:cs typeface="%s"/>`, name, name, name))
	}
	sb.WriteString(fmt.Sprintf(`</a:rPr><a:t>%s</a:t></a:r>`, escapeXML(run.Content)))
	return sb.String()
}

func formatGradFill(g *Gradient) string {
	if g == nil || len(g.Stops) == 0 {
		return `<a:solidFill><a:srgbClr val="FFFFFF"/></a:solidFill>`
	}
	ang := int64(math.Round(g.Angle * 60000))
	ang %= 21600000
	if ang < 0 {
		ang += 21600000
	}
	stops := g.Stops
	if len(stops) == 1 {
		stops = []GradientStop{stops[0], stops[0]}
	}
	var sb strings.Builder
	sb.WriteString(`<a:gradFill flip="none" rotWithShape="1"><a:gsLst>`)
	for _, s := range stops {
		pos := int(math.Round(s.Pos * 100000))
		if pos < 0 {
			pos = 0
		}
		if pos > 100000 {
			pos = 100000
		}
		sb.WriteString(fmt.Sprintf(`<a:gs pos="%d"><a:srgbClr val="%02X%02X%02X"/></a:gs>`, pos, s.R, s.G, s.B))
	}
	if g.Kind == "radial" {
		// pos 0 is the center, matching a PDF type-3 shading whose t=0 circle is the focus.
		sb.WriteString(`</a:gsLst><a:path path="circle"><a:fillToRect l="50000" t="50000" r="50000" b="50000"/></a:path></a:gradFill>`)
	} else {
		// scaled="1" fits the axis to the shape, which is how CSS sizes a linear-gradient on a box.
		sb.WriteString(fmt.Sprintf(`</a:gsLst><a:lin ang="%d" scaled="1"/></a:gradFill>`, ang))
	}
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
	sb.WriteString(fmt.Sprintf(`<a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm>`, sh.X, sh.Y, clampExtent(sh.Width), clampExtent(sh.Height)))
	if sh.SimpleRect || len(sh.Path) == 0 {
		sb.WriteString(`<a:prstGeom prst="rect"><a:avLst/></a:prstGeom>`)
	} else {
		sb.WriteString(formatCustGeom(sh))
	}
	if sh.Gradient != nil {
		sb.WriteString(formatGradFill(sh.Gradient))
	} else if sh.HasFill && sh.FillAlpha > 0.001 {
		sb.WriteString(`<a:solidFill><a:srgbClr val="` + fmt.Sprintf("%02X%02X%02X", sh.FillR, sh.FillG, sh.FillB) + `"`)
		if sh.FillAlpha < 0.999 {
			alpha := int(math.Round(sh.FillAlpha * 100000))
			if alpha < 0 {
				alpha = 0
			}
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
	// ST_PathFillMode is none|norm|lighten|lightenLess|darken|darkenLess.
	// "evenOdd" is not in that list; PowerPoint repairs it.
	fill := "norm"
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
	started := false
	for _, op := range sh.Path {
		if !started && op.Op != "M" {
			sb.WriteString(fmt.Sprintf(`<a:moveTo><a:pt x="%d" y="%d"/></a:moveTo>`, clamp(op.X, w), clamp(op.Y, h)))
			started = true
		}
		if op.Op == "M" {
			started = true
		}
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
	rot := ""
	if math.Abs(img.Rotation) > 0.4 {
		deg := img.Rotation
		for deg < 0 {
			deg += 360
		}
		for deg >= 360 {
			deg -= 360
		}
		rot = fmt.Sprintf(` rot="%d"`, int64(math.Round(deg*60000)))
	}
	sb.WriteString(fmt.Sprintf(`<p:spPr><a:xfrm%s><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm>`, rot, img.X, img.Y, clampExtent(img.Width), clampExtent(img.Height)))
	sb.WriteString(`<a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr></p:pic>`)
	return sb.String()
}

func writeSlideRels(w *zip.Writer, slide *Slide, slideNum int) error {
	images := slide.Images()
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	sb.WriteString(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	sb.WriteString(`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>`)

	for i, img := range images {
		ext := img.Format
		if ext == "" {
			ext = "png"
		}
		imgPath := fmt.Sprintf("../media/image_s%d_%d.%s", slideNum, i+1, ext)
		sb.WriteString(fmt.Sprintf(`<Relationship Id="rImg%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="%s"/>`, i+1, imgPath))

		// Also write the image data
		if len(img.Data) > 0 {
			imgFile, err := createPart(w, fmt.Sprintf("ppt/media/image_s%d_%d.%s", slideNum, i+1, ext))
			if err != nil {
				return fmt.Errorf("failed to create image file: %w", err)
			}
			if _, err := imgFile.Write(img.Data); err != nil {
				return fmt.Errorf("failed to write image data: %w", err)
			}
		}
	}

	sb.WriteString(`</Relationships>`)

	f, err := createPart(w, fmt.Sprintf("ppt/slides/_rels/slide%d.xml.rels", slideNum))
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
	var b strings.Builder
	for _, r := range s {
		// XML 1.0 rejects most C0 controls and surrogate code points.
		if r == 0x9 || r == 0xA || r == 0xD || (r >= 0x20 && r <= 0xD7FF) || (r >= 0xE000 && r <= 0xFFFD) || (r >= 0x10000 && r <= 0x10FFFF) {
			b.WriteRune(r)
		}
	}
	return xmlReplacer.Replace(b.String())
}

func clampSlideSize(cx, cy int64) (int64, int64) {
	const min, max int64 = 914400, 51206400
	if cx < min {
		cx = min
	}
	if cy < min {
		cy = min
	}
	if cx > max {
		cx = max
	}
	if cy > max {
		cy = max
	}
	return cx, cy
}

func clampExtent(v int64) int64 {
	if v < 1 {
		return 1
	}
	if v > 51206400 {
		return 51206400
	}
	return v
}

func writePackageParts(w *zip.Writer, p *Presentation) error {
	cx, cy := clampSlideSize(p.slideSize())
	if err := writeString(w, "ppt/theme/theme1.xml", themeXML); err != nil {
		return err
	}
	if err := writeString(w, "ppt/slideMasters/slideMaster1.xml", slideMasterXML(cx, cy)); err != nil {
		return err
	}
	if err := writeString(w, "ppt/slideMasters/_rels/slideMaster1.xml.rels", slideMasterRels); err != nil {
		return err
	}
	if err := writeString(w, "ppt/slideLayouts/slideLayout1.xml", slideLayoutXML(cx, cy)); err != nil {
		return err
	}
	if err := writeString(w, "ppt/slideLayouts/_rels/slideLayout1.xml.rels", slideLayoutRels); err != nil {
		return err
	}
	if err := writeString(w, "docProps/core.xml", corePropsXML); err != nil {
		return err
	}
	return writeString(w, "docProps/app.xml", appPropsXML(len(p.slides)))
}

func writeString(w *zip.Writer, name, content string) error {
	f, err := createPart(w, name)
	if err != nil {
		return err
	}
	_, err = io.WriteString(f, content)
	return err
}

func slideMasterXML(cx, cy int64) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`+
		`<p:sldMaster xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">`+
		`<p:cSld><p:spTree>`+
		`<p:nvGrpSpPr><p:cNvPr id="1" name="Master"/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>`+
		`<p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="%d" cy="%d"/><a:chOff x="0" y="0"/><a:chExt cx="%d" cy="%d"/></a:xfrm></p:grpSpPr>`+
		`</p:spTree></p:cSld>`+
		`<p:clrMap bg1="lt1" tx1="dk1" bg2="lt2" tx2="dk2" accent1="accent1" accent2="accent2" accent3="accent3" accent4="accent4" accent5="accent5" accent6="accent6" hlink="hlink" folHlink="folHlink"/>`+
		`<p:sldLayoutIdLst><p:sldLayoutId id="2147483649" r:id="rId1"/></p:sldLayoutIdLst>`+
		`<p:txStyles><p:titleStyle/><p:bodyStyle/><p:otherStyle/></p:txStyles>`+
		`</p:sldMaster>`, cx, cy, cx, cy)
}

func slideLayoutXML(cx, cy int64) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`+
		`<p:sldLayout xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" type="blank" preserve="1">`+
		`<p:cSld name="Blank"><p:spTree>`+
		`<p:nvGrpSpPr><p:cNvPr id="1" name="Layout"/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>`+
		`<p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="%d" cy="%d"/><a:chOff x="0" y="0"/><a:chExt cx="%d" cy="%d"/></a:xfrm></p:grpSpPr>`+
		`</p:spTree></p:cSld>`+
		`<p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr>`+
		`</p:sldLayout>`, cx, cy, cx, cy)
}

const slideMasterRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
	`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>` +
	`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="../theme/theme1.xml"/>` +
	`</Relationships>`

const slideLayoutRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
	`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="../slideMasters/slideMaster1.xml"/>` +
	`</Relationships>`

const corePropsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/">` +
	`<dc:title>Presentation</dc:title><dc:creator>pdf2ppt</dc:creator><cp:lastModifiedBy>pdf2ppt</cp:lastModifiedBy>` +
	`</cp:coreProperties>`

func appPropsXML(slides int) string {
	if slides < 0 {
		slides = 0
	}
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`+
		`<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties" xmlns:vt="http://schemas.openxmlformats.org/officeDocument/2006/docPropsVTypes">`+
		`<Application>pdf2ppt</Application><Slides>%d</Slides><Notes>0</Notes><PresentationFormat>Custom</PresentationFormat>`+
		`</Properties>`, slides)
}

// themeXML is a minimal theme. fmtScheme needs three fills, three lines, three effects, and three background fills.
const themeXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="pdf2ppt">` +
	`<a:themeElements>` +
	`<a:clrScheme name="pdf2ppt">` +
	`<a:dk1><a:srgbClr val="000000"/></a:dk1><a:lt1><a:srgbClr val="FFFFFF"/></a:lt1>` +
	`<a:dk2><a:srgbClr val="1F4E79"/></a:dk2><a:lt2><a:srgbClr val="E7E6E6"/></a:lt2>` +
	`<a:accent1><a:srgbClr val="5B9BD5"/></a:accent1><a:accent2><a:srgbClr val="ED7D31"/></a:accent2>` +
	`<a:accent3><a:srgbClr val="A5A5A5"/></a:accent3><a:accent4><a:srgbClr val="FFC000"/></a:accent4>` +
	`<a:accent5><a:srgbClr val="4472C4"/></a:accent5><a:accent6><a:srgbClr val="70AD47"/></a:accent6>` +
	`<a:hlink><a:srgbClr val="0563C1"/></a:hlink><a:folHlink><a:srgbClr val="954F72"/></a:folHlink>` +
	`</a:clrScheme>` +
	`<a:fontScheme name="pdf2ppt">` +
	`<a:majorFont><a:latin typeface="Calibri"/><a:ea typeface=""/><a:cs typeface=""/></a:majorFont>` +
	`<a:minorFont><a:latin typeface="Calibri"/><a:ea typeface=""/><a:cs typeface=""/></a:minorFont>` +
	`</a:fontScheme>` +
	`<a:fmtScheme name="pdf2ppt">` +
	`<a:fillStyleLst>` +
	`<a:solidFill><a:schemeClr val="phClr"/></a:solidFill>` +
	`<a:solidFill><a:schemeClr val="phClr"/></a:solidFill>` +
	`<a:solidFill><a:schemeClr val="phClr"/></a:solidFill>` +
	`</a:fillStyleLst>` +
	`<a:lnStyleLst>` +
	`<a:ln w="6350"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln>` +
	`<a:ln w="12700"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln>` +
	`<a:ln w="19050"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln>` +
	`</a:lnStyleLst>` +
	`<a:effectStyleLst><a:effectStyle><a:effectLst/></a:effectStyle><a:effectStyle><a:effectLst/></a:effectStyle><a:effectStyle><a:effectLst/></a:effectStyle></a:effectStyleLst>` +
	`<a:bgFillStyleLst>` +
	`<a:solidFill><a:schemeClr val="phClr"/></a:solidFill>` +
	`<a:solidFill><a:schemeClr val="phClr"/></a:solidFill>` +
	`<a:solidFill><a:schemeClr val="phClr"/></a:solidFill>` +
	`</a:bgFillStyleLst>` +
	`</a:fmtScheme>` +
	`</a:themeElements>` +
	`<a:objectDefaults/><a:extraClrSchemeLst/>` +
	`</a:theme>`
