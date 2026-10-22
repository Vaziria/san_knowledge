"""The game's copy of a place, for the Godot project in project/bargawa/game (scripts/game/<place>.py, run.ps1 game <place>).

A phone can't draw the film place: 1.9 million triangles, box-projected textures, hair-particle grass. The export makes a game
version of every distinct mesh and writes two files into game/assets/export/<place>/:
    <place>.glb    one node per distinct mesh (K0001, K0002 ...), at the origin, with plain glTF materials
    <place>.json   where each mesh stands (Godot transforms), what kind it is, its collider and how far it is drawn; the sun, the
                   sky and the walkable area. game/scripts/place.gd reads both and draws repeated meshes as MultiMeshes in chunks.
Nothing in the game is edited by hand: change the place, then export again.

Game versions:
    - Leaves, blades and plumes (small loose parts made of leafy materials) are thinned to a share, the kept ones grown a little so
      the plant keeps its fill; trunks and the dark inner leaf masses stay. Godot's own LODs can't do this: they only simplify
      connected surfaces.
    - Textures: the Poly Haven image mixed with the palette colour as the Blender shader mixes them (common/textures.py), baked into
      one image; UVs from the piece's own grain UVs (timber) or a box projection in object space, both scaled by the tile size.
      Bump, wear and weathering layers are left out. A material without a texture keeps its palette colour.
Axes: Blender (x, y, z) is Godot (x, z, -y). Godot's south is +Z."""
import bpy, bmesh, math, os, random, shutil
import numpy as np
from mathutils import Matrix, Vector
from . import paths, polyhaven as PH

GAME = os.path.join(paths.ROOT, "game")
TEX_WORK = os.path.join(paths.ROOT, "_work", "game_textures")      # the mixed textures until the GLB has them; removed after

def export_dir(name):
    return os.path.join(GAME, "assets", "export", name)

# --- axes -----------------------------------------------------------------------------------------------------------------
C = Matrix(((1, 0, 0, 0), (0, 0, 1, 0), (0, -1, 0, 0), (0, 0, 0, 1)))
CI = C.inverted()

def godot_xform(mw):
    """A Blender world matrix as a Godot Transform3D: the basis columns x, y, z, then the origin (12 numbers)."""
    t = C @ mw @ CI
    return [round(t[r][c], 5) for c in range(3) for r in range(3)] + [round(t[r][3], 4) for r in range(3)]

def godot_vec(v):
    return [round(v[0], 4), round(v[2], 4), round(-v[1], 4)]

def srgb(c):
    return [round(max(0.0, c[i])**(1/2.2), 4) for i in range(3)]

# --- loose parts and thinning ----------------------------------------------------------------------------------------------
def islands(me):
    """The loose part of every face, as one number per face."""
    parent = list(range(len(me.vertices)))
    def find(a):
        r = a
        while parent[r] != r: r = parent[r]
        while parent[a] != r: parent[a], a = r, parent[a]
        return r
    ev = np.empty(len(me.edges)*2, np.int32); me.edges.foreach_get("vertices", ev)
    for a, b in ev.reshape(-1, 2).tolist():
        ra, rb = find(a), find(b)
        if ra != rb: parent[ra] = rb
    ls = np.empty(len(me.polygons), np.int32); me.polygons.foreach_get("loop_start", ls)
    lv = np.empty(len(me.loops), np.int32); me.loops.foreach_get("vertex_index", lv)
    return [find(v) for v in lv[ls].tolist()]

def thin(me, keep, leafy, max_faces=24, seed=0, grow=0.25):
    """Keep about `keep` of the small loose parts (at most max_faces faces, all of `leafy` materials), each kept one grown by
    keep**-grow about its centre. Bigger parts and other materials stay whole."""
    if keep >= 1.0: return
    leafy_idx = {i for i, m in enumerate(me.materials) if m and m.name in leafy}
    mi = np.empty(len(me.polygons), np.int32); me.polygons.foreach_get("material_index", mi); mi = mi.tolist()
    parts = {}
    for f, i in enumerate(islands(me)): parts.setdefault(i, []).append(f)
    rng = random.Random(seed); drop, grown = [], []
    for fs in parts.values():
        if len(fs) > max_faces or any(mi[f] not in leafy_idx for f in fs): continue
        (grown if rng.random() < keep else drop).append(fs)
    bm = bmesh.new(); bm.from_mesh(me); bm.faces.ensure_lookup_table()
    s = keep**-grow
    for fs in grown:
        vs = {v for f in fs for v in bm.faces[f].verts}
        c = sum((v.co for v in vs), Vector())/len(vs)
        for v in vs: v.co = c + (v.co - c)*s
    bmesh.ops.delete(bm, geom=[bm.faces[f] for fs in drop for f in fs], context="FACES")
    bm.to_mesh(me); bm.free()

