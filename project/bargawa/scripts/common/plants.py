"""The vegetation of Jatisrana, built by script. The hero plants (banyan, bamboo, banana, kamboja, jasmine, lotus, glagah, reeds,
tree ferns) are modeled with real leaves: a dark inner blob per leaf clump for body, leaves on its skin for the silhouette, the
stylized look of the environment design. The far forest stays as simple tree shapes. Grass is in grass.py."""
import math, random
from . import mesh, geo, jatisrana as J

LEAF_MATS = ("leaf", "leaf_dark", "leaf_light")

def _pick(rng, names, weights):
    r = rng.random()*sum(weights)
    for n, w in zip(names, weights):
        r -= w
        if r <= 0: return n
    return names[-1]

def _area(r):
    a, b, c = r
    return 4*math.pi*((a*b + a*c + b*c)/3.0)

def leaf_clump(P, M, rng, c, r, density, size, weights=(0.5, 0.25, 0.25), fill="leaf_dark", up_bias=0.3, curl=0.18):
    """A clump of foliage: a slightly smaller dark blob (body and self-shadow) with leaves on its skin facing out."""
    P.add(geo.ellipsoid(c[0], c[1], c[2], r[0]*0.88, r[1]*0.88, r[2]*0.88, nu=12, nv=7), M[fill])
    for k in range(int(_area(r)*density)):
        u = rng.uniform(-1 + up_bias, 1.0); a = rng.uniform(0, 2*math.pi); s = math.sqrt(max(0.0, 1 - u*u))
        nx, ny, nz = s*math.cos(a), s*math.sin(a), u; f = rng.uniform(0.9, 1.05)
        p = (c[0] + r[0]*nx*f, c[1] + r[1]*ny*f, c[2] + r[2]*nz*f)
        n = geo._norm((nx/r[0], ny/r[1], nz/r[2]))
        t = geo._norm(geo._cross(n, (rng.uniform(-1, 1), rng.uniform(-1, 1), rng.uniform(-1, 1))))
        t = geo._norm((t[0], t[1], t[2] - 0.5))                                                  # leaves hang a little
        L = size*rng.uniform(0.7, 1.3)
        P.add(geo.leaf(p, t, n, L, L*0.5, fold=0.2, curl=curl), M[_pick(rng, LEAF_MATS, weights)])

