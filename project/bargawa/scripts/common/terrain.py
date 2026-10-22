"""The ground of Jatisrana: terrain, the lake's water, the paths, the bare ground (the user's photo grounds, with relief),
and the mountains behind."""
import bpy, math
from mathutils.bvhtree import BVHTree
from . import mesh, geo, jatisrana as J

def ground_material():
    """Earth and grass mixed in patches (a stand-in until the Poly Haven ground textures, milestone 4)."""
    m = bpy.data.materials.get("M_ground")
    if m: return m
    m = bpy.data.materials.new("M_ground"); p = mesh._principled(m); nt = m.node_tree
    from . import textures
    if textures.textured(m, "ground", mesh.lin(80, 92, 52)):                                     # muddy ground with a green cast
        m.diffuse_color = (*mesh.lin(88, 86, 52), 1); return m
    tc = nt.nodes.new("ShaderNodeTexCoord")
    noise = nt.nodes.new("ShaderNodeTexNoise"); noise.inputs["Scale"].default_value = 0.06; noise.inputs["Detail"].default_value = 6.0
    ramp = nt.nodes.new("ShaderNodeValToRGB")
    ramp.color_ramp.elements[0].position = 0.30; ramp.color_ramp.elements[0].color = (*mesh.lin(112, 90, 62), 1)     # mostly grass,
    ramp.color_ramp.elements[1].position = 0.46; ramp.color_ramp.elements[1].color = (*mesh.lin(76, 98, 50), 1)      # bare earth in patches
    nt.links.new(tc.outputs["Object"], noise.inputs["Vector"])
    nt.links.new(noise.outputs[0], ramp.inputs[0])
    nt.links.new(ramp.outputs[0], p.inputs["Base Color"])
    p.inputs["Roughness"].default_value = 0.95
    m.diffuse_color = (*mesh.lin(88, 86, 52), 1)
    return m

def water_material():
    return mesh.material("M_water", mesh.lin(18, 30, 32), rough=0.04)

def terrain(col):
    t = J.TERRAIN; nx = int(round((t["x1"] - t["x0"])/t["step"])) + 1; ny = int(round((t["y1"] - t["y0"])/t["step"])) + 1
    v = []
    for j in range(ny):
        y = t["y0"] + j*t["step"]
        for i in range(nx):
            x = t["x0"] + i*t["step"]; v.append((x, y, J.height(x, y)))
    f = [(j*nx + i, j*nx + i + 1, (j + 1)*nx + i + 1, (j + 1)*nx + i) for j in range(ny - 1) for i in range(nx - 1)]
    return mesh.obj("Terrain", (v, f), col, ground_material(), smooth=True, recalc=False)

def lake(col):
    """The water: one flat sheet a little larger than the lake, so its edge hides under the banks."""
    n = 72; ring = geo.ellipse(J.LAKE_C[0], J.LAKE_C[1], J.WATER_Z, J.LAKE_R[0]*1.25, J.LAKE_R[1]*1.25, n)
    v = [(J.LAKE_C[0], J.LAKE_C[1], J.WATER_Z)] + ring
    f = [(0, 1 + i, 1 + (i + 1) % n) for i in range(n)]
    return mesh.obj("Lake_Water", (v, f), col, water_material(), recalc=False)

def ribbon(pts, width, lift=0.04, step=0.5):
    """A strip `width` wide along the polyline pts (x, y), lying on the ground."""
    dense = []
    for (x0, y0), (x1, y1) in zip(pts, pts[1:]):
        n = max(1, int(math.hypot(x1 - x0, y1 - y0)/step))
        dense += [(x0 + (x1 - x0)*k/n, y0 + (y1 - y0)*k/n) for k in range(n)]
    dense.append(pts[-1])
    v = []
    for k, (x, y) in enumerate(dense):
        a, b = dense[max(k - 1, 0)], dense[min(k + 1, len(dense) - 1)]
        tx, ty = b[0] - a[0], b[1] - a[1]; L = math.hypot(tx, ty) or 1.0
        sx, sy = -ty/L*width/2, tx/L*width/2
        for s in (-1, 1):
            px, py = x + s*sx, y + s*sy; v.append((px, py, J.height(px, py) + lift))
    f = [(2*k, 2*k + 2, 2*k + 3, 2*k + 1) for k in range(len(dense) - 1)]          # facing up
    return v, f

def paths(col, M):
    P = mesh.Parts()
    for pl in J.PATHS: P.add(ribbon(pl, 1.2), M["earth"])
    return P.build("Paths", col, recalc=False, smooth=True)