def drop_faces(me, test):
    """Delete the faces for which test(centre, material name) is true."""
    names = [m.name if m else "" for m in me.materials]
    bm = bmesh.new(); bm.from_mesh(me)
    gone = [f for f in bm.faces if test(f.calc_center_median(), names[f.material_index] if f.material_index < len(names) else "")]
    if gone: bmesh.ops.delete(bm, geom=gone, context="FACES")
    bm.to_mesh(me); bm.free()
    return len(gone)

# --- materials ------------------------------------------------------------------------------------------------------------
_MATS = {}

def _to_lin(c): return np.where(c <= 0.04045, c/12.92, ((c + 0.055)/1.055)**2.4)
def _to_srgb(c): return np.where(c <= 0.0031308, c*12.92, 1.055*np.power(np.clip(c, 0, None), 1/2.4) - 0.055)

def _mix_input(m):
    """The palette colour and factor the shader mixes into the texture (common/textures.py), or None. Only the mix fed by the
    image counts: the chipped stone has a second mix for its worn edges (textures.add_wear)."""
    for n in m.node_tree.nodes if m.node_tree else []:
        if n.type == "MIX" and getattr(n, "data_type", "") == "RGBA":
            f = next(s for s in n.inputs if s.name == "Factor" and s.type == "VALUE")
            a = next(s for s in n.inputs if s.name == "A" and s.type == "RGBA")
            b = next(s for s in n.inputs if s.name == "B" and s.type == "RGBA")
            if a.is_linked and a.links[0].from_node.type == "TEX_IMAGE" and not b.is_linked:
                return tuple(b.default_value[:3]), f.default_value
    return None

def _mixed_image(path, base, mix, name):
    src = bpy.data.images.load(path, check_existing=True)
    im = src.copy(); im.name = name
    px = np.empty(im.size[0]*im.size[1]*4, np.float32); im.pixels.foreach_get(px); px = px.reshape(-1, 4)
    lin = _to_lin(px[:, :3])*(1.0 - mix) + np.array(base, np.float32)*mix        # the shader mixes in linear
    px[:, :3] = np.clip(_to_srgb(lin), 0.0, 1.0)
    im.pixels.foreach_set(px.ravel())
    os.makedirs(TEX_WORK, exist_ok=True)
    out = os.path.join(TEX_WORK, name + ".png")
    im.filepath_raw = out; im.file_format = "PNG"; im.save()
    bpy.data.images.remove(im)
    img = bpy.data.images.load(out); img.name = name
    return img

def game_material(m):
    """(glTF material, tile size or None, use the piece's own UVs) for Blender material m, made once per name."""
    if m.name in _MATS: return _MATS[m.name]
    key = m.name[2:] if m.name.startswith("M_") else m.name
    p = next((n for n in m.node_tree.nodes if n.type == "BSDF_PRINCIPLED"), None) if m.node_tree else None
    def value(sock, default):
        return p.inputs[sock].default_value if p and not p.inputs[sock].is_linked else default
    g = bpy.data.materials.new("G_" + key)
    try: g.use_nodes = True
    except Exception: pass
    gp = next(n for n in g.node_tree.nodes if n.type == "BSDF_PRINCIPLED")
    gp.inputs["Roughness"].default_value = value("Roughness", 0.8)
    gp.inputs["Metallic"].default_value = value("Metallic", 0.0)
    tex = PH.TEXTURES.get(key); diff = PH.file(tex[0], "Diffuse") if tex else None
    mixed = _mix_input(m)
    if diff and mixed:
        base, mix = mixed
        n = g.node_tree.nodes.new("ShaderNodeTexImage"); n.image = _mixed_image(diff, base, mix, "G_" + key)
        g.node_tree.links.new(n.outputs["Color"], gp.inputs["Base Color"])
        res = (g, tex[1], key in PH.UV_KEYS)
    else:
        gp.inputs["Base Color"].default_value = (*m.diffuse_color[:3], 1.0)
        res = (g, None, False)
    _MATS[m.name] = res
    return res

