"""Poly Haven textures on the palette materials (common/polyhaven.py lists them). The meshes have no UV maps (but timber and
thatch, see mesh.Parts), so every texture is box-projected in object space and gets its relief from the height map through a
Bump node (a normal map would need UV tangents). The user's own photo materials (the thatch, the spiky ground, the soil) come after.
Each texture is mixed a little with its palette colour, so the stylized palette keeps deciding the mood.
A material whose files aren't downloaded keeps its plain palette colour (run scripts\\run.ps1 fetch).

The texture library (the user, 2026-10-20: "lets focus on reusable texture place in blender/textures"): every material is built
once, by family, into blender/textures/<family>.blend (scripts/textures/build.py, run.ps1 texture <family>|all), marked as an
asset in a catalog of its family; every other file links its materials from there (mesh.mats -> link_all), so a texture changed
once changes everywhere. Each family's summary is beside its file (blender/textures/<family>.md)."""
import bpy, math, os, uuid
from . import mesh, paths, polyhaven as PH

# The families, one file each (the user chose "by family"). Every palette material is in one of them.
FAMILIES = {
    "wood":    ["teak", "teak_dark", "teak_turned", "firewood"],
    "stone":   ["stone", "stone_kit", "stone_dark"],
    "plants":  ["bark", "bark_dark", "thatch", "woven"],
    "ground":  ["ground", "earth", "water", "ground_spiky", "ground_soil"],
    "metal":   ["bronze"],
    "palette": ["ridge", "bamboo", "bamboo_culm", "door", "clay", "clay_dark", "mat", "soot", "oil", "wick", "mountain",
                "leaf", "leaf_dark", "leaf_light", "dry_leaf", "banana_stem", "grass", "grass_dark", "grass_dry", "plume",
                "reed_plume", "lotus", "lotus_light", "lotus_pad", "flower", "flower_center", "figure"],
}
CATALOG_NS = uuid.UUID("6f1c2b1e-7a1d-4c55-9b3e-2a7d0c9e5b11")       # catalog ids are derived from this, so they never change

def catalog_id(family):
    return str(uuid.uuid5(CATALOG_NS, "bargawa/" + family))

def write_catalogs():
    """blender/textures/blender_assets.cats.txt: a catalog per family ("Bargawa/Wood" ...), for the Asset Browser."""
    os.makedirs(paths.TEXTURES, exist_ok=True)
    lines = ["# This is an Asset Catalog Definition file for Blender.", "#",
             "# Empty lines and lines starting with `#` will be ignored.",
             "# The first non-ignored line should be the version indicator.",
             '# Other lines are of the format "UUID:catalog/path/for/assets:simple catalog name"', "", "VERSION 1", ""]
    lines += [f"{catalog_id(f)}:Bargawa/{f.capitalize()}:Bargawa-{f.capitalize()}" for f in FAMILIES]
    with open(os.path.join(paths.TEXTURES, "blender_assets.cats.txt"), "w", encoding="utf-8", newline="\n") as fh:
        fh.write("\n".join(lines) + "\n")

def _describe(key, family):
    tex = PH.TEXTURES.get(key)
    if tex:
        asset, tile, mix, bump = tex
        d = (f"Poly Haven {asset} (CC0), {tile} m tiles, mixed with {round(mix*100)}% of its palette colour, bump {bump}. "
             + ("Textured along each piece's grain (UVs from mesh.Parts). " if key in PH.UV_KEYS else "Box-projected in object space. "))
        if key in PH.UV_KEYS and key not in PH.CARED_KEYS: d += "With the wood reference's weathering layers. "
        if key == "stone_kit": d += "Worn edges lighter where the mesh's 'wear' attribute is high. "
        return d.strip()
    if key in METALS: return "Aged metal: patina in the hollows and patches, polished where the mesh's 'wear' attribute is high."
    if key == "ground_spiky":
        return ("The user's photo (blender/assets/photos/ground_spiky.jpg) on the mesh's UVMap, mirrored tiles, matte. Its spikes "
                "are the Geometry Nodes group GN_ground_spiky in this file: put it on a fine grid as a modifier.")
    if key == "ground_soil":
        return ("The user's soil photo (blender/assets/photos/1cb0165c04eb35eb7bd78df3f83cc534.jpg, seamless) on the mesh's UVMap, "
                "5.6 m tiles, matte, bump 1.0 from the photo. Its relief (only the brightest leaves and pebbles rise) is the "
                "Geometry Nodes group GN_ground_soil in this file: put it on a fine grid as a modifier.")
    if key == "thatch":
        return ("The user's sasak photo (blender/assets/photos/sasak.jpg) on the mesh's UVMap in metres (tiles 0.686 x 1.2 m), the "
                "straw down each slope (mesh._slope_uvs), matte, bump 1.0. Its relief is the Geometry Nodes group GN_thatch in "
                "this file, after a Solidify that marks the inner shell 'thatch_inner' (textures.thatch_relief).")
    return "A plain palette colour."