# --- banyan -------------------------------------------------------------------------------------------------------------------
def banyan(col, M, x, y):
    """The beringin: a fluted trunk flaring into buttress roots, seven spreading limbs with side branches, curtains of aerial roots
    (some grown down into pillar roots), and a broad crown of leaf clumps over the limbs."""
    rng = random.Random(11); z = J.height(x, y); W = mesh.Parts(); B, BD = M["bark"], M["bark_dark"]
    ground = lambda px, py: J.height(x + px, y + py) - z
    rings = []
    for zz, r, amp in ((-0.6, 2.7, 0.3), (0.0, 2.35, 0.28), (0.6, 1.8, 0.2), (1.5, 1.45, 0.14), (3.0, 1.25, 0.1), (4.5, 1.12, 0.08), (5.5, 1.02, 0.07)):
        rings.append([(r*(1 + amp*math.sin(7*a + 0.6) + 0.05*math.sin(13*a))*math.cos(a), r*(1 + amp*math.sin(7*a + 0.6))*math.sin(a), zz)
                      for a in (2*math.pi*i/28 for i in range(28))])
    W.add(geo.loft(rings, True, True), B)
    for k in range(7):                                                                           # buttress roots over the ground
        a = 2*math.pi*k/7 + 0.45 + rng.uniform(-0.2, 0.2); L = rng.uniform(3.5, 5.5)
        pts = []
        for i in range(7):
            t = i/6; d = 1.7 + L*t; b = a + 0.25*math.sin(3*t + k)
            px, py = d*math.cos(b), d*math.sin(b); pts.append((px, py, ground(px, py) + 0.35*(1 - t)**1.5 - 0.05))
        W.add(geo.tube(pts, [0.55*(1 - i/6)**1.3 + 0.06 for i in range(7)], n=7, wobble=0.15, seed=k), B)
    paths = []
    for k in range(7):                                                                           # limbs and their side branches
        a = 2*math.pi*k/7 + rng.uniform(-0.2, 0.2); L = rng.uniform(7.0, 9.5); rise = rng.uniform(2.0, 3.2)
        limb = geo.bezier((0.5*math.cos(a), 0.5*math.sin(a), 4.9), (0.45*L*math.cos(a), 0.45*L*math.sin(a), 6.7 + 0.3*rise),
                          (L*math.cos(a), L*math.sin(a), 6.2 + rise), 6)
        W.add(geo.tube(limb, [0.62 - 0.46*i/6 for i in range(7)], n=9, wobble=0.12, seed=k), B); paths.append((limb, 0.4))
        for u, side in ((0.5, 1), (0.8, -1)):
            base = limb[int(u*6)]; b = a + side*rng.uniform(0.5, 0.9); Ls = rng.uniform(3.0, 4.5); up = rng.uniform(1.0, 2.0)
            tip = (base[0] + Ls*math.cos(b), base[1] + Ls*math.sin(b), base[2] + up)
            sub = geo.bezier(base, ((base[0] + tip[0])/2, (base[1] + tip[1])/2, base[2] + up*0.8), tip, 4)
            W.add(geo.tube(sub, [0.24 - 0.17*i/4 for i in range(5)], n=7, wobble=0.12, seed=k + u), B); paths.append((sub, 0.25))
    for path, reach in paths:                                                                    # aerial roots
        for i in range(1, len(path)):
            for j in range(rng.randint(1, 4)):
                sx, sy, sz = path[i][0] + rng.uniform(-0.25, 0.25), path[i][1] + rng.uniform(-0.25, 0.25), path[i][2] - 0.15
                gz = ground(sx, sy); dist = math.hypot(sx, sy)
                if dist > 4.0 and reach > 0.3 and rng.random() < 0.07:                          # down into the ground: a pillar root
                    r0 = rng.uniform(0.1, 0.2); bend = rng.uniform(-0.25, 0.25)
                    W.add(geo.tube([(sx, sy, sz), (sx + bend, sy, sz - (sz - gz)*0.35), (sx + bend*0.6, sy + bend*0.4, gz + 1.0),
                                    (sx, sy, gz + 0.15), (sx, sy, gz - 0.15)],
                                   [r0*0.45, r0*0.7, r0*0.9, r0*1.3, r0*1.7], n=7, wobble=0.25, seed=sx), B)
                else:
                    end = gz + rng.uniform(1.2, max(1.3, sz - gz - 0.8)); wig = rng.uniform(-0.15, 0.15)
                    pts = [(sx, sy, sz), (sx + wig, sy, (sz*2 + end)/3), (sx, sy + wig, (sz + end*2)/3), (sx + wig*0.5, sy, end)]
                    W.add(geo.tube(pts, [0.04, 0.035, 0.03, 0.012], n=5, seed=sy), BD)
    trunk = W.build("Banyan_Trunk", col, loc=(x, y, z), smooth=True)
    C = mesh.Parts()
    for path, reach in paths:
        e = path[-1]; s = rng.uniform(2.5, 3.3) if reach > 0.3 else rng.uniform(2.2, 2.9)
        leaf_clump(C, M, rng, (e[0], e[1], e[2] + 0.9), (s, s, s*0.62), 14, 0.42)
        if reach > 0.3:
            m = path[3]; s = rng.uniform(2.4, 3.0)
            leaf_clump(C, M, rng, (m[0], m[1], m[2] + 1.9), (s, s, s*0.6), 12, 0.42)
    leaf_clump(C, M, rng, (0, 0, 10.8), (4.8, 4.8, 2.7), 12, 0.42)
    crown = C.build("Banyan_Crown", col, loc=(x, y, z), smooth=True)
    return trunk, crown

