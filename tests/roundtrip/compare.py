#!/usr/bin/env python3
"""Compare an original PPTX with a pdf2ppt round trip.

Usage:
  compare.py <original.pptx> <roundtrip.pptx> <orig.png> <round.png> [more pairs...]

Each PNG pair is one slide (pdftoppm output). Pass every slide, in order,
after the two PPTX paths. Slide count is also checked from the PPTX files.

Prints one JSON object and exits 0 when every check passes.
"""

import json
import re
import sys

import numpy as np
from PIL import Image
from pptx import Presentation
from skimage.metrics import structural_similarity as ssim

MIN_SSIM = 0.97
MAX_DIM_PT = 1.5  # slide width/height tolerance


def norm_text(s: str) -> str:
    s = s.replace("•", "").replace("\u2022", "").replace("·", "")
    return re.sub(r"\s+", "", s)


def deck_text_and_size(path):
    prs = Presentation(path)
    parts = []
    for slide in prs.slides:
        for shape in slide.shapes:
            if shape.has_text_frame:
                for p in shape.text_frame.paragraphs:
                    if p.text.strip():
                        parts.append(p.text)
            if getattr(shape, "has_table", False) and shape.has_table:
                for row in shape.table.rows:
                    for cell in row.cells:
                        if cell.text.strip():
                            parts.append(cell.text)
    return {
        "slides": len(prs.slides),
        "width_pt": prs.slide_width / 12700.0,
        "height_pt": prs.slide_height / 12700.0,
        "text": norm_text("".join(parts)),
    }


def load_rgb(path):
    return np.array(Image.open(path).convert("RGB"))


def main():
    if len(sys.argv) < 5:
        print("usage: compare.py original.pptx roundtrip.pptx orig1.png rt1.png ...", file=sys.stderr)
        sys.exit(2)
    orig_pptx, rt_pptx = sys.argv[1], sys.argv[2]
    pngs = sys.argv[3:]
    if len(pngs) % 2 != 0:
        print("png arguments must be pairs", file=sys.stderr)
        sys.exit(2)
    orig = deck_text_and_size(orig_pptx)
    rt = deck_text_and_size(rt_pptx)
    failures = []
    if orig["slides"] != rt["slides"]:
        failures.append(f"slide count {orig['slides']} vs {rt['slides']}")
    if abs(orig["width_pt"] - rt["width_pt"]) > MAX_DIM_PT or abs(orig["height_pt"] - rt["height_pt"]) > MAX_DIM_PT:
        failures.append(
            f"slide size {orig['width_pt']:.2f}x{orig['height_pt']:.2f} vs {rt['width_pt']:.2f}x{rt['height_pt']:.2f}"
        )
    if orig["text"] != rt["text"]:
        failures.append("extracted text differs")
    scores = []
    for i in range(0, len(pngs), 2):
        a = load_rgb(pngs[i])
        b = load_rgb(pngs[i + 1])
        if a.shape != b.shape:
            b = np.array(Image.fromarray(b).resize((a.shape[1], a.shape[0]), Image.Resampling.BILINEAR))
        score = float(ssim(a, b, channel_axis=2))
        scores.append(score)
        if score < MIN_SSIM:
            failures.append(f"slide {i//2+1} SSIM {score:.4f} < {MIN_SSIM}")
    report = {
        "slides": rt["slides"],
        "width_pt": round(rt["width_pt"], 3),
        "height_pt": round(rt["height_pt"], 3),
        "text_match": orig["text"] == rt["text"],
        "ssim": [round(s, 4) for s in scores],
        "ssim_mean": round(sum(scores) / len(scores), 4) if scores else 0,
        "ok": not failures,
        "failures": failures,
    }
    print(json.dumps(report, ensure_ascii=False))
    sys.exit(0 if report["ok"] else 1)


if __name__ == "__main__":
    main()
