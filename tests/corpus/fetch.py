#!/usr/bin/env python3
"""Download the external corpus into tests/corpus/cache. Nothing here is committed."""

import json
import urllib.error
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
        # tsmc-2q25-zh currently answers with a Cloudflare challenge (HTTP 403,
        # cf-mitigated: challenge) instead of the PDF. Leave the cache empty
        # and keep going so one blocked deck does not stop the rest.
        try:
            urllib.request.urlretrieve(url, dest)
        except urllib.error.HTTPError as exc:
            if item["id"] == "tsmc-2q25-zh" and exc.code == 403:
                print(" ", item["id"], "skipped: Cloudflare 403")
                continue
            raise
        print(" ", dest.stat().st_size, "bytes")


if __name__ == "__main__":
    main()
