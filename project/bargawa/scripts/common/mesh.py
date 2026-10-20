"""The bpy side of the builders: an empty file, collections, the material palette, Parts (pieces -> one mesh object), instances,
the sun and the sky."""
import bpy, bmesh, math, os
from mathutils import Vector

def fresh_file(path):
    """Empty the factory startup file and save it as `path` right away (so later links get relative paths). Returns the scene."""
    for o in list(bpy.data.objects): bpy.data.objects.remove(o, do_unlink=True)
    for c in list(bpy.data.collections): bpy.data.collections.remove(c)
    for m in list(bpy.data.meshes): bpy.data.meshes.remove(m)
    for m in list(bpy.data.materials): bpy.data.materials.remove(m)
    sc = bpy.context.scene; sc.name = "Scene"
    bpy.context.preferences.filepaths.save_version = 0      # no .blend1 copies: the scripts rebuild every file (the user: disk space)
    os.makedirs(os.path.dirname(path), exist_ok=True)
    bpy.ops.wm.save_as_mainfile(filepath=path)
    return sc

def coll(name, parent=None):
    c = bpy.data.collections.new(name); (parent or bpy.context.scene.collection).children.link(c); return c

def _principled(m):
    try: m.use_nodes = True
    except Exception: pass
    nt = m.node_tree
    p = next((n for n in nt.nodes if n.type == "BSDF_PRINCIPLED"), None)
    if p is None:
        p = nt.nodes.new("ShaderNodeBsdfPrincipled")
        out = next((n for n in nt.nodes if n.type == "OUTPUT_MATERIAL"), None) or nt.nodes.new("ShaderNodeOutputMaterial")
        nt.links.new(p.outputs[0], out.inputs["Surface"])
    return p

def material(name, rgb, rough=0.8, metal=0.0):
    """A plain material (made once per name). rgb is linear 0-1."""
    m = bpy.data.materials.get(name)
    if m: return m
    m = bpy.data.materials.new(name); p = _principled(m)
    p.inputs["Base Color"].default_value = (*rgb, 1.0)
    p.inputs["Roughness"].default_value = rough
    p.inputs["Metallic"].default_value = metal
    m.diffuse_color = (*rgb, 1.0)
    return m

def lin(r, g, b):
    """sRGB 0-255 -> linear 0-1 (colours are picked by eye in sRGB)."""
    f = lambda c: (c/255.0)/12.92 if c/255.0 <= 0.04045 else ((c/255.0 + 0.055)/1.055)**2.4
    return (f(r), f(g), f(b))

# The look-development palette (stylized, slightly desaturated). Textures from Poly Haven replace these later (milestone 4).
PALETTE = {
    "stone":      (lin(128, 124, 116), 0.85),   # andesite plinths and steps
    "stone_kit":  (lin(136, 132, 124), 0.85),   # chipped stone blocks (common/stone.py); worn edges lighter
    "stone_dark": (lin(86, 84, 80), 0.9),       # umpak, foot courses, shrine
    "teak":       (lin(92, 58, 34), 0.6),       # pillars, beams
    "teak_dark":  (lin(58, 36, 22), 0.6),       # rafters, door frames
    "thatch":     (lin(150, 122, 78), 0.95),    # alang-alang roofs
    "ridge":      (lin(70, 56, 40), 0.9),       # ridge caps
    "bamboo":     (lin(170, 146, 98), 0.8),     # woven walls, fences
    "door":       (lin(30, 22, 16), 0.9),       # openings
    "earth":      (lin(120, 92, 66), 0.95),     # paths, clay stove
    "clay":       (lin(158, 92, 60), 0.8),      # pots, kendi
    "mat":        (lin(176, 156, 112), 0.9),    # pandan mats
    "bark":       (lin(92, 82, 70), 0.9),
    "bark_dark":  (lin(62, 54, 46), 0.9),
    "leaf":       (lin(52, 78, 40), 0.8),
    "leaf_dark":  (lin(34, 54, 30), 0.8),
    "leaf_light": (lin(92, 118, 52), 0.8),
    "bamboo_culm": (lin(118, 128, 66), 0.6),
    "plume":      (lin(214, 206, 188), 0.9),    # glagah plumes
    "lotus_pad":  (lin(58, 92, 50), 0.4),
    "lotus":      (lin(222, 150, 170), 0.6),
    "flower":     (lin(240, 236, 226), 0.7),    # jasmine, kamboja
    "mountain":   (lin(96, 110, 124), 1.0),
    "soot":       (lin(44, 38, 34), 0.7),       # blackened pots, the wok
    "banana_stem": (lin(150, 160, 96), 0.7),
    "dry_leaf":   (lin(128, 98, 60), 0.9),      # dead banana leaves
    "flower_center": (lin(240, 196, 80), 0.7),  # kamboja and lotus centres
    "lotus_light": (lin(244, 206, 214), 0.6),
    "reed_plume": (lin(150, 120, 108), 0.9),
    "grass":      (lin(84, 112, 52), 0.85),
    "grass_dark": (lin(62, 90, 42), 0.85),
    "grass_dry":  (lin(140, 136, 76), 0.9),
    "clay_dark":  (lin(110, 66, 44), 0.8),      # the stove
    "figure":     (lin(200, 80, 40), 0.7),      # scale figures (review only)
    "teak_turned": (lin(64, 40, 24), 0.5),      # small turned, oiled wood (the damar's stand)
    "bronze":     (lin(148, 102, 54), 0.35),    # cast bronze; patina and polish in textures.bronze
    "oil":        (lin(46, 34, 14), 0.05),      # coconut oil in the lamp's dishes, dark and glossy
    "wick":       (lin(168, 146, 106), 0.8),    # twisted cotton, soaked with oil (clean white glared)
    "firewood":   (lin(96, 76, 58), 0.9),       # logs in the fire samples; fire.charred burns them where 'wear' is high
}

