"""The other buildings of Jatisrana: the two huts, the pawon (kitchen), the sanggar pamujan (stone shrine), the gate, the kandhang
(livestock pen), and the stone steps down to the lake with the bathing stone. Each is built at the origin and moved into place.

Javanese names used here:
    saka      posts               blandar   wall plates          pengeret  tie beams across
    ander     king posts          molo      the ridge beam       gording   purlins
    usuk      rafters             gedhek    woven bamboo walls   emper     the porch under the lowered roof
    kusen     door and window frames                             lincak    a bamboo bench
    luweng    the clay stove      gentong   a big water jar      siwur     a coconut-shell ladle
    pipi tangga  the side walls of a stair                       ratna     the stone finials on a shrine's roof"""
import math
from . import mesh, geo, textures, jatisrana as J

THATCH = 0.24                    # thatch thickness on the huts
ROOF_STEP = 0.3                  # thatch faces at most this big: the thatch's relief subdivides them to 3.75 cm (textures.THATCH)

def _dense(prof, step=ROOF_STEP):
    """The profile with points added along each segment, at most `step` apart (the same shape)."""
    out = [prof[0]]
    for (ya, za), (yb, zb) in zip(prof, prof[1:]):
        n = max(1, math.ceil(math.hypot(yb - ya, zb - za)/step - 1e-6))
        out += [(ya + (yb - ya)*k/n, za + (zb - za)*k/n) for k in range(1, n + 1)]
    return out

def _roof_strip(x0, x1, prof, nseg=10, jitter=0.07, seed=0.0):
    """One side of a thatched roof: prof is [(y, z), ...] from the eave up to the ridge (y rising), rows along x. Faces up.
    Rows and columns are at most ROOF_STEP apart, for the thatch's relief."""
    rows = []; prof = _dense(prof); nseg = max(nseg, math.ceil((x1 - x0)/ROOF_STEP - 1e-6))
    for r, (y, z) in enumerate(prof):
        row = []
        for i in range(nseg + 1):
            x = x0 + (x1 - x0)*i/nseg
            dz = -jitter*geo.hash01(x, seed, 7) if r == 0 else 0.0
            dy = -0.4*jitter*geo.hash01(seed, x, 3) if r == 0 else 0.0
            row.append((x, y + dy, z + dz))
        rows.append(row)
    n = nseg + 1; v = [p for row in rows for p in row]
    f = [(r*n + i, r*n + i + 1, (r + 1)*n + i + 1, (r + 1)*n + i) for r in range(len(rows) - 1) for i in range(nseg)]
    return v, f

def _rafters(x0, x1, prof, spacing=0.5, w=0.07, d=0.09, thick=THATCH):
    """Rafters under a roof strip of the same profile, each following its bends."""
    pieces = []; x = x0
    while x <= x1 + 1e-6:
        for k, ((ya, za), (yb, zb)) in enumerate(zip(prof, prof[1:])):
            off = (thick + 0.02 + d/2)/math.cos(math.atan2(abs(zb - za), abs(yb - ya)))
            if k == 0:                                                                           # start just inside the eave
                t = 0.1/math.hypot(yb - ya, zb - za); ya, za = ya + (yb - ya)*t, za + (zb - za)*t
            pieces.append(geo.beam((x, ya, za - off), (x, yb, zb - off), w, d))
        x += spacing
    return geo.merge(*pieces)

def _wall(P, mat, rect_list, axis, at, thick=0.05):
    """Wall panels in the plane x = at (axis 'x') or y = at (axis 'y'); rect_list holds (u0, u1, z0, z1) along the other axis."""
    for u0, u1, z0, z1 in rect_list:
        if axis == "y": P.add(geo.box((u0 + u1)/2, at, z0, u1 - u0, thick, z1 - z0), mat)
        else: P.add(geo.box(at, (u0 + u1)/2, z0, thick, u1 - u0, z1 - z0), mat)