def build_family(family):
    """Make the family's materials in the open file (a texture library file) and mark them as assets. Returns {key: material}."""
    from . import terrain
    M = {}
    for k in FAMILIES[family]:
        if k == "woven": M[k] = mesh.woven_material()
        elif k == "ground": M[k] = terrain.ground_material()                # textured by itself (terrain.ground_material)
        elif k == "water": M[k] = terrain.water_material()
        elif k in terrain.RELIEF_GROUNDS: M[k] = terrain.relief_ground_material(k)   # with its Geometry Nodes, GN_<key>
        elif k == "thatch": M[k] = thatch_material()                       # with GN_thatch
        else:
            rgb, rough = mesh.PALETTE[k]; M[k] = mesh.material("M_" + k, rgb, rough)
    apply(M)
    for k, m in M.items():
        m.asset_mark()
        m.asset_data.catalog_id = catalog_id(family)
        m.asset_data.description = _describe(k, family)
        m.asset_data.author = "project/bargawa/scripts (common/textures.py)"
        for tag in (family, PH.TEXTURES[k][0] if k in PH.TEXTURES else "photo" if k in ("ground_spiky", "ground_soil", "thatch") else "palette"):
            m.asset_data.tags.new(tag, skip_if_exists=True)
    return M

def link_all():
    """Every library material, linked from blender/textures/<family>.blend (read-only in the file that links it: change this
    module and rebuild the family). Returns {key: material}. Call it after the file is saved, so the paths are relative."""
    M = {}
    for family, keys in FAMILIES.items():
        path = paths.texture_file(family, built=True)
        if not os.path.exists(path):
            raise FileNotFoundError(f"{path} is missing: build the texture library first (scripts\\run.ps1 texture all)")
        names = ["M_" + k for k in keys]
        with bpy.data.libraries.load(bpy.path.relpath(path), link=True, relative=True) as (src, dst):
            missing = [n for n in names if n not in src.materials]
            if missing: raise KeyError(f"no material {', '.join(missing)} in {path}: rebuild it (run.ps1 texture {family})")
            dst.materials = names
        M.update(zip(keys, dst.materials))
    return M

# --- the user's photo materials, with real relief --------------------------------------------------------------------------
# Made by hand in blender/textures/material_manual.blend (the user, 2026-10-20): a photo as Base Color, and its brightness
# through a Color Ramp as displacement. Here the relief is a Geometry Nodes group (relief_nodes) on a fine mesh rather than shader
# displacement, so it is real geometry in EEVEE, Cycles and the game's export. Realistic shading values (the user chose them).
# The bare grounds (terrain.RELIEF_GROUNDS: the spiky ground, the soil) and the thatch.
THATCH = dict(photo="sasak.jpg", tile=(0.686, 1.2),          # the photo's own shape (384 x 672 px), 7 courses of about 17 cm
              ramp=(0.0, 1.0), height=0.1, mid=0.0,          # the user's sasak material: RGB to BW, ramp 0 -> 1, Scale 0.1
              detail=3,                                      # the roofs' faces are about 0.3 m (ROOF_STEP): 3.75 cm after it
              rough=0.85, bump=1.0, bump_dist=0.2)           # the user's bump: Strength 1, Distance 0.8 on the photo 4.5 m tall,
                                                             # so 0.2 at this tile

def photo(name):
    """A photo in blender/assets/photos, loaded once."""
    return bpy.data.images.load(os.path.join(paths.PHOTOS, name), check_existing=True)

