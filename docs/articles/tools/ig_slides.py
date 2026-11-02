"""Render Instagram carousel slides (1080x1350) from HTML with headless Edge.

Usage: python ig_slides.py <spec.json>
spec: {"out_dir": "...", "series": "Radio Live 24 Jam", "handle": "@archetipe__",
       "slides": [{"kind": "cover"|"diagram"|"text", "title": "...", "sub": "...",
                   "image": "abs path (diagram)", "lines": ["..."] (text)}]}
Each slide is written as slide-NN.png in out_dir. Uses its own Edge profile
in the scratch dir and waits for every Edge it starts; never touches others.
"""
import html, json, os, pathlib, subprocess, sys

EDGE = r"C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe"
SCRATCH = pathlib.Path(os.environ.get("LOCALAPPDATA", r"C:\Users\heris\AppData\Local")) / "hermes/cache/scratch/ig_slides"

CSS = """
*{box-sizing:border-box;margin:0;padding:0}
html,body{width:1080px;height:1350px;background:#ffffff;overflow:hidden}
body{font-family:-apple-system,'Segoe UI',Helvetica,Arial,sans-serif;color:#1f1f1f;
     display:flex;flex-direction:column;padding:72px 64px 56px}
.label{font-size:26px;font-weight:600;color:#2f76d6;letter-spacing:.5px;text-transform:uppercase}
.title{font-size:58px;font-weight:700;line-height:1.15;margin-top:22px}
.cover .title{font-size:84px;line-height:1.1;margin-top:40px}
.sub{font-size:32px;line-height:1.4;color:#5f5f5f;margin-top:22px}
.cover .sub{font-size:38px}
.fig{flex:1;display:flex;align-items:center;justify-content:center;margin:24px -40px 0}
.fig img{max-width:100%;max-height:100%;}
.lines{flex:1;display:flex;flex-direction:column;justify-content:center;gap:34px;margin-top:20px}
.lines div{font-size:40px;line-height:1.35;padding-left:28px;border-left:6px solid #2f76d6}
.foot{display:flex;justify-content:space-between;font-size:24px;color:#5f5f5f;margin-top:24px}
.bar{height:10px;background:#2f76d6;opacity:.14;margin:0 -64px 40px}
.cover .spacer{flex:1}
.cover .fig{margin:40px 0 0}
.cover .fig img{border-radius:18px;box-shadow:0 14px 44px rgba(0,0,0,.20)}
"""


def page(slide, i, n, series, handle):
    e = html.escape
    kind = slide["kind"]
    body = [f'<div class="label">{e(series)}</div>']
    body.append(f'<div class="title">{e(slide["title"])}</div>')
    if slide.get("sub"):
        body.append(f'<div class="sub">{e(slide["sub"])}</div>')
    if kind == "diagram" or (kind == "cover" and slide.get("image")):
        uri = pathlib.Path(slide["image"]).resolve().as_uri()
        body.append(f'<div class="fig"><img src="{uri}"></div>')
    elif kind == "text":
        body.append('<div class="lines">' + "".join(f"<div>{e(l)}</div>" for l in slide["lines"]) + "</div>")
    else:
        body.append('<div class="spacer"></div>')
    body.append(f'<div class="foot"><span>{e(handle)}</span><span>{i}/{n}</span></div>')
    cls = "cover" if kind == "cover" else ""
    return f'<!doctype html><html><head><meta charset="utf-8"><style>{CSS}</style></head><body class="{cls}">{"".join(body)}</body></html>'


def main():
    spec = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
    out = pathlib.Path(spec["out_dir"]); out.mkdir(parents=True, exist_ok=True)
    SCRATCH.mkdir(parents=True, exist_ok=True)
    n = len(spec["slides"])
    for i, s in enumerate(spec["slides"], 1):
        src = SCRATCH / f"slide-{i:02d}.html"
        src.write_text(page(s, i, n, spec["series"], spec["handle"]), encoding="utf-8")
        png = out / f"slide-{i:02d}.png"
        if png.exists():
            png.unlink()
        subprocess.run([EDGE, "--headless=new", "--disable-gpu", "--hide-scrollbars",
                        f"--user-data-dir={SCRATCH / 'edge-profile'}", "--window-size=1080,1350",
                        "--force-device-scale-factor=1", "--virtual-time-budget=3000",
                        f"--screenshot={png}", src.as_uri()], check=True, timeout=120,
                       stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        print(png, png.stat().st_size)


if __name__ == "__main__":
    main()