def _bars(P, mat, axis, at, u0, u1, z0, z1, n=5):
    """A window opening with vertical wooden bars and a frame."""
    for k in range(n):
        u = u0 + (u1 - u0)*(k + 0.5)/n
        P.add(geo.cylinder(u if axis == "y" else at, at if axis == "y" else u, z0, 0.022, z1 - z0, n=6), mat)
    for zz in (z0 - 0.06, z1):
        if axis == "y": P.add(geo.box((u0 + u1)/2, at, zz, u1 - u0 + 0.16, 0.1, 0.06), mat)
        else: P.add(geo.box(at, (u0 + u1)/2, zz, 0.1, u1 - u0 + 0.16, 0.06), mat)

def hut(col, M, name, x, y, ang=0.0, w=5.0, d=4.0, pd=1.6):
    """A hut on a low stone base: a timber frame with woven bamboo walls, a thatched gable roof whose south side drops into a
    porch roof on four posts, the door in the middle of the south wall, barred windows east and west, a bench on the porch.
    True to Javanese size (blender/places/jatisrana.md): the door 0.9 x 1.89 m, the porch beam 1.70 m over the porch floor, so
    the tallest people bow under it."""
    main = 0.786                                           # main slopes fall 0.786 m per metre (38 deg)
    # A deeper hut has a higher ridge at the same pitch, so the porch keeps its height (the sons' 4.5 m hut had 1.50 m under
    # its porch beam against the elders' 1.70 m): the slope breaks 0.35 m above the wall top on every hut.
    z = J.height(x, y); FZ, WH = 0.3, 2.2; top = FZ + WH; ridge = top + 0.35 + main*(d/2 + 0.1)
    emper = 0.325                                          # the porch roof, 18 deg
    brk = (-d/2 - 0.1, ridge - main*(d/2 + 0.1))           # where the south slope turns into the porch roof
    yp = -d/2 - pd + 0.15                                  # the porch posts' line
    z_at_p = brk[1] - (brk[0] - yp)*emper
    south = [(yp - 0.45, z_at_p - 0.45*emper), brk, (0.0, ridge)]
    north = [(-(d/2 + 0.7), ridge - main*(d/2 + 0.7)), (0.0, ridge)]               # built as a south side, then turned
    beam_top = z_at_p - THATCH/math.cos(math.atan(emper)) - 0.02
    T, TD, W = M["teak"], M["teak_dark"], M["woven"]
    P = mesh.Parts()
    # base, floor, step
    P.add(geo.molded([(w/2 + 0.3, d/2 + 0.3, -0.25), (w/2 + 0.3, d/2 + 0.3, FZ - 0.06), (w/2 + 0.34, d/2 + 0.34, FZ)], 0, 0), M["stone_dark"])
    P.add(geo.box(0, (yp - 0.3 - d/2)/2, -0.25, w + 0.6, abs(yp - 0.3 + d/2), FZ + 0.25), M["stone_dark"])
    P.add(geo.box(0, yp - 0.6, -0.25, 1.6, 0.5, FZ + 0.25 - 0.14), M["stone"])
    # frame: corner posts, two middle posts on the north wall, wall plates, tie beams, king posts, ridge beam, purlins
    posts = [(-w/2, -d/2), (w/2, -d/2), (-w/2, d/2), (w/2, d/2), (-w/6, d/2), (w/6, d/2)]
    for px, py in posts:
        P.add(geo.box(px, py, FZ, 0.28, 0.28, 0.1), M["stone_dark"]); P.add(geo.taper(px, py, FZ + 0.1, 0.16, 0.14, WH - 0.1), T)
    zp = top + 0.07
    for s in (-1, 1): P.add(geo.beam((-w/2 - 0.25, s*d/2, zp), (w/2 + 0.25, s*d/2, zp), 0.14, 0.14), T)
    for px in (-w/2, 0.0, w/2): P.add(geo.beam((px, -d/2 - 0.2, zp), (px, d/2 + 0.2, zp), 0.14, 0.14), T)
    for px in (-w/2, 0.0, w/2): P.add(geo.box(px, 0, zp + 0.07, 0.12, 0.12, ridge - zp - 0.42), T)
    P.add(geo.beam((-w/2 - 0.5, 0, ridge - 0.33), (w/2 + 0.5, 0, ridge - 0.33), 0.14, 0.14), T)
    for s in (-1, 1):
        yy = s*d/4; P.add(geo.beam((-w/2 - 0.5, yy, ridge - main*abs(yy) - 0.37), (w/2 + 0.5, yy, ridge - main*abs(yy) - 0.37), 0.12, 0.12), T)
    # rafters under both roof sides
    P.add(_rafters(-w/2 - 0.4, w/2 + 0.4, south), TD)
    P.add(geo.moved(_rafters(-w/2 - 0.4, w/2 + 0.4, north), ang=math.pi), TD)
    # porch: four posts on stones and their beam; the middle two flank the door, so none stands in its way
    for px in (-w/2 + 0.1, -0.95, 0.95, w/2 - 0.1):
        P.add(geo.box(px, yp, FZ, 0.26, 0.26, 0.12), M["stone_dark"]); P.add(geo.taper(px, yp, FZ + 0.12, 0.15, 0.13, beam_top - 0.14 - FZ - 0.12), T)
    P.add(geo.beam((-w/2 - 0.3, yp, beam_top - 0.07), (w/2 + 0.3, yp, beam_top - 0.07), 0.14, 0.14), T)
    # woven walls with a bamboo rail top and bottom; the door bay in the south wall; windows east and west
    dw, dh = 0.9, 1.85; wz0, wz1, wy0, wy1 = FZ + 1.0, FZ + 1.55, 0.05, 0.85
    _wall(P, W, [(-w/2, w/2, FZ, top)], "y", d/2)
    _wall(P, W, [(-w/2, -dw/2 - 0.06, FZ, top), (dw/2 + 0.06, w/2, FZ, top), (-dw/2 - 0.06, dw/2 + 0.06, FZ + dh + 0.1, top)], "y", -d/2)
    for s in (-1, 1):
        _wall(P, W, [(-d/2, d/2, FZ, wz0), (-d/2, d/2, wz1, top), (-d/2, wy0, wz0, wz1), (wy1, d/2, wz0, wz1)], "x", s*w/2)
        _bars(P, TD, "x", s*w/2, wy0, wy1, wz0, wz1)
        P.add(([(s*w/2*1.004, -d/2, top + 0.14), (s*w/2*1.004, d/2, top + 0.14), (s*w/2*1.004, 0, ridge - 0.4)], [(0, 1, 2)]), W)   # gable
    for at in (d/2 + 0.03, -d/2 - 0.03):                                                       # bamboo rails along the long walls
        for zz in (FZ + 0.06, top - 0.06): P.add(geo.beam((-w/2, at, zz), (w/2, at, zz), 0.06, 0.06), M["bamboo"])
    # the door: frame, a leaf of four planks with two battens
    for sx in (-1, 1): P.add(geo.box(sx*(dw/2 + 0.05), -d/2 - 0.03, FZ, 0.1, 0.12, dh + 0.1), TD)
    P.add(geo.box(0, -d/2 - 0.03, FZ + dh + 0.04, dw + 0.4, 0.12, 0.1), TD)
    for k in range(4): P.add(geo.box(-dw/2 + dw*(k + 0.5)/4, -d/2 - 0.01, FZ, dw/4 - 0.012, 0.04, dh), T)
    for zz in (FZ + 0.35, FZ + 1.45): P.add(geo.box(0, -d/2 - 0.045, zz, dw - 0.08, 0.03, 0.1), TD)
    # a lincak on the porch
    bx, by, bz = -w/4 - 0.3, -d/2 - pd/2, FZ + 0.42
    for sx in (-1, 1):
        for sy in (-1, 1): P.add(geo.cylinder(bx + sx*0.72, by + sy*0.22, FZ, 0.03, 0.42, n=6), M["bamboo"])
    for k in range(6): P.add(geo.beam((bx - 0.8, by - 0.25 + 0.1*k, bz), (bx + 0.8, by - 0.25 + 0.1*k, bz), 0.07, 0.04), M["bamboo"])
    body = P.build(name, col, loc=(x, y, z), rot_z=ang)
    roof = mesh.Parts().add(geo.merge(_roof_strip(-w/2 - 0.5, w/2 + 0.5, south, seed=x + y),
                                      geo.moved(_roof_strip(-w/2 - 0.5, w/2 + 0.5, north, seed=x - y), ang=math.pi)), M["thatch"])
    roof = roof.build(name + "_Roof", col, loc=(x, y, z), rot_z=ang, recalc=False, weld=1e-4, solidify=THATCH)
    textures.thatch_relief(roof)                                        # the sasak thatch's real relief
    R = mesh.Parts()
    R.add(geo.tube([(-w/2 - 0.55, 0, ridge + 0.05), (w/2 + 0.55, 0, ridge + 0.05)], 0.17, n=10), M["ridge"])
    for s in (-1, 1): R.add(geo.ellipsoid(s*(w/2 + 0.55), 0, ridge + 0.05, 0.12, 0.2, 0.2, nu=8, nv=6), M["ridge"])
    R.build(name + "_Ridge", col, loc=(x, y, z), rot_z=ang, smooth=True)
    return body, roof

