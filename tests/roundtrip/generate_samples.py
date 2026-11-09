#!/usr/bin/env python3
"""Generate PPTX fixtures that exercise pdf2ppt round-trip fidelity."""

import sys
from pathlib import Path

from PIL import Image, ImageDraw
from pptx import Presentation
from pptx.dml.color import RGBColor
from pptx.enum.shapes import MSO_SHAPE
from pptx.enum.text import PP_ALIGN
from pptx.oxml.ns import qn
from pptx.util import Emu, Inches, Pt

OUT = Path(__file__).resolve().parent / "samples"


def _set_run_font(run, name, size, color, bold=False, italic=False, ea=None):
    run.font.size = Pt(size)
    run.font.bold = bold
    run.font.italic = italic
    run.font.color.rgb = RGBColor(*color)
    run.font.name = name
    rPr = run._r.get_or_add_rPr()
    for tag in ("a:latin", "a:ea", "a:cs"):
        el = rPr.find(qn(tag))
        if el is not None:
            rPr.remove(el)
    rPr.append(rPr.makeelement(qn("a:latin"), {"typeface": name}))
    rPr.append(rPr.makeelement(qn("a:ea"), {"typeface": ea or name}))
    rPr.append(rPr.makeelement(qn("a:cs"), {"typeface": name}))


def _blank(w_in, h_in):
    prs = Presentation()
    prs.slide_width = Inches(w_in)
    prs.slide_height = Inches(h_in)
    return prs, prs.slide_layouts[6]


def _bg(slide, rgb):
    fill = slide.background.fill
    fill.solid()
    fill.fore_color.rgb = RGBColor(*rgb)


def _tb(slide, l, t, w, h, text, font, size, color, bold=False, italic=False, ea=None, align=None):
    box = slide.shapes.add_textbox(Inches(l), Inches(t), Inches(w), Inches(h))
    tf = box.text_frame
    tf.word_wrap = True
    p = tf.paragraphs[0]
    if align is not None:
        p.alignment = align
    run = p.add_run()
    run.text = text
    _set_run_font(run, font, size, color, bold, italic, ea)
    return box


def make_images(img_dir: Path):
    img_dir.mkdir(parents=True, exist_ok=True)
    photo = Image.new("RGB", (640, 360), (20, 90, 160))
    d = ImageDraw.Draw(photo)
    d.rectangle([30, 30, 610, 330], outline=(255, 220, 80), width=12)
    d.ellipse([220, 70, 420, 290], fill=(230, 70, 50))
    d.polygon([(80, 280), (180, 120), (280, 280)], fill=(40, 180, 120))
    photo.save(img_dir / "photo.png")

    jpg = Image.new("RGB", (480, 320), (240, 240, 240))
    d = ImageDraw.Draw(jpg)
    for i, c in enumerate([(200, 40, 40), (40, 160, 80), (40, 80, 200), (220, 160, 30)]):
        d.rectangle([20 + i * 110, 40, 110 + i * 110, 280], fill=c)
    jpg.save(img_dir / "blocks.jpg", quality=90)

    bg = Image.new("RGB", (1280, 720), (30, 40, 70))
    d = ImageDraw.Draw(bg)
    for i in range(8):
        d.ellipse([i * 160, 80, i * 160 + 220, 640], outline=(80, 140, 200), width=6)
    bg.save(img_dir / "bg.png")
    return img_dir


def sample_text(path, img_dir):
    prs, blank = _blank(13.333, 7.5)
    s = prs.slides.add_slide(blank)
    _bg(s, (255, 255, 255))
    _tb(s, 0.5, 0.35, 12, 0.8, "Title in Liberation Sans", "Liberation Sans", 36, (20, 40, 80), bold=True)
    _tb(s, 0.5, 1.4, 12, 0.6, "Serif body at eighteen points", "Liberation Serif", 18, (180, 40, 40))
    _tb(s, 0.5, 2.2, 12, 0.6, "Italic sans note", "Liberation Sans", 20, (20, 100, 60), italic=True)
    _tb(s, 0.5, 3.0, 12, 0.6, "Bold serif heading", "Liberation Serif", 24, (40, 40, 140), bold=True)
    _tb(s, 0.5, 3.9, 12, 0.7, "Small caption 12pt in gray", "Liberation Sans", 12, (90, 90, 90))
    _tb(s, 0.5, 4.7, 12, 0.8, "Large 44 pt navy statement", "Liberation Sans", 44, (10, 30, 90), bold=True)
    prs.save(path)


