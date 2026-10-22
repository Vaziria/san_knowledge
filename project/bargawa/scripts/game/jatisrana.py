"""The game's copy of Pertapan Jatisrana: blender -b blender/places/jatisrana.blend --python scripts/game/jatisrana.py, or
scripts\\run.ps1 game jatisrana. Reads the built place, saves nothing in it, and writes game/assets/export/jatisrana/ (common/game.py).

Settings per kind (docs/summaries/models/jatisrana.md, "From Blender to Godot"):
    kind     grass | forest | plant | ground | water | solid (buildings, stone, dressing); the game sets shadows and ranges by it
    thin     the share of leaves, blades and plumes kept (common/game.thin)
    drop     materials whose faces the game leaves out (the bamboo's node rings and 1 cm twigs: invisible at game distances)
    keep_instances   the share of placements kept (grass)
    collide  "trimesh" (the mesh itself) or {"r", "h"}: a cylinder round the placement's origin
    range    drawn up to this distance (m); 0 = always
First numbers for milestone 1 (the first walk); the frame rate measured in the game decides the next ones."""
import os, sys; sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))   # scripts/
import common; common.reload()
import bpy, json, math, zlib
from mathutils import Vector
from common import game, jatisrana as J

NAME = "jatisrana"
LEAFY = {"M_leaf", "M_leaf_dark", "M_leaf_light", "M_dry_leaf", "M_grass", "M_grass_dark", "M_grass_dry", "M_plume",
         "M_reed_plume"}
DROP_MATS = {"M_Damar_Flame"}                 # flames are cards with a film shader; the game draws its own fire (milestone 5)

RULES = [                                     # (object name starts with, settings); first match wins
    ("GrassClump",   dict(kind="grass", keep_instances=0.4, range=26.0)),
    ("Tree.",        dict(kind="forest", range=150.0)),
    ("Bamboo",       dict(kind="plant", thin=0.15, drop={"M_bamboo"}, collide={"r": 1.1, "h": 8.0})),
    ("Banyan_Crown", dict(kind="plant", thin=0.35)),
    ("Banyan_Trunk", dict(kind="plant", collide={"r": 2.2, "h": 6.0})),
    ("Banana",       dict(kind="plant", collide={"r": 0.3, "h": 2.0})),
    ("TreeFern",     dict(kind="plant", collide={"r": 0.35, "h": 2.0})),
    ("Kamboja",      dict(kind="plant", collide={"r": 0.35, "h": 2.0})),
    ("Jasmine",      dict(kind="plant", thin=0.5)),
    ("Glagah",       dict(kind="plant", thin=0.5, range=90.0)),
    ("Reeds",        dict(kind="plant", thin=0.5, range=90.0)),
    ("Lotus",        dict(kind="plant", range=90.0)),
    ("Terrain",      dict(kind="ground", collide="trimesh")),
    ("BareGround",   dict(kind="ground")),          # the bare patches lie on the terrain, which the player walks on
    ("Lake_Water",   dict(kind="water")),
    ("Paths",        dict(kind="ground")),
    ("Mountains",    dict(kind="far")),
]
TERRAIN_KEEP = (-140.0, 140.0, -150.0, 120.0)      # x0, x1, y0, y1: the forest's ground; beyond it only haze

def rules(ob):
    for prefix, st in RULES:
        if ob.name.startswith(prefix): return dict(st)
    return dict(kind="solid", collide="trimesh")       # buildings, the pendopo, stone pieces, dressing

def skip(ob):
    return (len(ob.particle_systems) > 0                                          # the grass emitter
            or any(c.name == "_ScaleFigures" for c in ob.users_collection))

def walkable(x, y):
    """The clearing and the open slope down to the lake, but not the water; the lake steps' foot and the bathing stone are in."""
    land = J.in_clearing(x, y) or (-26.0 < x < 26.0 and -30.0 < y < -4.0)
    if land and J.lake_d(x, y) > 1.04: return True
    for (cx, cy), r in ((J.STEPS_BOTTOM, 1.3), (J.BATHING_STONE, 1.4)):
        if math.hypot(x - cx, y - cy) < r: return True
    return False

def main():
    print("STEP collect", flush=True)
    groups = game.collect(rules, skip)
    before = sum(game.tris(g["me"])*len(g["xforms"]) for g in groups.values())
    print("STEP game meshes", flush=True)
    for g in groups.values():
        me, st = g["me"], g["settings"]
        drop = DROP_MATS | st.get("drop", set())
        n = game.drop_faces(me, lambda c, m: m in drop)
        if g["name"] == "Terrain":
            x0, x1, y0, y1 = TERRAIN_KEEP
            n += game.drop_faces(me, lambda c, m: not (x0 < c.x < x1 and y0 < c.y < y1))
        if st.get("thin", 1.0) < 1.0:
            game.thin(me, st["thin"], LEAFY, seed=zlib.crc32(g["name"].encode()))
        game.game_materials(me)
    after = sum(game.tris(g["me"])*len(g["xforms"]) for g in groups.values())
    print(f"STEP triangles placed: film {before}, game {after}", flush=True)

    out = game.export_dir(NAME)
    print("STEP glb", flush=True)
    nodes = game.write_glb(groups, os.path.join(out, NAME + ".glb"))

    print("STEP json", flush=True)
    sun = bpy.data.objects["Sun"]
    to_sun = sun.matrix_world.to_quaternion() @ Vector((0, 0, 1))
    bg = next(n for n in bpy.context.scene.world.node_tree.nodes if n.type == "BACKGROUND")
    layout = {
        "source": "blender/places/jatisrana.blend (scripts/places/jatisrana.py), exported by scripts/game/jatisrana.py",
        "meshes": {name: {"name": g["name"], "tris": game.tris(g["me"]),
                          **{k: v for k, v in g["settings"].items() if k not in ("thin", "keep_instances", "drop")},
                          "xforms": [game.godot_xform(m) for m in g["xforms"]]}
                   for name, g in nodes.items()},
        "sun": {"to_sun": game.godot_vec(to_sun), "color": game.srgb(sun.data.color), "energy": round(sun.data.energy, 3)},
        "sky": {"color": game.srgb(bg.inputs["Color"].default_value), "strength": round(bg.inputs["Strength"].default_value, 3)},
        "walk": game.walk_mask(walkable, -34.0, 34.0, -32.0, 30.0, 0.5),
        # where the player starts (scene 1 wakes him in the sons' hut): in front of its porch, facing south
        "spawn": {"x": J.HUT_SONS[0], "y": J.HUT_SONS[1] - 4.8, "face": "south"},
    }
    with open(os.path.join(out, NAME + ".json"), "w", encoding="utf-8", newline="\n") as f:
        json.dump(layout, f, separators=(",", ":"))
    kinds = {}
    for e in layout["meshes"].values():
        k = kinds.setdefault(e["kind"], [0, 0]); k[0] += len(e["xforms"]); k[1] += e["tris"]*len(e["xforms"])
    for k, (n, t) in sorted(kinds.items()): print(f"STEP   {k:7s} {n:5d} placed, {t:8d} triangles", flush=True)
    glb = os.path.join(out, NAME + ".glb")
    print(f"DONE {glb} ({os.path.getsize(glb)/1e6:.1f} MB), {len(nodes)} meshes", flush=True)

main()
