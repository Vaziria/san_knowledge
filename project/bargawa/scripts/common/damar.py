"""The damar: Jamadagni's oil lamp, an Old Javanese bronze pole lamp (dipa) on a turned teak stand (the user, 2026-10-18:
"create new model in building. the model is javanese lamp", "do research first", then "do it all").
Research: docs/stories/ramabargawa/objects/damar.md. After the East Javanese pole lamps of the 14th-15th century (LACMA 82.5):
four burner dishes at right angles with foliate flanges round a tiered square plinth, on it an emaciated ascetic holding a
manuscript, flames rising behind him. Stylized and simplified.

Sizes in metres; the foot's underside at the origin; the ascetic faces south (-y).
    stand    a turned teak foot and shaft (teak_turned)
    socket   the bronze sleeve over the shaft's top
    plinth   three square tiers
    dishes   four leaf-shaped oil dishes on the diagonals (none stands in front of the ascetic), a spout at each tip, a leaf flange
             rising from each side, a curved strut below
    ascetic  cross-legged on a lotus cushion, a lontar manuscript in his hands, a topknot, a flame aureole (prabha) behind him
    oil, wicks   coconut oil in each dish; a twisted cotton wick lying in each spout, its tip sooted
    flames   one per wick, a piece of its own with a small warm light; FLAMES says where, so a light state can light any of them;
             drawn on cards by common/fire.py, moving with time
Pure geometry (no bpy); built into blender/objects/damar.blend by scripts/objects/damar.py."""
import math
from . import geo

H = 0.70                            # top of the teak stand; the bronze head sits on it
ZD = H + 0.068                      # the dishes' rim
Z0 = H + 0.08                       # the plinth's top, where the ascetic sits
DISH_R0, DISH_L, WALL = 0.04, 0.08, 0.0025   # a dish runs from radius 0.04 out to its spout tip at 0.12; its wall
ANGLES = [math.radians(45 + 90*k) for k in range(4)]
FLAME_H, FLAME_W = 0.034, 0.0058    # a flame's height and widest half-width (fire.flame_piece draws it on cards)

def _shape(t):
    """Half-width, depth and lift of a dish at t (0 at its inner end, 1 at the spout tip): a leaf, narrowing to a raised spout."""
    s = math.sin(math.pi*min(t/0.85, 1.0))
    return 0.006 + 0.018*s**0.7, 0.003 + 0.012*s**0.8, 0.006*max(0.0, (t - 0.7)/0.3)**2

def _at(ang, u, v, z):
    """A point of the dish at angle ang: u along it from its inner end, v across it."""
    r = DISH_R0 + u; c, s = math.cos(ang), math.sin(ang)
    return (c*r - s*v, s*r + c*v, z)

def _dish(ang, n=14, m=7):
    """The dish as a thin shell: each section is the outer U from rim to rim, then the inner U back. Rims carry wear 1."""
    rings, wear = [], []
    for i in range(n + 1):
        t = i/n; w, d, lift = _shape(t); u = DISH_L*t; z = ZD + lift
        wi, di = max(w - WALL, 0.0008), max(d - WALL, 0.0008)
        ring = [_at(ang, u, w*math.cos(math.pi*j/m), z - d*math.sin(math.pi*j/m)) for j in range(m + 1)]
        ring += [_at(ang, u, wi*math.cos(math.pi*(m - j)/m), z - di*math.sin(math.pi*(m - j)/m)) for j in range(m + 1)]
        rings.append(ring); wear += [1.0 if j in (0, m) else 0.0 for j in range(m + 1)]*2
    return geo.loft(rings, True, True), wear

def _oil(ang, n=10):
    """The oil's surface, a third of the way down each dish."""
    path, widths = [], []
    for i in range(n + 1):
        t = 0.06 + 0.76*i/n; w, d, lift = _shape(t); wi, di = w - WALL, d - WALL
        k = min(0.35*d/di, 0.95)
        path.append(_at(ang, DISH_L*t, 0.0, ZD + lift - 0.35*d)); widths.append(2*wi*math.sqrt(1 - k*k))
    return geo.ribbon(path, widths)

