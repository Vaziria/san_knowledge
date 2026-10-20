"""Stylized stone blocks, after the modular stone video (project/bargawa/references/video-modular-stone-wall-brainchild.md):
rounded, softened edges, scooped chips at edges and corners, a gently uneven surface, small joints between blocks. Each point
carries a 'wear' value (1 on rounded edges and chip facets, 0 on flat faces) that the stone material uses to lighten worn edges,
as the video's texturing does.

block(), umpak() and padma() are pure geometry (no bpy); stair() lays the pendopo's steps from blocks. The pieces are built into
their own file, blender/objects/stone.blend (scripts/objects/stone.py), which the pendopo links (joglo.STONE_PIECES)."""
import math, random
from . import geo, mesh

def _vnoise(x, y, z, seed):
    """Smooth value noise in 0..1."""
    xi, yi, zi = math.floor(x), math.floor(y), math.floor(z); xf, yf, zf = x - xi, y - yi, z - zi
    s = lambda t: t*t*(3 - 2*t); u, v, w = s(xf), s(yf), s(zf)
    h = lambda a, b, c: geo.hash01(xi + a, yi + b, zi + c, seed)
    lerp = lambda a, b, t: a + (b - a)*t
    return lerp(lerp(lerp(h(0, 0, 0), h(1, 0, 0), u), lerp(h(0, 1, 0), h(1, 1, 0), u), v),
                lerp(lerp(h(0, 0, 1), h(1, 0, 1), u), lerp(h(0, 1, 1), h(1, 1, 1), u), v), w)

def _grid(h, n):
    """n + 1 positions across [-h, h], closer together near the ends (where the edges are rounded and chipped)."""
    return [h*math.sin(math.pi/2*(-1 + 2*i/n)) for i in range(n + 1)]

