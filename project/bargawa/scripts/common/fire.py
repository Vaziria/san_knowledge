"""Fire, after Max Hay's "How to Make Beautiful Fire in Blender" (the user, 2026-10-18: "i have new reference for creating fire";
summary: project/bargawa/references/video-fire-max-hay.md).

The video's method, and how it is done here:
    fire footage on a plane      the footage is drawn by the shader (Blender only, no stock video) on upright cards
                                 (geo.cards), several through the flame's axis so it reads from any side
    brightness keyed into alpha  the same: a ColorRamp from the flame's brightness to alpha, its black handle raised a little
                                 so the faint edge is cut
    an orange multiplied in      the same: Flame_Tint, a Multiply mix; the light states can change it
    the footage loops            the flame moves with time: Flame_Time is a driver on the frame (`frame / 24`, seconds)
    copies offset in time        each instance starts at its own moment (Object Info > Random), so flames side by side differ
    low emission, a point light  the same: the light does the lighting, its radius the flame's size; flicker() makes it waver
flame_piece() puts the flame and its light together as a collection that lamps place on their wicks.

Wood fires (objects/fire.py builds the samples; research docs/stories/ramabargawa/objects/fire.md):
    fire_material   a wood fire on crossed cards: rising turbulence over a brightness fading with height, so the flame is solid
                    low down and breaks into tongues higher up (Heskestad's zones); it breathes at its puffing rate; it adds its
                    light to what is behind it
    fire_piece      that fire and its light, which flickers at the fire's rate and sways a little, so shadows move
    sparks          the video's sparks (no gravity, swirling, already in the air on frame 1) in Geometry Nodes, worked out from
                    the time alone: no particle bake, and they work inside a linked collection instance
    ember_material  charcoal with ash on top and cracks glowing, brightening and dimming; charred() burns a wood material the
                    same way where its 'wear' is high
The video's haze belongs to each scene's light state, its motion blur to the story cameras."""
import math as pymath
import bpy
from . import mesh, textures, geo, fuel

lin = mesh.lin

def _time(K, name):
    """Seconds from the frame (a driver, `frame / 24`: the video's looping footage), plus this instance's own offset (Object
    Info > Random x 100), so copies side by side are at different moments."""
    tv = K.N("ShaderNodeValue"); tv.name = tv.label = name
    d = tv.outputs[0].driver_add("default_value").driver; d.type = "SCRIPTED"; d.expression = "frame / 24"
    return K.math("ADD", tv.outputs[0], K.math("MULTIPLY", K.N("ShaderNodeObjectInfo").outputs["Random"], 100.0))

def _channels(K, noise):
    """A noise's colour split into three values round 0 (about -0.25 .. 0.25)."""
    sc = K.N("ShaderNodeSeparateColor"); K.L(noise.outputs["Color"], sc.inputs[0])
    return [K.math("SUBTRACT", c, 0.5) for c in sc.outputs]