# --- the bare ground: the user's photo grounds, with real relief --------------------------------------------------------------
# Each look is the user's own material made by hand: a photo as Base Color, its brightness through a Colour Ramp as the height
# (midlevel 0, so only up), on a fine flat-shaded grid. The relief is real geometry (a Geometry Nodes modifier, not shader
# displacement), so EEVEE, Cycles and the game's export all see it. The movie comes first (bargawa_game.md): the photo goes in as
# it is, without the palette colour, with realistic shading (Metallic 0, Roughness 0.85, IOR 1.5; the user chose it for the thatch).
# Both looks are in the texture library (blender/textures/ground.blend); BARE says which one Jatisrana's bare ground wears.
#   ground_spiky  their Ground_2.blend (2026-10-20, "no i like this spiky"): ramp 0 -> 0.427, so every point is a spike. The photo
#                 isn't seamless, so its tiles are mirrored.
#   ground_soil   their Soil in blender/textures/material_manual.blend (2026-10-21, "this is better for jatisrana"): soil with
#                 leaf litter, ramp 0.655 -> 0.932, so only the brightest leaves, straws and pebbles rise; a seamless photo, once
#                 over their 5.59 m plane; its bump from the photo itself (Strength 1, Distance 0.8), as theirs.
SPIKY = dict(photo="ground_spiky.jpg", tile=(2.86, 4.5),      # the photo's own shape (689 x 1085 px), 4.5 m tall
             ramp=(0.0, 0.427), height=0.1, mid=0.0, extension="MIRROR",
             grid=0.14, detail=1,                              # the grid, subdivided `detail` times by the modifier: 7 cm
             rough=0.85, bump=0.3, bump_dist=0.02, ramp_bump=True,   # the bump from the ramp, as the spikes
             lift=0.02, viewport=(104, 84, 64))                # lift: over the terrain, under the paths (0.04)
SOIL = dict(photo="1cb0165c04eb35eb7bd78df3f83cc534.jpg", tile=(5.6, 5.6),   # the user's file name (their material uses it)
            ramp=(0.655, 0.932), height=0.1, mid=0.0, extension="REPEAT",
            grid=0.14, detail=1,                               # their plane: 8.7 cm (Simple subdivision 6 over 5.59 m)
            rough=0.85, bump=1.0, bump_dist=0.8, ramp_bump=False,
            lift=0.02, viewport=(92, 74, 54))
RELIEF_GROUNDS = {"ground_spiky": SPIKY, "ground_soil": SOIL}
BARE = "ground_soil"                       # Jatisrana's bare ground: the soil (2026-10-21; it was the spiky ground)

def relief_ground_material(key):
    """M_<key> for a look in RELIEF_GROUNDS: the photo on the patches' UVs (in tiles of the photo, world-aligned), matte, with a
    bump from the photo or from the relief's ramp. Where the mesh's 'fade_out' attribute is high it turns see-through
    (dithered), so the ground under a patch's edge shows. Also makes GN_<key>, the relief: the grid subdivided `Detail` times,
    each point raised by the ramp x Height x the 'spikes' vertex group (0 by the paths and the water; a mesh without the group
    gets the full height)."""
    m = bpy.data.materials.get("M_" + key)
    if m: return m
    from . import textures as T
    S = RELIEF_GROUNDS[key]; img = T.photo(S["photo"])
    m = T.photo_material("M_" + key, img, (1.0, 1.0), S["extension"], S["rough"], S["bump"], S["bump_dist"],
                         ramp=S["ramp"] if S["ramp_bump"] else None, fade="fade_out")
    m.diffuse_color = (*mesh.lin(*S["viewport"]), 1)
    T.relief_nodes("GN_" + key, img, S["ramp"], S["height"], S["mid"], S["detail"], extension=S["extension"], weight="spikes")
    return m

def relief_patch(name, col, key, m, nodes, box, area, spikes, ground=None, keep=None):
    """A flat-shaded grid of the look's `grid` over box (x0, x1, y0, y1), its `lift` over `ground` (found by rays, so it lies on
    the terrain's own triangles) or z 0: the faces whose centre has area(x, y) > 0 and keep(x, y). Its UVs count tiles of the
    photo from the world's origin; 'fade_out' is 1 - area (the material's see-through edge); the vertex group 'spikes' is
    spikes(x, y, z) (the relief's height). The modifier 'Relief' runs `nodes`."""
    S = RELIEF_GROUNDS[key]; s = S["grid"]; x0, x1, y0, y1 = box
    nx, ny = int(math.ceil((x1 - x0)/s - 1e-6)), int(math.ceil((y1 - y0)/s - 1e-6))
    idx, faces = {}, []
    vid = lambda i, j: idx.setdefault((i, j), len(idx))
    for j in range(ny):
        for i in range(nx):
            cx, cy = x0 + (i + 0.5)*s, y0 + (j + 0.5)*s
            if area(cx, cy) > 0.0 and (keep is None or keep(cx, cy)):
                faces.append((vid(i, j), vid(i + 1, j), vid(i + 1, j + 1), vid(i, j + 1)))
    if not faces: return None
    bvh = BVHTree.FromObject(ground, bpy.context.evaluated_depsgraph_get()) if ground else None
    pts = [None]*len(idx)
    for (i, j), k in idx.items():
        x, y = x0 + i*s, y0 + j*s; z = 0.0
        if bvh:
            hit = bvh.ray_cast((x, y, 1000.0), (0.0, 0.0, -1.0))
            z = hit[0].z if hit[0] else J.height(x, y)
        pts[k] = (x, y, z + S["lift"])
    me = bpy.data.meshes.new(name); me.from_pydata(pts, [], faces)
    tx, ty = S["tile"]
    me.uv_layers.new(name="UVMap")
    lv = [0]*len(me.loops); me.loops.foreach_get("vertex_index", lv)
    me.attributes["UVMap"].data.foreach_set("vector", [c for v in lv for c in (pts[v][0]/tx, pts[v][1]/ty)])
    fo = me.attributes.new("fade_out", "FLOAT", "POINT"); fo.data.foreach_set("value", [1.0 - area(p[0], p[1]) for p in pts])
    me.materials.append(m); me.validate(); me.update()
    ob = bpy.data.objects.new(name, me); col.objects.link(ob)
    vg = ob.vertex_groups.new(name="spikes"); by = {}
    for k, p in enumerate(pts): by.setdefault(round(spikes(*p), 3), []).append(k)
    for wt, ks in by.items():
        if wt > 0: vg.add(ks, wt, "REPLACE")
    md = ob.modifiers.new("Relief", "NODES"); md.node_group = nodes
    return ob

