"""Review stills of a place: temporary cameras on its models (not story shots; nothing is saved). Writes one PNG per view and a
contact sheet into renders/checks/<place>/.
    blender -b <place.blend> --python scripts/tools/views.py        env: VIEWS_OUT (folder), VIEWS_PCT (size %), VIEWS_ONLY ("a,b")
Turns the _ScaleFigures collection on for the stills."""
import os, sys; sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))   # scripts/
import common; common.reload()
import bpy, numpy as np
from mathutils import Vector
from common import mesh, joglo, jatisrana as J

F = joglo.F
VIEWS = {   # name: (camera position, look-at point, lens mm)
    "map":            ((0.0, -12.0, 140.0), (0.0, -11.99, 0.0), 30),
    "overview":       ((4.0, -88.0, 52.0), (0.0, 0.0, 0.0), 30),
    "pendapa":        ((-12.5, -15.0, 1.7), (0.0, 0.0, 3.2), 28),
    "pendapa_inside": ((-2.6, -3.3, F + 1.5), (0.4, 0.6, F + 4.9), 14),
    "from_lake":      ((6.0, -44.0, J.WATER_Z + 1.6), (0.0, -2.0, 2.5), 26),
    "huts":           ((14.0, -3.0, 1.7), (-3.0, 11.0, 1.8), 22),
    "hut_elders":     ((-1.6, 4.2, 1.6), (-6.0, 10.2, 1.7), 26),
    "pawon":          ((8.2, 0.6, 1.6), (10.6, 6.6, 1.1), 24),
    "sanggar":        ((-11.6, 4.4, 1.6), (-17.0, 8.0, 2.0), 26),
    "gate":           ((-9.0, -5.8, 1.6), (-13.5, -2.0, 1.6), 26),
    "banyan":         ((-4.0, -22.5, -1.8), (8.5, -13.5, 4.5), 20),
    "kamboja":        ((-13.8, 17.8, 2.0), (-19.8, 11.2, 3.0), 30),
    "jasmine":        ((-0.8, 5.6, 1.3), (-3.0, 8.4, 0.6), 35),
    "lake_edge":      ((-11.5, -23.5, -2.2), (-19.0, -30.0, -3.0), 24),
    "lotus":          ((-6.0, -27.2, -2.6), (-9.0, -31.0, -3.9), 30),
    "bamboo":         ((3.0, 17.5, 1.7), (1.0, 26.0, 6.5), 22),
    "yard":           ((-9.5, -9.5, 1.3), (-3.5, -3.0, 0.1), 24),               # the bare yard by the pendapa's south-west corner
    "roof_eave":      ((-9.0, -9.0, 2.0), (-5.0, -5.5, 3.4), 28),               # the thatch's relief at the pendapa's south-west eave
    "far_bank":       ((-3.5, -54.0, J.height(-3.5, -54.0) + 1.5), (-10.0, -47.5, J.height(-10.0, -47.5) + 0.2), 24),
    "lamp":           ((0.75, -0.55, F + 0.85), (0.0, 1.3, F + 0.55), 32),       # the damar between the mats and the bale
    "lamp_night":     ((0.75, -0.55, F + 0.85), (0.0, 1.3, F + 0.55), 32, {"night": True}),
}
BUILDING_VIEWS = {   # a building file: the building at the origin, facing south
    "front":   ((-9.5, -13.0, 1.7), (0.0, 0.0, 3.0), 28),
    "back":    ((11.0, 12.5, 3.0), (0.0, 0.0, 3.2), 28),
    "side":    ((17.0, -1.0, 2.0), (0.0, 0.0, 3.6), 30),
    "inside":  ((-2.6, -3.3, F + 1.5), (0.4, 0.6, F + 4.9), 14),
    "eave":    ((-6.4, -7.8, 2.0), (-4.6, -5.0, 3.3), 35),
    "ceiling": ((0.5, -0.9, F + 1.4), (0.0, 0.0, F + 4.8), 16),
    "umpak":   ((-0.7, -2.9, F + 0.7), (-1.6, -1.6, F + 0.2), 40),
    "umpak_pen": ((-2.45, -6.3, F + 0.75), (-1.567, -4.7, F + 0.2), 40),
    "soko_wood": ((-0.75, -2.55, F + 2.0), (-1.6, -1.6, F + 2.3), 45),
    "beam_wood": ((-0.2, -3.0, F + 3.2), (-1.0, -1.6, F + 4.1), 35),
    "sunduk":  ((-0.1, -3.4, F + 2.9), (-1.7, -1.6, F + 3.45), 40),
    "stair":   ((2.6, -9.2, 1.2), (0.0, -5.8, 0.25), 35),
    "stair_close": ((1.0, -7.1, 0.7), (0.1, -5.95, 0.25), 50),
    "top":     ((0.0, 0.0, 42.0), (0.0, 0.01, 0.0), 30),
}
STONE_VIEWS = {   # objects/stone.blend: padma umpak in a row at y 1.2, the stair's back edge at y -0.6, plain umpak at y -2.3
    "all":     ((5.4, -6.8, 2.6), (1.35, -0.5, 0.3), 30),
    "padma":   ((1.25, -0.45, 0.8), (0.9, 1.2, 0.38), 45),
    "umpak":   ((1.45, -3.25, 0.55), (1.05, -2.3, 0.12), 55),
    "stair":   ((3.9, -4.2, 1.2), (1.35, -1.1, 0.15), 35),
    "top":     ((1.35, -0.5, 9.5), (1.35, -0.49, 0.0), 35),
}
SOKO_VIEWS = {   # objects/soko.blend: soko guru at x 0, 0.7, 1.4, 2.1 (y 0.6), soko penanggap in front of the gaps (y -0.6)
    "all":       ((4.6, -6.4, 2.2), (1.2, 0.0, 1.4), 30),
    "guru":      ((0.7, -1.4, 1.7), (0.7, 0.6, 1.6), 40),
    "penanggap": ((1.05, -1.9, 1.1), (1.05, -0.6, 1.0), 45),
    "top":       ((1.2, -0.01, 9.0), (1.2, 0.0, 0.0), 35),
}
NIGHT = {"night": True}   # a 4th value: the sun off and the sky nearly black, to see lamps and their light
# a 4th value {"frame": n} renders that frame (flames move with the frame); the default is the file's current frame
DAMAR_VIEWS = {   # objects/damar.blend: the lamp at the origin (0.89 m tall), the lone flame piece at x 0.3
    "lamp":    ((0.62, -1.45, 0.62), (0.0, 0.0, 0.44), 30),
    "head":    ((0.16, -0.30, 0.87), (0.0, 0.0, 0.78), 50),
    "ascetic": ((0.035, -0.15, 0.83), (0.0, 0.0, 0.82), 50),
    "dish":    ((0.2, -0.02, 0.86), (0.07, 0.07, 0.765), 50),
    "top":     ((0.0, -0.001, 1.35), (0.0, 0.0, 0.75), 50),
    "night":   ((0.62, -1.45, 0.62), (0.0, 0.0, 0.44), 30, NIGHT),
    "night_head": ((0.16, -0.30, 0.87), (0.0, 0.0, 0.78), 50, NIGHT),
    "flame":   ((0.24, -0.11, 0.80), (0.089, 0.089, 0.797), 85),                 # the south-east flame from the side, frame 1
    "flame_f7":  ((0.24, -0.11, 0.80), (0.089, 0.089, 0.797), 85, {"frame": 7}),
    "flame_f15": ((0.24, -0.11, 0.80), (0.089, 0.089, 0.797), 85, {"frame": 15}),
    "flame_night": ((0.24, -0.11, 0.80), (0.089, 0.089, 0.797), 85, {"night": True, "frame": 7}),
}
FIRE_VIEWS = {   # objects/fire.blend: the samples along x (fuel.SAMPLES): lamp -2.4, hearth -1.4, wood 0, embers 1.4, large 5
    "all":          ((1.2, -10.5, 2.6), (1.3, 0.0, 0.5), 30),
    "all_night":    ((1.2, -10.5, 2.6), (1.3, 0.0, 0.5), 30, NIGHT),
    "wood":         ((0.95, -1.55, 0.7), (0.0, 0.0, 0.32), 35),
    "wood_night":   ((0.95, -1.55, 0.7), (0.0, 0.0, 0.32), 35, NIGHT),
    "wood_f12":     ((0.95, -1.55, 0.7), (0.0, 0.0, 0.32), 35, {"night": True, "frame": 12}),
    "hearth_night": ((-0.65, -0.45, 0.35), (-1.4, -0.15, 0.1), 35, NIGHT),
    "embers_night": ((1.95, -0.75, 0.6), (1.4, 0.0, 0.03), 40, NIGHT),
    "large_night":  ((8.0, -4.6, 1.5), (5.0, 0.0, 1.2), 30, NIGHT),
    "large_f12":    ((8.0, -4.6, 1.5), (5.0, 0.0, 1.2), 30, {"night": True, "frame": 12}),
}
VIEWS = {"building": BUILDING_VIEWS, "stone": STONE_VIEWS, "soko": SOKO_VIEWS, "damar": DAMAR_VIEWS, "fire": FIRE_VIEWS}.get(os.environ.get("VIEWS_SET"), VIEWS)

