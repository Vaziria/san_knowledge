"""Runs inside the tutorial Blender (GUI), started by `driver.py launch <part>`:

    blender --factory-startup --python blender_session.py -- <channel_dir> <ui_scale> <part>

Factory startup keeps the user's own preferences, add-ons and files out of the recording (and
factory preferences are not saved on exit). It sets up the start scene of one tutorial part
(fisika, batu, mirror), makes the window fullscreen, and serves a file command channel:
driver.py drops `<channel>/in/<id>.py`, this runs it on Blender's main thread and writes
`<channel>/out/<id>.json` with `result`, stdout and any error.
"""
import contextlib
import io
import json
import math
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
PART = argv[2] if len(argv) > 2 else "fisika"
IN = os.path.join(CHANNEL, "in")
OUT = os.path.join(CHANNEL, "out")
os.makedirs(IN, exist_ok=True)
os.makedirs(OUT, exist_ok=True)

prefs = bpy.context.preferences
prefs.view.show_splash = False
prefs.view.ui_scale = UI_SCALE
prefs.view.show_tooltips = False  # captions explain; tooltips would cover the panel


def collection():
    return bpy.data.collections.get("Collection") or bpy.context.scene.collection


def add_mesh(name, build):
    mesh = bpy.data.meshes.new(name)
    bm = bmesh.new()
    build(bm)
    bm.to_mesh(mesh)
    bm.free()
    ob = bpy.data.objects.new(name, mesh)
    collection().objects.link(ob)
    return ob


def ground(size):
    return add_mesh("Plane", lambda bm: bmesh.ops.create_grid(bm, x_segments=1, y_segments=1, size=size / 2))


def setup_scene():
    """The start state of each part, nothing selected."""
    scene = bpy.context.scene
    scene.frame_start, scene.frame_end, scene.frame_current = 1, 150, 1
    cube = bpy.data.objects.get("Cube")
    if PART == "fisika":
        # A cube hanging in the air over the ground; no physics yet.
        cube.location = (0.0, 0.0, 4.5)
        cube.rotation_euler = (math.radians(25), math.radians(18), 0.0)
        ground(12.0)
    else:
        bpy.data.objects.remove(cube, do_unlink=True)
        if PART == "batu":
            ground(8.0)
            scene.cursor.location = (0.0, 0.0, 0.6)  # flattened to 0.6 later, the stone then rests on the ground
        elif PART == "mirror":
            ground(10.0)

            def pillar(bm):
                # 12 sides, radius 0.25, 3 m tall, standing on the ground; origin at the world centre.
                bmesh.ops.create_cone(bm, cap_ends=True, cap_tris=False, segments=12,
                                      radius1=0.25, radius2=0.25, depth=3.0)
                bmesh.ops.translate(bm, verts=bm.verts, vec=(0.0, 0.0, 1.5))
                for v in bm.verts:
                    v.select = True
                for e in bm.edges:
                    e.select = True
                for f in bm.faces:
                    f.select = True

            add_mesh("Pillar", pillar)
    for ob in scene.objects:
        ob.select_set(False)
    bpy.context.view_layer.objects.active = None


def hide_helpers():
    """Camera and light only clutter the shot; they stay listed in the Outliner."""
    for name in ("Camera", "Light"):
        ob = bpy.data.objects.get(name)
        if ob:
            ob.hide_set(True)


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


def view3d():
    for a in win().screen.areas:
        if a.type == "VIEW_3D":
            return a
    return None


def project(co):
    """Screen pixel of a world-space point in the first 3D viewport."""
    w = win()
    a = view3d()
    region = next(r for r in a.regions if r.type == "WINDOW")
    p = view3d_utils.location_3d_to_region_2d(region, a.spaces.active.region_3d, co)
    return [round(region.x + p.x), round(w.height - (region.y + p.y))]


VIEWS = {
    # view_location, view_distance, view rotation as XYZ Euler degrees (None: the factory angle)
    "fisika": ((0.0, 0.0, 1.2), 21.0, None),
    "batu": ((0.0, 0.0, 0.9), 9.5, None),
    "mirror": ((0.0, 0.0, 1.4), 11.0, (75.0, 0.0, 18.0)),  # from the front, so the pillars stand side by side
}


def frame_view(location=None, distance=None):
    from mathutils import Euler
    loc, dist, rot = VIEWS[PART]
    space = view3d().spaces.active
    r3d = space.region_3d
    r3d.view_location = location or loc
    r3d.view_distance = distance or dist
    if rot:
        r3d.view_rotation = Euler([math.radians(v) for v in rot]).to_quaternion()
    if PART == "batu":
        space.overlay.show_stats = True  # the triangle count is part of the lesson


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
        ns = {"bpy": bpy, "layout": layout, "project": project, "frame_view": frame_view, "win": win,
              "view3d": view3d, "PART": PART, "math": math, "result": None}
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
    hide_helpers()
    with bpy.context.temp_override(window=wm.windows[0]):
        bpy.ops.wm.window_fullscreen_toggle()
    _write(os.path.join(CHANNEL, "ready.json"), {"pid": os.getpid()})
    return None


setup_scene()
bpy.app.timers.register(poll, first_interval=0.5, persistent=True)
bpy.app.timers.register(window_setup, first_interval=1.0)