def flame_material(name="M_flame", h=0.034, w=0.0058, strength=1.6, tint=(1.0, 0.82, 0.6)):
    """One flame on cards through its axis, in object space: its root at the origin, h tall, w its widest half-width (an
    oil-lamp flame: widest a quarter of the way up, a narrow root, a pointed tip). The flame sways, breathes and licks with time;
    pale gold inside, orange to deep red at the edge and the tip, a touch of blue at the root."""
    m = bpy.data.materials.get(name)
    if m: return m
    m = bpy.data.materials.new(name); m.use_nodes = True; nt = m.node_tree
    for n in list(nt.nodes): nt.nodes.remove(n)
    K = textures._kit(nt); N, L, math, maprange = K.N, K.L, K.math, K.maprange
    t = _time(K, "Flame_Time")
    xyz = N("ShaderNodeSeparateXYZ"); L(N("ShaderNodeTexCoord").outputs["Object"], xyz.inputs[0]); x, y, z = xyz.outputs
    channels = lambda noise: _channels(K, noise)
    # the whole flame, slowly: it sways (two directions) and breathes (its height), about once a second
    slow = N("ShaderNodeTexNoise"); slow.noise_dimensions = "1D"; L(math("MULTIPLY", t, 1.3), slow.inputs["W"])
    slow.inputs["Scale"].default_value = 1.0; slow.inputs["Detail"].default_value = 2.0; sx, sy, sh = channels(slow)
    hz = math("MULTIPLY", math("ADD", sh, 1.0), h)
    u = math("DIVIDE", z, hz)                                                  # 0 at the root, 1 at the tip
    bend = math("MULTIPLY", math("MULTIPLY", u, u), 0.59*h)                    # sway grows with height (4 mm at a 3.4 cm tip)
    # licks: small, fast, rising through the flame
    rise = N("ShaderNodeCombineXYZ"); L(x, rise.inputs[0]); L(y, rise.inputs[1]); L(math("SUBTRACT", z, math("MULTIPLY", t, 0.03)), rise.inputs[2])
    fast = N("ShaderNodeTexNoise"); fast.noise_dimensions = "4D"; L(rise.outputs[0], fast.inputs["Vector"])
    L(math("MULTIPLY", t, 2.0), fast.inputs["W"]); fast.inputs["Scale"].default_value = 1.1/w     # licks about w/2 across
    fast.inputs["Detail"].default_value = 2.0; lr, lz, _ = channels(fast)
    ax = math("SUBTRACT", x, math("MULTIPLY", sx, bend)); ay = math("SUBTRACT", y, math("MULTIPLY", sy, bend))
    r = math("SQRT", math("ADD", math("MULTIPLY", ax, ax), math("MULTIPLY", ay, ay)))            # distance from the swaying axis
    r = math("ADD", r, math("MULTIPLY", math("MULTIPLY", lr, u), 0.7*w))
    u = math("DIVIDE", math("ADD", z, math("MULTIPLY", math("MULTIPLY", lz, u), 0.18*h)), hz)
    # the teardrop: its half-width at each height (as the old lathed flame)
    fc = N("ShaderNodeFloatCurve"); pts = fc.mapping.curves[0].points
    prof = [(0.0, 0.22), (0.09, 0.8), (0.24, 1.0), (0.41, 0.91), (0.62, 0.69), (0.82, 0.36), (1.0, 0.0)]
    pts[0].location, pts[1].location = prof[0], prof[-1]
    for p in prof[1:-1]: pts.new(*p)
    fc.mapping.extend = "HORIZONTAL"; fc.mapping.update(); L(u, fc.inputs["Value"])   # extrapolated, the band above the tip lit
    dist = math("DIVIDE", r, math("MAXIMUM", math("MULTIPLY", fc.outputs[0], w), 1e-5))       # 1 at the flame's edge
    body = math("MULTIPLY", maprange(dist, 0.5, 1.05, 1.0, 0.0), maprange(u, -0.06, 0.04))   # soft edge; nothing under the root
    heat = math("MULTIPLY", body, maprange(u, 0.15, 1.0, 1.0, 0.55))                        # cooler towards the tip
    heat = math("MULTIPLY", heat, math("ADD", 0.75, math("MULTIPLY", maprange(dist, 0.2, 0.7, 1.0, 0.0), 0.25)))
    col = K.ramp(heat, [(0.0, lin(150, 40, 4)), (0.3, lin(235, 96, 16)), (0.6, lin(255, 140, 40)), (0.85, lin(255, 214, 130)),
                        (1.0, lin(255, 240, 200))]).outputs["Color"]                # a deep red rim read pink over a pale sky
    blue = math("MULTIPLY", math("MULTIPLY", maprange(u, 0.02, 0.16, 1.0, 0.0), maprange(dist, 0.35, 0.9)), body)
    col = K.mix(col, lin(70, 110, 255), math("MULTIPLY", blue, 0.7))
    col = K.mix(col, tint, 1.0, "MULTIPLY"); col.node.name = col.node.label = "Flame_Tint"
    # alpha from the brightness, as the video keys its footage (the dim red rim fades instead of outlining the flame), the black
    # handle raised to cut the faint edge; a card turned edge-on to the camera fades out, or it shows as a thin line
    alpha = K.ramp(heat, [(0.04, (0.0, 0.0, 0.0)), (0.7, (1.0, 1.0, 1.0))]).outputs["Color"]
    lw = N("ShaderNodeLayerWeight"); lw.inputs["Blend"].default_value = 0.5
    alpha = math("MULTIPLY", alpha, maprange(lw.outputs["Facing"], 0.55, 0.85, 1.0, 0.0))     # one card is always within 30 deg
    em = N("ShaderNodeEmission"); L(col, em.inputs["Color"]); em.inputs["Strength"].default_value = strength
    mx = N("ShaderNodeMixShader"); L(alpha, mx.inputs[0]); L(N("ShaderNodeBsdfTransparent").outputs[0], mx.inputs[1])
    L(em.outputs[0], mx.inputs[2]); L(mx.outputs[0], N("ShaderNodeOutputMaterial").inputs["Surface"])
    m.surface_render_method = "DITHERED"                     # cards crossing each other sort right; smooth after the TAA samples
    m.diffuse_color = (*lin(255, 170, 70), 1)
    return m

