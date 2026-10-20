"""What the fire samples burn (objects/fire.py; research docs/stories/ramabargawa/objects/fire.md): firewood and beds of coals.
Every point carries how burned it is, written as the 'wear' attribute (0 bark or cold ash, 1 glowing charcoal), which
fire.charred and fire.ember_material read. SAMPLES gives each fire's size: base D, mean flame height L (Heskestad), light.

Sizes in metres; each sample's fuel is centred on the origin, the ground at z 0. Pure geometry (no bpy)."""
import math
from . import geo

hash01 = geo.hash01

# name: base diameter D, mean flame height L (Heskestad: L = 0.235 Q^0.4 - 1.02 D), the flames' root z0, light W, the flames'
# emission strength (the crossing cards add up: the big fire's four cards glowed white at 2.5), where it stands in fire.blend
SAMPLES = {
    "Hearth": dict(D=0.20, L=0.24, z0=0.02, energy=30.0, strength=1.8, at=(-1.4, 0.0, 0.0)),   # one fire mouth of the luweng
    "Wood":   dict(D=0.45, L=0.50, z0=0.03, energy=150.0, strength=1.6, at=(0.0, 0.0, 0.0)),   # a small campfire, about 40 kW
    "Embers": dict(D=0.50, L=0.0, z0=0.02, energy=12.0, strength=0.0, at=(1.4, 0.0, 0.0)),     # smouldering, no flame (S5)
    "Large":  dict(D=1.40, L=1.80, z0=0.10, energy=900.0, strength=1.1, at=(5.0, 0.0, 0.0)),   # a big log fire, about 700 kW
}
LAMP_AT = (-2.4, 0.0, 0.02)       # Fire_Lamp, the damar's flame, beside them

def puff_hz(D):
    """How often a fire D across puffs (big bulges forming at its base and rising): 1.68 / sqrt(D) Hz (Malalasekera et al.
    1996). The bigger the fire, the slower it breathes."""
    return 1.68/math.sqrt(D)

def _smooth(a, b, x):
    t = min(max((x - a)/(b - a), 0.0), 1.0)
    return t*t*(3 - 2*t)

def log(p0, p1, r, seed, burn, bend=0.25):
    """A piece of firewood from p0 to p1, radius r (a little thinner at p1), its bark uneven, slightly bent; burn(point) -> 0..1,
    how charred each point is. Returns ((verts, faces), wear)."""
    length = math.dist(p0, p1); ns = max(3, int(length/(2.5*r)))
    side = [hash01(seed, k) - 0.5 for k in (1, 2, 3)]
    path, radii = [], []
    for i in range(ns + 1):
        t = i/ns; b = bend*r*math.sin(math.pi*t)
        path.append(tuple(p0[k] + (p1[k] - p0[k])*t + b*side[k] for k in range(3)))
        radii.append(r*(1 - 0.12*t)*(1 + 0.18*(hash01(seed, i, 7) - 0.5)))       # knots and swellings along it
    v, f = geo.tube(path, radii, n=9, wobble=0.16, seed=seed)
    return (v, f), [burn(p) for p in v]

def coals(R, n, size, seed, heat=1.0, pile=1.2):
    """A bed of n coals within radius R: irregular chunks about `size` across, heaped a little in the middle (pile x size),
    hotter towards the centre; and the bed of ash under them, warm in the middle. Returns [((verts, faces), wear), ...]."""
    out = []
    for k in range(n):
        a = 2*math.pi*hash01(seed, k, 1); rr = R*hash01(seed, k, 2)**0.75
        s = size*(0.55 + 0.9*hash01(seed, k, 3))
        rx, ry, rz = s*(0.8 + 0.4*hash01(seed, k, 4)), s*(0.6 + 0.4*hash01(seed, k, 5)), s*(0.35 + 0.25*hash01(seed, k, 6))
        cz = rz*0.3 + pile*size*(1 - (rr/R)**2)
        v, f = geo.ellipsoid(0.0, 0.0, 0.0, rx, ry, rz, nu=7, nv=4)
        v = [(x*(1 + 0.35*(hash01(seed, k, j, 8) - 0.5)), y*(1 + 0.35*(hash01(seed, k, j, 9) - 0.5)), z) for j, (x, y, z) in enumerate(v)]
        v, f = geo.moved((v, f), rr*math.cos(a), rr*math.sin(a), cz, ang=2*math.pi*hash01(seed, k, 10))
        h = heat*(1 - (rr/R)**1.6)*(0.55 + 0.45*hash01(seed, k, 11))
        out.append(((v, f), [h]*len(v)))
    prof = [(1.15*R, -0.01), (0.9*R, 0.35*size), (0.5*R, 0.7*size + 0.5*pile*size), (0.0, 0.8*size + pile*size)]
    v, f = geo.lathe(prof, n=28, cap1=False)
    v = [(x*(1 + 0.3*(hash01(seed, j, 12) - 0.5)), y*(1 + 0.3*(hash01(seed, j, 12) - 0.5)), z) for j, (x, y, z) in enumerate(v)]
    out.append(((v, f), [0.45*heat*_smooth(R, 0.2*R, math.hypot(p[0], p[1])) for p in v]))
    return out

