"""Turns a recorded take's input log into the edit's data and clips.

    python make_data.py take3

Writes ../assets/footage.mp4 (the take at constant 30 fps), ../assets/tutorial-data.js (cursor path, clicks, step times) and rewrites the generated
caption and key-label clips between the `<!-- generated:... -->` markers in ../index.html and
../compositions/vertical.html. Run it again after a new take; hand edits belong outside the markers.
"""
import html
import json
import math
import os
import re
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.dirname(HERE)
FPS = 30

# Chip text for clicks the step script did not label: (step id, nth click in the step) -> label.
# None means no chip (the ripple alone shows the click).
CLICK_LABELS = {
    (3, 0): "Bola",
    (6, 0): "Base Color",
    (6, 1): "Roda warna",
    (6, 2): None,
    (7, 0): "Metallic",
}
KIND = {"key": "Tekan", "type": "Ketik", "click": "Klik", "drag": "Geser"}
CHIP_HOLD = 1.6


def load(take):
    with open(os.path.join(HERE, "out", take + ".events.json"), encoding="utf-8") as f:
        return json.load(f)


def cursor_path(events, duration):
    """The cursor position at every video frame, kept only where it changes, plus an anchor on
    the frame before each move so a hold stays still instead of drifting toward the next spot."""
    raw = sorted((e["t"], e["x"], e["y"]) for e in events if e["type"] in ("pos", "down", "up"))
    pts, j, cur, prev = [], 0, raw[0][1:], None
    for k in range(int(duration * FPS) + 1):
        f = k / FPS
        while j < len(raw) and raw[j][0] <= f:
            cur = raw[j][1:]
            j += 1
        if cur != prev:
            if prev is not None and pts[-1][0] < round(f - 1 / FPS, 4):
                pts.append([round(f - 1 / FPS, 4), *prev])
            pts.append([round(f, 4), *cur])
            prev = cur
    return pts


def chips(events):
    res = []
    step, nth = 0, {}
    drag = None
    for e in events:
        if e["type"] == "step":
            step = e["id"]
        elif e["type"] == "click":
            n = nth.get(step, 0)
            nth[step] = n + 1
            label = e.get("label") or CLICK_LABELS.get((step, n))
            if label:
                res.append({"t": e["t"], "kind": "click", "text": label})
        elif e["type"] in ("key", "type"):
            typed = e["type"] == "type" or re.fullmatch(r"[0-9.]+", e["label"] or "")
            res.append({"t": e["t"], "kind": "type" if typed else "key", "text": e["label"]})
        elif e["type"] == "drag":
            drag = {"t": e["t"], "kind": "drag", "text": (e.get("label") or "").replace("Geser ", "")}
        elif e["type"] == "up" and drag:
            drag["end"] = e["t"] + 0.6
            res.append(drag)
            drag = None
    for i, c in enumerate(res):
        nxt = res[i + 1]["t"] if i + 1 < len(res) else None
        end = c.get("end", c["t"] + CHIP_HOLD)
        if nxt is not None:
            end = min(end, nxt - 0.05)
        c["start"] = round(c["t"] - 0.05, 2)
        c["dur"] = round(end - c["start"], 2)
    return res


def clip_html(steps, chip_list, duration):
    cap = []
    for i, s in enumerate(steps):
        start = round(s["t"], 2)
        end = round(steps[i + 1]["t"], 2) if i + 1 < len(steps) else duration
        cap.append(
            f'<div class="caption clip" id="cap-{s["id"]}" data-start="{start}" data-duration="{round(end - start, 2)}" '
            f'data-track-index="2">{html.escape(s["caption"])}</div>')
    chip = []
    for i, c in enumerate(chip_list):
        key_class = " is-key" if c["kind"] in ("key", "type") else ""
        chip.append(
            f'<div class="chip clip" id="chip-{i + 1}" data-start="{c["start"]}" data-duration="{c["dur"]}" '
            f'data-track-index="3"><span class="chip-kind">{KIND[c["kind"]]}</span>'
            f'<span class="chip-text{key_class}">{html.escape(c["text"])}</span></div>')
    return "\n      ".join(cap), "\n      ".join(chip)


def patch(path, caps, chip_html, duration):
    with open(path, encoding="utf-8") as f:
        text = f.read()
    for name, body in (("captions", caps), ("chips", chip_html)):
        pat = re.compile(rf"(<!-- generated:{name} -->)(.*?)(\s*<!-- /generated:{name} -->)", re.S)
        if not pat.search(text):
            raise SystemExit(f"{path}: missing <!-- generated:{name} --> markers")
        text = pat.sub(lambda m: m.group(1) + "\n      " + body + m.group(3), text)
    # The root and the footage clips last as long as the take.
    text = re.sub(r'(<(?:div|video)\b[^>]*?\bid="(?:root|footage[^"]*)"[^>]*?\bdata-duration=")[\d.]+(")',
                  lambda m: f"{m.group(1)}{duration}{m.group(2)}", text)
    with open(path, "w", encoding="utf-8", newline="\n") as f:
        f.write(text)


def footage(take):
    """assets/footage.mp4: the take at a constant 30 fps (gdigrab drops the odd frame), with a
    keyframe every half second so preview and render seek exactly."""
    dst = os.path.join(ROOT, "assets", "footage.mp4")
    stamp = dst + ".source"
    if os.path.exists(dst) and os.path.exists(stamp) and open(stamp).read().strip() == take:
        return
    subprocess.run(["ffmpeg", "-v", "error", "-i", os.path.join(HERE, "out", take + ".mp4"), "-vf", f"fps={FPS}",
                    "-c:v", "libx264", "-preset", "medium", "-crf", "14", "-g", "15", "-pix_fmt", "yuv420p",
                    "-movflags", "+faststart", "-an", "-y", dst], check=True)
    with open(stamp, "w") as f:
        f.write(take)


def main(take):
    footage(take)
    data = load(take)
    events = data["events"]
    steps = [{"id": e["id"], "t": round(e["t"], 3), "caption": e["caption"]} for e in events if e["type"] == "step"]
    clicks = [[round(e["t"], 3), e["x"], e["y"]] for e in events if e["type"] == "click"]
    drags = [[round(e["t"], 3)] for e in events if e["type"] == "drag"]
    keys = [[round(e["t"], 3), e["label"]] for e in events if e["type"] in ("key", "type")]
    # The last hold logs nothing, so the length comes from the recording (less a frame of margin).
    probe = subprocess.run(["ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0",
                            os.path.join(HERE, "out", take + ".mp4")], capture_output=True, text=True, check=True)
    duration = math.floor((float(probe.stdout) - 1 / FPS) * 10) / 10
    out = {
        "take": take,
        "duration": duration,
        "steps": steps,
        "clicks": clicks,
        "drags": drags,
        "keys": keys,
        "path": cursor_path(events, duration),
    }
    with open(os.path.join(ROOT, "assets", "tutorial-data.js"), "w", encoding="utf-8", newline="\n") as f:
        f.write("// Generated by recording/make_data.py from recording/out/%s.events.json\n" % take)
        f.write("window.TUTORIAL = " + json.dumps(out, separators=(",", ":")) + ";\n")
    caps, chip_html = clip_html(steps, chips(events), duration)
    for page in ("index.html", "compositions/vertical.html"):
        path = os.path.join(ROOT, page)
        if os.path.exists(path):
            patch(path, caps, chip_html, duration)
    print(f"{take}: {len(steps)} steps, {len(clicks)} clicks, {len(out['path'])} cursor points, duration {duration}s")


if __name__ == "__main__":
    main(sys.argv[1] if len(sys.argv) > 1 else "take3")