def flame_piece(name, h, w, energy, at=(0.0, 0.0, 0.0), rgb=None, radius=None, desc=None):
    """A flame and its flickering light as a collection of its own, an asset: the flame drawn on cards (h tall, w its widest
    half-width), the light inside it a third of the way up (`energy` W, its radius about the flame's size), the flame's root
    at `at`, where the piece stands in its file, which is also the instance offset. Lamps put one on each wick with
    mesh.place_collection; a light state can add or remove one. Returns (collection, flame object, light object)."""
    c = mesh.coll(name)
    fl = mesh.Parts().add(geo.cards(2.07*w, -0.09*h, 1.41*h), flame_material("M_" + name, h=h, w=w)).build(name, c, loc=at)
    fl.visible_shadow = False                                    # the light sits inside the flame: casting shadows, it hid it
    li = mesh.point_light(name + "_Light", c, (at[0], at[1], at[2] + 0.35*h), energy, rgb or lin(255, 178, 102),
                          radius=radius or 0.6*h)                # the flame's size: its glint on metal is the flame's own
    flicker(li, energy)
    c.instance_offset = at
    c.asset_mark(); c.asset_data.description = desc or f"A {100*h:.1f} cm flame with its flickering light; its root is the origin"
    return c, fl, li

def flicker(light, energy, amount=0.25, hz=None, sway=0.0):
    """Make a light waver round `energy`: a driver on the frame, sums of sines, a simple expression that runs without Python
    scripts allowed. With no hz, the oil lamp's gentle waver (about +-12%, 1 to 9 times a second). With hz (a fire's puffing
    rate), about +-amount at that rate and faster; sway (m) also moves the light about, so the shadows it casts move."""
    d = light.data.driver_add("energy").driver; d.type = "SCRIPTED"
    if hz is None:
        d.expression = f"{energy}*(1 + 0.06*sin(frame*0.37) + 0.04*sin(frame*0.91 + 1.3) + 0.025*sin(frame*2.3 + 0.4))"
        return light
    w = 2*pymath.pi*hz/24                                        # radians per frame; kept under 2.6 (24 fps can't show faster)
    terms = " + ".join(f"{amount*k:.3f}*sin(frame*{min(w*f, 2.6):.3f} + {p})" for k, f, p in ((0.5, 1.0, 0.0), (0.3, 2.37, 1.3), (0.2, 4.13, 0.4)))
    d.expression = f"{energy}*(1 + {terms})"
    for i, ph in ((0, 0.7), (1, 2.1)) if sway else ():
        dd = light.driver_add("location", i).driver; dd.type = "SCRIPTED"
        dd.expression = (f"{light.location[i]:.4f} + {sway:.4f}*(0.6*sin(frame*{min(0.8*w, 2.6):.3f} + {ph}) + "
                         f"0.4*sin(frame*{min(1.9*w, 2.6):.3f} + {2*ph}))")
    return light

