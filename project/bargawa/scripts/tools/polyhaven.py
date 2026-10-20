"""Downloads the Poly Haven textures listed in common/polyhaven.py (CC0) into blender/assets/polyhaven/. Files already there are kept.
Run: scripts\\run.ps1 fetch      (headless Blender, for its Python)"""
import os, sys; sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))   # scripts/
import common; common.reload()
import json, urllib.request
from common import polyhaven as PH

UA = {"User-Agent": "bargawa-blender-project"}

def get(url):
    with urllib.request.urlopen(urllib.request.Request(url, headers=UA), timeout=60) as r: return r.read()

def main():
    for asset in PH.ids():
        files = json.loads(get(f"https://api.polyhaven.com/files/{asset}"))
        d = os.path.join(PH.DIR, asset); os.makedirs(d, exist_ok=True)
        for kind in PH.MAPS:
            if PH.file(asset, kind): print("FETCH have", asset, kind, flush=True); continue
            entry = files.get(kind, {}).get(PH.RES, {})
            fmt = "jpg" if "jpg" in entry else ("png" if "png" in entry else None)
            if not fmt: print("FETCH missing", asset, kind, "maps:", ",".join(files.keys()), flush=True); continue
            out = os.path.join(d, f"{asset}_{kind}_{PH.RES}.{fmt}")
            open(out, "wb").write(get(entry[fmt]["url"]))
            print("FETCH got", asset, kind, os.path.getsize(out)//1024, "KB", flush=True)
    with open(os.path.join(PH.DIR, "SOURCE.txt"), "w", encoding="utf-8") as f:
        f.write("Textures from Poly Haven (polyhaven.com), CC0. Listed in scripts/common/polyhaven.py; fetched by run.ps1 fetch.\n")
    print("DONE", flush=True)

main()