def sample_cjk(path, img_dir):
    prs, blank = _blank(13.333, 7.5)
    s = prs.slides.add_slide(blank)
    _bg(s, (252, 248, 240))
    _tb(s, 0.6, 0.4, 12, 1.0, "中文标题：季度业务回顾", "WenQuanYi Micro Hei", 36, (120, 30, 20), bold=True, ea="WenQuanYi Micro Hei")
    _tb(s, 0.6, 1.7, 12, 1.0, "本季度收入增长百分之十八，客户满意度持续提升。", "WenQuanYi Micro Hei", 20, (30, 30, 30), ea="WenQuanYi Micro Hei")
    _tb(s, 0.6, 2.9, 12, 0.8, "Mixed 中英文 English and 汉字 together", "Liberation Sans", 18, (20, 60, 120), ea="WenQuanYi Micro Hei")
    _tb(s, 0.6, 4.0, 12, 1.2, "第二行：北京、上海、深圳与广州。", "WenQuanYi Micro Hei", 22, (0, 90, 70), ea="WenQuanYi Micro Hei")
    _tb(s, 0.6, 5.4, 12, 0.8, "标点测试：你好，世界！（2026）", "WenQuanYi Micro Hei", 18, (60, 60, 60), ea="WenQuanYi Micro Hei")
    prs.save(path)


def sample_bullets(path, img_dir):
    prs, blank = _blank(13.333, 7.5)
    s = prs.slides.add_slide(blank)
    _bg(s, (255, 255, 255))
    _tb(s, 0.5, 0.3, 10, 0.7, "Agenda", "Liberation Sans", 32, (20, 40, 80), bold=True)
    box = s.shapes.add_textbox(Inches(0.7), Inches(1.4), Inches(11), Inches(5))
    tf = box.text_frame
    tf.word_wrap = True
    items = [
        (0, "Opening remarks and goals"),
        (0, "Product updates for the quarter"),
        (1, "Mobile release highlights"),
        (1, "Platform reliability work"),
        (0, "Customer stories from APAC"),
        (0, "Next steps and owners"),
    ]
    for i, (level, text) in enumerate(items):
        p = tf.paragraphs[0] if i == 0 else tf.add_paragraph()
        p.level = level
        pPr = p._p.get_or_add_pPr()
        pPr.append(pPr.makeelement(qn("a:buFont"), {"typeface": "Arial"}))
        pPr.append(pPr.makeelement(qn("a:buChar"), {"char": "•"}))
        run = p.add_run()
        run.text = text
        _set_run_font(run, "Liberation Sans", 20 if level == 0 else 16, (30, 30, 30))
    prs.save(path)


def sample_images(path, img_dir):
    prs, blank = _blank(13.333, 7.5)
    s = prs.slides.add_slide(blank)
    _bg(s, (245, 245, 245))
    _tb(s, 0.4, 0.25, 8, 0.6, "Images", "Liberation Sans", 28, (20, 20, 20), bold=True)
    s.shapes.add_picture(str(img_dir / "photo.png"), Inches(0.4), Inches(1.2), Inches(6.4), Inches(3.6))
    s.shapes.add_picture(str(img_dir / "blocks.jpg"), Inches(7.2), Inches(1.2), Inches(5.4), Inches(3.6))
    _tb(s, 0.4, 5.1, 6, 0.5, "PNG illustration", "Liberation Sans", 14, (60, 60, 60))
    _tb(s, 7.2, 5.1, 5, 0.5, "JPEG color blocks", "Liberation Sans", 14, (60, 60, 60))
    prs.save(path)