# --- bamboo -------------------------------------------------------------------------------------------------------------------
def _bamboo_clump(col, M, name, seed):
    """A clump of bamboo: culms with nodes arching outward, twigs at the upper nodes with fans of narrow drooping leaves."""
    rng = random.Random(seed); P = mesh.Parts()
    for k in range(18):
        a = rng.uniform(0, 2*math.pi); r0 = rng.uniform(0.05, 0.55); h = rng.uniform(9.5, 14.0); out = rng.uniform(1.5, 4.0)
        c, s = math.cos(a), math.sin(a)
        base, mid, top = (r0*c, r0*s, -0.2), (r0*c + 0.25*out*c, r0*s + 0.25*out*s, h*0.55), (r0*c + out*c, r0*s + out*s, h)
        path = geo.bezier(base, mid, top, 12)
        P.add(geo.tube(path, [0.07 - 0.045*i/12 for i in range(13)], n=6, cap0=False), M["bamboo_culm"])
        dense = geo.bezier(base, mid, top, 34)
        for i in range(2, 34):
            p, q = dense[i], dense[i + 1] if i < 34 else dense[i]
            rr = (0.07 - 0.045*i/34)*1.2
            P.add(geo.tube([p, tuple(p[j] + (q[j] - p[j])*0.08 for j in range(3))], rr, n=6), M["bamboo"])
            if i >= 13:                                                                         # twigs with fans of leaves
                for tw in range(1 if i < 20 else 2):
                    b = a + rng.uniform(-1.6, 1.6); tl = rng.uniform(0.4, 0.9)
                    tip = (p[0] + tl*math.cos(b), p[1] + tl*math.sin(b), p[2] - rng.uniform(0.0, 0.3))
                    P.add(geo.tube([p, tip], 0.01, n=4), M["bamboo"])
                    for j in range(rng.randint(10, 16)):
                        u = rng.uniform(0.3, 1.0); q0 = tuple(p[m] + (tip[m] - p[m])*u for m in range(3))
                        d = (math.cos(b + rng.uniform(-1.1, 1.1)), math.sin(b + rng.uniform(-1.1, 1.1)), rng.uniform(-1.0, -0.15))
                        P.add(geo.leaf(q0, d, (0, 0, 1), rng.uniform(0.24, 0.34), 0.045, fold=0.1, curl=0.22),
                              M[_pick(rng, ("leaf_light", "leaf", "leaf_dark"), (0.5, 0.35, 0.15))])
    ob = P.build(name, col, smooth=True); ob.hide_render = True; ob.hide_viewport = True
    return ob

def bamboo(col, src_col, M):
    b = J.BAMBOO_BAND; rng = random.Random(9); vs = [_bamboo_clump(src_col, M, "Bamboo_A", 1), _bamboo_clump(src_col, M, "Bamboo_B", 2)]
    for k in range(b["count"]):
        x = b["x0"] + (b["x1"] - b["x0"])*(k + rng.uniform(0.15, 0.85))/b["count"]; y = rng.uniform(b["y0"], b["y1"])
        mesh.instance(vs[k % 2], "Bamboo.%02d" % k, col, (x, y, J.height(x, y)), rng.uniform(0, 6.28), rng.uniform(0.85, 1.15))