# --- wood fires (objects/fire.py) ---

FIRE_RAMP = [(0.0, lin(150, 40, 4)), (0.25, lin(235, 96, 16)), (0.5, lin(255, 140, 36)), (0.75, lin(255, 196, 96)),
             (0.92, lin(255, 228, 160)), (1.0, lin(255, 242, 205))]      # hot soot: orange-red tips (a deeper red, added over
                                                                         # daylight, read pink) to pale yellow in the core

def fire_material(name, D, L, strength=2.5, tint=(1.0, 1.0, 1.0)):
    """A wood fire on cards through its axis, in object space: its base a disc D across at z 0, its flames L tall on average
    (Heskestad's mean height, reached half the time). A turbulence pattern rising through the fire is added to a brightness
    that fades with height and towards the sides: low down the flame is solid; higher up only the turbulence's peaks show, as
    tongues that break off and burn out; above 1.6 L nothing (the zones in objects/fire.md). The fire breathes at its puffing
    rate (fuel.puff_hz) and leans slowly; the colour cools from pale yellow in the core to deep red at the tips. It adds its
    light to what is behind it (real flames hardly block light), so overlapping cards brighten the core."""
    m = bpy.data.materials.get(name)
    if m: return m
    m = bpy.data.materials.new(name); m.use_nodes = True; nt = m.node_tree
    for n in list(nt.nodes): nt.nodes.remove(n)
    K = textures._kit(nt); N, Lk, math, maprange = K.N, K.L, K.math, K.maprange
    t = _time(K, "Fire_Time"); hz = fuel.puff_hz(D)
    xyz = N("ShaderNodeSeparateXYZ"); Lk(N("ShaderNodeTexCoord").outputs["Object"], xyz.inputs[0]); x, y, z = xyz.outputs
    # the whole fire: it breathes (height) at its puffing rate, unevenly, and leans now one way, now another
    slow = N("ShaderNodeTexNoise"); slow.noise_dimensions = "1D"; Lk(math("MULTIPLY", t, 0.6*hz), slow.inputs["W"])
    slow.inputs["Scale"].default_value = 1.0; slow.inputs["Detail"].default_value = 1.0; sx, sy, sh = _channels(K, slow)
    puff = math("SINE", math("MULTIPLY", t, 2*pymath.pi*hz))
    lh = math("MULTIPLY", math("ADD", math("ADD", 1.0, math("MULTIPLY", puff, 0.1)), math("MULTIPLY", sh, 0.6)), L)
    hgt = math("DIVIDE", z, lh)                                                   # 1 at the mean flame height
    lean = math("MULTIPLY", math("MULTIPLY", hgt, hgt), 0.35*D)
    ax = math("SUBTRACT", x, math("MULTIPLY", sx, lean)); ay = math("SUBTRACT", y, math("MULTIPLY", sy, lean))
    # rising turbulence, warped by a larger, slower pattern; its features taller than wide
    q = N("ShaderNodeCombineXYZ"); Lk(ax, q.inputs[0]); Lk(ay, q.inputs[1])
    Lk(math("SUBTRACT", z, math("MULTIPLY", t, 1.6*pymath.sqrt(L))), q.inputs[2])
    warp = N("ShaderNodeTexNoise"); warp.noise_dimensions = "4D"; Lk(q.outputs[0], warp.inputs["Vector"])
    Lk(math("MULTIPLY", t, 0.5), warp.inputs["W"]); warp.inputs["Scale"].default_value = 1.5/D; warp.inputs["Detail"].default_value = 1.0
    wv = N("ShaderNodeVectorMath"); wv.operation = "SUBTRACT"; Lk(warp.outputs["Color"], wv.inputs[0]); wv.inputs[1].default_value = (0.5, 0.5, 0.5)
    ws = N("ShaderNodeVectorMath"); ws.operation = "SCALE"; Lk(wv.outputs[0], ws.inputs[0]); ws.inputs["Scale"].default_value = 0.45*D
    q2 = N("ShaderNodeVectorMath"); q2.operation = "ADD"; Lk(q.outputs[0], q2.inputs[0]); Lk(ws.outputs[0], q2.inputs[1])
    turb = N("ShaderNodeTexNoise"); turb.noise_dimensions = "4D"; Lk(K.stretch(q2.outputs[0], 1.0, 1.0, 0.5), turb.inputs["Vector"])
    Lk(math("MULTIPLY", t, 0.7), turb.inputs["W"]); turb.inputs["Scale"].default_value = 3.5/D
    turb.inputs["Detail"].default_value = 4.0; turb.inputs["Roughness"].default_value = 0.6
    # where the fire can be: a cone from the base, narrowing with height; solid low down, fading to nothing by 1.6 L
    rr = math("DIVIDE", math("SQRT", math("ADD", math("MULTIPLY", ax, ax), math("MULTIPLY", ay, ay))), 0.5*D)
    edge = math("DIVIDE", rr, maprange(hgt, 0.0, 1.6, 1.0, 0.25))
    base = math("MULTIPLY", math("MULTIPLY", maprange(hgt, -0.02, 0.08), maprange(hgt, 0.15, 1.6, 1.0, 0.0)),
                maprange(edge, 0.35, 1.0, 1.0, 0.0))
    field = math("ADD", base, math("MULTIPLY", math("SUBTRACT", turb.outputs["Fac"], 0.5), 2.4))   # at 1.6, a soft blob
    heat = maprange(field, 0.45, 1.15)
    col = K.ramp(math("MULTIPLY", heat, maprange(hgt, 0.2, 1.5, 1.0, 0.55)), FIRE_RAMP).outputs["Color"]
    col = K.mix(col, tint, 1.0, "MULTIPLY"); col.node.name = col.node.label = "Fire_Tint"
    alpha = maprange(heat, 0.0, 0.15)                                             # crisp-edged tongues
    lw = N("ShaderNodeLayerWeight"); lw.inputs["Blend"].default_value = 0.5
    alpha = math("MULTIPLY", alpha, maprange(lw.outputs["Facing"], 0.55, 0.85, 1.0, 0.0))   # a card edge-on fades out
    em = N("ShaderNodeEmission"); Lk(col, em.inputs["Color"]); Lk(math("MULTIPLY", alpha, strength), em.inputs["Strength"])
    add = N("ShaderNodeAddShader"); Lk(N("ShaderNodeBsdfTransparent").outputs[0], add.inputs[0]); Lk(em.outputs[0], add.inputs[1])
    Lk(add.outputs[0], N("ShaderNodeOutputMaterial").inputs["Surface"])
    m.surface_render_method = "BLENDED"                           # added light: the order of the crossing cards doesn't matter
    m.diffuse_color = (*lin(255, 150, 50), 1)
    return m