def sample_shapes(path, img_dir):
    prs, blank = _blank(13.333, 7.5)
    s = prs.slides.add_slide(blank)
    _bg(s, (255, 255, 255))
    _tb(s, 0.4, 0.25, 8, 0.6, "Shapes", "Liberation Sans", 28, (20, 20, 20), bold=True)
    rect = s.shapes.add_shape(MSO_SHAPE.RECTANGLE, Inches(0.5), Inches(1.3), Inches(3.2), Inches(1.8))
    rect.fill.solid()
    rect.fill.fore_color.rgb = RGBColor(0x2E, 0x86, 0xAB)
    rect.line.color.rgb = RGBColor(0x1B, 0x4F, 0x72)
    rect.line.width = Pt(1.5)
    tf = rect.text_frame
    tf.word_wrap = False
    p = tf.paragraphs[0]
    p.alignment = PP_ALIGN.CENTER
    run = p.add_run()
    run.text = "Rectangle"
    _set_run_font(run, "Liberation Sans", 18, (255, 255, 255), bold=True)

    oval = s.shapes.add_shape(MSO_SHAPE.OVAL, Inches(4.2), Inches(1.3), Inches(2.6), Inches(1.8))
    oval.fill.solid()
    oval.fill.fore_color.rgb = RGBColor(0xE7, 0x4C, 0x3C)
    oval.line.fill.background()
    tf = oval.text_frame
    p = tf.paragraphs[0]
    p.alignment = PP_ALIGN.CENTER
    run = p.add_run()
    run.text = "Oval"
    _set_run_font(run, "Liberation Sans", 18, (255, 255, 255), bold=True)

    rnd = s.shapes.add_shape(MSO_SHAPE.ROUNDED_RECTANGLE, Inches(7.3), Inches(1.3), Inches(5.2), Inches(1.8))
    rnd.fill.solid()
    rnd.fill.fore_color.rgb = RGBColor(0x27, 0xAE, 0x60)
    rnd.line.color.rgb = RGBColor(0x1E, 0x84, 0x49)
    tf = rnd.text_frame
    p = tf.paragraphs[0]
    p.alignment = PP_ALIGN.CENTER
    run = p.add_run()
    run.text = "Rounded"
    _set_run_font(run, "Liberation Sans", 18, (255, 255, 255), bold=True)

    # accent bar
    bar = s.shapes.add_shape(MSO_SHAPE.RECTANGLE, Inches(0.5), Inches(3.6), Inches(12.3), Inches(0.25))
    bar.fill.solid()
    bar.fill.fore_color.rgb = RGBColor(0xF1, 0xC4, 0x0F)
    bar.line.fill.background()

    line = s.shapes.add_shape(MSO_SHAPE.RECTANGLE, Inches(0.5), Inches(4.3), Inches(4), Inches(0.06))
    line.fill.solid()
    line.fill.fore_color.rgb = RGBColor(0x33, 0x33, 0x33)
    line.line.fill.background()

    _tb(s, 0.5, 4.6, 8, 0.6, "Gold bar and a thin rule", "Liberation Sans", 16, (40, 40, 40))

    # right triangle-like chevron using a right arrow preset if available, else diamond
    dia = s.shapes.add_shape(MSO_SHAPE.DIAMOND, Inches(8.5), Inches(4.2), Inches(2.2), Inches(2.2))
    dia.fill.solid()
    dia.fill.fore_color.rgb = RGBColor(0x8E, 0x44, 0xAD)
    dia.line.color.rgb = RGBColor(0x5B, 0x2C, 0x6F)
    prs.save(path)


def sample_table(path, img_dir):
    prs, blank = _blank(13.333, 7.5)
    s = prs.slides.add_slide(blank)
    _bg(s, (255, 255, 255))
    _tb(s, 0.5, 0.3, 10, 0.7, "Regional summary", "Liberation Sans", 28, (20, 40, 80), bold=True)
    rows, cols = 4, 4
    ts = s.shapes.add_table(rows, cols, Inches(0.5), Inches(1.4), Inches(12.2), Inches(3.6))
    table = ts.table
    data = [
        ["Region", "数量", "Revenue", "Status"],
        ["North", "120", "$1.2M", "Ahead"],
        ["South", "86", "$860K", "On track"],
        ["华东", "64", "$640K", "Watch"],
    ]
    for r in range(rows):
        for c in range(cols):
            cell = table.cell(r, c)
            cell.text = data[r][c]
            for p in cell.text_frame.paragraphs:
                for run in p.runs:
                    _set_run_font(
                        run,
                        "Liberation Sans" if not any("\u4e00" <= ch <= "\u9fff" for ch in run.text) else "WenQuanYi Micro Hei",
                        16 if r == 0 else 14,
                        (255, 255, 255) if r == 0 else (20, 20, 20),
                        bold=(r == 0),
                        ea="WenQuanYi Micro Hei",
                    )
            tc = cell._tc
            tcPr = tc.get_or_add_tcPr()
            solid = tcPr.makeelement(qn("a:solidFill"), {})
            color = "1F4E79" if r == 0 else ("D6EAF8" if r % 2 == 0 else "FFFFFF")
            srgb = solid.makeelement(qn("a:srgbClr"), {"val": color})
            solid.append(srgb)
            tcPr.append(solid)
    prs.save(path)


def sample_backgrounds(path, img_dir):
    prs, blank = _blank(13.333, 7.5)
    s1 = prs.slides.add_slide(blank)
    _bg(s1, (0x1A, 0x36, 0x5D))
    _tb(s1, 0.8, 2.4, 11, 1.2, "Solid navy background", "Liberation Sans", 40, (255, 255, 255), bold=True)
    _tb(s1, 0.8, 3.8, 11, 0.8, "Subtitle stays readable on the fill", "Liberation Sans", 20, (200, 220, 240))

    s2 = prs.slides.add_slide(blank)
    # picture background via a full-slide picture behind text
    s2.shapes.add_picture(str(img_dir / "bg.png"), Inches(0), Inches(0), Inches(13.333), Inches(7.5))
    _tb(s2, 0.8, 2.6, 11, 1.0, "Image background", "Liberation Sans", 40, (255, 255, 255), bold=True)
    _tb(s2, 0.8, 3.8, 11, 0.7, "Text over a full-slide picture", "Liberation Sans", 20, (230, 240, 255))
    prs.save(path)


