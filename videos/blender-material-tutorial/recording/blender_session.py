"""Runs inside the tutorial Blender (GUI), started by `driver.py launch`:

    blender --factory-startup --python blender_session.py -- <channel_dir> <ui_scale>

Factory startup keeps the user's own preferences, add-ons and files out of the recording (and
factory preferences are not saved on exit). It sets up the start scene for the material
tutorial, makes the window fullscreen, and serves a file command channel: driver.py drops
`<channel>/in/<id>.py`, this runs it on Blender's main thread and writes `<channel>/out/<id>.json`
with `result`, stdout and any error.
"""
import contextlib
import io
import json
import os
import sys
import time
import traceback

import bmesh
import bpy
from bpy_extras import view3d_utils

argv = sys.argv[sys.argv.index("--") + 1:] if "--" in sys.argv else []
CHANNEL = argv[0]
UI_SCALE = float(argv[1]) if len(argv) > 1 else 1.25
IN = os.path.join(CHANNEL, "in")
OUT = os.path.join(CHANNEL, "out")
os.makedirs(IN, exist_ok=True)
os.makedirs(OUT, exist_ok=True)

prefs = bpy.context.preferences
prefs.view.show_splash = False
prefs.view.ui_scale = UI_SCALE
prefs.view.show_tooltips = False  # captions explain; tooltips would cover the panel


def setup_scene():
    """Default scene minus the cube, plus a smooth sphere with no material, nothing selected."""
    scene = bpy.context.scene
    cube = bpy.data.objects.get("Cube")
    if cube:
        bpy.data.objects.remove(cube, do_unlink=True)
    for mat in list(bpy.data.materials):
        bpy.data.materials.remove(mat)
    mesh = bpy.data.meshes.new("Sphere")
    bm = bmesh.new()
    bmesh.ops.create_uvsphere(bm, u_segments=64, v_segments=32, radius=1.0, calc_uvs=True)
    bm.to_mesh(mesh)
    bm.free()
    mesh.shade_smooth()
    sphere = bpy.data.objects.new("Sphere", mesh)
    (bpy.data.collections.get("Collection") or scene.collection).objects.link(sphere)
    for ob in scene.objects:
        ob.select_set(False)
    bpy.context.view_layer.objects.active = None


def win():
    return bpy.context.window_manager.windows[0]


def rect(o, height):
    """Blender rects have a bottom-left origin; screen pixels have a top-left one."""
    return {"x": o.x, "y": height - (o.y + o.height), "w": o.width, "h": o.height}


def layout():
    w = win()
    areas = []
    for a in w.screen.areas:
        regions = [dict(type=r.type, **rect(r, w.height)) for r in a.regions if r.width > 1 and r.height > 1]
        areas.append(dict(type=a.type, ui_type=a.ui_type, regions=regions, **rect(a, w.height)))
    return {
        "window": {"x": w.x, "y": w.y, "w": w.width, "h": w.height},
        "workspace": w.workspace.name,
        "areas": areas,
    }


def project(co):
    """Screen pixel of a world-space point in the first 3D viewport."""
    w = win()
    for a in w.screen.areas:
        if a.type == "VIEW_3D":
            region = next(r for r in a.regions if r.type == "WINDOW")
            p = view3d_utils.location_3d_to_region_2d(region, a.spaces.active.region_3d, co)
            return [round(region.x + p.x), round(w.height - (region.y + p.y))]
    return None


def frame_view(distance=5.0):
    for a in win().screen.areas:
        if a.type == "VIEW_3D":
            r3d = a.spaces.active.region_3d
            r3d.view_location = (0.0, 0.0, 0.0)
            r3d.view_distance = distance


def _write(path, data):
    tmp = path + ".tmp"
    with open(tmp, "w", encoding="utf-8") as f:
        json.dump(data, f)
    for _ in range(50):
        try:
            os.replace(tmp, path)
            return
        except OSError:
            time.sleep(0.01)


def poll():
    for name in sorted(os.listdir(IN)):
        if not name.endswith(".py"):
            continue
        path = os.path.join(IN, name)
        try:
            with open(path, encoding="utf-8") as f:
                code = f.read()
            os.remove(path)
        except OSError:
            # Windows Defender scans new files and briefly locks them; try again next tick.
            # (An exception escaping this timer would unregister it and end the channel.)
            continue
        out = io.StringIO()
        ns = {"bpy": bpy, "layout": layout, "project": project, "frame_view": frame_view, "win": win, "result": None}
        res = {"ok": True}
        try:
            with contextlib.redirect_stdout(out):
                exec(compile(code, name, "exec"), ns)
            res["result"] = ns.get("result")
        except Exception:
            res = {"ok": False, "error": traceback.format_exc()}
        res["stdout"] = out.getvalue()
        _write(os.path.join(OUT, name[:-3] + ".json"), res)
    return 0.02


def window_setup():
    wm = bpy.context.window_manager
    if not wm.windows:
        return 0.2
    frame_view()
    with bpy.context.temp_override(window=wm.windows[0]):
        bpy.ops.wm.window_fullscreen_toggle()
    _write(os.path.join(CHANNEL, "ready.json"), {"pid": os.getpid()})
    return None


setup_scene()
bpy.app.timers.register(poll, first_interval=0.5, persistent=True)
bpy.app.timers.register(window_setup, first_interval=1.0)