# --- banana -------------------------------------------------------------------------------------------------------------------
def _banana_leaf(P, mat, a, h, L, droop, torn, rng):
    """One banana leaf: petiole, then a broad blade along an arching midrib, its halves folded down a little; torn leaves have
    gaps in the blade."""
    c, s = math.cos(a), math.sin(a); side = (-s, c, 0.0)
    p1 = (0.35*c, 0.35*s, h + 0.45)
    mid = geo.bezier(p1, (0.55*L*c, 0.55*L*s, h + 1.15), (L*c, L*s, h + 0.6 - droop), 12)
    P.add(geo.tube([(0, 0, h - 0.1), p1], [0.05, 0.035], n=5), mat)
    v, f = [], []
    for i, p in enumerate(mid):
        t = i/12; w = 0.36*math.sin(math.pi*min(t*1.05, 1.0))**0.7
        for sg in (-1, 1): v.append(tuple(p[k] + sg*side[k]*w + (0, 0, -0.12*w)[k] for k in range(3)))
        v.append(p)
    for i in range(12):
        a0, a1 = 3*i, 3*(i + 1)
        for sg, e in ((0, 0), (1, 1)):
            if torn and (i % 3 == 1) and (sg == 0 or rng.random() < 0.4): continue
            ei, mi = a0 + e, a0 + 2
            f.append((ei, a1 + e, a1 + 2, mi))
    P.add((v, f), mat)

def banana(col, M, name, x, y, seed):
    rng = random.Random(seed); z = J.height(x, y); P = mesh.Parts(); h = rng.uniform(2.2, 2.7)
    P.add(geo.tube([(0, 0, -0.1), (0.02, 0, h*0.5), (0, 0.02, h)], [0.2, 0.16, 0.12], n=10), M["banana_stem"])
    for k in range(8):
        a = 2*math.pi*k/8 + rng.uniform(-0.3, 0.3)
        _banana_leaf(P, M[_pick(rng, ("leaf_light", "leaf"), (0.6, 0.4))], a, h, rng.uniform(2.0, 2.7), rng.uniform(0.3, 1.0), rng.random() < 0.5, rng)
    for k in range(2):                                                                           # dead leaves hanging down the stem
        a = rng.uniform(0, 2*math.pi); c, s = math.cos(a), math.sin(a)
        P.add(geo.ribbon([(0.18*c, 0.18*s, h - 0.2), (0.32*c, 0.32*s, h - 0.6), (0.3*c, 0.3*s, h - 1.4), (0.24*c, 0.24*s, h - 1.9)],
                         [0.08, 0.22, 0.2, 0.06], up=(c, s, 0)), M["dry_leaf"])
    return P.build(name, col, loc=(x, y, z), smooth=True)

# --- kamboja (frangipani) -----------------------------------------------------------------------------------------------------
def _flower(P, M, c, up, size, rng, petal="flower"):
    """A five-petalled flower facing `up`, petals turned like a pinwheel, a small yellow centre."""
    up = geo._norm(up); s = geo._norm(geo._cross(up, (0.3, 0.7, 0.1))); t = geo._cross(up, s); turn = rng.uniform(0, 2*math.pi)
    for k in range(5):
        a = turn + 2*math.pi*k/5
        d = tuple(math.cos(a)*s[j] + math.sin(a)*t[j] + 0.25*up[j] for j in range(3))
        n = tuple(up[j] + 0.35*(math.cos(a + 0.6)*s[j] + math.sin(a + 0.6)*t[j]) for j in range(3))
        P.add(geo.leaf(c, d, n, size, size*0.75, fold=-0.15, curl=0.05), M[petal])
    P.add(geo.ellipsoid(c[0], c[1], c[2], size*0.18, size*0.18, size*0.12, nu=6, nv=4), M["flower_center"])