def sample_aspect(path, img_dir):
    prs, blank = _blank(10, 7.5)
    s = prs.slides.add_slide(blank)
    _bg(s, (255, 250, 240))
    _tb(s, 0.5, 0.4, 9, 0.8, "4:3 slide", "Liberation Sans", 36, (80, 40, 10), bold=True)
    _tb(s, 0.5, 1.6, 9, 0.7, "Standard ten by seven point five inches.", "Liberation Serif", 20, (40, 40, 40))
    box = s.shapes.add_shape(MSO_SHAPE.RECTANGLE, Inches(0.5), Inches(3.0), Inches(4), Inches(2))
    box.fill.solid()
    box.fill.fore_color.rgb = RGBColor(0xD3, 0x54, 0x00)
    box.line.fill.background()
    prs.save(path)


def sample_mixed(path, img_dir):
    prs, blank = _blank(13.333, 7.5)
    s = prs.slides.add_slide(blank)
    _bg(s, (0xF4, 0xF7, 0xFB))
    _tb(s, 0.5, 0.3, 8, 0.8, "Quarterly Report 季度报告", "Liberation Sans", 32, (0x1A, 0x36, 0x5D), bold=True, ea="WenQuanYi Micro Hei")
    _tb(s, 0.5, 1.3, 7.2, 0.7, "Revenue grew 18% in Q3. 收入增长百分之十八。", "Liberation Serif", 18, (0xC0, 0x39, 0x2B), ea="WenQuanYi Micro Hei")

    box = s.shapes.add_textbox(Inches(0.5), Inches(2.2), Inches(7), Inches(2.4))
    tf = box.text_frame
    tf.word_wrap = True
    items = ["First bullet item", "第二项：中文要点", "Third item with detail"]
    for i, t in enumerate(items):
        p = tf.paragraphs[0] if i == 0 else tf.add_paragraph()
        p.level = 0
        pPr = p._p.get_or_add_pPr()
        pPr.append(pPr.makeelement(qn("a:buFont"), {"typeface": "Arial"}))
        pPr.append(pPr.makeelement(qn("a:buChar"), {"char": "•"}))
        run = p.add_run()
        run.text = t
        ea = "WenQuanYi Micro Hei"
        latin = "WenQuanYi Micro Hei" if any("\u4e00" <= ch <= "\u9fff" for ch in t) else "Liberation Sans"
        _set_run_font(run, latin, 16, (0x22, 0x22, 0x22), ea=ea)

    shape = s.shapes.add_shape(MSO_SHAPE.ROUNDED_RECTANGLE, Inches(8.2), Inches(1.3), Inches(4.5), Inches(1.7))
    shape.fill.solid()
    shape.fill.fore_color.rgb = RGBColor(0x2E, 0x86, 0xAB)
    shape.line.color.rgb = RGBColor(0x1B, 0x4F, 0x72)
    tf = shape.text_frame
    p = tf.paragraphs[0]
    p.alignment = PP_ALIGN.CENTER
    run = p.add_run()
    run.text = "Shape label"
    _set_run_font(run, "Liberation Sans", 20, (255, 255, 255), bold=True)

    s.shapes.add_picture(str(img_dir / "photo.png"), Inches(8.2), Inches(3.3), Inches(4.5), Inches(2.2))

    rows, cols = 3, 3
    ts = s.shapes.add_table(rows, cols, Inches(0.5), Inches(5.0), Inches(7.2), Inches(2.0))
    table = ts.table
    data = [["Item", "数量", "Amount"], ["Widgets", "12", "$1,200"], ["Gadgets", "7", "$860"]]
    for r in range(rows):
        for c in range(cols):
            cell = table.cell(r, c)
            cell.text = data[r][c]
            for p in cell.text_frame.paragraphs:
                for run in p.runs:
                    latin = "WenQuanYi Micro Hei" if any("\u4e00" <= ch <= "\u9fff" for ch in run.text) else "Liberation Sans"
                    _set_run_font(run, latin, 14, (20, 20, 20), bold=(r == 0), ea="WenQuanYi Micro Hei")
    prs.save(path)


SAMPLES = [
    ("text_styles", sample_text),
    ("cjk", sample_cjk),
    ("bullets", sample_bullets),
    ("images", sample_images),
    ("shapes", sample_shapes),
    ("table", sample_table),
    ("backgrounds", sample_backgrounds),
    ("aspect_4x3", sample_aspect),
    ("mixed", sample_mixed),
]


def main():
    out = OUT
    if len(sys.argv) > 1:
        out = Path(sys.argv[1])
    out.mkdir(parents=True, exist_ok=True)
    img_dir = make_images(out / "_img")
    for name, fn in SAMPLES:
        dest = out / f"{name}.pptx"
        fn(dest, img_dir)
        print(dest)


if __name__ == "__main__":
    main()
