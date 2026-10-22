"""Grass over the hermitage's clearing: a hidden emitter mesh on the ground with a 'grass' density map (none on the paths, under
the buildings, in the bare yard round them, on the steps, in the water or at the banyan's foot), and a hair particle system
that scatters three grass-clump shapes over it.

Particles with orientation Global Z and a random phase turn each instance so that its local +Y points up and spin it about the
vertical (tested in Blender 5.2), so the clumps are modelled with +Y as up."""
import bpy, math, random
from . import mesh, geo, jatisrana as J

AREA = (-38.0, 38.0, -30.0, 30.0)          # x0, x1, y0, y1 of the emitter
STEP = 0.5

def _clump(col, M, name, seed, mats):
    rng = random.Random(seed); P = mesh.Parts()
    for k in range(13):
        a = rng.uniform(0, 2*math.pi); c, s = math.cos(a), math.sin(a); h = rng.uniform(0.22, 0.48); lean = rng.uniform(0.05, 0.22)
        b0 = (0.05*c*rng.random(), 0.05*s*rng.random(), -0.02)
        path = geo.bezier(b0, (b0[0] + lean*0.3*c, b0[1] + lean*0.3*s, h*0.6), (b0[0] + lean*c, b0[1] + lean*s, h), 4)
        v, f = geo.ribbon(path, [0.02, 0.018, 0.014, 0.008, 0.0015], up=(c, s, 0))
        v = [(px, pz, -py) for px, py, pz in v]                      # +Z up -> +Y up (see the module note)
        P.add((v, f), M[mats[k % len(mats)]])
    return P.build(name, col, smooth=True)

def _dist_to_path(px, py, pts):
    best = 1e9
    for (x0, y0), (x1, y1) in zip(pts, pts[1:]):
        dx, dy = x1 - x0, y1 - y0; L2 = dx*dx + dy*dy or 1.0
        t = max(0.0, min(1.0, ((px - x0)*dx + (py - y0)*dy)/L2))
        best = min(best, math.hypot(px - x0 - t*dx, py - y0 - t*dy))
    return best

def _in_rect(px, py, cx, cy, ang, hx, hy):
    c, s = math.cos(-ang), math.sin(-ang); x, y = px - cx, py - cy
    return abs(c*x - s*y) < hx and abs(s*x + c*y) < hy

def density(px, py):
    """How much grass grows here (0-1)."""
    if J.height(px, py) < J.WATER_Z + 0.12: return 0.0
    for pl in J.PATHS:
        if _dist_to_path(px, py, pl) < 0.8: return 0.0
    if _dist_to_path(px, py, [J.STEPS_TOP, J.STEPS_BOTTOM]) < 1.5: return 0.0
    for (bx, by, ba), hx, hy in ((J.PENDAPA, 5.7, 6.9), (J.HUT_ELDERS, 3.0, 4.2), (J.HUT_SONS, 3.5, 4.5), (J.PAWON, 2.5, 2.0),
                                 (J.SANGGAR, 3.0, 2.2), (J.KANDHANG, 2.7, 2.2)):
        cyo = -1.0 if (bx, by) in (J.HUT_ELDERS[:2], J.HUT_SONS[:2]) else (-0.6 if (bx, by) == J.PENDAPA[:2] else 0.0)
        cxo = 0.7 if (bx, by) == J.SANGGAR[:2] else 0.0
        if _in_rect(px, py, bx + cxo, by + cyo, ba, hx, hy): return 0.0
    if math.hypot(px - J.GATE[0], py - J.GATE[1]) < 2.2: return 0.0
    if math.hypot(px - J.BANYAN[0], py - J.BANYAN[1]) < 3.2: return 0.15
    edge = math.hypot(px/30.0, (py - 4.0)/24.0)
    d = 1.0 if edge < 0.9 else max(0.0, 1.0 - (edge - 0.9)*2.5)
    return d*(1.0 - J.yard(px, py))                          # none in the bare yard round the pendapa and the huts

def grass(col, src_col, M, count=46000):
    gc = bpy.data.collections.new("_GrassClumps")               # kept out of the scene: the particles use it, nothing else shows it
    for k, mats in enumerate((("grass", "grass_dark"), ("grass", "grass", "grass_dry"), ("grass_dark", "grass", "leaf_light"))):
        _clump(gc, M, "GrassClump_%d" % k, 100 + k, mats)
    gc.hide_render = False
    x0, x1, y0, y1 = AREA; nx = int(round((x1 - x0)/STEP)) + 1; ny = int(round((y1 - y0)/STEP)) + 1
    v, w = [], []
    for j in range(ny):
        for i in range(nx):
            px, py = x0 + i*STEP, y0 + j*STEP; v.append((px, py, J.height(px, py) + 0.01)); w.append(density(px, py))
    f = [(j*nx + i, j*nx + i + 1, (j + 1)*nx + i + 1, (j + 1)*nx + i) for j in range(ny - 1) for i in range(nx - 1)]
    em = mesh.obj("Grass_Emitter", (v, f), col, M["grass"], recalc=False)
    vg = em.vertex_groups.new(name="grass")
    by = {}
    for i, wt in enumerate(w):
        if wt > 0: by.setdefault(round(wt, 2), []).append(i)
    for wt, idx in by.items(): vg.add(idx, wt, "REPLACE")
    md = em.modifiers.new("Grass", "PARTICLE_SYSTEM"); ps = md.particle_system; st = ps.settings; st.name = "Grass"
    st.type = "HAIR"; st.use_advanced_hair = True; st.count = count; st.emit_from = "FACE"
    st.render_type = "COLLECTION"; st.instance_collection = gc; st.use_collection_pick_random = True
    st.hair_length = 1.0                                         # instances are scaled by hair length too (default 4: grass 4x tall)
    st.particle_size = 1.0; st.size_random = 0.45
    st.use_rotations = True; st.rotation_mode = "GLOB_Z"; st.phase_factor_random = 2.0
    st.display_percentage = 10
    ps.vertex_group_density = "grass"; ps.seed = 7
    em.show_instancer_for_render = False; em.show_instancer_for_viewport = False
    return em