def fire_piece(name, col, D, L, energy, at=(0.0, 0.0, 0.0), cards=3, strength=2.5, rgb=None):
    """A wood fire D across with flames L tall (fire_material on `cards` crossed cards, up to 2 L), its root at `at`, and a
    point light inside it at 0.3 L, its radius a third of the fire's width, flickering at the fire's puffing rate and swaying.
    Returns (fire object, light object)."""
    vf = geo.cards(0.62*D, -0.03*L, 2.0*L, n=cards)
    fl = mesh.Parts().add(vf, fire_material("M_" + name, D, L, strength)).build(name, col, loc=at)
    fl.visible_shadow = False                                    # the light sits inside it
    li = mesh.point_light(name + "_Light", col, (at[0], at[1], at[2] + 0.3*L), energy, rgb or lin(255, 150, 64), radius=0.3*D)
    flicker(li, energy, amount=0.25, hz=fuel.puff_hz(D), sway=0.06*D)
    return fl, li

def spark_material(name="M_fire_spark", strength=8.0):
    """Sparks: emission only, cooling with age (the sparks' 'age' attribute, 0 born .. 1 gone) from pale yellow to deep red."""
    m = bpy.data.materials.get(name)
    if m: return m
    m = bpy.data.materials.new(name); m.use_nodes = True; nt = m.node_tree
    for n in list(nt.nodes): nt.nodes.remove(n)
    K = textures._kit(nt)
    age = K.N("ShaderNodeAttribute"); age.attribute_type = "INSTANCER"; age.attribute_name = "age"; age = age.outputs["Fac"]
    col = K.ramp(age, [(0.0, lin(255, 236, 190)), (0.3, lin(255, 160, 60)), (0.7, lin(220, 70, 12)), (1.0, lin(120, 24, 4))])
    em = K.N("ShaderNodeEmission"); K.L(col.outputs["Color"], em.inputs["Color"])
    K.L(K.maprange(age, 0.2, 1.0, strength, 0.2*strength), em.inputs["Strength"])
    K.L(em.outputs[0], K.N("ShaderNodeOutputMaterial").inputs["Surface"])
    m.diffuse_color = (*lin(255, 170, 70), 1)
    return m