def point_light(name, col, loc, energy, rgb, radius=0.01):
    ob = bpy.data.objects.new(name, bpy.data.lights.new(name, "POINT")); col.objects.link(ob)
    ob.data.energy = energy; ob.data.color = rgb; ob.data.shadow_soft_size = radius; ob.location = loc
    return ob

def woven_material():
    """Woven bamboo (gedhek): two tones in a small checker, so walls read as woven before the textures come."""
    m = bpy.data.materials.get("M_woven")
    if m: return m
    m = bpy.data.materials.new("M_woven"); p = _principled(m); nt = m.node_tree
    tc = nt.nodes.new("ShaderNodeTexCoord"); ck = nt.nodes.new("ShaderNodeTexChecker")
    ck.inputs["Scale"].default_value = 36.0
    ck.inputs["Color1"].default_value = (*lin(176, 150, 100), 1); ck.inputs["Color2"].default_value = (*lin(140, 114, 72), 1)
    nt.links.new(tc.outputs["Object"], ck.inputs["Vector"]); nt.links.new(ck.outputs["Color"], p.inputs["Base Color"])
    p.inputs["Roughness"].default_value = 0.85; m.diffuse_color = (*lin(160, 134, 88), 1)
    return m

def mats(textures=True):
    """Every palette material by name, plus the woven bamboo; with the Poly Haven textures on those that have one."""
    M = {k: material("M_" + k, rgb, rough) for k, (rgb, rough) in PALETTE.items()}
    M["woven"] = woven_material()
    if textures:
        from . import textures as T
        T.apply(M)
    return M

GRAIN_MATS = {"M_teak", "M_teak_dark", "M_teak_turned"}   # timber: textured through UVs that follow each piece's length

def _components(n_verts, faces):
    """Groups of faces that share points: the separate pieces inside one vertex/face list (a merge of four beams is four)."""
    parent = list(range(n_verts))
    def find(a):
        while parent[a] != a: parent[a] = parent[parent[a]]; a = parent[a]
        return a
    for fc in faces:
        r = find(fc[0])
        for i in fc[1:]:
            ri = find(i)
            if ri != r: parent[ri] = r
    groups = {}
    for fi, fc in enumerate(faces): groups.setdefault(find(fc[0]), []).append(fi)
    return list(groups.values())

def _long_axis(pts):
    """The direction a piece is longest in (power iteration on its points' covariance)."""
    n = len(pts); c = [sum(p[k] for p in pts)/n for k in range(3)]
    C = [[sum((p[i] - c[i])*(p[j] - c[j]) for p in pts) for j in range(3)] for i in range(3)]
    v = [0.577, 0.577, 0.578]
    for _ in range(30):
        w = [sum(C[i][j]*v[j] for j in range(3)) for i in range(3)]; L = math.sqrt(sum(x*x for x in w)) or 1.0
        v = [x/L for x in w]
    return v, c