def kamboja(col, M, x, y):
    """The frangipani by the shrine: a short crooked trunk forking again and again into thick blunt branches, a rosette of long
    leaves at each tip with a cluster of white flowers, and fallen flowers on the ground around it."""
    rng = random.Random(21); z = J.height(x, y); P = mesh.Parts(); B = M["bark"]
    # An old tree, about 5 m (Plumeria rubra grows to 2-8 m; blender/places/jatisrana.md): a taller, thicker trunk and longer
    # limbs; the leaves and flowers keep their real size.
    P.add(geo.tube([(0, 0, -0.1), (0.2, 0.07, 1.2), (0.3, 0.15, 2.3)], [0.3, 0.25, 0.21], n=9, wobble=0.12), B)
    tips = []
    def grow(p, d, r, depth):
        L = (rng.uniform(0.7, 1.0) - 0.12*depth)*1.45
        d = geo._norm((d[0], d[1], d[2] + 0.35))
        end = tuple(p[k] + d[k]*L for k in range(3)); mid = tuple(p[k] + d[k]*L*0.5 + (0, 0, 0.05)[k] for k in range(3))
        P.add(geo.tube([p, mid, end], [r, r*0.92, r*0.85], n=7, wobble=0.08, seed=depth + p[0]), B)
        if depth >= 3: P.add(geo.ellipsoid(end[0], end[1], end[2], r*0.85, r*0.85, r, nu=7, nv=5), B); tips.append((end, d)); return
        for sgn in (-1, 1):
            a = math.atan2(d[1], d[0]) + sgn*rng.uniform(0.4, 0.8)
            grow(end, (math.cos(a), math.sin(a), d[2]*0.5 + rng.uniform(0.0, 0.3)), r*0.82, depth + 1)
    for k in range(3):
        a = 2*math.pi*k/3 + rng.uniform(-0.3, 0.3); grow((0.3, 0.15, 2.3), (math.cos(a), math.sin(a), 0.4), 0.18, 1)
    for end, d in tips:
        for k in range(13):                                                                      # leaf rosette
            a = 2*math.pi*k/13 + rng.uniform(-0.2, 0.2)
            dd = (math.cos(a)*0.8 + d[0]*0.3, math.sin(a)*0.8 + d[1]*0.3, rng.uniform(0.3, 1.0))
            P.add(geo.leaf(end, dd, (0, 0, 1), rng.uniform(0.38, 0.52), 0.14, fold=0.15, curl=0.15), M[_pick(rng, LEAF_MATS, (0.5, 0.3, 0.2))])
        for k in range(rng.randint(4, 7)):                                                       # the flower cluster
            a = rng.uniform(0, 2*math.pi); c = (end[0] + 0.1*math.cos(a), end[1] + 0.1*math.sin(a), end[2] + rng.uniform(0.12, 0.22))
            _flower(P, M, c, (0.4*math.cos(a), 0.4*math.sin(a), 1.0), 0.05, rng)
    for k in range(14):                                                                          # fallen flowers
        a, r = rng.uniform(0, 2*math.pi), rng.uniform(0.8, 3.6); fx, fy = r*math.cos(a), r*math.sin(a)
        _flower(P, M, (fx, fy, J.height(x + fx, y + fy) - z + 0.03), (rng.uniform(-0.3, 0.3), rng.uniform(-0.3, 0.3), 1.0), 0.05, rng)
    return P.build("Kamboja", col, loc=(x, y, z), smooth=True)

# --- jasmine ------------------------------------------------------------------------------------------------------------------
def jasmine(col, M, x, y):
    """Renuka's jasmine: a round bush of small dark leaves dotted with white star flowers, a few stems arching out of it."""
    rng = random.Random(31); z = J.height(x, y); P = mesh.Parts()
    for k, (cx, cy, cz, r) in enumerate(((0.0, 0.0, 0.62, 0.62), (0.45, 0.2, 0.5, 0.48), (-0.4, 0.25, 0.48, 0.46), (0.05, -0.42, 0.45, 0.44))):
        leaf_clump(P, M, rng, (cx, cy, cz), (r, r, r*0.85), 110, 0.085, weights=(0.3, 0.6, 0.1), curl=0.1)
        for j in range(16):                                                                      # flowers on the skin
            u = rng.uniform(-0.2, 1.0); a = rng.uniform(0, 2*math.pi); s = math.sqrt(1 - u*u)
            p = (cx + r*s*math.cos(a)*1.02, cy + r*s*math.sin(a)*1.02, cz + r*0.85*u*1.02)
            _flower(P, M, p, (s*math.cos(a), s*math.sin(a), u + 0.3), 0.026, rng)
    for k in range(14):
        a = rng.uniform(0, 2*math.pi); out = rng.uniform(0.6, 0.95); hgt = rng.uniform(0.8, 1.3); c, s = math.cos(a), math.sin(a)
        stem = geo.bezier((0.08*c, 0.08*s, 0.0), (out*0.4*c, out*0.4*s, hgt*1.05), (out*c, out*s, hgt*rng.uniform(0.6, 0.95)), 10)
        P.add(geo.tube(stem, 0.012, n=4), M["leaf_dark"])
        for i in range(2, 11):
            p = stem[i]; t = geo._norm(tuple(stem[min(i + 1, 10)][j] - stem[i - 1][j] for j in range(3)))
            side = geo._norm(geo._cross(t, (0, 0, 1)))
            for sg in (-1, 1):
                d = tuple(sg*side[j] + 0.6*t[j] + (0, 0, 0.2)[j] for j in range(3))
                P.add(geo.leaf(p, d, (0, 0, 1), rng.uniform(0.05, 0.07), 0.032, fold=0.2, curl=0.1), M[_pick(rng, ("leaf_dark", "leaf"), (0.7, 0.3))])
            if i == 10 or rng.random() < 0.18: _flower(P, M, (p[0], p[1], p[2] + 0.02), (0, 0, 1), 0.024, rng)
    return P.build("Jasmine", col, loc=(x, y, z), smooth=True)

