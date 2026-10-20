"""Vertex/face lists for simple solids (no bpy). Every function returns (verts, faces); combine them with mesh.Parts.

Rings are closed loops, counter-clockwise seen from above, joined bottom to top, so lofted solids face outward."""
import math

def rot(p, ang, cx=0.0, cy=0.0):
    """p turned by ang (radians) about the vertical axis through (cx, cy)."""
    if not ang: return p
    c, s = math.cos(ang), math.sin(ang); x, y = p[0] - cx, p[1] - cy
    return (cx + c*x - s*y, cy + s*x + c*y, p[2])

def loft(rings, cap0=False, cap1=False):
    """Rings (lists of (x, y, z), all the same length) joined in order by quads; cap0 / cap1 close the first / last ring."""
    n = len(rings[0]); v = [p for r in rings for p in r]; f = []
    for i in range(len(rings) - 1):
        a, b = i*n, (i + 1)*n
        for j in range(n):
            k = (j + 1) % n
            f.append((a + j, a + k, b + k, b + j))
    if cap0: f.append(tuple(reversed(range(n))))
    if cap1: f.append(tuple(range((len(rings) - 1)*n, len(rings)*n)))
    return v, f

def merge(*vfs):
    v, f = [], []
    for vv, ff in vfs:
        o = len(v); v += list(vv); f += [tuple(i + o for i in fc) for fc in ff]
    return v, f

def moved(vf, dx=0.0, dy=0.0, dz=0.0, ang=0.0):
    """vf turned by ang about the origin, then moved."""
    v, f = vf
    return [(q[0] + dx, q[1] + dy, q[2] + dz) for q in (rot(p, ang) for p in v)], f

def rect(cx, cy, z, hx, hy, ang=0.0):
    return [rot((cx + x, cy + y, z), ang, cx, cy) for x, y in ((-hx, -hy), (hx, -hy), (hx, hy), (-hx, hy))]

def box(cx, cy, z0, sx, sy, sz, ang=0.0):
    """A box standing on z0, centred on (cx, cy)."""
    return loft([rect(cx, cy, z0, sx/2, sy/2, ang), rect(cx, cy, z0 + sz, sx/2, sy/2, ang)], True, True)

def taper(cx, cy, z0, s0, s1, h, ang=0.0):
    """A square post or base that narrows from side s0 at the bottom to s1 at the top."""
    return loft([rect(cx, cy, z0, s0/2, s0/2, ang), rect(cx, cy, z0 + h, s1/2, s1/2, ang)], True, True)

def ellipse(cx, cy, z, rx, ry, n, phase=0.0):
    return [(cx + rx*math.cos(phase + 2*math.pi*i/n), cy + ry*math.sin(phase + 2*math.pi*i/n), z) for i in range(n)]

def cylinder(cx, cy, z0, r0, h, n=12, r1=None, lean=(0.0, 0.0)):
    """A round post or cone (r1 = 0 makes a point); lean moves the top sideways."""
    r1 = r0 if r1 is None else max(r1, 1e-4)
    return loft([ellipse(cx, cy, z0, r0, r0, n), ellipse(cx + lean[0], cy + lean[1], z0 + h, r1, r1, n)], True, True)

def lathe(profile, n=24, cx=0.0, cy=0.0, cap0=True, cap1=True):
    """A turned solid: profile is a list of (radius, z) from the bottom up, spun round the vertical axis through (cx, cy).
    A radius of 0 at an end makes a point there."""
    rings = [ellipse(cx, cy, z, max(r, 1e-4), max(r, 1e-4), n) for r, z in profile]
    return loft(rings, cap0, cap1)

def cards(half_w, z0, z1, n=3):
    """n upright quads through the vertical axis, turned 180/n degrees apart, half_w either side of it, from z0 to z1: something
    drawn by its shader on them (a flame) reads from any side, not only from the front. Straight down they show edge-on."""
    v, f = [], []
    for k in range(n):
        c, s = half_w*math.cos(math.pi*k/n), half_w*math.sin(math.pi*k/n); i = len(v)
        v += [(-c, -s, z0), (c, s, z0), (c, s, z1), (-c, -s, z1)]; f.append((i, i + 1, i + 2, i + 3))
    return v, f