def photo_material(name, img, tile=(1.0, 1.0), extension="REPEAT", rough=0.85, bump=0.3, bump_dist=0.02, ramp=None, fade=None,
                   depth=None):
    """A matte material of one photo on the mesh's UVMap (scaled by 1/tile, metres -> tiles). Its bump comes from the photo, or
    from the photo through `ramp` (black, white positions) when the relief uses one. With `fade` (an attribute name) the material
    turns see-through (dithered) where that attribute is high. With depth (attribute, metres) V drops by attribute x metres, so
    the photo runs on down a thickened shell's cut edge."""
    m = bpy.data.materials.new(name); p = mesh._principled(m); K = _kit(m.node_tree)
    uv = K.N("ShaderNodeUVMap"); uv.uv_map = "UVMap"; vec = uv.outputs["UV"]
    if depth:
        a = K.N("ShaderNodeAttribute"); a.attribute_name = depth[0]                       # missing: 0, no change
        cmb = K.N("ShaderNodeCombineXYZ"); K.L(K.math("MULTIPLY", a.outputs["Fac"], depth[1]), cmb.inputs["Y"])
        vm = K.N("ShaderNodeVectorMath"); vm.operation = "SUBTRACT"; K.L(vec, vm.inputs[0]); K.L(cmb.outputs[0], vm.inputs[1])
        vec = vm.outputs["Vector"]
    if tuple(tile) != (1.0, 1.0): vec = K.stretch(vec, 1.0/tile[0], 1.0/tile[1], 1.0)
    im = K.N("ShaderNodeTexImage"); im.image = img; im.extension = extension; K.L(vec, im.inputs["Vector"])
    K.L(im.outputs["Color"], p.inputs["Base Color"])
    p.inputs["Roughness"].default_value = rough; p.inputs["Metallic"].default_value = 0.0; p.inputs["IOR"].default_value = 1.5
    height = K.ramp(im.outputs["Color"], [(ramp[0], (0.0, 0.0, 0.0)), (ramp[1], (1.0, 1.0, 1.0))]).outputs["Color"] if ramp else im.outputs["Color"]
    b = K.N("ShaderNodeBump"); b.inputs["Strength"].default_value = bump; b.inputs["Distance"].default_value = bump_dist
    K.L(height, b.inputs["Height"]); K.L(b.outputs["Normal"], p.inputs["Normal"])
    if fade:
        at = K.N("ShaderNodeAttribute"); at.attribute_name = fade                           # missing on other meshes: 0, opaque
        K.L(K.math("SUBTRACT", 1.0, at.outputs["Fac"]), p.inputs["Alpha"])
        m.surface_render_method = "DITHERED"
    return m