def block(sx, sy, sz, seed, r=0.03, rvar=0.75, bump=0.01, chips=6, chip_depth=(0.03, 0.08), taper=0.03, top_scale=None,
          chip_span=(0.12, 0.55)):
    """A stone block sx × sy × sz centred on the origin. Returns ((verts, faces), wear per vertex).
    r: edge rounding, varied along the edges by ±rvar (an even radius looked like soap bars); chip_depth: how deep the scooped
    chips go (at 1.5-4.5 cm they hid inside the rounding). top_scale: the top's size against the bottom's (an umpak's truncated
    pyramid, 0.67); otherwise a small random taper. Chips are cut before the taper, so they sit on the real edges."""
    rng = random.Random(seed); H = (sx/2, sy/2, sz/2)
    n = [max(4, min(16, int(round(s/0.07)))) for s in (sx, sy, sz)]
    G = [_grid(H[a], n[a]) for a in range(3)]
    v, f, wear, index = [], [], [], {}
    def point(p):
        key = tuple(round(c, 6) for c in p)
        if key not in index: index[key] = len(v); v.append(p)
        return index[key]
    for a in range(3):                                                                    # the six faces as grids
        u, w = (a + 1) % 3, (a + 2) % 3
        for s in (-1, 1):
            ids = [[0]*(n[w] + 1) for _ in range(n[u] + 1)]
            for i, pu in enumerate(G[u]):
                for j, pw in enumerate(G[w]):
                    p = [0.0, 0.0, 0.0]; p[a] = s*H[a]; p[u] = pu; p[w] = pw; ids[i][j] = point(tuple(p))
            for i in range(n[u]):
                for j in range(n[w]):
                    q = (ids[i][j], ids[i + 1][j], ids[i + 1][j + 1], ids[i][j + 1])
                    f.append(q if s > 0 else q[::-1])
    if top_scale is not None:                                                             # a truncated pyramid
        tx = ty = 1 - top_scale; tx += rng.uniform(-0.02, 0.02); ty += rng.uniform(-0.02, 0.02)
    else: tx, ty = rng.uniform(-taper, taper), rng.uniform(-taper, taper)               # a slightly irregular shape
    tilt = (rng.uniform(-0.012, 0.012), rng.uniform(-0.012, 0.012))
    out = []
    for p in v:
        rl = r*(1 + rvar*(_vnoise(p[0]*9 + 11, p[1]*9, p[2]*9, seed + 5) - 0.5)*2)            # rounding that wanders along the edge
        q = [max(-(H[k] - rl), min(H[k] - rl, p[k])) for k in range(3)]                # rounded box
        d = [p[k] - q[k] for k in range(3)]; L = math.sqrt(sum(c*c for c in d))
        nrm = [c/L for c in d] if L > 1e-9 else [0.0, 0.0, 1.0]
        pp = [q[k] + nrm[k]*rl for k in range(3)] if L > 1e-9 else list(p)
        t = [min(1.0, max(0.0, (abs(p[k]) - (H[k] - rl))/rl)) for k in range(3)]
        e = max(min(t[0], t[1]), min(t[1], t[2]), min(t[0], t[2]))                          # 1 on the rounded edges
        nz = (_vnoise(pp[0]*4 + 3, pp[1]*4, pp[2]*4, seed) - 0.5)*2                          # an uneven surface
        pp = [pp[k] + nrm[k]*bump*nz for k in range(3)]
        out.append(pp); wear.append(e)
    for k in range(chips):                                                                # scooped chips at edges and corners
        dep = rng.uniform(*chip_depth)
        if rng.random() < 0.35:                                                               # a corner
            sg = [rng.choice((-1, 1)) for _ in range(3)]
            nrm = geo._norm((sg[0] + rng.uniform(-0.3, 0.3), sg[1] + rng.uniform(-0.3, 0.3), sg[2] + rng.uniform(-0.3, 0.3)))
            E = [sg[i]*H[i] for i in range(3)]; cut = 0.73*r + dep
            span = None
        else:                                                                                 # along an edge
            a, b = rng.sample(range(3), 2); c = 3 - a - b; sa, sb = rng.choice((-1, 1)), rng.choice((-1, 1))
            dv = [0.0, 0.0, 0.0]; dv[a] = sa; dv[b] = sb; dv[c] = rng.uniform(-0.35, 0.35)
            dv[a] += rng.uniform(-0.25, 0.25); nrm = geo._norm(tuple(dv))
            c0 = rng.uniform(-(H[c] - r), H[c] - r); Lc = rng.uniform(chip_span[0], min(chip_span[1], H[c]*1.4))
            E = [0.0, 0.0, 0.0]; E[a] = sa*H[a]; E[b] = sb*H[b]; E[c] = c0; cut = 0.41*r + dep
            span = (c, c0, Lc/2)
        for i, p in enumerate(out):
            local = dep
            if span:
                tt = (p[span[0]] - span[1])/span[2]
                if abs(tt) >= 1: continue
                local = dep*(1 - tt*tt)
            dist = sum((p[k] - E[k])*nrm[k] for k in range(3)) + (cut - dep + local)
            if dist > 0:
                out[i] = [p[k] - nrm[k]*dist for k in range(3)]; wear[i] = max(wear[i], 0.85)
    for p in out:                                                                         # then taper and tilt the top
        up = (p[2] + H[2])/(2*H[2])
        p[0] *= 1 - tx*up; p[1] *= 1 - ty*up; p[2] += (tilt[0]*p[0] + tilt[1]*p[1])*up
    return (out, f), wear

def umpak(seed, base=0.54, top=0.36, h=0.3):
    """A plain umpak (a post's stone base): a truncated pyramid of chipped stone, sized like the soko penanggap's."""
    return block(base, base, h, seed, r=0.022, rvar=0.6, bump=0.004, chips=4, chip_depth=(0.012, 0.035),
                 top_scale=top/base, chip_span=(0.06, 0.25))