SPARK_INPUTS = ("Count", "Radius", "Height", "Life", "Swirl", "Size", "Seed", "Material")

def sparks_group():
    """The Geometry Nodes that make sparks (made once per file): Count points, each a spark with its own life (Life x 0.6..1.3
    s) and moment in it, so they are already in the air on frame 1 and need no bake. Each rises from a disc of Radius about Height
    over its life, spreading out, swirled by a 4D noise (Swirl m, growing with age; the video's Turbulence field), its size
    (Size m) growing in, then shrinking to nothing before it starts again. Writes 'age' (0..1) for the material."""
    ng = bpy.data.node_groups.get("GN_Fire_Sparks")
    if ng: return ng
    ng = bpy.data.node_groups.new("GN_Fire_Sparks", "GeometryNodeTree")
    try: ng.is_modifier = True
    except AttributeError: pass
    ng.interface.new_socket(name="Geometry", in_out="OUTPUT", socket_type="NodeSocketGeometry")
    for nm in SPARK_INPUTS:
        typ = "NodeSocketInt" if nm in ("Count", "Seed") else ("NodeSocketMaterial" if nm == "Material" else "NodeSocketFloat")
        ng.interface.new_socket(name=nm, in_out="INPUT", socket_type=typ)
    K = textures._kit(ng); N, L, math, maprange = K.N, K.L, K.math, K.maprange
    g = N("NodeGroupInput").outputs; out = N("NodeGroupOutput")
    sec = N("GeometryNodeInputSceneTime").outputs["Seconds"]; idx = N("GeometryNodeInputIndex").outputs[0]
    def rnd(lo, hi, k):
        r = N("FunctionNodeRandomValue"); r.data_type = "FLOAT"
        K.sock(r.inputs, "Min", "VALUE").default_value = lo; K.sock(r.inputs, "Max", "VALUE").default_value = hi
        L(idx, r.inputs["ID"]); L(math("ADD", g["Seed"], float(7*k)), r.inputs["Seed"])
        return K.sock(r.outputs, "Value", "VALUE")
    life = math("MULTIPLY", g["Life"], rnd(0.6, 1.3, 1))
    age = math("FRACT", math("ADD", math("DIVIDE", sec, life), rnd(0.0, 1.0, 2)))
    a = rnd(0.0, 2*pymath.pi, 3)
    rr = math("MULTIPLY", math("MULTIPLY", g["Radius"], math("SQRT", rnd(0.0, 1.0, 4))), math("ADD", 1.0, math("MULTIPLY", age, 1.5)))
    p = N("ShaderNodeCombineXYZ")
    L(math("MULTIPLY", rr, math("COSINE", a)), p.inputs[0]); L(math("MULTIPLY", rr, math("SINE", a)), p.inputs[1])
    L(math("MULTIPLY", math("MULTIPLY", g["Height"], rnd(0.5, 1.3, 5)), age), p.inputs[2])
    sw = N("ShaderNodeTexNoise"); sw.noise_dimensions = "4D"; L(p.outputs[0], sw.inputs["Vector"])
    L(math("ADD", math("MULTIPLY", sec, 0.35), rnd(0.0, 10.0, 6)), sw.inputs["W"]); L(math("DIVIDE", 2.0, g["Height"]), sw.inputs["Scale"])
    sw.inputs["Detail"].default_value = 1.0
    off = N("ShaderNodeVectorMath"); off.operation = "SUBTRACT"; L(sw.outputs["Color"], off.inputs[0]); off.inputs[1].default_value = (0.5, 0.5, 0.5)
    sc = N("ShaderNodeVectorMath"); sc.operation = "SCALE"; L(off.outputs[0], sc.inputs[0])
    L(math("MULTIPLY", math("MULTIPLY", g["Swirl"], 2.0), age), sc.inputs["Scale"])
    pos = N("ShaderNodeVectorMath"); pos.operation = "ADD"; L(p.outputs[0], pos.inputs[0]); L(sc.outputs[0], pos.inputs[1])
    pts = N("GeometryNodePoints"); L(g["Count"], pts.inputs["Count"]); L(pos.outputs[0], pts.inputs["Position"])
    st = N("GeometryNodeStoreNamedAttribute"); st.data_type = "FLOAT"; st.domain = "POINT"
    L(pts.outputs[0], st.inputs["Geometry"]); st.inputs["Name"].default_value = "age"; L(age, st.inputs["Value"])
    ico = N("GeometryNodeMeshIcoSphere"); ico.inputs["Radius"].default_value = 1.0; ico.inputs["Subdivisions"].default_value = 1
    sm = N("GeometryNodeSetMaterial"); L(ico.outputs["Mesh"], sm.inputs["Geometry"]); L(g["Material"], sm.inputs["Material"])
    size = math("MULTIPLY", math("MULTIPLY", g["Size"], rnd(0.5, 1.5, 7)),
                math("MULTIPLY", maprange(age, 0.0, 0.05), maprange(age, 0.55, 1.0, 1.0, 0.0)))
    iop = N("GeometryNodeInstanceOnPoints"); L(st.outputs[0], iop.inputs["Points"]); L(sm.outputs[0], iop.inputs["Instance"])
    L(size, iop.inputs["Scale"]); L(iop.outputs["Instances"], out.inputs["Geometry"])
    return ng