def relief_nodes(name, img, ramp, height, mid, detail, tile=(1.0, 1.0), extension="REPEAT", weight=None, flat=None):
    """A Geometry Nodes group of the user's node chain: the mesh subdivided `Detail` times, then every point raised along its
    normal by (Color Ramp(photo at the UVMap / tile) - Midlevel) x Height x a weight. The weight is the point attribute `weight`
    (a mesh without it gets 1), or 1 - the attribute `flat`; the faces wholly in `flat` (a roof's inner shell) are neither
    subdivided nor raised. Inputs on the modifier: Height, Midlevel, Detail."""
    g = bpy.data.node_groups.get(name)
    if g: return g
    g = bpy.data.node_groups.new(name, "GeometryNodeTree"); g.use_fake_user = True
    I = g.interface
    I.new_socket("Geometry", in_out="INPUT", socket_type="NodeSocketGeometry")
    I.new_socket("Geometry", in_out="OUTPUT", socket_type="NodeSocketGeometry")
    for nm, typ, val, hi, desc in (("Height", "NodeSocketFloat", height, 1.0, "How high the brightest points rise (m)"),
                                   ("Midlevel", "NodeSocketFloat", mid, 1.0, "The ramp's value that stays flat"),
                                   ("Detail", "NodeSocketInt", detail, 5, "Times the faces are halved before the relief")):
        s = I.new_socket(nm, in_out="INPUT", socket_type=typ); s.default_value = val; s.min_value = 0; s.max_value = hi
        s.description = desc
    N, L = g.nodes.new, g.links.new
    def at(n, x, y): n.location = (x, y); return n
    def attr(nm, x, y):
        a = at(N("GeometryNodeInputNamedAttribute"), x, y); a.data_type = "FLOAT"; a.inputs["Name"].default_value = nm; return a
    def math(op, a, b, x, y):
        n = at(N("ShaderNodeMath"), x, y); n.operation = op
        for i, v in enumerate((a, b)):
            if isinstance(v, float): n.inputs[i].default_value = v
            else: L(v, n.inputs[i])
        return n.outputs[0]
    gi = at(N("NodeGroupInput"), -1100, 0); go = at(N("NodeGroupOutput"), 700, 0)
    geo_in, keep = gi.outputs["Geometry"], None
    if flat:                                                                           # the inner shell passes untouched
        sep = at(N("GeometryNodeSeparateGeometry"), -850, 250); sep.domain = "FACE"
        cmp = at(N("FunctionNodeCompare"), -1100, 350); cmp.data_type = "FLOAT"; cmp.operation = "GREATER_THAN"
        L(attr(flat, -1300, 350).outputs["Attribute"], cmp.inputs["A"]); cmp.inputs["B"].default_value = 0.99
        L(geo_in, sep.inputs["Geometry"]); L(cmp.outputs["Result"], sep.inputs["Selection"])
        geo_in, keep = sep.outputs["Inverted"], sep.outputs["Selection"]
    sub = at(N("GeometryNodeSubdivideMesh"), -600, 200); L(geo_in, sub.inputs["Mesh"]); L(gi.outputs["Detail"], sub.inputs["Level"])
    uv = at(N("GeometryNodeInputNamedAttribute"), -1100, -250); uv.data_type = "FLOAT_VECTOR"; uv.inputs["Name"].default_value = "UVMap"
    vec = uv.outputs["Attribute"]
    if tuple(tile) != (1.0, 1.0):
        sc = at(N("ShaderNodeVectorMath"), -880, -250); sc.operation = "MULTIPLY"
        L(vec, sc.inputs[0]); sc.inputs[1].default_value = (1.0/tile[0], 1.0/tile[1], 1.0); vec = sc.outputs["Vector"]
    tex = at(N("GeometryNodeImageTexture"), -650, -250); tex.inputs["Image"].default_value = img; tex.extension = extension
    L(vec, tex.inputs["Vector"])
    rp = at(N("ShaderNodeValToRGB"), -400, -250); el = rp.color_ramp.elements
    for e, pos, c in zip(el, ramp, (0.0, 1.0)): e.position = pos; e.color = (c, c, c, 1.0)
    L(tex.outputs["Color"], rp.inputs["Fac"])
    h = math("SUBTRACT", rp.outputs["Color"], gi.outputs["Midlevel"], -120, -250)
    h = math("MULTIPLY", h, gi.outputs["Height"], 60, -250)
    if flat: w = math("SUBTRACT", 1.0, attr(flat, -400, -550).outputs["Attribute"], -120, -500)
    elif weight:
        a = attr(weight, -400, -550)
        sw = at(N("GeometryNodeSwitch"), -120, -500); sw.input_type = "FLOAT"
        L(a.outputs["Exists"], sw.inputs["Switch"]); sw.inputs["False"].default_value = 1.0; L(a.outputs["Attribute"], sw.inputs["True"])
        w = sw.outputs[0]
    else: w = None
    if w is not None: h = math("MULTIPLY", h, w, 60, -450)
    nrm = at(N("GeometryNodeInputNormal"), 60, -650)
    off = at(N("ShaderNodeVectorMath"), 240, -450); off.operation = "SCALE"; L(nrm.outputs["Normal"], off.inputs[0]); L(h, off.inputs["Scale"])
    sp = at(N("GeometryNodeSetPosition"), 260, 150); L(sub.outputs["Mesh"], sp.inputs["Geometry"]); L(off.outputs["Vector"], sp.inputs["Offset"])
    out = sp.outputs["Geometry"]
    if keep is not None:
        j = at(N("GeometryNodeJoinGeometry"), 480, 100); L(keep, j.inputs[0]); L(out, j.inputs[0]); out = j.outputs[0]
        # `flat` + '_depth' for the material (a shader can't read vertex groups): 0 outside, 1 on the inner shell, in between down
        # the cut edge (the rim), whose UVs the Solidify only copies, so the material moves them by it
        st = at(N("GeometryNodeStoreNamedAttribute"), 600, 100); st.data_type = "FLOAT"; st.domain = "POINT"
        st.inputs["Name"].default_value = flat + "_depth"; L(out, st.inputs["Geometry"])
        L(attr(flat, 400, -100).outputs["Attribute"], st.inputs["Value"]); out = st.outputs["Geometry"]; go.location = (820, 0)
    L(out, go.inputs["Geometry"])
    return g

def thatch_material():
    """M_thatch: the user's sasak photo (blender/assets/photos/sasak.jpg), the straw down each roof slope (the UVs from
    mesh._slope_uvs), with its relief GN_thatch."""
    T = THATCH; img = photo(T["photo"])
    m = photo_material("M_thatch", img, T["tile"], "REPEAT", T["rough"], T["bump"], T["bump_dist"],
                       depth=("thatch_inner_depth", 0.25))                         # the roofs are 0.18-0.3 m thick
    m.diffuse_color = (*mesh.PALETTE["thatch"][0], 1.0)
    relief_nodes("GN_thatch", img, T["ramp"], T["height"], T["mid"], T["detail"], T["tile"], "REPEAT", flat="thatch_inner")
    return m

