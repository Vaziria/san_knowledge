"""Set dressing for Jatisrana's first state (S1, the family whole): Jamadagni's bale and the five mats in the pendapa, the oil lamp,
the woodcutter's stump, the kitchen's pots and woodpile, Renuka's kendi, the shrine's incense burner. Plus scale figures for review."""
import math, random
from . import mesh, geo, joglo, paths, jatisrana as J

LAMP = (0.0, 1.3)   # the damar, from the pendapa's centre: in the sacred centre inside the soko guru, between Jamadagni's bale
                    # (front edge y 2.25) and the sons' mats (back edge y 0.45), the ascetic facing the sons

def pendapa_dressing(col, M):
    x0, y0, _ = J.PENDAPA; F = joglo.F; P = mesh.Parts()
    P.add(geo.box(x0, y0 + 2.9, F, 2.0, 1.3, 0.36), M["teak"])                                   # Jamadagni's bale
    P.add(geo.box(x0, y0 + 2.9, F + 0.36, 1.8, 1.1, 0.03), M["mat"])
    for k in range(5):                                                                           # the five sons' mats
        P.add(geo.box(x0 - 2.8 + 1.4*k, y0 - 0.4, F, 1.0, 1.7, 0.02), M["mat"])
    damar = mesh.link_collection(paths.object_file("damar", built=True), "Damar")                # the oil lamp, all four lit (S1)
    mesh.place_collection(damar, "Damar", col, (x0 + LAMP[0], y0 + LAMP[1], F))
    return P.build("Pendapa_Dressing", col)

def yard_dressing(col, M):
    P = mesh.Parts(); rng = random.Random(3)
    sx, sy = J.STUMP; sz = J.height(sx, sy)
    P.add(geo.cylinder(sx, sy, sz - 0.1, 0.4, 0.62, n=12, r1=0.36), M["bark"])                 # the woodcutter's stump
    P.add(geo.beam((sx + 0.05, sy - 0.05, sz + 0.5), (sx + 0.35, sy - 0.6, sz + 0.02), 0.035, 0.035), M["teak"])   # an axe leaning on it
    P.add(geo.box(sx + 0.06, sy - 0.02, sz + 0.44, 0.06, 0.2, 0.14, ang=0.5), M["stone_dark"])
    px, py, pa = J.PAWON; pz = J.height(px, py)
    for k in range(3):                                                                           # pots by the kitchen
        a = pa + 0.6*k; ox, oy = geo.rot((2.6 + 0.4*k, -0.9, 0), pa)[:2]
        P.add(geo.ellipsoid(px + ox, py + oy, pz + 0.18, 0.2, 0.2, 0.2, nu=10, nv=6), M["clay"])
    for k in range(9):                                                                           # the woodpile
        ox, oy = geo.rot((-2.9, -0.4 + 0.16*(k % 5), 0), pa)[:2]; lz = pz + 0.07 + 0.14*(k // 5)
        a0 = geo.rot((-0.55, 0, 0), pa); a1 = geo.rot((0.55, 0, 0), pa)
        P.add(geo.beam((px + ox + a0[0], py + oy + a0[1], lz), (px + ox + a1[0], py + oy + a1[1], lz), 0.13, 0.13), M["bark"])
    hx, hy, _ = J.HUT_ELDERS; kx, ky = hx + 1.3, hy - 2.7; kz = J.height(hx, hy) + 0.3         # Renuka's kendi on the elders' porch,
    k = 0.54                                  # about 0.27 m tall, as an East Javanese kendi of c. 14th century (LACMA M.90.180.2:
    P.add(geo.ellipsoid(kx, ky, kz + 0.2*k, 0.17*k, 0.17*k, 0.18*k, nu=12, nv=7), M["clay"])            # 27.9 x 12.7 x 15.9 cm)
    P.add(geo.cylinder(kx, ky, kz + 0.34*k, 0.045*k, 0.16*k, n=8, r1=0.035*k), M["clay"])
    P.add(geo.cylinder(kx + 0.13*k, ky, kz + 0.25*k, 0.022*k, 0.1*k, n=6, r1=0.012*k, lean=(0.06*k, 0)), M["clay"])
    gx, gy, _ = J.SANGGAR; gz = J.height(gx, gy)                                                 # the incense burner on the altar stone
    P.add(geo.cylinder(gx + 0.97, gy, gz + 1.7, 0.07, 0.07, n=10, r1=0.09), M["stone_dark"])
    return P.build("Yard_Dressing", col, smooth=True)

def _figure(P, M, x, y, z, seated=False):
    if seated:
        P.add(geo.cylinder(x, y, z, 0.28, 0.55, n=10, r1=0.18), M["figure"]); P.add(geo.ellipsoid(x, y, z + 0.72, 0.11, 0.11, 0.13, nu=10, nv=6), M["figure"])
    else:
        P.add(geo.cylinder(x, y, z, 0.2, 1.42, n=10, r1=0.17), M["figure"]); P.add(geo.ellipsoid(x, y, z + 1.6, 0.11, 0.11, 0.13, nu=10, nv=6), M["figure"])

def scale_figures(col, M):
    """Plain 1.75 m figures where people will be, to judge scale (review only: not rendered unless a review turns them on)."""
    P = mesh.Parts(); F = joglo.F
    _figure(P, M, 0.0, 2.9, F + 0.39, seated=True)                                              # Jamadagni on the bale
    for k in range(5): _figure(P, M, -2.8 + 1.4*k, -0.4, F + 0.02, seated=True)                 # the five sons
    hx, hy, _ = J.HUT_ELDERS; _figure(P, M, hx + 0.6, hy - 2.9, J.height(hx, hy) + 0.3)         # Renuka on her porch
    bx, by = J.BATHING_STONE; _figure(P, M, bx, by, J.WATER_Z + 0.22)                           # someone on the bathing stone
    cx, cy = J.CITRARATA_SPOT; _figure(P, M, cx, cy, J.height(cx, cy))                          # the far bank
    return P.build("ScaleFigures", col, smooth=True)