def _grain_uvs(verts, faces, seed=0.0):
    """Per-corner UVs in metres for timber, and a tone per point: u runs along each piece's length (the grain), v across the
    face; each piece gets its own random offset into the texture and its own tone, so no two posts look the same."""
    from . import geo
    uv = [None]*len(faces); tone = [0.5]*len(verts); edge = [None]*len(faces)
    for comp in _components(len(verts), faces):
        ids = sorted({i for fi in comp for i in faces[fi]}); pts = [verts[i] for i in ids]
        ax, c = _long_axis(pts)
        h = geo.hash01(c[0], c[1], c[2], seed); u0, v0 = 7.3*h, 5.1*geo.hash01(c[2], c[0], c[1], seed + 1)
        t = geo.hash01(c[1], c[2], c[0], seed + 2)
        for i in ids: tone[i] = t
        for fi in comp:
            fc = faces[fi]; P = [verts[i] for i in fc]
            nx = ny = nz = 0.0                                                                # the face's normal (Newell)
            for a, b in zip(P, P[1:] + P[:1]):
                nx += (a[1] - b[1])*(a[2] + b[2]); ny += (a[2] - b[2])*(a[0] + b[0]); nz += (a[0] - b[0])*(a[1] + b[1])
            acr = geo._cross((nx, ny, nz), ax)
            end = math.sqrt(sum(x*x for x in acr)) < 1e-6*max(1e-9, math.sqrt(nx*nx + ny*ny + nz*nz))   # end grain
            if end: acr = geo._cross(ax, (0.0, 0.0, 1.0)) if abs(ax[2]) < 0.9 else geo._cross(ax, (1.0, 0.0, 0.0))
            acr = geo._norm(acr)
            vs = [sum(p[k]*acr[k] for k in range(3)) for p in P]
            uv[fi] = [(sum(p[k]*ax[k] for k in range(3)) + u0, v + v0) for p, v in zip(P, vs)]
            lo, hi = min(vs), max(vs); wdt = hi - lo                                            # where the face's long edges are:
            edge[fi] = [(0.5, 10.0)]*len(P) if end or wdt < 1e-6 else [((v - lo)/wdt, wdt) for v in vs]   # (0..1 across, width)
    return uv, tone, edge

class Parts:
    """Collects pieces (vertex/face lists), each with a material, and makes one mesh object of them. A piece may carry a value
    per point (wear), written as the float attribute 'wear' that materials can read. Pieces in a timber material (GRAIN_MATS)
    also get UVs along their length and a 'tone' per piece (see _grain_uvs)."""
    def __init__(self):
        self.v, self.f, self.mi, self.mats, self.w = [], [], [], [], []
        self.uv, self.tone, self.edge = [], [], []

    def add(self, vf, mat, wear=None, grain=None):
        verts, faces = vf
        if mat not in self.mats: self.mats.append(mat)
        k = self.mats.index(mat); o = len(self.v)
        self.v += [tuple(p) for p in verts]
        self.f += [tuple(i + o for i in fc) for fc in faces]
        self.mi += [k]*len(faces)
        self.w += list(wear) if wear is not None else [0.0]*len(verts)
        if grain if grain is not None else mat.name in GRAIN_MATS:
            uv, tone, edge = _grain_uvs(verts, faces, seed=len(self.f))
            self.uv += uv; self.tone += tone; self.edge += edge
        else:
            self.uv += [[(0.0, 0.0)]*len(fc) for fc in faces]; self.tone += [0.5]*len(verts)
            self.edge += [[(0.5, 10.0)]*len(fc) for fc in faces]
        return self

    def build(self, name, col, loc=(0.0, 0.0, 0.0), rot_z=0.0, smooth=False, recalc=True, weld=0.0, solidify=0.0, sharp=None):
        """The object. recalc: make each closed piece face outward. weld: merge points closer than this.
        solidify: give an open shell (a roof) this thickness, inward. sharp: with smooth, keep edges sharper than this many
        degrees hard (chipped stone: soft rounded edges, crisp chip facets)."""
        me = bpy.data.meshes.new(name)
        me.from_pydata(self.v, [], self.f)
        me.polygons.foreach_set("material_index", self.mi)
        if any(self.w):
            at = me.attributes.new("wear", "FLOAT", "POINT"); at.data.foreach_set("value", self.w)
        if any(u != (0.0, 0.0) for fu in self.uv for u in fu):                              # timber UVs and tones
            me.uv_layers.new(name="UVMap")
            me.attributes["UVMap"].data.foreach_set("vector", [c for fu in self.uv for u in fu for c in u])
            me.uv_layers.new(name="EdgeUV")                                                  # x: 0..1 across the face, y: its width
            me.attributes["EdgeUV"].data.foreach_set("vector", [c for fe in self.edge for e in fe for c in e])
            at = me.attributes.new("tone", "FLOAT", "POINT"); at.data.foreach_set("value", self.tone)
        if recalc or weld:
            bm = bmesh.new(); bm.from_mesh(me)
            if weld: bmesh.ops.remove_doubles(bm, verts=bm.verts, dist=weld)
            if recalc: bmesh.ops.recalc_face_normals(bm, faces=bm.faces)
            bm.to_mesh(me); bm.free()
        me.validate(); me.update()
        for m in self.mats: me.materials.append(m)
        if smooth:
            me.shade_smooth()
            if sharp: me.set_sharp_from_angle(angle=math.radians(sharp))
        ob = bpy.data.objects.new(name, me); col.objects.link(ob)
        ob.location = loc; ob.rotation_euler.z = rot_z
        if solidify:
            md = ob.modifiers.new("Thickness", "SOLIDIFY"); md.thickness = solidify; md.offset = -1.0; md.use_even_offset = True
        return ob