def padma(x, y, z, seed, h=0.3, n=48, base=0.58, seat=0.32):
    """A padma (lotus) umpak in the same stone, h tall on a base `base` wide, its seat `seat` wide for the post. Up to about
    0.45 m: a chipped square foot, two staggered rows of eight petals, a neck and the seat. Taller (the pakem makes the soko
    guru's umpak as tall as the post's perimeter, 0.72 m): a pedestal, with a plain chipped shaft (dado) between the foot and
    the lotus. The petal tips carry wear, so the carving shows the way worn edges do. Returns ((verts, faces), wear)."""
    s = base/0.58; fh = 0.07; lotus_h = min(h - fh, 0.24)                # sizes of the original 0.3 m padma, scaled to `base`
    foot, fw = block(base, base, fh, seed, r=0.015, rvar=0.6, bump=0.003, chips=3, chip_depth=(0.01, 0.025),
                     chip_span=(0.06, 0.2))
    parts, wears = [geo.moved(foot, dx=x, dy=y, dz=z + fh/2)], [fw]
    dado = h - fh - lotus_h
    if dado > 0.02:                                                       # the pedestal's shaft
        sh, sw = block(0.86*base, 0.86*base, dado + 0.01, seed + 7, r=0.018, rvar=0.6, bump=0.004, chips=4,
                       chip_depth=(0.012, 0.03), top_scale=0.94, chip_span=(0.06, 0.25))
        parts.append(geo.moved(sh, dx=x, dy=y, dz=z + fh + dado/2)); wears.append(sw)
    zl = z + fh + max(dado, 0.0) - 0.01                                  # where the lotus starts
    rel = lambda t: zl + (z + h - zl)*t
    sq = lambda a, zz: (geo.square_ring(a, zz, n//4, x, y), [0.0]*n)
    def lobed(r, zz, amp, phase=0.0):
        pts, w = [], []
        for i in range(n):
            t = -0.75*math.pi + 2*math.pi*i/n; lobe = abs(math.cos(4*(t - phase)))**3
            rr = r*s*(1 + amp*lobe)*(1 + 0.02*(_vnoise(math.cos(t)*3, math.sin(t)*3, zz*20, seed) - 0.5))
            pts.append((x + rr*math.cos(t), y + rr*math.sin(t), zz)); w.append(0.9*lobe*min(1.0, amp/0.28))
        return pts, w
    rings = [sq(0.26*s, rel(0.0)), lobed(0.25, rel(0.06), 0.28), lobed(0.25, rel(0.25), 0.3),
             lobed(0.22, rel(0.37), 0.32, math.pi/8), lobed(0.2, rel(0.62), 0.3, math.pi/8),
             lobed(0.17, rel(0.79), 0.1, math.pi/8), sq(seat/2, rel(0.87)), sq(seat/2, rel(1.0))]
    parts.append(geo.loft([r_[0] for r_ in rings], True, True)); wears.append([w for r_ in rings for w in r_[1]])
    return geo.merge(*parts), [w for ws in wears for w in ws]

STAIR_BUILD = dict(smooth=True, sharp=30, recalc=True)                   # at 38 deg the chip borders looked melted

def stair(M, x0=0.0, y_back=0.0, steps=((3.4, 0.15), (3.2, 0.30), (3.0, 0.45)), tread=0.36, bottom=-0.3, seed=1):
    """A stair of stone blocks, as Parts (build them with STAIR_BUILD): steps listed from the lowest (outermost, furthest from
    y_back) to the top one, each as (width, top height). Each step is a row of blocks of different lengths with small joints,
    reaching a little under the next step."""
    rng = random.Random(seed); P = mesh.Parts(); mat = M.get("stone_kit", M["stone"]); n = len(steps)
    for k, (width, top) in enumerate(steps):
        yc = y_back - tread*(n - 1 - k) - tread/2                                            # this step's centre line
        x = x0 - width/2
        while x < x0 + width/2 - 0.05:
            L = min(rng.uniform(0.55, 1.15), x0 + width/2 - x)
            if x0 + width/2 - (x + L) < 0.35: L = x0 + width/2 - x                            # no sliver at the end
            hz = top + rng.uniform(-0.008, 0.008) - bottom; gap = 0.014
            vf, wear = block(L - gap, tread + 0.04 - gap, hz, seed*1000 + k*37 + int(x*100), chips=rng.randint(5, 9))
            P.add(geo.moved(vf, dx=x + L/2, dy=yc + 0.02, dz=bottom + hz/2), mat, wear)
            x += L
    return P