def thatch_relief(ob, nodes=None):
    """Real relief on a thatch roof, a shell thickened by Parts.build's Solidify: the Solidify marks its new inner shell
    'thatch_inner', and GN_thatch after it subdivides and raises only the outside. nodes: the group (linked from plants.blend
    unless given)."""
    ob.vertex_groups.new(name="thatch_inner")
    sd = next(md for md in ob.modifiers if md.type == "SOLIDIFY"); sd.shell_vertex_group = "thatch_inner"
    md = ob.modifiers.new("Relief", "NODES"); md.node_group = nodes or link_group("plants", "GN_thatch")
    return ob

def thatch_sample(col, m, x):
    """A 1.5 m piece of roof at the usual pitch, its eave at the front, thickened, with its relief: the library's sample."""
    k, t = 6, math.tan(mesh.SLOPE_PITCH)
    pts = [(x - 0.75 + 1.5*i/(k - 1), -0.6 + 1.2*j/(k - 1), 0.25 + 1.2*j/(k - 1)*t) for j in range(k) for i in range(k)]
    fs = [(j*k + i, j*k + i + 1, (j + 1)*k + i + 1, (j + 1)*k + i) for j in range(k - 1) for i in range(k - 1)]
    ob = mesh.Parts().add((pts, fs), m).build("Sample_thatch", col, recalc=False, solidify=0.24)
    return thatch_relief(ob, bpy.data.node_groups["GN_thatch"])

def link_group(family, name):
    """A node group of a library file (a bare ground's GN_ground_soil), linked like the materials."""
    path = paths.texture_file(family, built=True)
    with bpy.data.libraries.load(bpy.path.relpath(path), link=True, relative=True) as (src, dst):
        if name not in src.node_groups: raise KeyError(f"no node group {name} in {path}: rebuild it (run.ps1 texture {family})")
        dst.node_groups = [name]
    return dst.node_groups[0]

def textured(m, key, base_rgb):
    """Rebuild material m's nodes with the texture for `key`. Returns False (and leaves m alone) when its files are missing."""
    asset, tile, mix, bump = PH.TEXTURES[key]
    diff, rough, disp = (PH.file(asset, k) for k in PH.MAPS)
    if not diff: return False
    nt = m.node_tree; p = mesh._principled(m); use_uv = key in PH.UV_KEYS
    for n in [n for n in nt.nodes if n.type not in ("BSDF_PRINCIPLED", "OUTPUT_MATERIAL")]: nt.nodes.remove(n)
    tc = nt.nodes.new("ShaderNodeTexCoord"); mp = nt.nodes.new("ShaderNodeMapping")
    mp.inputs["Scale"].default_value = (1.0/tile, 1.0/tile, 1.0/tile)
    nt.links.new(tc.outputs["UV" if use_uv else "Object"], mp.inputs["Vector"])
    def image(path, color):
        n = nt.nodes.new("ShaderNodeTexImage"); n.image = bpy.data.images.load(path, check_existing=True)
        n.image.colorspace_settings.name = "sRGB" if color else "Non-Color"
        if use_uv: n.projection = "FLAT"                                   # the UVs follow each piece's grain
        else: n.projection = "BOX"; n.projection_blend = 0.3
        nt.links.new(mp.outputs["Vector"], n.inputs["Vector"]); return n
    d = image(diff, True)
    mixn = nt.nodes.new("ShaderNodeMix"); mixn.data_type = "RGBA"; mixn.blend_type = "MIX"
    sock = lambda socks, name, typ: next(s for s in socks if s.name == name and s.type == typ)   # Mix has a socket per data type
    sock(mixn.inputs, "Factor", "VALUE").default_value = mix
    nt.links.new(d.outputs["Color"], sock(mixn.inputs, "A", "RGBA")); sock(mixn.inputs, "B", "RGBA").default_value = (*base_rgb, 1.0)
    out = sock(mixn.outputs, "Result", "RGBA")
    if use_uv:                                                             # each piece a little lighter or darker ('tone')
        at = nt.nodes.new("ShaderNodeAttribute"); at.attribute_name = "tone"
        ma = nt.nodes.new("ShaderNodeMath"); ma.operation = "MULTIPLY_ADD"
        ma.inputs[1].default_value = 0.36; ma.inputs[2].default_value = 0.82                  # value 0.82 .. 1.18
        hsv = nt.nodes.new("ShaderNodeHueSaturation")
        nt.links.new(at.outputs["Fac"], ma.inputs[0]); nt.links.new(ma.outputs[0], hsv.inputs["Value"])
        nt.links.new(out, hsv.inputs["Color"]); out = hsv.outputs["Color"]
    rough_out = image(rough, False).outputs["Color"] if rough else None
    if use_uv and key not in PH.CARED_KEYS: out, rough_out = weathered_wood(nt, mp, out, rough_out)   # the wood reference's layers
    nt.links.new(out, p.inputs["Base Color"])
    if rough_out: nt.links.new(rough_out, p.inputs["Roughness"])
    if disp and bump > 0:
        b = nt.nodes.new("ShaderNodeBump"); b.inputs["Strength"].default_value = bump; b.inputs["Distance"].default_value = 0.02
        nt.links.new(image(disp, False).outputs["Color"], b.inputs["Height"]); nt.links.new(b.outputs["Normal"], p.inputs["Normal"])
    return True

