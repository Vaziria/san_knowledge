"""Poly Haven textures on the palette materials (common/polyhaven.py lists them). The meshes have no UV maps, so every texture is
box-projected in object space and gets its relief from the height map through a Bump node (a normal map would need UV tangents).
Each texture is mixed a little with its palette colour, so the stylized palette keeps deciding the mood.
A material whose files aren't downloaded keeps its plain palette colour (run scripts\\run.ps1 fetch)."""
import bpy
from . import mesh, polyhaven as PH

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
    print("STEP textures:", ", ".join(done) or "none (run.ps1 fetch)", flush=True)
    return M