# --- the lake's edge ----------------------------------------------------------------------------------------------------------
def _plume_tuft(P, M, rng, x, y, z, blade, plume, stalks=(2, 4), height=(2.2, 3.0)):
    for j in range(18):                                                                          # blades arching out
        b = rng.uniform(0, 2*math.pi); out = rng.uniform(0.4, 0.9); hh = rng.uniform(0.7, 1.4); c, s = math.cos(b), math.sin(b)
        path = geo.bezier((x + 0.05*c, y + 0.05*s, z - 0.05), (x + out*0.35*c, y + out*0.35*s, z + hh), (x + out*c, y + out*s, z + hh*0.55), 5)
        P.add(geo.ribbon(path, [0.022, 0.02, 0.017, 0.013, 0.008, 0.002], up=(c, s, 0)), M[blade])
    for j in range(rng.randint(*stalks)):                                                        # stalks with feathery plumes
        h = rng.uniform(*height); lx, ly = rng.uniform(-0.3, 0.3), rng.uniform(-0.3, 0.3); top = (x + lx, y + ly, z + h)
        P.add(geo.tube([(x, y, z - 0.05), (x + lx*0.4, y + ly*0.4, z + h*0.5), top], [0.01, 0.008, 0.005], n=4), M[blade])
        for k in range(20):
            b = rng.uniform(0, 2*math.pi); c, s = math.cos(b), math.sin(b); st = (top[0], top[1], top[2] - rng.uniform(0.0, 0.3))
            path = geo.bezier(st, (st[0] + 0.08*c + lx*0.2, st[1] + 0.08*s + ly*0.2, st[2] + 0.03), (st[0] + 0.16*c + lx*0.4, st[1] + 0.16*s + ly*0.4, st[2] - 0.3), 4)
            P.add(geo.ribbon(path, [0.004, 0.02, 0.018, 0.012, 0.003], up=(c, s, 0)), M[plume])

def glagah(col, M):
    """Wild glagah on the west shore: tufts of arching blades and tall stalks with white feathery plumes."""
    g = J.GLAGAH; rng = random.Random(41); P = mesh.Parts()
    for k in range(g["count"]):
        a, r = rng.uniform(0, 2*math.pi), g["r"]*math.sqrt(rng.random())
        x, y = g["c"][0] + r*math.cos(a), g["c"][1] + r*math.sin(a)
        _plume_tuft(P, M, rng, x, y, J.height(x, y), "leaf_light", "plume")
    return P.build("Glagah", col, smooth=True)

