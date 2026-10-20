"""The ground of Jatisrana: terrain, the lake's water, the paths, and the mountains behind."""
import bpy, math
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