def weathered_wood(nt, uv_map, color, rough):
    """The layer stack of the wood reference (project/bargawa/references/video-wood-plank-material-javad.md), rebuilt in nodes
    on top of the teak texture: gradient re-colour, fibres along the grain, lighter worn edges, dirt rising from the foot, damp
    streaks running down, a roughness pass, a final balance. color/rough are the sockets so far; returns the new ones.
    Needs the mesh's UVMap (along the grain), EdgeUV (x 0..1 across a face, y its width) and Object coordinates (z up)."""
    K = _kit(nt); N, L, mix, math, ramp, maprange, noise, stretch = K.N, K.L, K.mix, K.math, K.ramp, K.maprange, K.noise, K.stretch
    lin = mesh.lin; tc = N("ShaderNodeTexCoord"); obj = tc.outputs["Object"]
    # 1. gradient re-colour (the video's Gradient filter): the teak's light/dark through a ramp of teak browns
    bw = N("ShaderNodeRGBToBW"); L(color, bw.inputs[0])
    grad = ramp(bw.outputs[0], [(0.05, lin(52, 32, 20)), (0.4, lin(104, 72, 44)), (0.8, lin(150, 112, 72))])
    c = mix(color, grad.outputs["Color"], 0.55)
    # 2. fibres along the grain (Wood fibers): noise stretched along U, the piece's length
    fib = maprange(noise(stretch(uv_map.outputs["Vector"], 0.6, 40.0, 1.0), 6.0, 6.0), 0.48, 0.62)
    c = mix(c, mix(c, (0.62, 0.62, 0.62), 1.0, "MULTIPLY"), math("MULTIPLY", fib, 0.45))
    # 3. worn, lighter edges (Mask Builder / Edge damage): close to a face's long edges, broken up along the length
    eu = N("ShaderNodeUVMap"); eu.uv_map = "EdgeUV"; sep = N("ShaderNodeSeparateXYZ"); L(eu.outputs["UV"], sep.inputs[0])
    s = sep.outputs["X"]; d = math("MULTIPLY", math("MINIMUM", s, math("SUBTRACT", 1.0, s)), sep.outputs["Y"])
    brk = maprange(noise(stretch(uv_map.outputs["Vector"], 2.0, 0.5, 1.0), 4.0, 3.0), 0.35, 0.6)
    edge = math("MULTIPLY", maprange(d, 0.022, 0.004), brk)
    c = mix(c, lin(170, 136, 96), math("MULTIPLY", edge, 0.5))           # at 0.7 they read like painted lines
    # 4. dirt rising from the foot (Dirt, Grunge Spots): low on each post, blotchy
    dirt = math("MULTIPLY", maprange(_z(nt, obj), 2.0, 0.9), maprange(noise(obj, 3.5, 5.0), 0.35, 0.65))   # 0 above 2 m, full at 0.9 m
    c = mix(c, lin(44, 34, 26), math("MULTIPLY", dirt, 0.6))
    # 5. damp streaks running down (Moisture Noise, Grunge Map, streaks)
    streak = maprange(noise(stretch(obj, 9.0, 9.0, 0.7), 2.5, 4.0), 0.56, 0.7)
    c = mix(c, mix(c, (0.72, 0.7, 0.68), 1.0, "MULTIPLY"), math("MULTIPLY", streak, 0.5))
    # 6. final balance (Color Balance): a touch less saturated, warm
    hsv = N("ShaderNodeHueSaturation"); L(c, hsv.inputs["Color"]); hsv.inputs["Saturation"].default_value = 0.9
    # roughness pass: the texture's roughness lifted, rougher where worn and dirty
    r = maprange(rough, 0.0, 1.0, 0.5, 0.78) if rough is not None else None
    if r is not None: r = math("MINIMUM", math("ADD", r, math("ADD", math("MULTIPLY", edge, 0.15), math("MULTIPLY", dirt, 0.12))), 1.0)
    return hsv.outputs["Color"], r