def reeds(col, M):
    """Reeds on the east shore, standing in the shallows: greener blades, brownish plumes."""
    g = J.REEDS; rng = random.Random(51); P = mesh.Parts()
    for k in range(g["count"]):
        a, r = rng.uniform(0, 2*math.pi), g["r"]*math.sqrt(rng.random())
        x, y = g["c"][0] + r*math.cos(a), g["c"][1] + r*math.sin(a)
        _plume_tuft(P, M, rng, x, y, max(J.height(x, y), J.WATER_Z - 0.3), "leaf", "reed_plume", stalks=(1, 3), height=(1.8, 2.5))
    return P.build("Reeds", col, smooth=True)

def _pad(P, mat, x, y, z, r, rng, tilt=0.0):
    """A lotus pad: a round leaf with its notch, the rim turned up a little."""
    n = 18; a0 = rng.uniform(0, 2*math.pi); gap = 0.35; v = [(x, y, z)]
    for i in range(n + 1):
        a = a0 + (2*math.pi - gap)*i/n; rr = r*(1 + 0.04*math.sin(5*a))
        v.append((x + rr*math.cos(a), y + rr*math.sin(a), z + 0.02 + tilt*rr*math.cos(a)))
    P.add((v, [(0, i, i + 1) for i in range(1, n + 1)]), mat)

def _lotus_flower(P, M, c, rng, size=0.15):
    for layer, (open_, k0) in enumerate(((0.95, 0.0), (0.45, 0.5))):
        for k in range(8):
            a = 2*math.pi*(k + k0)/8; ca, sa = math.cos(a), math.sin(a)
            d = (ca*open_, sa*open_, 1.0); n = (ca, sa, -open_)
            P.add(geo.leaf(c, d, n, size*(1.0 - 0.15*layer), size*0.6, fold=-0.3, curl=-0.12), M["lotus" if layer == 0 else "lotus_light"])
    P.add(geo.cylinder(c[0], c[1], c[2] + 0.02, 0.035, 0.035, n=10, r1=0.045), M["flower_center"])

def lotus(col, M):
    """Lotus in three patches: pads on the water (some raised on their stalks), open flowers and closed buds."""
    rng = random.Random(61); P = mesh.Parts(); W = J.WATER_Z
    for patch in J.LOTUS:
        for k in range(patch["n"]):
            a, r = rng.uniform(0, 2*math.pi), patch["r"]*math.sqrt(rng.random())
            x, y = patch["c"][0] + r*math.cos(a), patch["c"][1] + r*math.sin(a)
            if rng.random() < 0.25:
                h = rng.uniform(0.15, 0.4)
                P.add(geo.tube([(x, y, W - 0.2), (x, y, W + h)], 0.012, n=4), M["lotus_pad"])
                _pad(P, M["lotus_pad"], x, y, W + h, rng.uniform(0.25, 0.4), rng, tilt=rng.uniform(-0.25, 0.25))
            else: _pad(P, M["lotus_pad"], x, y, W + 0.004, rng.uniform(0.25, 0.5), rng)
            if k % 4 == 0:
                h = rng.uniform(0.35, 0.6); fx = x + rng.uniform(-0.3, 0.3)
                P.add(geo.tube([(fx, y, W - 0.2), (fx, y, W + h)], 0.012, n=4), M["lotus_pad"])
                _lotus_flower(P, M, (fx, y, W + h), rng)
            elif k % 4 == 2:
                h = rng.uniform(0.3, 0.5); fx = x + rng.uniform(-0.3, 0.3)
                P.add(geo.tube([(fx, y, W - 0.2), (fx, y, W + h)], 0.012, n=4), M["lotus_pad"])
                P.add(geo.ellipsoid(fx, y, W + h + 0.08, 0.06, 0.06, 0.1, nu=8, nv=5), M["lotus"])
                P.add(geo.cylinder(fx, y, W + h + 0.15, 0.03, 0.08, n=8, r1=0.0), M["lotus"])
    return P.build("Lotus", col, smooth=True)