def game_materials(me):
    """Swap mesh me's materials for game ones and give the textured faces their UVs (the only UV map left)."""
    tiles = {}
    for i, m in enumerate(me.materials):
        if not m: continue
        g, tile, use_uv = game_material(m); me.materials[i] = g
        if tile: tiles[i] = (tile, use_uv)
    bm = bmesh.new(); bm.from_mesh(me)
    old = bm.loops.layers.uv.active
    uv = bm.loops.layers.uv.new("GameUV")
    for f in bm.faces:
        t = tiles.get(f.material_index)
        if not t: continue
        tile, use_uv = t
        n = f.normal; ax = max(range(3), key=lambda k: abs(n[k]))
        for lp in f.loops:
            if use_uv and old is not None: u, v = lp[old].uv
            else:
                co = lp.vert.co
                u, v = (co.y, co.z) if ax == 0 else (co.x, co.z) if ax == 1 else (co.x, co.y)
            lp[uv].uv = (u/tile, v/tile)
    for layer in [l for l in bm.loops.layers.uv.values() if l.name != "GameUV"]:
        bm.loops.layers.uv.remove(layer)
    bm.to_mesh(me); bm.free()

# --- collecting what the place draws ---------------------------------------------------------------------------------------
def collect(rules, skip, keep_seed=7):
    """Every mesh the place draws, grouped by its mesh: {key: dict(name, settings, me, xforms)}. rules(ob) -> settings dict for the
    original object ob, skip(ob) -> True to leave it out. A settings 'keep_instances' < 1 keeps that share of the placements."""
    dg = bpy.context.evaluated_depsgraph_get()
    groups = {}; rng = random.Random(keep_seed)
    for inst in dg.object_instances:
        ob = inst.object
        if ob.type != "MESH" or not inst.show_self: continue
        orig = ob.original
        if skip(orig): continue
        data = orig.data
        key = (orig.name, "object") if orig.modifiers else (data.name, data.library.filepath if data.library else "")
        g = groups.get(key)
        if g is None:
            st = rules(orig)
            me = bpy.data.meshes.new_from_object(ob, preserve_all_data_layers=True, depsgraph=dg)
            me.name = "G_" + orig.name
            g = groups[key] = dict(name=orig.name, settings=st, me=me, xforms=[])
        if g["settings"].get("keep_instances", 1.0) < 1.0 and rng.random() >= g["settings"]["keep_instances"]:
            continue
        g["xforms"].append(inst.matrix_world.copy())
    return groups

def tris(me):
    return sum(len(p.vertices) - 2 for p in me.polygons)

# --- writing ------------------------------------------------------------------------------------------------------------
def write_glb(groups, path):
    """One node per group (K0001 ...) at the origin. Returns {node name: group}."""
    sc = bpy.context.scene
    col = bpy.data.collections.new("GameExport"); sc.collection.children.link(col)
    for o in bpy.context.view_layer.objects: o.select_set(False)
    nodes = {}
    for k, g in enumerate(sorted(groups.values(), key=lambda g: g["name"])):
        if not g["xforms"] or not len(g["me"].polygons): continue
        name = "K%04d" % (k + 1)
        ob = bpy.data.objects.new(name, g["me"]); col.objects.link(ob); ob.select_set(True)
        nodes[name] = g
    os.makedirs(os.path.dirname(path), exist_ok=True)
    bpy.ops.export_scene.gltf(filepath=path, export_format="GLB", use_selection=True, export_yup=True, export_apply=False,
                              export_texcoords=True, export_normals=True, export_tangents=False, export_materials="EXPORT",
                              export_image_format="JPEG", export_jpeg_quality=88, export_image_quality=88, export_cameras=False,
                              export_lights=False, export_extras=False, export_animations=False, export_skins=False,
                              export_morph=False)
    shutil.rmtree(TEX_WORK, ignore_errors=True)
    return nodes

def walk_mask(test, x0, x1, y0, y1, cell):
    """The walkable area as rows of '0'/'1' (row 0 at y0, column 0 at x0), in Blender coordinates."""
    cols = int(round((x1 - x0)/cell)); rows = int(round((y1 - y0)/cell))
    data = ["".join("1" if test(x0 + (i + 0.5)*cell, y0 + (j + 0.5)*cell) else "0" for i in range(cols)) for j in range(rows)]
    return {"x0": x0, "y0": y0, "cell": cell, "cols": cols, "rows": rows, "data": data}