def _kit(nt):
    """Small builders for shader nodes in node tree nt: mix (colours), math, ramp, maprange, noise, stretch (a scaled vector).
    Inputs may be sockets or plain numbers / rgb tuples."""
    from types import SimpleNamespace
    N = nt.nodes.new; L = nt.links.new
    sock = lambda socks, name, typ: next(s for s in socks if s.name == name and s.type == typ)
    def mix(a, b, fac, mode="MIX"):
        m = N("ShaderNodeMix"); m.data_type = "RGBA"; m.blend_type = mode
        f = sock(m.inputs, "Factor", "VALUE")
        if isinstance(fac, float): f.default_value = fac
        else: L(fac, f)
        for name, v in (("A", a), ("B", b)):
            s = sock(m.inputs, name, "RGBA")
            if isinstance(v, tuple): s.default_value = (*v, 1.0)
            else: L(v, s)
        return sock(m.outputs, "Result", "RGBA")
    def math(op, a, b=None, c=None):
        m = N("ShaderNodeMath"); m.operation = op
        for i, v in enumerate((a, b, c)):
            if v is None: continue
            if isinstance(v, float): m.inputs[i].default_value = v
            else: L(v, m.inputs[i])
        return m.outputs[0]
    def ramp(fac, stops):
        r = N("ShaderNodeValToRGB"); L(fac, r.inputs[0]); el = r.color_ramp.elements
        while len(el) < len(stops): el.new(0.5)
        for e, (pos, rgb) in zip(el, stops): e.position = pos; e.color = (*rgb, 1.0)
        return r
    def maprange(v, a, b, lo=0.0, hi=1.0):
        m = N("ShaderNodeMapRange"); m.interpolation_type = "SMOOTHSTEP"; L(v, sock(m.inputs, "Value", "VALUE"))
        for name, x in (("From Min", a), ("From Max", b), ("To Min", lo), ("To Max", hi)): sock(m.inputs, name, "VALUE").default_value = x
        return sock(m.outputs, "Result", "VALUE")
    def noise(vec, scale, detail=4.0):
        n = N("ShaderNodeTexNoise"); L(vec, n.inputs["Vector"]); n.inputs["Scale"].default_value = scale
        n.inputs["Detail"].default_value = detail; return n.outputs[0]
    def stretch(vec, sx, sy, sz):
        m = N("ShaderNodeMapping"); L(vec, m.inputs["Vector"]); m.inputs["Scale"].default_value = (sx, sy, sz); return m.outputs["Vector"]
    return SimpleNamespace(N=N, L=L, sock=sock, mix=mix, math=math, ramp=ramp, maprange=maprange, noise=noise, stretch=stretch)

METALS = {   # palette key: (bare metal, patina or tarnish, polished, how much the patches show, the bare metal's roughness)
    "bronze": (mesh.lin(148, 102, 54), mesh.lin(62, 66, 46), mesh.lin(218, 168, 98), 0.25, 0.3),   # the damar, the dipa
}

def bronze(m):
    """Old cast bronze (the damar): see aged_metal."""
    return aged_metal(m, *METALS["bronze"])

def aged_metal(m, metal, patina_rgb, polish, mottle_amt=0.25, rough=0.3):
    """Old metal (bronze, brass, tin): a patina or tarnish gathered in the hollows (Ambient Occlusion) and in mottled patches,
    bright polished metal where hands and cloths rub (the mesh's 'wear' attribute: rims, the socket's lip, the ascetic's knees
    and head). Patina is rougher and less metallic than the bare metal."""
    nt = m.node_tree; p = mesh._principled(m)
    for n in [n for n in nt.nodes if n.type not in ("BSDF_PRINCIPLED", "OUTPUT_MATERIAL")]: nt.nodes.remove(n)
    K = _kit(nt); N, L, mix, math, maprange, noise = K.N, K.L, K.mix, K.math, K.maprange, K.noise
    lin = mesh.lin; obj = N("ShaderNodeTexCoord").outputs["Object"]
    wear = N("ShaderNodeAttribute"); wear.attribute_name = "wear"; wear = wear.outputs["Fac"]
    ao = N("ShaderNodeAmbientOcclusion"); ao.inputs["Distance"].default_value = 0.012; ao.only_local = True
    hollow = maprange(ao.outputs["AO"], 0.95, 0.45)                                           # 1 deep in the hollows
    mottle = maprange(noise(obj, 70.0, 6.0), 0.42, 0.68)                                      # patches over the surface
    patina = math("MULTIPLY", math("MINIMUM", math("ADD", hollow, math("MULTIPLY", mottle, mottle_amt)), 1.0),
                  math("SUBTRACT", 1.0, wear))
    c = mix(metal, patina_rgb, patina)                                                        # bare metal -> patina
    c = mix(c, polish, math("MULTIPLY", wear, 0.75))                                          # polished where rubbed
    L(c, p.inputs["Base Color"])
    L(math("SUBTRACT", math("ADD", rough, math("MULTIPLY", patina, 0.42)), math("MULTIPLY", wear, 0.12)), p.inputs["Roughness"])
    L(math("SUBTRACT", 1.0, math("MULTIPLY", patina, 0.65)), p.inputs["Metallic"])
    return m