def sparks(name, col, at, count, radius, height, life, size, swirl, seed=0):
    """An object whose Geometry Nodes (sparks_group) make sparks rising from `at`. Returns it."""
    ob = bpy.data.objects.new(name, bpy.data.meshes.new(name)); col.objects.link(ob); ob.location = at
    ng = sparks_group(); md = ob.modifiers.new("Sparks", "NODES"); md.node_group = ng
    vals = dict(Count=count, Radius=radius, Height=height, Life=life, Swirl=swirl, Size=size, Seed=seed, Material=spark_material())
    for it in ng.interface.items_tree:
        if it.item_type != "SOCKET" or it.in_out != "INPUT": continue
        try: md[it.identifier] = vals[it.name]                                    # Blender 4.x
        except TypeError: getattr(md.properties.inputs, it.identifier).value = vals[it.name]   # 5.2: no ID properties
    ob.visible_shadow = False
    return ob

def _glow(K, heat, t, scale=70.0):
    """Glowing charcoal: brightest in thin cracks (Voronoi edges, cells about 1/scale m across, warped by a noise so they are
    not tidy polygons), faintly on the faces between, in patches, all of it brightening and dimming slowly as air moves over
    it. heat (0..1) is how hot. Returns (emission colour, glow 0..1). (Cells 3 cm across with wide cracks read as a giraffe.)"""
    co = K.N("ShaderNodeTexCoord").outputs["Object"]
    wn = K.N("ShaderNodeTexNoise"); K.L(co, wn.inputs["Vector"]); wn.inputs["Scale"].default_value = 0.6*scale
    wv = K.N("ShaderNodeVectorMath"); wv.operation = "MULTIPLY_ADD"; K.L(wn.outputs["Color"], wv.inputs[0])
    wv.inputs[1].default_value = (1.2/scale,)*3; K.L(co, wv.inputs[2])
    vo = K.N("ShaderNodeTexVoronoi"); vo.feature = "DISTANCE_TO_EDGE"; K.L(wv.outputs[0], vo.inputs["Vector"]); vo.inputs["Scale"].default_value = scale
    crack = K.maprange(vo.outputs["Distance"], 0.0, 0.05, 1.0, 0.0)
    br = K.N("ShaderNodeTexNoise"); br.noise_dimensions = "4D"; K.L(co, br.inputs["Vector"]); K.L(K.math("MULTIPLY", t, 0.4), br.inputs["W"])
    br.inputs["Scale"].default_value = 5.0; br.inputs["Detail"].default_value = 2.0
    patch = K.maprange(K.noise(co, 9.0, 2.0), 0.42, 0.62, 0.25, 1.0)
    glow = K.math("MULTIPLY", K.math("MULTIPLY", heat, K.maprange(br.outputs["Fac"], 0.35, 0.65, 0.35, 1.0)),
                  K.math("MULTIPLY", patch, K.math("ADD", 0.15, K.math("MULTIPLY", crack, 0.85))))
    col = K.ramp(glow, [(0.0, lin(60, 6, 0)), (0.35, lin(180, 30, 4)), (0.65, lin(255, 96, 16)), (1.0, lin(255, 180, 80))])
    return col.outputs["Color"], glow