def night(on, saved=None):
    """Turn the sun off and the sky down for a night still (on=True; returns what to restore), or restore it (on=False)."""
    sc = bpy.context.scene; bg = None
    if sc.world and sc.world.node_tree:
        bg = next((n for n in sc.world.node_tree.nodes if n.type == "BACKGROUND"), None)
    if on:
        saved = {"suns": [(o, o.hide_render) for o in sc.objects if o.type == "LIGHT" and o.data.type == "SUN"],
                 "sky": bg.inputs["Strength"].default_value if bg else None}
        for o, _ in saved["suns"]: o.hide_render = True
        if bg: bg.inputs["Strength"].default_value = 0.004
        return saved
    for o, h in saved["suns"]: o.hide_render = h
    if bg and saved["sky"] is not None: bg.inputs["Strength"].default_value = saved["sky"]

def main():
    sc = bpy.context.scene
    out = os.environ.get("VIEWS_OUT") or os.path.join(os.path.dirname(bpy.data.filepath), "_views")
    pct = int(os.environ.get("VIEWS_PCT", "35")); only = [s for s in os.environ.get("VIEWS_ONLY", "").split(",") if s]
    os.makedirs(out, exist_ok=True)
    for c in bpy.data.collections:
        if c.name == "_ScaleFigures": c.hide_render = False
    sc.render.resolution_percentage = pct
    try: sc.eevee.taa_render_samples = 24
    except AttributeError: pass
    sc.render.image_settings.file_format = "PNG"
    names = [n for n in VIEWS if not only or n in only]; files = []
    for name in names:
        loc, target, lens, *opts = VIEWS[name]; opts = opts[0] if opts else {}
        cam = bpy.data.objects.new("_view_" + name, bpy.data.cameras.new("_view_" + name)); sc.collection.objects.link(cam)
        dist = (Vector(target) - Vector(loc)).length
        cam.data.lens = lens; cam.data.clip_start = min(0.1, dist/20); cam.data.clip_end = 8000.0      # close-ups of small things
        cam.location = loc; mesh.aim(cam, Vector(target) - Vector(loc))
        sc.camera = cam
        sc.render.filepath = os.path.join(out, name + ".png")
        saved = night(True) if opts.get("night") else None
        f0 = sc.frame_current; sc.frame_set(opts.get("frame", f0))
        bpy.ops.render.render(write_still=True)
        sc.frame_set(f0)
        if saved: night(False, saved)
        img = bpy.data.images.load(sc.render.filepath); px = np.empty(img.size[0]*img.size[1]*4, np.float32); img.pixels.foreach_get(px)
        print("SHOT %-15s mean %5.1f / 255" % (name, px.reshape(-1, 4)[:, :3].mean()*255), flush=True)
        files.append(sc.render.filepath); bpy.data.images.remove(img)
    sheet(files, os.path.join(out, "sheet.png"))

def sheet(files, path, cols=2, gap=6):
    """The stills in a grid, in order left to right, top to bottom."""
    imgs = []
    for f in files:
        im = bpy.data.images.load(f); w, h = im.size; a = np.empty(w*h*4, np.float32); im.pixels.foreach_get(a)
        imgs.append(a.reshape(h, w, 4)); bpy.data.images.remove(im)
    h, w = imgs[0].shape[:2]; rows = (len(imgs) + cols - 1)//cols
    H, W = rows*h + (rows - 1)*gap, cols*w + (cols - 1)*gap
    canvas = np.ones((H, W, 4), np.float32)
    for k, a in enumerate(imgs):
        r, c = divmod(k, cols); y0 = H - (r + 1)*h - r*gap; x0 = c*(w + gap)          # pixel rows run bottom-up
        canvas[y0:y0 + h, x0:x0 + w] = a
    out = bpy.data.images.new("sheet", W, H, alpha=False); out.pixels.foreach_set(canvas.ravel())
    out.filepath_raw = path; out.file_format = "PNG"; out.save()
    print("SHEET", path, "order:", " ".join(os.path.splitext(os.path.basename(f))[0] for f in files), flush=True)

main()