def glass(m, tint=(0.92, 0.95, 0.9), soot=None):
    """Thin blown glass (lamp chimneys, a lantern's globe): mostly see-through, reflecting more at grazing angles (Fresnel), the
    same in EEVEE and Cycles without refraction. With soot (an rgb), the 'wear' attribute darkens it (a chimney's smoky top)."""
    nt = m.node_tree
    for n in list(nt.nodes): nt.nodes.remove(n)
    K = _kit(nt); N, L, math = K.N, K.L, K.math
    tr = N("ShaderNodeBsdfTransparent"); tr.inputs["Color"].default_value = (*tint, 1.0)
    gl = N("ShaderNodeBsdfPrincipled"); gl.inputs["Base Color"].default_value = (*tint, 1.0)
    gl.inputs["Roughness"].default_value = 0.04; gl.inputs["Metallic"].default_value = 0.0
    fr = N("ShaderNodeFresnel"); fr.inputs["IOR"].default_value = 1.5
    fac = math("ADD", 0.06, math("MULTIPLY", fr.outputs[0], 0.8))
    if soot:
        wear = N("ShaderNodeAttribute"); wear.attribute_name = "wear"
        gl.inputs["Base Color"].default_value = (*soot, 1.0); gl.inputs["Roughness"].default_value = 0.6
        fac = math("MAXIMUM", fac, math("MULTIPLY", wear.outputs["Fac"], 0.85))
    mx = N("ShaderNodeMixShader"); L(fac, mx.inputs[0]); L(tr.outputs[0], mx.inputs[1]); L(gl.outputs[0], mx.inputs[2])
    L(mx.outputs[0], N("ShaderNodeOutputMaterial").inputs["Surface"])
    m.surface_render_method = "DITHERED"; m.diffuse_color = (*tint, 0.3)
    return m

def _z(nt, vec):
    sep = nt.nodes.new("ShaderNodeSeparateXYZ"); nt.links.new(vec, sep.inputs[0]); return sep.outputs["Z"]

def add_wear(m, rgb, amount):
    """Lighten the base colour where the mesh's 'wear' attribute is high (rounded edges, chip facets): the worn-edge highlight
    of hand-textured stone. Works on a textured or a plain material."""
    nt = m.node_tree; p = mesh._principled(m); base = p.inputs["Base Color"]
    src = base.links[0].from_socket if base.links else None
    attr = nt.nodes.new("ShaderNodeAttribute"); attr.attribute_name = "wear"
    mul = nt.nodes.new("ShaderNodeMath"); mul.operation = "MULTIPLY"; mul.inputs[1].default_value = amount
    nt.links.new(attr.outputs["Fac"], mul.inputs[0])
    mx = nt.nodes.new("ShaderNodeMix"); mx.data_type = "RGBA"; mx.blend_type = "MIX"
    sock = lambda socks, name, typ: next(s for s in socks if s.name == name and s.type == typ)
    nt.links.new(mul.outputs[0], sock(mx.inputs, "Factor", "VALUE"))
    if src: nt.links.new(src, sock(mx.inputs, "A", "RGBA"))
    else: sock(mx.inputs, "A", "RGBA").default_value = tuple(base.default_value)
    sock(mx.inputs, "B", "RGBA").default_value = (*rgb, 1.0)
    nt.links.new(sock(mx.outputs, "Result", "RGBA"), base)

def apply(M):
    """Texture every palette material listed in polyhaven.TEXTURES (except the ground, see ground_material)."""
    done = []
    for key in PH.TEXTURES:
        if key == "ground" or key not in M: continue
        base = mesh.PALETTE[key][0] if key in mesh.PALETTE else (0.4, 0.33, 0.22)
        if textured(M[key], key, base): done.append(key)
    if "stone_kit" in M: add_wear(M["stone_kit"], mesh.lin(206, 202, 192), 0.45)        # at 0.6 every edge looked chalk-white
    for key, args in METALS.items():
        if key in M: aged_metal(M[key], *args)
    if "glass" in M: glass(M["glass"])
    wanted = [k for k in PH.TEXTURES if k != "ground" and k in M]
    print("STEP textures:", ", ".join(done) or ("none (run.ps1 fetch)" if wanted else "none in this set"), flush=True)
    return M