def pawon(col, M, x, y, ang=0.0, w=4.4, d=3.4):
    """The kitchen: an earth-floored shed, woven walls on the north and west (low on the east, open to the south), the clay
    stove with a pot and a wok, a water jar, a shelf of pots, a drying rack over the fire, firewood, and a raised cap over the
    ridge to let the smoke out."""
    z = J.height(x, y); T, B, W = M["teak"], M["bamboo"], M["woven"]; P = mesh.Parts(); top = 2.15
    P.add(geo.box(0, 0, -0.2, w + 0.4, d + 0.4, 0.3), M["earth"])
    for px in (-w/2, 0.0, w/2):
        for py in (-d/2, d/2): P.add(geo.cylinder(px, py, 0.1, 0.075, top - 0.1, n=8), T)
    for s in (-1, 1): P.add(geo.beam((-w/2 - 0.2, s*d/2, top + 0.06), (w/2 + 0.2, s*d/2, top + 0.06), 0.13, 0.13), T)
    for px in (-w/2, w/2): P.add(geo.beam((px, -d/2 - 0.2, top + 0.06), (px, d/2 + 0.2, top + 0.06), 0.13, 0.13), T)
    _wall(P, W, [(-w/2, w/2, 0.1, top)], "y", d/2)
    _wall(P, W, [(-d/2, d/2, 0.1, top)], "x", -w/2)
    _wall(P, W, [(-d/2, d/2, 0.1, 1.0)], "x", w/2)
    sx0, sy0 = 0.35, d/2 - 0.55                                                                  # the luweng
    P.add(geo.molded([(0.85, 0.38, 0.1), (0.85, 0.38, 0.62), (0.88, 0.41, 0.68), (0.88, 0.41, 0.72)], sx0, sy0), M["clay_dark"])
    for hx in (-0.4, 0.42):
        P.add(geo.cylinder(sx0 + hx, sy0, 0.71, 0.17, 0.02, n=12), M["door"])
        P.add(geo.box(sx0 + hx, sy0 - 0.4, 0.14, 0.3, 0.04, 0.28), M["door"])
    P.add(geo.ellipsoid(sx0 - 0.4, sy0, 0.9, 0.2, 0.2, 0.19, nu=12, nv=7), M["soot"])         # kendil
    P.add(geo.ellipsoid(sx0 + 0.42, sy0, 0.78, 0.3, 0.3, 0.07, nu=14, nv=5), M["soot"])       # wajan
    gx, gy = -w/2 + 0.6, d/2 - 0.6                                                               # gentong, lid, siwur
    P.add(geo.ellipsoid(gx, gy, 0.48, 0.36, 0.36, 0.42, nu=14, nv=8), M["clay"])
    P.add(geo.cylinder(gx, gy, 0.86, 0.26, 0.03, n=14), T)
    P.add(geo.beam((gx, gy, 0.92), (gx + 0.35, gy - 0.25, 0.98), 0.025, 0.025), M["bark"])
    P.add(geo.ellipsoid(gx - 0.02, gy + 0.01, 0.93, 0.07, 0.07, 0.04, nu=8, nv=4), M["bark_dark"])
    for zz in (1.25, 1.6):                                                                       # a shelf of pots on the north wall
        P.add(geo.box(-0.9, d/2 - 0.2, zz, 1.4, 0.3, 0.04), B)
        for k in range(3): P.add(geo.ellipsoid(-1.4 + 0.45*k, d/2 - 0.2, zz + 0.14, 0.12, 0.12, 0.12, nu=10, nv=6), M["clay"])
    for k in range(7): P.add(geo.beam((sx0 - 0.9, sy0 - 0.45 + 0.15*k, 1.9), (sx0 + 0.9, sy0 - 0.45 + 0.15*k, 1.9), 0.05, 0.05), B)   # para-para
    for k in range(10):                                                                          # firewood along the west wall
        ly = -d/2 + 0.4 + 0.14*(k % 5); lz = 0.17 + 0.13*(k // 5)
        P.add(geo.beam((-w/2 + 0.2, ly, lz), (-w/2 + 1.2, ly + 0.03*(k % 3 - 1), lz), 0.12, 0.12), M["bark"])
    body = P.build("Pawon", col, loc=(x, y, z), rot_z=ang)
    ridge, slope, slot = 3.5, 0.63, 0.25
    side = [(-(d/2 + 0.6), ridge - slope*(d/2 + 0.6)), (-slot, ridge - slope*slot)]
    cap = [(-0.65, ridge - slope*0.65 + 0.3), (0.0, ridge + 0.36)]
    R = mesh.Parts()
    R.add(geo.merge(_roof_strip(-w/2 - 0.5, w/2 + 0.5, side, seed=x), geo.moved(_roof_strip(-w/2 - 0.5, w/2 + 0.5, side, seed=y), ang=math.pi)), M["thatch"])
    R.add(geo.merge(_roof_strip(-1.3, 1.3, cap, nseg=4, seed=x + 1), geo.moved(_roof_strip(-1.3, 1.3, cap, nseg=4, seed=y + 1), ang=math.pi)), M["thatch"])
    textures.thatch_relief(R.build("Pawon_Roof", col, loc=(x, y, z), rot_z=ang, recalc=False, weld=1e-4, solidify=0.2))
    F = mesh.Parts()
    for s in (-1, 1):
        for sx in (-1.1, 1.1): F.add(geo.cylinder(sx, s*0.3, ridge - slope*0.3 - 0.25, 0.04, 0.45, n=6), T)
    F.add(geo.beam((-w/2 - 0.5, 0, ridge - 0.25), (w/2 + 0.5, 0, ridge - 0.25), 0.13, 0.13), T)
    F.build("Pawon_RidgeFrame", col, loc=(x, y, z), rot_z=ang)
    return body

def _prism_xz(points, y0, y1):
    """A flat shape in the x-z plane (points (x, z)) given thickness from y0 to y1."""
    v, f = geo.prism([(px, pz) for px, pz in points], y0, y1)
    return [(b, a, c) for a, b, c in v], f                       # prism builds in y-z: swap x and y

def sanggar(col, M, x, y, ang=0.0):
    """The sanggar pamujan: a small andesite shrine in the old Javanese manner. Three moulded base tiers, a body with corner
    pilasters and a framed niche under a plain kala boss, three roof tiers with small ratna at their corners and a crowning
    ratna; the stair between its side walls faces east (+x as built)."""
    z = J.height(x, y); P = mesh.Parts(); S, D = M["stone"], M["stone_dark"]
    band = lambda h, z0, z1: [(h, h, z0), (h, h, z0 + 0.06), (h - 0.07, h - 0.07, z0 + 0.12), (h - 0.07, h - 0.07, z1 - 0.1),
                              (h + 0.0, h + 0.0, z1 - 0.05), (h + 0.02, h + 0.02, z1)]
    P.add(geo.molded(band(2.05, -0.3, 0.45)), S)
    P.add(geo.molded(band(1.6, 0.45, 0.95)), S)
    P.add(geo.molded(band(1.2, 0.95, 1.4)), D)
    P.add(geo.molded([(0.82, 0.82, 1.4), (0.82, 0.82, 1.48), (0.76, 0.76, 1.54), (0.76, 0.76, 2.66), (0.84, 0.84, 2.72),
                      (0.92, 0.92, 2.82), (0.92, 0.92, 2.92)]), D)
    for sx in (-1, 1):
        for sy in (-1, 1): P.add(geo.box(sx*0.73, sy*0.73, 1.54, 0.16, 0.16, 1.12), D)
    P.add(geo.box(0.77, 0, 1.62, 0.04, 0.6, 0.86), M["door"])                                  # the niche
    for sy in (-1, 1): P.add(geo.box(0.8, sy*0.37, 1.58, 0.08, 0.13, 0.98), S)
    P.add(geo.box(0.8, 0, 2.48, 0.1, 0.88, 0.15), S)
    P.add(geo.ellipsoid(0.86, 0, 2.62, 0.08, 0.17, 0.11, nu=10, nv=6), S)                     # kala boss over the lintel
    for (h, zb, th) in ((0.86, 2.92, 0.36), (0.66, 3.28, 0.32), (0.48, 3.6, 0.28)):
        P.add(geo.molded([(h, h, zb), (h, h, zb + th*0.25), (h - 0.05, h - 0.05, zb + th*0.4), (h - 0.05, h - 0.05, zb + th*0.8),
                          (h + 0.02, h + 0.02, zb + th*0.88), (h + 0.02, h + 0.02, zb + th)]), D)
        for sx in (-1, 1):
            for sy in (-1, 1):
                P.add(geo.cylinder(sx*(h - 0.07), sy*(h - 0.07), zb + th, 0.07, 0.12, n=8, r1=0.05), S)
                P.add(geo.cylinder(sx*(h - 0.07), sy*(h - 0.07), zb + th + 0.12, 0.05, 0.15, n=8, r1=0.0), S)
    P.add(geo.cylinder(0, 0, 3.88, 0.3, 0.1, n=12, r1=0.26), S)                                 # the crowning ratna
    P.add(geo.ellipsoid(0, 0, 4.12, 0.22, 0.22, 0.2, nu=12, nv=8), S)
    P.add(geo.cylinder(0, 0, 4.28, 0.1, 0.08, n=10), S)
    P.add(geo.cylinder(0, 0, 4.36, 0.07, 0.42, n=8, r1=0.0), S)
    for k in range(1, 5):                     # the stair: four equal steps of 0.19 m, treads 0.27 m, up to the second tier (0.95);
        x0 = 1.62 + 0.27*(4 - k)              # 2R + T = 0.65 m, steep as a shrine's stair (blender/places/jatisrana.md)
        P.add(geo.box(x0 + 0.125, 0, -0.3, 0.29, 1.3, 0.19*k + 0.3), S)
    for sy in (-1, 1):                                                                           # pipi tangga, ending in a scroll
        y0, y1 = sorted((sy*0.66, sy*0.84))
        P.add(_prism_xz([(1.55, -0.3), (2.75, -0.3), (2.75, 0.3), (1.55, 1.08)], y0, y1), D)
        P.add(geo.tube([(2.75, y0 - 0.02, 0.24), (2.75, y1 + 0.02, 0.24)], 0.13, n=10), D)
    P.add(geo.box(0.97, 0, 1.4, 0.3, 0.7, 0.3), S)                                              # the altar stone before the niche
    P.add(geo.ellipsoid(4.1, 0, 0.02, 0.75, 0.65, 0.16, nu=12, nv=6), S)                       # a flat stone to sit on, east of the stair
    return P.build("Sanggar", col, loc=(x, y, z), rot_z=ang)

def gate(col, M, x, y, ang=0.0):
    """A stone gate: two moulded pillars with stepped crowns and small ratna, and low walls with a coping; the path goes through
    west-east."""
    z = J.height(x, y); P = mesh.Parts(); S, D = M["stone"], M["stone_dark"]
    for s in (-1, 1):
        cy = s*1.55
        P.add(geo.molded([(0.56, 0.56, -0.2), (0.56, 0.56, 0.16), (0.48, 0.48, 0.22), (0.43, 0.43, 0.3), (0.43, 0.43, 1.92),
                          (0.49, 0.49, 2.0), (0.53, 0.53, 2.1), (0.53, 0.53, 2.2), (0.45, 0.45, 2.26), (0.45, 0.45, 2.46),
                          (0.37, 0.37, 2.5), (0.37, 0.37, 2.66), (0.29, 0.29, 2.7), (0.29, 0.29, 2.82)], 0, cy), S)
        P.add(geo.ellipsoid(0, cy, 2.98, 0.17, 0.17, 0.16, nu=10, nv=6), D)
        P.add(geo.cylinder(0, cy, 3.1, 0.06, 0.3, n=8, r1=0.0), D)
        P.add(geo.molded([(0.24, 1.25, -0.2), (0.24, 1.25, 0.95), (0.3, 1.25, 1.0), (0.3, 1.25, 1.08), (0.2, 1.25, 1.16)], 0, s*3.35), S)
    return P.build("Gate", col, loc=(x, y, z), rot_z=ang)

def kandhang(col, M, x, y, ang=0.0, w=5.0, d=4.0):
    """A bamboo pen for the livestock: posts and two rails, a gate in the south side, a feeding trough, straw on the ground,
    and a small lean-to roof over its north half."""
    z = J.height(x, y); P = mesh.Parts(); B = M["bamboo"]
    corners = [(-w/2, -d/2), (w/2, -d/2), (w/2, d/2), (-w/2, d/2)]
    for (ax, ay), (bx, by) in zip(corners, corners[1:] + corners[:1]):
        n = max(2, int(math.hypot(bx - ax, by - ay)/1.0))
        for k in range(n): P.add(geo.cylinder(ax + (bx - ax)*k/n, ay + (by - ay)*k/n, -0.1, 0.055, 1.4, n=6), B)
        for hz in (0.5, 1.05):
            if ay == by == -d/2:                                                                 # leave the gate gap in the south side
                P.add(geo.beam((ax, ay, hz), (-0.6, ay, hz), 0.06, 0.06), B); P.add(geo.beam((0.6, ay, hz), (bx, by, hz), 0.06, 0.06), B)
            else: P.add(geo.beam((ax, ay, hz), (bx, by, hz), 0.06, 0.06), B)
    for k in range(4): P.add(geo.beam((-0.55, -d/2 - 0.12, 0.25 + 0.3*k), (0.55, -d/2 - 0.12, 0.25 + 0.3*k), 0.05, 0.05), B)   # the gate
    P.add(geo.molded([(0.8, 0.22, 0.0), (0.8, 0.22, 0.32), (0.85, 0.27, 0.36)], 0.6, d/2 - 0.5), M["teak"])                    # trough
    P.add(geo.box(0, 0.3, -0.05, w - 0.3, d - 1.0, 0.07), M["thatch"])                                                           # straw
    for px, py, h in ((-w/2 + 0.2, 0.0, 2.1), (w/2 - 0.2, 0.0, 2.1), (-w/2 + 0.2, d/2 - 0.1, 1.75), (w/2 - 0.2, d/2 - 0.1, 1.75)):
        P.add(geo.cylinder(px, py, -0.1, 0.07, h + 0.1, n=6), M["teak"])
    body = P.build("Kandhang", col, loc=(x, y, z), rot_z=ang)
    slope = _roof_strip(-w/2 - 0.3, w/2 + 0.3, [(-0.35, 2.12), (d/2 + 0.35, 1.72)], nseg=5, seed=y)
    textures.thatch_relief(mesh.Parts().add(slope, M["thatch"]).build("Kandhang_Roof", col, loc=(x, y, z), rot_z=ang, recalc=False,
                                                                      solidify=0.18))
    return body

def lake_steps(col, M):
    """Stone steps from the terrace's edge down to the bathing stone, and the stone itself: J.STEPS_N equal steps on a straight
    line; the ground is cut to that line (jatisrana.height), so none is buried."""
    (x0, y0), (x1, y1) = J.STEPS_TOP, J.STEPS_BOTTOM; n = J.STEPS_N
    z0, z1 = J.STEPS_Z0, J.STEPS_Z1
    th = math.atan2(y1 - y0, x1 - x0); tread = math.hypot(x1 - x0, y1 - y0)/n
    P = mesh.Parts()
    for k in range(n):
        t = (k + 0.5)/n; zt = z0 + (z1 - z0)*k/(n - 1)
        P.add(geo.box(x0 + (x1 - x0)*t, y0 + (y1 - y0)*t, zt - 0.8, J.STEPS_W, tread + 0.04, 0.8, ang=th - math.pi/2), M["stone"])
    bx, by = J.BATHING_STONE
    P.add(geo.ellipsoid(bx, by, J.WATER_Z + 0.22 - 0.32, 1.35, 1.0, 0.32, nu=14, nv=6), M["stone_dark"])
    return P.build("Lake_Steps", col)