def _under_building(x, y):
    """Under the pendapa's plinth or a hut's stone base (both less 0.3 m a side), where the yard is never seen. The huts' sizes
    are those places/jatisrana.py gives buildings.hut (its base is w + 0.6 by d + 0.6)."""
    from . import joglo
    h = joglo.PLINTH/2 - 0.3
    if abs(x - J.PENDAPA[0]) < h and abs(y - J.PENDAPA[1]) < h: return True
    return any(abs(x - hx) < w/2 and abs(y - hy) < d/2 for (hx, hy, _), w, d in ((J.HUT_ELDERS, 5.0, 4.0), (J.HUT_SONS, 6.0, 4.5)))

def bare_ground(col, M, ground, key=BARE):
    """The bare patches of Jatisrana (J.yard, J.far_bank) in the look `key`, on the terrain object `ground`. Their Geometry Nodes
    are linked from the texture library (blender/textures/ground.blend), like their material."""
    from . import textures, grass
    nodes = textures.link_group("ground", "GN_" + key); m = M[key]
    by_paths = lambda x, y: J.smooth(0.7, 1.2, min(grass._dist_to_path(x, y, pl) for pl in J.PATHS))   # paths: 1.2 m wide
    pad = J.BARE_FADE + 0.5
    box = (min(c[0] - h[0] for c, h in J.YARD) - pad, max(c[0] + h[0] for c, h in J.YARD) + pad,
           min(c[1] - h[1] for c, h in J.YARD) - pad, max(c[1] + h[1] for c, h in J.YARD) + pad)
    yard = relief_patch("BareGround_Yard", col, key, m, nodes, box, J.yard, lambda x, y, z: J.yard(x, y)*by_paths(x, y), ground,
                        keep=lambda x, y: not _under_building(x, y))
    shore = J.WATER_Z + 0.03
    bank = lambda x, y: J.far_bank(x, y) if J.height(x, y) > shore else 0.0
    cx, cy = J.CITRARATA_SPOT; r = J.FAR_BANK_R + 1.0
    far = relief_patch("BareGround_FarBank", col, key, m, nodes, (cx - r, cx + r, cy - r, cy + r), bank,
                       lambda x, y, z: bank(x, y)*J.smooth(shore, shore + 0.3, z), ground)     # no relief out of the water
    return yard, far

def relief_sample(col, m, x, key):
    """A 1.4 m square of a relief ground for the texture library's Samples, its front edge at y -0.7."""
    return relief_patch("Sample_" + key, col, key, m, bpy.data.node_groups["GN_" + key], (x - 0.7, x + 0.7, -0.7, 0.7),
                        lambda px, py: 1.0, lambda px, py, pz: 1.0)

def mountains(col, M):
    """Far peaks to the north: a backdrop, never reached. Open at the bottom, their feet below the ground and beyond the terrain
    (a closed base once lay over the whole valley at z 0)."""
    P = mesh.Parts()
    for (x, y, r, h, n) in ((0.0, 2400.0, 1600.0, 1300.0, 24), (-1700.0, 2700.0, 1200.0, 900.0, 20), (1800.0, 2600.0, 1100.0, 800.0, 20)):
        ring = [(px + 0.08*r*(geo.hash01(px, py) - 0.5), py + 0.08*r*(geo.hash01(py, px) - 0.5), -40.0)
                for (px, py, _) in geo.ellipse(x, y, 0.0, r, r, n)]
        mid = [(x + 0.45*(px - x), y + 0.45*(py - y), 0.62*h*(0.9 + 0.2*geo.hash01(px)))
               for (px, py, _) in ring]
        apex = [(x + 0.04*(px - x), y + 0.04*(py - y), h) for (px, py, _) in ring]
        P.add(geo.loft([ring, mid, apex], False, True), M["mountain"])
    return P.build("Mountains", col)