# --- tree ferns, forest -------------------------------------------------------------------------------------------------------
def tree_fern(col, M, name, x, y, seed):
    rng = random.Random(seed); z = J.height(x, y); P = mesh.Parts(); h = rng.uniform(2.4, 3.4)
    P.add(geo.tube([(0, 0, -0.1), (0.05, 0, h*0.5), (0, 0.04, h)], [0.18, 0.16, 0.14], n=8, wobble=0.25), M["bark_dark"])
    for k in range(12):
        a = 2*math.pi*k/12 + rng.uniform(-0.2, 0.2); c, s = math.cos(a), math.sin(a); L = rng.uniform(1.6, 2.3)
        frond = geo.bezier((0, 0, h), (L*0.45*c, L*0.45*s, h + 0.6), (L*c, L*s, h - rng.uniform(0.1, 0.6)), 14)
        P.add(geo.tube(frond, 0.015, n=4), M["leaf_dark"])
        side = (-s, c, 0.0)
        for i in range(1, 14):
            p = frond[i]; ln = 0.28*math.sin(math.pi*i/14) + 0.05
            for sg in (-1, 1):
                P.add(geo.leaf(p, (sg*side[0] + 0.3*c, sg*side[1] + 0.3*s, -0.15), (0, 0, 1), ln, 0.06, fold=0.1, curl=0.15), M["leaf"])
    return P.build(name, col, loc=(x, y, z), smooth=True)

def _tree_variant(col, M, name, h, r, leaf, seed):
    rng = random.Random(seed); P = mesh.Parts()
    P.add(geo.cylinder(0, 0, -0.3, 0.32*h/16, h*0.62, n=8, r1=0.14*h/16), M["bark"])
    for k in range(4):
        a = rng.uniform(0, 2*math.pi); d = rng.uniform(0, 0.45*r)
        P.add(geo.ellipsoid(d*math.cos(a), d*math.sin(a), h*rng.uniform(0.62, 0.8), r*rng.uniform(0.6, 0.85), r*rng.uniform(0.6, 0.85),
                            h*rng.uniform(0.16, 0.24), nu=10, nv=6), leaf)
    ob = P.build(name, col, smooth=True); ob.hide_render = True; ob.hide_viewport = True
    return ob

def forest(col, src_col, M, spacing=8.0, bounds=(-130.0, 130.0, -140.0, 110.0)):
    """The forest round the clearing and on the far bank: linked copies of four simple tree shapes on a jittered grid."""
    variants = [_tree_variant(src_col, M, "Tree_A", 16, 4.5, M["leaf"], 1), _tree_variant(src_col, M, "Tree_B", 20, 5.5, M["leaf_dark"], 2),
                _tree_variant(src_col, M, "Tree_C", 13, 3.4, M["leaf_light"], 3), _tree_variant(src_col, M, "Tree_D", 24, 6.0, M["leaf_dark"], 4)]
    rng = random.Random(5); n = 0
    x0, x1, y0, y1 = bounds; y = y0
    while y < y1:
        x = x0
        while x < x1:
            px, py = x + rng.uniform(-0.4, 0.4)*spacing, y + rng.uniform(-0.4, 0.4)*spacing
            if J.forest_ok(px, py) and rng.random() > 0.12:
                v = variants[rng.randrange(4)]
                mesh.instance(v, "Tree.%04d" % n, col, (px, py, J.height(px, py) - 0.2), rng.uniform(0, 2*math.pi), rng.uniform(0.8, 1.2)); n += 1
            x += spacing
        y += spacing
    return n

def all_plants(col, M):
    src = mesh.coll("_PlantSources", col)
    banyan(col, M, *J.BANYAN)
    kamboja(col, M, *J.KAMBOJA)
    jasmine(col, M, *J.JASMINE)
    for k, (x, y) in enumerate(J.BANANAS): banana(col, M, "Banana.%d" % k, x, y, 70 + k)
    for k, (x, y) in enumerate(J.TREE_FERNS): tree_fern(col, M, "TreeFern.%d" % k, x, y, 80 + k)
    glagah(col, M); reeds(col, M); lotus(col, M)
    bamboo(col, src, M)
    return forest(col, src, M)