CHARCOAL, ASH = lin(26, 23, 21), lin(120, 116, 112)       # a paler ash read cream in the firelight

def ember_material(name="M_fire_embers", strength=6.0):
    """Coals and their bed of ash: charcoal, grey ash over the cooler parts, glowing where hot (the mesh's 'wear', 0..1)."""
    m = bpy.data.materials.get(name)
    if m: return m
    m = bpy.data.materials.new(name); p = mesh._principled(m); nt = m.node_tree; K = textures._kit(nt)
    heat = K.N("ShaderNodeAttribute"); heat.attribute_name = "wear"; heat = heat.outputs["Fac"]
    col, glow = _glow(K, heat, _time(K, "Fire_Time"))
    co = K.N("ShaderNodeTexCoord").outputs["Object"]
    ash = K.math("MULTIPLY", K.maprange(K.noise(co, 60.0, 4.0), 0.38, 0.66), K.maprange(heat, 0.1, 0.6, 1.0, 0.0))   # mottled grey,
                                                                  # none on hot coals (a sharp 0.48..0.6 step made camouflage blobs)
    K.L(K.mix(CHARCOAL, ASH, ash), p.inputs["Base Color"]); p.inputs["Roughness"].default_value = 0.9
    K.L(col, p.inputs["Emission Color"])
    K.L(K.math("MULTIPLY", K.math("MULTIPLY", glow, strength), K.math("SUBTRACT", 1.0, K.math("MULTIPLY", ash, 0.8))),
        p.inputs["Emission Strength"])
    m.diffuse_color = (*lin(60, 40, 30), 1)
    return m

def charred(m, strength=4.0):
    """Burn a wood material where the mesh's 'wear' is high: its colour turns to charcoal from 0.15 to 0.6, and above 0.6 it
    glows in its cracks as the coals do (firewood in the fire samples)."""
    nt = m.node_tree; p = mesh._principled(m); K = textures._kit(nt)
    char = K.N("ShaderNodeAttribute"); char.attribute_name = "wear"; char = char.outputs["Fac"]
    base = p.inputs["Base Color"]; src = base.links[0].from_socket if base.links else tuple(base.default_value)[:3]
    K.L(K.mix(src, CHARCOAL, K.maprange(char, 0.15, 0.6)), base)
    col, glow = _glow(K, K.maprange(char, 0.6, 1.0), _time(K, "Fire_Time"), scale=55.0)
    K.L(col, p.inputs["Emission Color"]); K.L(K.math("MULTIPLY", glow, strength), p.inputs["Emission Strength"])
    return m