def ellipsoid(cx, cy, cz, rx, ry, rz, nu=14, nv=8):
    v = [(cx, cy, cz - rz)]
    for j in range(1, nv):
        t = math.pi*j/nv; z = cz - rz*math.cos(t); s = math.sin(t)
        v += [(cx + rx*s*math.cos(2*math.pi*i/nu), cy + ry*s*math.sin(2*math.pi*i/nu), z) for i in range(nu)]
    v.append((cx, cy, cz + rz)); top = len(v) - 1; f = []
    for i in range(nu): f.append((0, 1 + (i + 1) % nu, 1 + i))
    for j in range(nv - 2):
        a, b = 1 + j*nu, 1 + (j + 1)*nu
        for i in range(nu):
            k = (i + 1) % nu; f.append((a + i, a + k, b + k, b + i))
    a = 1 + (nv - 2)*nu
    for i in range(nu): f.append((a + i, a + (i + 1) % nu, top))
    return v, f

def beam(p0, p1, w, d):
    """A rectangular beam along the centre line p0 -> p1, w wide (sideways) and d deep (up)."""
    dx, dy, dz = p1[0] - p0[0], p1[1] - p0[1], p1[2] - p0[2]; L = math.sqrt(dx*dx + dy*dy + dz*dz)
    ux, uy, uz = dx/L, dy/L, dz/L
    sx, sy = -uy, ux; sl = math.hypot(sx, sy)
    if sl < 1e-6: sx, sy, sl = 1.0, 0.0, 1.0
    sx, sy = sx/sl, sy/sl
    upx, upy, upz = -uz*sy, uz*sx, ux*sy - uy*sx                      # u x s
    def ring(c):
        return [(c[0] + a*sx*w/2 + b*upx*d/2, c[1] + a*sy*w/2 + b*upy*d/2, c[2] + b*upz*d/2)
                for a, b in ((-1, -1), (1, -1), (1, 1), (-1, 1))]
    return loft([ring(p0), ring(p1)], True, True)

def molded(profile, cx=0.0, cy=0.0, ang=0.0):
    """A stone block with mouldings: profile is a list of (half-width x, half-width y, z) from the bottom up; each pair of rings
    is joined, so a step inwards makes a recessed band and a step outwards a cornice."""
    rings = [rect(cx, cy, z, hx, hy, ang) for hx, hy, z in profile]
    return loft(rings, True, True)

def prism(points, x0, x1):
    """A flat shape in the y-z plane (points (y, z), any order round it) given thickness from x0 to x1."""
    a = [(x0, y, z) for y, z in points]; b = [(x1, y, z) for y, z in points]; n = len(points)
    f = [tuple(range(n)), tuple(n + i for i in reversed(range(n)))]
    f += [(i, (i + 1) % n, n + (i + 1) % n, n + i) for i in range(n)]
    return a + b, f

def _frames(path):
    """A side and an up vector at every point of a path (parallel transport, so tubes don't twist)."""
    import math as m
    tang = []
    for i in range(len(path)):
        a, b = path[max(i - 1, 0)], path[min(i + 1, len(path) - 1)]
        t = [b[k] - a[k] for k in range(3)]; L = m.sqrt(sum(c*c for c in t)) or 1.0; tang.append([c/L for c in t])
    t0 = tang[0]; ref = (0.0, 0.0, 1.0) if abs(t0[2]) < 0.9 else (1.0, 0.0, 0.0)
    s = [t0[1]*ref[2] - t0[2]*ref[1], t0[2]*ref[0] - t0[0]*ref[2], t0[0]*ref[1] - t0[1]*ref[0]]
    L = m.sqrt(sum(c*c for c in s)); s = [c/L for c in s]; out = []
    for t in tang:
        d = sum(s[k]*t[k] for k in range(3)); s = [s[k] - d*t[k] for k in range(3)]
        L = m.sqrt(sum(c*c for c in s)) or 1.0; s = [c/L for c in s]
        u = [t[1]*s[2] - t[2]*s[1], t[2]*s[0] - t[0]*s[2], t[0]*s[1] - t[1]*s[0]]
        out.append((s, u))
    return out

