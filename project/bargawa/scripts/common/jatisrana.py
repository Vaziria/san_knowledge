"""Pertapan Jatisrana: the hermitage place's measures (no bpy). One source for the terrain, the buildings and the plants.

Axes: x east, y north, z up, metres. The origin is the centre of the pendapa at ground level; the hermitage terrace is at z 0.
The mountain is to the north, the lake to the south, below the terrace (docs/stories/ramabargawa/environment.md, "Layout")."""
import math

# --- the lake -------------------------------------------------------------------------------------------------------------
WATER_Z = -4.0                 # the lake's surface
LAKE_C = (0.0, -35.0)          # its centre
LAKE_R = (20.0, 12.5)          # ellipse radii: 40 x 25 m
LAKE_DEPTH = 1.6               # deepest point below the surface

def smooth(e0, e1, x):
    """0 at e0, 1 at e1 (either order), smooth in between."""
    t = min(1.0, max(0.0, (x - e0)/(e1 - e0)))
    return t*t*(3.0 - 2.0*t)

def lake_d(x, y):
    """Normalised distance from the lake's centre: below 1 is water."""
    return math.hypot((x - LAKE_C[0])/LAKE_R[0], (y - LAKE_C[1])/LAKE_R[1])

# --- the ground -----------------------------------------------------------------------------------------------------------
FLAT_C, FLAT_R = (0.0, 4.0), (18.0, 30.0)     # inside FLAT_R[0] of FLAT_C the terrace is level; bumps fade in up to FLAT_R[1]

def _base(y):
    """The valley's profile north-south: the terrace (0) steps down to the lake (-3.6) and rises again on the far bank."""
    if y >= LAKE_C[1]:
        return -3.6 + 3.6*smooth(-24.0, -10.0, y)
    return -3.6 + 3.0*smooth(-48.0, -75.0, y)

def _bumps(x, y):
    return (0.35*math.sin(0.11*x + 0.7)*math.cos(0.13*y + 1.3) + 0.20*math.sin(0.23*x + 0.31*y + 2.1)
            + 0.12*math.sin(0.47*x - 0.41*y))

def land(x, y):
    """The ground without the lake basin."""
    h = _base(y)
    n = max(0.0, y - 14.0); h += 0.10*n + 0.0015*n*n                         # rising north, towards the mountain
    r = math.hypot(x, (y + 12.0)/1.15)
    h += 22.0*smooth(75.0, 190.0, r)*(0.75 + 0.25*math.sin(math.atan2(y + 12.0, x)*3.0 + 0.8))   # the valley's wooded walls
    f = smooth(FLAT_R[0], FLAT_R[1], math.hypot(x - FLAT_C[0], y - FLAT_C[1]))
    return h + _bumps(x, y)*f*smooth(1.0, 1.4, lake_d(x, y))

def height(x, y):
    """Ground height, the lake bed included."""
    d = lake_d(x, y)
    bed = WATER_Z - 0.1 - LAKE_DEPTH*(1.0 - min(d, 1.0)**2)
    return bed + (land(x, y) - bed)*smooth(0.85, 1.25, d)

TERRAIN = dict(x0=-200.0, x1=200.0, y0=-210.0, y1=190.0, step=2.0)

# --- the hermitage ----------------------------------------------------------------------------------------------------------
# Buildings: centre (x, y) and turn (radians, 0 = as built: huts face south, the shrine faces east, the gate lets the path through west-east).
PENDAPA = (0.0, 0.0, 0.0)
HUT_ELDERS = (-6.0, 11.5, 0.0)       # Jamadagni and Renuka
HUT_SONS = (5.5, 12.0, 0.0)
PAWON = (10.5, 6.5, -0.35)           # the kitchen
SANGGAR = (-17.0, 8.0, 0.0)          # the stone shrine, facing east
GATE = (-13.5, -2.0, 0.0)
KANDHANG = (17.0, 10.5, 0.2)         # the livestock pen

# The stone steps down to the lake, and the flat bathing stone at the bottom.
STEPS_TOP = (1.5, -12.8)
STEPS_BOTTOM = (2.2, -22.4)
BATHING_STONE = (2.6, -23.6)

# Paths (polylines of (x, y)), 1.2 m wide.
PATHS = [
    [(-17.5, -2.4), GATE[:2], (-9.0, -1.2), (-5.6, -0.8)],                    # in through the gate to the pendapa's west side
    [(0.0, -5.6), (1.0, -9.0), STEPS_TOP],                                     # pendapa steps -> past the banyan -> lake steps
    [(0.0, 5.3), (0.0, 8.5), (-6.0, 8.8)],                                     # pendapa -> the elders' hut
    [(0.0, 8.5), (5.5, 9.4)],                                                  # -> the sons' hut
    [(-5.6, 2.0), (-10.0, 5.5), (-14.6, 8.0)],                                 # -> the shrine
    [(5.6, 2.0), (8.8, 4.6)],                                                  # -> the kitchen
]

# --- plants ---------------------------------------------------------------------------------------------------------------
BANYAN = (8.5, -13.5)                # at the terrace's edge above the lake, south-east of the pendapa; at (5, -9.5) its crown hid the
                                     # pendapa from above and from the lake
KAMBOJA = (-19.8, 11.2)              # the frangipani by the shrine
JASMINE = (-3.0, 8.4)                # Renuka's jasmine by the elders' hut
STUMP = (7.6, -4.6)                  # the woodcutter's stump
BANANAS = [(9.0, 14.0), (13.0, 9.0), (-10.0, 13.5), (-8.8, 15.2)]
TREE_FERNS = [(20.0, 18.0), (24.0, 13.5), (-24.0, 16.0), (-27.0, 9.0)]
BAMBOO_BAND = dict(x0=-32.0, x1=32.0, y0=21.0, y1=30.0, count=14)
GLAGAH = dict(c=(-19.0, -29.0), r=3.5, count=34)      # white plumes on the west shore
REEDS = dict(c=(19.0, -36.0), r=3.0, count=28)        # on the east shore
LOTUS = [dict(c=(-9.0, -31.0), r=3.5, n=14), dict(c=(8.0, -39.0), r=3.0, n=11), dict(c=(-2.0, -43.0), r=2.0, n=6)]
CITRARATA_SPOT = (-10.0, -47.5)      # where the stranger stands on the far bank

def in_clearing(x, y):
    """Open ground around the hermitage, where no forest tree stands."""
    return math.hypot(x/30.0, (y - 4.0)/24.0) < 1.0

def forest_ok(x, y):
    """May a forest tree stand here? Not in the clearing, on the lake or its shore, nor on the open slope down to the water."""
    if in_clearing(x, y) or lake_d(x, y) < 1.35: return False
    if -26.0 < x < 26.0 and -30.0 < y < -4.0: return False
    return True