def _flanges(ang, n=10):
    """A leaf flange rising from each side of the dish and curling outward like a flame, its face towards the dish's length
    (a 5-point leaf read as a flat diamond). Polished towards the tip (wear)."""
    t = 0.42; w, d, lift = _shape(t); u = DISH_L*t; c, s = math.cos(ang), math.sin(ang); out = []
    for side in (-1, 1):
        b = (-s*side, c*side, 0.0)                                                       # across the dish, outward
        p = _at(ang, u, side*(w - WALL/2), ZD + lift - 0.002)
        path = geo.bezier(p, tuple(p[k] + b[k]*0.002 + (0.0, 0.0, 0.02)[k] for k in range(3)),
                          tuple(p[k] + b[k]*0.016 + (0.0, 0.0, 0.03)[k] for k in range(3)), n)
        widths = [0.0035*(1 - i/n) + 0.017*math.sin(math.pi*i/n)**0.9 for i in range(n + 1)]
        vf = geo.ribbon(path, widths, up=b)
        out.append((vf, [min(1.0, (i/n)**2*1.2) for i in range(n + 1) for _ in range(2)]))
    return out

def _strut(ang):
    """A curved strut from the socket out under the dish."""
    c, s = math.cos(ang), math.sin(ang); P = lambda r, z: (c*r, s*r, z)
    return geo.tube(geo.bezier(P(0.022, H + 0.034), P(0.064, H + 0.03), P(0.078, ZD - 0.012), 6), [0.0036, 0.0034, 0.0032, 0.003, 0.0028, 0.0027, 0.0026], n=12)

def _wick(ang):
    """A twisted cotton wick lying in the spout, its tip standing out past the spout and sooted. Returns (wick, soot, tip)."""
    w, d, lift = _shape(0.68)
    path = [_at(ang, DISH_L*0.66, 0.0, ZD + lift - 0.45*d), _at(ang, DISH_L*0.86, 0.0, ZD + _shape(0.86)[2] - 0.0005),
            _at(ang, DISH_L + 0.002, 0.0, ZD + 0.006 + 0.0025)]
    tip = _at(ang, DISH_L + 0.0055, 0.0, ZD + 0.006 + 0.0075)
    wick = geo.tube(path, [0.0021, 0.002, 0.0019], n=10, wobble=0.25, seed=ang)
    soot = geo.tube([path[-1], tip], [0.0019, 0.0013], n=10, cap0=False)
    return wick, soot, tip

FLAMES = [(lambda p: (p[0], p[1], p[2] + 0.0008))(_wick(a)[2]) for a in ANGLES]   # each flame's root, at its wick's tip

def stand():
    """The turned teak foot and shaft: [(vertices, faces)]."""
    foot = geo.lathe([(0.125, 0.0), (0.125, 0.018), (0.118, 0.024), (0.105, 0.028), (0.105, 0.04), (0.09, 0.046),
                      (0.065, 0.054), (0.045, 0.068), (0.032, 0.085)], n=28)
    shaft = geo.lathe([(0.032, 0.085), (0.026, 0.1), (0.022, 0.13), (0.03, 0.15), (0.022, 0.17), (0.019, 0.32), (0.024, 0.34),
                       (0.024, 0.355), (0.019, 0.375), (0.018, 0.56), (0.023, 0.58), (0.023, 0.595), (0.018, 0.615),
                       (0.018, 0.68), (0.026, 0.69), (0.026, H)], n=16)
    return [foot, shaft]