def tube(path, radii, n=8, cap0=True, cap1=True, wobble=0.0, seed=0.0):
    """A round tube along a path of (x, y, z) points, radius radii[i] at each (a number gives one radius everywhere).
    wobble makes the section a little irregular, like bark."""
    if not hasattr(radii, "__len__"): radii = [radii]*len(path)
    rings = []
    for i, (p, (s, u)) in enumerate(zip(path, _frames(path))):
        ring = []
        for j in range(n):
            a = 2*math.pi*j/n; r = radii[i]*(1 + wobble*(hash01(i, j, seed) - 0.5))
            ring.append(tuple(p[k] + r*(math.cos(a)*s[k] + math.sin(a)*u[k]) for k in range(3)))
        rings.append(ring)
    v, f = loft(rings, cap0, cap1)
    return v, f

def _norm(v):
    L = math.sqrt(sum(c*c for c in v)) or 1.0
    return tuple(c/L for c in v)

def _cross(a, b):
    return (a[1]*b[2] - a[2]*b[1], a[2]*b[0] - a[0]*b[2], a[0]*b[1] - a[1]*b[0])

def leaf(p, d, n, length, width, fold=0.15, curl=0.0):
    """One leaf as 5 points and 4 triangles: from p along direction d, its face towards n. fold lifts the midrib (a V-shaped
    leaf), curl bends the tip towards -n (a drooping or cupped leaf)."""
    d = _norm(d); s = _norm(_cross(d, n)); n = _norm(_cross(s, d))
    at = lambda a, b, c: tuple(p[k] + d[k]*a + s[k]*b + n[k]*c for k in range(3))
    v = [at(0, 0, 0), at(0.45*length, 0, fold*width), at(0.45*length, -width/2, 0), at(length, 0, -curl*length), at(0.45*length, width/2, 0)]
    return v, [(0, 2, 1), (1, 2, 3), (0, 1, 4), (1, 3, 4)]

def ribbon(path, widths, up=(0.0, 0.0, 1.0)):
    """A flat strip along a path (a grass blade, a strand of a plume): widths[i] across at each point, flat towards `up`."""
    v, f = [], []
    for i, p in enumerate(path):
        a, b = path[max(i - 1, 0)], path[min(i + 1, len(path) - 1)]
        t = _norm(tuple(b[k] - a[k] for k in range(3))); s = _norm(_cross(t, up))
        if s == (0.0, 0.0, 0.0): s = (1.0, 0.0, 0.0)
        w = widths[i]/2; v += [tuple(p[k] - s[k]*w for k in range(3)), tuple(p[k] + s[k]*w for k in range(3))]
    f = [(2*i, 2*i + 1, 2*i + 3, 2*i + 2) for i in range(len(path) - 1)]
    return v, f

def bezier(p0, p1, p2, n):
    """n + 1 points on the quadratic curve from p0 to p2, pulled towards p1."""
    out = []
    for i in range(n + 1):
        t = i/n; a, b, c = (1 - t)**2, 2*(1 - t)*t, t*t
        out.append(tuple(a*p0[k] + b*p1[k] + c*p2[k] for k in range(3)))
    return out

def square_ring(a, z, n, cx=0.0, cy=0.0):
    """A square loop of half-size a with n points per side, counter-clockwise from the south-west corner."""
    pts = []
    for side in range(4):
        for i in range(n):
            t = -a + 2*a*i/n
            x, y = ((t, -a), (a, t), (-t, a), (-a, -t))[side]
            pts.append((cx + x, cy + y, z))
    return pts

def hash01(*k):
    """A repeatable pseudo-random number in [0, 1) from a few numbers."""
    h = 0
    for x in k: h = (h*1000003 + int(round(x*1000))) & 0xFFFFFFFF
    h ^= h >> 15; h = (h*0x2C1B3C6D) & 0xFFFFFFFF; h ^= h >> 12; h = (h*0x297A2D39) & 0xFFFFFFFF; h ^= h >> 15
    return h/4294967296.0
