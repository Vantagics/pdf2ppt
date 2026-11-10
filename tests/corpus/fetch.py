#!/usr/bin/env python3
"""Download the corpus manifest.

PDFs land in $PDF2PPT_CORPUS, or in tests/corpus/cache when that variable is
unset. Nothing downloaded here is committed. A known-blocked URL (the TSMC
deck, which answers with a Cloudflare challenge) is a warning. Any other
failure, after retries, exits non-zero so CI does not go green with a short
corpus.
"""

import json
import os
import sys
import time
import urllib.error
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parent
MANIFEST = json.loads((ROOT / "manifest.json").read_text())
CACHE = Path(os.environ.get("PDF2PPT_CORPUS") or (ROOT / "cache"))
UA = "Mozilla/5.0 (compatible; pdf2ppt-corpus/1.4)"
RETRIES = 4
TIMEOUT = 180


def is_pdf(path: Path) -> bool:
    if not path.is_file() or path.stat().st_size < 1000:
        return False
    with path.open("rb") as f:
        return f.read(5).startswith(b"%PDF")


def download(url: str, dest: Path) -> None:
    req = urllib.request.Request(url, headers={"User-Agent": UA})
    with urllib.request.urlopen(req, timeout=TIMEOUT) as resp:
        data = resp.read()
    if not data.startswith(b"%PDF"):
        raise RuntimeError(f"response is not a PDF ({len(data)} bytes)")
    tmp = dest.with_suffix(".pdf.part")
    tmp.write_bytes(data)
    tmp.replace(dest)


def main() -> None:
    CACHE.mkdir(parents=True, exist_ok=True)
    ok = 0
    warned = []
    failed = []
    for item in MANIFEST["files"]:
        ident = item["id"]
        url = item.get("url") or ""
        if not url:
            print(f"skip generated {ident}", flush=True)
            continue
        dest = CACHE / f"{ident}.pdf"
        if is_pdf(dest):
            print(f"have {ident}", flush=True)
            ok += 1
            continue
        err = None
        for attempt in range(1, RETRIES + 1):
            print(f"get {ident} attempt {attempt} {url}", flush=True)
            try:
                download(url, dest)
                err = None
                break
            except (urllib.error.URLError, TimeoutError, OSError, RuntimeError) as exc:
                err = exc
                print(f"  fail {exc}", flush=True)
                if dest.exists():
                    dest.unlink()
                part = dest.with_suffix(".pdf.part")
                if part.exists():
                    part.unlink()
                if attempt < RETRIES:
                    time.sleep(2 ** attempt)
        if err is not None:
            if item.get("blocked"):
                print(f"warn {ident}: known-blocked ({item.get('blocked')}): {err}", flush=True)
                warned.append(ident)
                continue
            print(f"FAIL {ident}: {err}", flush=True)
            failed.append(ident)
            continue
        print(f"  {dest.stat().st_size} bytes", flush=True)
        ok += 1
    need = [item["id"] for item in MANIFEST["files"] if item.get("url") and not item.get("blocked")]
    have = {p.stem for p in CACHE.glob("*.pdf") if is_pdf(p)}
    missing = [ident for ident in need if ident not in have]
    print(f"ok {ok} warned {warned} failed {failed}", flush=True)
    print(f"required {len(need)} have {len(have)}", flush=True)
    if failed or missing:
        if missing:
            print("missing required PDFs:", ", ".join(missing), flush=True)
        sys.exit(1)


if __name__ == "__main__":
    main()