def _figure():
    """The ascetic on his lotus cushion, with the manuscript and the aureole: [(vf, wear)]. His knees and the top of his head are
    polished (wear), as on old bronzes that are touched."""
    z = Z0; out = []
    def worn(vf, f):
        return vf, [f(p) for p in vf[0]]
    out.append(worn(geo.lathe([(0.0225, z), (0.026, z + 0.004), (0.023, z + 0.008), (0.0, z + 0.0085)], n=16),
                    lambda p: 0.5 if p[2] > z + 0.003 and math.hypot(p[0], p[1]) > 0.024 else 0.0))
    out.append(worn(geo.ellipsoid(0.0, -0.003, z + 0.0145, 0.022, 0.015, 0.0068, nu=16, nv=8),
                    lambda p: 0.8 if p[1] < -0.009 and p[2] > z + 0.013 else 0.0))                  # the knees
    torso = [(0.0, z + 0.012, 0.0085, 0.0068), (0.002, z + 0.02, 0.0088, 0.0066),            # (y, z, half-width, half-depth)
             (0.002, z + 0.032, 0.0098, 0.0068), (0.002, z + 0.041, 0.0118, 0.0066),            # thin waist, bony chest,
             (0.002, z + 0.0465, 0.0124, 0.0058), (0.0015, z + 0.049, 0.0085, 0.0048),          # sloping shoulders, neck
             (0.001, z + 0.052, 0.0036, 0.0036)]
    out.append(worn(geo.loft([geo.ellipse(0.0, y, zz, a, b, 14) for y, zz, a, b in torso], True, True), lambda p: 0.0))
    out.append(worn(geo.ellipsoid(0.0, 0.0005, z + 0.0595, 0.0071, 0.0074, 0.0085, nu=12, nv=8),
                    lambda p: 0.7 if p[2] > z + 0.064 else 0.0))                                   # the crown of the head
    out.append(worn(geo.cylinder(0.0, 0.0015, z + 0.0655, 0.0046, 0.012, n=10, r1=0.0024), lambda p: 0.0))   # topknot
    for s in (-1, 1):                                                                               # arms to the lontar
        out.append(worn(geo.tube(geo.bezier((s*0.0118, 0.002, z + 0.0445), (s*0.0165, -0.002, z + 0.03),
                                            (s*0.0055, -0.0108, z + 0.0305), 6), [0.0031, 0.003, 0.0029, 0.0027, 0.0025, 0.0024, 0.0023], n=12),
                        lambda p: 0.0))
    out.append(worn(geo.box(0.0, -0.0125, z + 0.0275, 0.026, 0.0048, 0.006), lambda p: 0.3))      # the lontar
    outline = _aureole(z + 0.02, z + 0.104)
    out.append(worn(geo.moved(geo.prism(outline, 0.0108, 0.0128), ang=math.pi/2), lambda p: 0.0))
    return out

def _aureole(z0, z1, tongues=4, half=0.023):
    """The outline (x, z) of the flame aureole behind the ascetic: on each side, tongues that bulge out from a recess and curl
    up to a tip, then fall back in to the next recess higher up, ending in one tip at the top. (Tongues pointing straight out
    read as a stepped tower.)"""
    base = lambda s: half*(1 - s)**0.7
    side = []
    for k in range(tongues):
        s0, s1 = k/tongues, (k + 1)/tongues; za, zb = z0 + (z1 - z0)*s0, z0 + (z1 - z0)*s1
        r = (base(s0), za); tip = (base(s0) + 0.0058*(1 - s0)**0.4, za + 0.78*(zb - za)); nxt = (base(s1), zb)
        side += _bez2(r, (base(s0) + 0.0085*(1 - s0)**0.4, za + 0.2*(zb - za)), tip, 5)[:-1]
        side += _bez2(tip, (base(s1) + 0.0015, zb - 0.1*(zb - za)), nxt, 4)[:-1]
    side.append((0.0, z1 + 0.006))
    return side + [(-x, zz) for x, zz in reversed(side[:-1])]

def _bez2(p0, p1, p2, n):
    """n + 1 points (x, z) on a quadratic curve."""
    return [tuple((1 - t)**2*p0[k] + 2*(1 - t)*t*p1[k] + t*t*p2[k] for k in range(2)) for t in (i/n for i in range(n + 1))]

def head():
    """The bronze head, the oil and the wicks: {"bronze": [(vf, wear)], "oil": [vf], "wick": [vf], "soot": [vf]}."""
    bronze = []
    socket = geo.lathe([(0.029, H - 0.025), (0.029, H + 0.028), (0.0335, H + 0.033), (0.0335, H + 0.038), (0.027, H + 0.042),
                        (0.021, H + 0.046), (0.021, H + 0.05)], n=24)
    bronze.append((socket, [w for w in (0.0, 0.3, 1.0, 1.0, 0.3, 0.0, 0.0) for _ in range(24)]))
    tiers = [(0.036, H + 0.05), (0.036, H + 0.06), (0.031, H + 0.06), (0.031, H + 0.07), (0.026, H + 0.07), (0.026, Z0)]
    bronze.append((geo.molded([(a, a, zz) for a, zz in tiers]), [w for w in (0.0, 0.6, 0.0, 0.6, 0.0, 0.6) for _ in range(4)]))
    oil, wick, soot = [], [], []
    for a in ANGLES:
        bronze.append(_dish(a)); bronze += _flanges(a)
        st = _strut(a); bronze.append((st, [0.0]*len(st[0])))
        oil.append(_oil(a)); wk, so, _ = _wick(a); wick.append(wk); soot.append(so)
    bronze += _figure()
    return {"bronze": bronze, "oil": oil, "wick": wick, "soot": soot}