def obj(name, vf, col, mat, **kw):
    return Parts().add(vf, mat).build(name, col, **kw)

def instance(src, name, col, loc, rot_z=0.0, scale=1.0):
    """A linked copy of src (same mesh data, so many trees cost one mesh)."""
    ob = bpy.data.objects.new(name, src.data); col.objects.link(ob)
    ob.location = loc; ob.rotation_euler.z = rot_z; ob.scale = (scale, scale, scale)
    return ob

def link_collection(path, name):
    """Link collection `name` from another .blend (read-only here: rebuild that file and this one follows on reload). Call it after
    this file is saved, so the path is stored relative."""
    if not os.path.exists(path): raise FileNotFoundError(f"{path} is missing: build it first (scripts\\run.ps1 building ...)")
    with bpy.data.libraries.load(bpy.path.relpath(path), link=True, relative=True) as (src, dst):
        if name not in src.collections: raise KeyError(f"no collection {name} in {path}")
        dst.collections = [name]
    return dst.collections[0]

def link_collections(path, names):
    """Link several collections from one .blend at once (as link_collection). Returns {name: collection}."""
    if not os.path.exists(path): raise FileNotFoundError(f"{path} is missing: build it first (scripts\\run.ps1 object ...)")
    with bpy.data.libraries.load(bpy.path.relpath(path), link=True, relative=True) as (src, dst):
        missing = [n for n in names if n not in src.collections]
        if missing: raise KeyError(f"no collection {', '.join(missing)} in {path}: rebuild it")
        dst.collections = list(names)
    return dict(zip(names, dst.collections))

def place_collection(coll_, name, col, loc, rot_z=0.0):
    """An empty that shows collection coll_ at loc (a collection instance)."""
    ob = bpy.data.objects.new(name, None); col.objects.link(ob)
    ob.instance_type = "COLLECTION"; ob.instance_collection = coll_
    ob.location = loc; ob.rotation_euler.z = rot_z
    return ob

def aim(ob, direction):
    """Point an object's -Z (a light or camera) along `direction`."""
    ob.rotation_euler = Vector(direction).to_track_quat("-Z", "Y").to_euler()

def sun(name, col, elev, az, energy, rgb, angle=1.5):
    """A sun `elev` degrees above the horizon in compass direction `az` (0 north, 90 east)."""
    li = bpy.data.lights.new(name, "SUN"); li.energy = energy; li.color = rgb; li.angle = math.radians(angle)
    ob = bpy.data.objects.new(name, li); col.objects.link(ob)
    e, a = math.radians(elev), math.radians(az)
    aim(ob, (-math.sin(a)*math.cos(e), -math.cos(a)*math.cos(e), -math.sin(e)))
    return ob

def sky(name, rgb, strength):
    """A plain sky colour as the world (the light states replace it with HDRIs later)."""
    w = bpy.data.worlds.get(name) or bpy.data.worlds.new(name)
    try: w.use_nodes = True
    except Exception: pass
    bg = next(n for n in w.node_tree.nodes if n.type == "BACKGROUND")
    bg.inputs["Color"].default_value = (*rgb, 1.0); bg.inputs["Strength"].default_value = strength
    bpy.context.scene.world = w
    return w

def eevee(scene):
    """Use EEVEE (whatever its identifier is in this Blender) with the AgX view."""
    ids = [i.identifier for i in bpy.types.RenderSettings.bl_rna.properties["engine"].enum_items]
    scene.render.engine = next(i for i in ids if "EEVEE" in i)
    try: scene.view_settings.view_transform = "AgX"
    except TypeError: pass