def hearth(D, seed=11):
    """Three sticks pushed in from the front (-y), as into the luweng's fire mouth, their tips meeting in the fire, charred and
    glowing there; a small bed of coals under the tips."""
    burn = lambda p: _smooth(0.16, 0.0, math.hypot(p[0], p[1]))
    logs = [log((dx, 0.02, 0.03 + 0.012*k), (2.6*dx - 0.04 + 0.04*k, -0.68 + 0.04*k, 0.05 + 0.02*k), 0.022 + 0.005*k, seed + k, burn)
            for k, dx in enumerate((-0.035, 0.03, 0.0))]
    return logs, coals(0.6*D, 18, 0.018, seed, pile=1.0)

def campfire(D, seed=21):
    """Six logs leaning together (a tepee) over two laid crossed under them, charred towards the middle; a bed of coals."""
    R = 0.62*D
    burn = lambda p: _smooth(0.55*D, 0.05*D, math.hypot(p[0], p[1]))*(1 - 0.45*_smooth(0.12*D, 0.55*D, p[2]))   # the tips
    logs = []                                                            # above the flames black, not glowing (a glowing ball)
    for k in range(6):
        a = 2*math.pi*k/6 + 0.4*(hash01(seed, k) - 0.5)
        top = (0.07*D*math.cos(a + 2.2), 0.07*D*math.sin(a + 2.2), 0.62*D*(0.9 + 0.2*hash01(seed, k, 2)))
        logs.append(log((R*math.cos(a), R*math.sin(a), 0.02), top, (0.075 + 0.03*hash01(seed, k, 3))*D, seed + k, burn))
    for k, (p0, p1) in enumerate((((-0.55*D, -0.2*D, 0.035), (0.5*D, 0.15*D, 0.04)), ((-0.25*D, 0.5*D, 0.08), (0.2*D, -0.55*D, 0.07)))):
        logs.append(log(p0, p1, 0.085*D, seed + 10 + k, burn))
    return logs, coals(0.5*D, 35, 0.022, seed)

def smoulder(D, seed=31):
    """Embers with no flame: a wide bed of coals and ash, three charred stubs lying in it, glowing along their burned length."""
    burn = lambda p: 0.45 + 0.5*_smooth(0.45*D, 0.0, math.hypot(p[0], p[1]))
    stubs = (((-0.3*D, -0.12*D, 0.03), (0.15*D, 0.05*D, 0.035)), ((0.05*D, 0.3*D, 0.03), (0.2*D, -0.12*D, 0.04)),
             ((-0.1*D, 0.22*D, 0.025), (-0.35*D, 0.05*D, 0.03)))
    logs = [log(p0, p1, 0.065*D, seed + k, burn, bend=0.1) for k, (p0, p1) in enumerate(stubs)]
    return logs, coals(0.5*D, 40, 0.03, seed, heat=0.9, pile=0.8)

def bonfire(D, seed=41):
    """Four big logs crossed in two layers (as a pyre's are), burning through; a wide bed of coals."""
    burn = lambda p: 0.3 + 0.7*_smooth(0.6*D, 0.1*D, math.hypot(p[0], p[1]))
    a, L2, r = 0.22*D, 0.55*D, 0.075*D
    lay = [((-L2, -a, r), (L2, -a + 0.03, r)), ((-L2, a, r), (L2, a - 0.02, r + 0.01)),
           ((-a, -L2, 3*r), (-a + 0.02, L2, 3*r)), ((a, -L2, 3*r), (a - 0.03, L2, 3*r - 0.01))]
    logs = [log(p0, p1, r*(0.9 + 0.2*hash01(seed, k)), seed + k, burn, bend=0.15) for k, (p0, p1) in enumerate(lay)]
    return logs, coals(0.55*D, 110, 0.04, seed, pile=1.0)
