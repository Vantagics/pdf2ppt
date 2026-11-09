#!/usr/bin/env python3
"""Download the external corpus into tests/corpus/cache. Nothing here is committed."""

import json
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parent
CACHE = ROOT / "cache"
MANIFEST = json.loads((ROOT / "manifest.json").read_text())


def main():
    CACHE.mkdir(exist_ok=True)
    for item in MANIFEST["files"]:
        url = item.get("url") or ""
        if not url:
            print("skip generated", item["id"])
            continue
        dest = CACHE / f"{item['id']}.pdf"
        if dest.exists() and dest.stat().st_size > 1000:
            print("have", dest.name)
            continue
        print("get", item["id"], url)
        urllib.request.urlretrieve(url, dest)
        print(" ", dest.stat().st_size, "bytes")


if __name__ == "__main__":
    main()
