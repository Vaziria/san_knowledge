"""The pendapa of Jatisrana: an open joglo pavilion on a stone plinth, steps on the south side.

Parts of a joglo (names as used in Javanese building):
    bebatur        the raised stone plinth
    umpak          stone bases under the pillars
    soko guru      the four main pillars in the middle, carrying the tumpang sari and the top roof
    soko penanggap the ring of twelve pillars round them, carrying the middle roof
    blandar        the beams on top of the pillars
    sunduk         tie beams between pillars
    sunduk, kili   the tie beam through the soko guru below their tops, and the peg that locks it
    tumpang sari   stacked square frames on the soko guru: the outer band steps outward going up and carries the rafters, the
                   inner band steps inward and makes the stepped ceiling (uleng); the nanasan (prit gantil) hangs at its centre
    brunjung       the steep top roof up to the short ridge (molo); penanggap the gentler roof round it
    usuk           rafters; dudur the hip rafters at the corners; reng the battens across the rafters
    padma          the lotus-carved umpak under the soko guru
Research: docs/stories/ramabargawa/buildings/pendopo.md. Two roof tiers: a simple Joglo Lawakan, as suits a hermitage.
Built at the origin (ground level), facing south; the place moves it into position. The stone pieces (umpak, stair) come from
blender/objects/stone.blend and the soko from blender/objects/soko.blend, linked (see STONE_GURU, SOKO_GURU)."""
import math
from . import mesh, geo, paths, textures

# Proportions after the pakem (docs/stories/ramabargawa/buildings/pendopo.md, "Pakem soko"; the user, 2026-10-18: "yes, fix
# proportion"): the real Joglo Lawakan of the Pendopo Agung Tamansiswa scaled to our 3.2 m soko guru frame, and the Kawruh Kalang
# umpak rule (soko guru umpak height = the post's perimeter, soko penanggap umpak = half of it), which Tamansiswa confirms.
F = 0.6                          # floor (top of the plinth); Tamansiswa's is 0.6 m too
PLINTH = 10.4
GURU, PEN = 1.6, 4.7             # half-spacing of the soko guru and of the soko penanggap ring (was 4.0: the ring moved out so
                                 # the penanggap roof keeps its pitch over the lower ring)
SG_GURU, SG_PEN = 0.18, 0.13     # post sections (were 0.30 and 0.23, twice as thick as Tamansiswa's 1:28 and 1:25)
H_UMPAK_GURU = 4*SG_GURU         # 0.72 m: the perimeter of the soko guru (Kawruh Kalang)
H_UMPAK_PEN = 2*SG_PEN           # 0.26 m: half the perimeter of the soko penanggap
Z_GURU, Z_PEN = F + 4.0, F + 2.45  # post tops; soko guru 4.0 m above the floor (as before, in Tamansiswa's proportion), soko
                                   # penanggap 2.45 m (was 3.0), so guru : penanggap is 1.6, near Tamansiswa's 1.7
PA, PZ = 2.4, F + 4.81           # the penanggap roof's top edge, tucked under the brunjung
EA = 6.0                         # its eave half-size
ZE = PZ - (EA - PA)/(PEN - PA)*(PZ - (Z_PEN + 0.58))   # its eave height: the roof passes just over the ring's blandar
BA, BZ = 2.85, F + 4.70          # brunjung: eave half-size and height (it overhangs the penanggap)
ZR, RH = F + 8.0, 1.0            # ridge height and half-length (east-west)
THICK = 0.3                      # thatch thickness
N_SIDE = 40                      # roof points per side: 0.3 m apart along the eave, for the thatch's relief (was 16)
ROOF_STEP = 0.3                  # and rings at most this far apart up the slope

def _sides(vf):
    """A piece made for the south side, repeated on all four sides."""
    return geo.merge(*[geo.moved(vf, ang=k*math.pi/2) for k in range(4)])

def _base(P, M):
    P.add(geo.box(0, 0, -0.45, PLINTH + 0.4, PLINTH + 0.4, 0.6), M["stone_dark"])          # foot course, sunk in the ground
    P.add(geo.box(0, 0, 0.15, PLINTH, PLINTH, F - 0.15), M["stone"])
    P.add(geo.box(0, 0, F - 0.08, PLINTH + 0.16, PLINTH + 0.16, 0.08), M["stone_dark"])     # coping round the floor's edge
    # the steps are the stone piece Stair_Pendopo from objects/stone.blend, linked as Pendopo_Stair (pendapa() -> _linked)

GURU_POSTS = [(sx*GURU, sy*GURU) for sx in (-1, 1) for sy in (-1, 1)]                      # the 4 soko guru
PEN_POSTS = [(x, y) for x in (-PEN, -PEN/3, PEN/3, PEN) for y in (-PEN, -PEN/3, PEN/3, PEN)
             if abs(x) == PEN or abs(y) == PEN]                                                  # the 12 soko penanggap

# Every umpak is one carved stone in the chipped style of the stair (common/stone.py, after the modular stone video): softened
# wandering edges, chips, worn edges lighter (the user: "and refine stone in soko like our reference"). The stone pieces live in
# their own file, blender/objects/stone.blend (scripts/objects/stone.py; the user, 2026-10-18: "separate stone in objects/stone"),
# and the pendopo links them as collection instances, a modular kit as in the stone video:
#   Umpak_Guru_1..4        a padma umpak under each soko guru
#   Umpak_Penanggap_1..4   plain umpak, each under 3 soko penanggap, turned a quarter further each time, so neighbours differ
#   Stair_Pendopo          the south steps
STONE_GURU = [f"Umpak_Guru_{k + 1}" for k in range(4)]
STONE_PEN = [f"Umpak_Penanggap_{k + 1}" for k in range(4)]
STONE_STAIR = "Stair_Pendopo"

def stone_pieces(M):
    """The pendopo's stone pieces for objects/stone.blend, each standing on z 0 at its own origin (the stair's back edge, where
    it meets the plinth, at y 0): [(collection name, Parts, build options)]."""
    from . import stone
    soft = dict(smooth=True, sharp=30)                                                   # soft stone, crisp chips
    out = []
    for k, name in enumerate(STONE_GURU):
        vf, wear = stone.padma(0.0, 0.0, 0.0, seed=300 + k, h=H_UMPAK_GURU, base=3.1*SG_GURU, seat=SG_GURU + 0.03)
        out.append((name, mesh.Parts().add(vf, M["stone_kit"], wear), soft))
    base, top = 2.3*SG_PEN, SG_PEN + 0.07                                                # Tamansiswa: 32 cm umpak for 15 cm posts
    for k, name in enumerate(STONE_PEN):
        vf, wear = stone.umpak(seed=400 + k, base=base, top=top, h=H_UMPAK_PEN)
        out.append((name, mesh.Parts().add(geo.moved(vf, dz=H_UMPAK_PEN/2), M["stone_kit"], wear), soft))
    out.append((STONE_STAIR, stone.stair(M, y_back=0.0), stone.STAIR_BUILD))
    return out

# The soko live in their own file too, blender/objects/soko.blend (scripts/objects/soko.py; the user, 2026-10-21: "separate soko
# in other file"), linked the same way, each post on its umpak:
#   Soko_Guru_1..4         a soko guru on each padma umpak, each with its own grain
#   Soko_Penanggap_1..4    each 3 soko penanggap, turned as their umpak are, so neighbours show different faces of the grain
SOKO_GURU = [f"Soko_Guru_{k + 1}" for k in range(4)]
SOKO_PEN = [f"Soko_Penanggap_{k + 1}" for k in range(4)]

def soko_pieces(M):
    """The pendopo's soko for objects/soko.blend, each post standing on z 0 (the top of its umpak) at its own origin:
    [(collection name, Parts, build options)]."""
    out = []
    for k, name in enumerate(SOKO_GURU):
        h = Z_GURU - (F + H_UMPAK_GURU)
        out.append((name, mesh.Parts().add(geo.taper(0.0, 0.0, 0.0, SG_GURU, SG_GURU - 0.01, h), M["teak"], seed=500 + k), {}))
    for k, name in enumerate(SOKO_PEN):
        h = Z_PEN - (F + H_UMPAK_PEN)
        out.append((name, mesh.Parts().add(geo.taper(0.0, 0.0, 0.0, SG_PEN, SG_PEN - 0.01, h), M["teak"], seed=600 + k), {}))
    return out

def _linked(stone_file, soko_file, guru_col, pen_col, stair_col, x, y, z, ang):
    """Collection instances of the linked pieces: an umpak under every post and its soko on it, and the stair against the
    plinth's south side."""
    S = mesh.link_collections(stone_file, STONE_GURU + STONE_PEN + [STONE_STAIR])
    W = mesh.link_collections(soko_file, SOKO_GURU + SOKO_PEN)
    c, s = math.cos(ang), math.sin(ang)
    at = lambda px, py, pz: (x + c*px - s*py, y + s*px + c*py, z + pz)
    for k, (px, py) in enumerate(GURU_POSTS):
        mesh.place_collection(S[STONE_GURU[k]], f"Pendopo_UmpakGuru_{k + 1}", guru_col, at(px, py, F), ang)
        mesh.place_collection(W[SOKO_GURU[k]], f"Pendopo_SokoGuru_{k + 1}", guru_col, at(px, py, F + H_UMPAK_GURU), ang)
    for k, (px, py) in enumerate(PEN_POSTS):
        turn = ang + (k//4 + k % 2)*math.pi/2
        mesh.place_collection(S[STONE_PEN[k % 4]], f"Pendopo_UmpakPenanggap_{k + 1}", pen_col, at(px, py, F), turn)
        mesh.place_collection(W[SOKO_PEN[k % 4]], f"Pendopo_SokoPenanggap_{k + 1}", pen_col, at(px, py, F + H_UMPAK_PEN), turn)
    mesh.place_collection(S[STONE_STAIR], "Pendopo_Stair", stair_col, at(0.0, -PLINTH/2, 0.0), ang)

TS_TIERS, TS_H = 4, 0.15         # tumpang sari: tiers and the depth of each

def ts_edges(i):
    """Inner and outer half-size of tumpang sari tier i (0 at the bottom): the outer band steps out and carries the rafters, the
    inner band steps in and makes the stepped ceiling (uleng)."""
    return GURU - 0.12 - 0.19*i, GURU + 0.12 + 0.17*(i + 1)

def _tumpang_sari(P, M):
    z = Z_GURU + 0.24
    for i in range(TS_TIERS):
        a_in, a_out = ts_edges(i); w = a_out - a_in
        for s in (-1, 1):
            P.add(geo.box(0, s*(a_in + w/2), z, 2*a_out, w, TS_H), M["teak"])                    # south / north beams of the tier
            P.add(geo.box(s*(a_in + w/2), 0, z, w, 2*a_in, TS_H), M["teak"])                     # east / west beams between them
            P.add(geo.box(0, s*(a_in - 0.015), z, 2*a_in, 0.03, 0.04), M["teak_dark"])           # a carved lip on the inner edge
            P.add(geo.box(s*(a_in - 0.015), 0, z, 0.03, 2*a_in - 0.06, 0.04), M["teak_dark"])
        z += TS_H
    a_top = ts_edges(TS_TIERS - 1)[0]
    P.add(geo.box(0, 0, z - 0.03, 2*a_top + 0.06, 2*a_top + 0.06, 0.05), M["teak_dark"])       # the board closing the top
    P.add(geo.cylinder(0, 0, z - 0.15, 0.05, 0.12, n=10), M["teak"])                           # nanasan (prit gantil), hanging
    P.add(geo.ellipsoid(0, 0, z - 0.33, 0.12, 0.12, 0.19, nu=12, nv=8), M["teak"])
    for k in range(3): P.add(geo.cylinder(0, 0, z - 0.44 + 0.1*k, 0.122 - 0.012*abs(k - 1), 0.016, n=12), M["teak_dark"])
    P.add(geo.cylinder(0, 0, z - 0.66, 0.004, 0.16, n=10, r1=0.07), M["teak"])

def _beams(P, M):
    sq = lambda a, z, w, d, ext: _sides(geo.beam((-a - ext, -a, z), (a + ext, -a, z), w, d))
    # beam sections follow the slimmer posts (Kawruh Kalang: the sunduk as thick as the midhangan, its other side 3/4 of that)
    P.add(sq(PEN, Z_PEN + 0.1, 0.15, 0.2, 0.3), M["teak"])                                 # blandar on the penanggap ring
    P.add(sq(GURU, Z_GURU + 0.12, 0.22, 0.24, 0.3), M["teak"])                             # blandar (midhangan) on the soko guru
    zs = Z_GURU - 0.55
    P.add(sq(GURU, zs, 0.165, 0.22, 0.32), M["teak"])                                      # sunduk, through the soko guru
    P.add(_sides(geo.merge(*[geo.box(s*(GURU + 0.24), -GURU, zs - 0.16, 0.05, 0.09, 0.32) for s in (-1, 1)])), M["teak_dark"])   # kili
    for s in (-1, 1):                                                                        # tie beams from the ring to the soko guru
        P.add(_sides(geo.beam((s*GURU, -PEN, Z_PEN + 0.1), (s*GURU, -GURU, Z_PEN + 0.1), 0.12, 0.17)), M["teak"])
    _tumpang_sari(P, M)

def _face_normal(p0, p1, p2):
    ax, ay, az = (p1[i] - p0[i] for i in range(3)); bx, by, bz = (p2[i] - p0[i] for i in range(3))
    n = (ay*bz - az*by, az*bx - ax*bz, ax*by - ay*bx); L = math.sqrt(sum(c*c for c in n))
    return tuple(c/L for c in n)

def _under(p, n, off):
    return tuple(p[i] - n[i]*off for i in range(3))

def _rafters(P, M):
    off = THICK + 0.07
    # penanggap: south face from the eave (y = -EA) up to its top edge (y = -PA); rafters end on the hips at the corners
    n = _face_normal((-1, -EA, ZE), (1, -EA, ZE), (0, -PA, PZ))
    zat = lambda a: ZE + (EA - a)/(EA - PA)*(PZ - ZE)
    south = []
    t = -EA + 0.35
    while t < EA - 0.3:
        top = max(abs(t), PA)
        if EA - top > 0.3: south.append(geo.beam(_under((t, -EA + 0.05, zat(EA - 0.05)), n, off), _under((t, -top, zat(top)), n, off), 0.07, 0.09))
        t += 0.45
    hip_n = (-n[1]*0.5**0.5, n[1]*0.5**0.5, n[2]); L = math.sqrt(sum(c*c for c in hip_n)); hip_n = tuple(c/L for c in hip_n)   # between the south and east faces
    south.append(geo.beam(_under((EA, -EA, ZE), hip_n, off), _under((PA, -PA, PZ), hip_n, off), 0.12, 0.13))   # dudur
    P.add(_sides(geo.merge(*south)), M["teak_dark"])
    # brunjung: the long faces (south, north) rise to the ridge; the ends (east, west) to the ridge's tips
    ns = _face_normal((-1, -BA, BZ), (1, -BA, BZ), (0, 0, ZR))
    long_face = []
    t = -BA + 0.3
    while t < BA - 0.25:
        if abs(t) <= RH: end = (t, 0.0, ZR)
        else:
            s = (BA - abs(t))/(BA - RH); end = (t, -BA*(1 - s), BZ + s*(ZR - BZ))
        if math.dist((t, -BA, BZ), end) > 0.3: long_face.append(geo.beam(_under((t, -BA + 0.04, BZ + 0.03), ns, off), _under(end, ns, off), 0.06, 0.08))
        t += 0.4
    long_face = geo.merge(*long_face)
    P.add(geo.merge(long_face, geo.moved(long_face, ang=math.pi)), M["teak_dark"])
    ne = _face_normal((BA, -1, BZ), (BA, 1, BZ), (RH, 0, ZR))
    end_face = []
    t = -BA + 0.3
    while t < BA - 0.25:
        s = 1.0 - abs(t)/BA; end = (BA - (BA - RH)*s, t, BZ + s*(ZR - BZ))
        if math.dist((BA, t, BZ), end) > 0.3: end_face.append(geo.beam(_under((BA - 0.04, t, BZ + 0.03), ne, off), _under(end, ne, off), 0.06, 0.08))
        t += 0.4
    end_face = geo.merge(*end_face)
    P.add(geo.merge(end_face, geo.moved(end_face, ang=math.pi)), M["teak_dark"])
    for sx in (-1, 1):                                                                       # brunjung dudur: corners up to the ridge's tips
        for sy in (-1, 1):
            nc = (sx*abs(ne[0]), sy*abs(ns[1]), ns[2] + ne[2]); L = math.sqrt(sum(c*c for c in nc)); nc = tuple(c/L for c in nc)
            P.add(geo.beam(_under((sx*BA, sy*BA, BZ), nc, off), _under((sx*RH, 0.0, ZR), nc, off), 0.11, 0.12), M["teak_dark"])
    _battens(P, M, n, ns, ne)

def _battens(P, M, n, ns, ne, spacing=0.3):
    """Reng: battens across the rafters, just under the thatch, on every face (they show from inside)."""
    off = THICK + 0.018; mat = M["bamboo"] if "bamboo" in M else M["teak"]
    def rows(L):
        k = max(2, int(L/spacing)); return [(i + 0.5)/k for i in range(k)]
    pen = []
    for u in rows(math.hypot(EA - PA, PZ - ZE)):
        a = EA - (EA - PA)*u; z = ZE + (PZ - ZE)*u
        pen.append(geo.beam(_under((-a, -a, z), n, off), _under((a, -a, z), n, off), 0.04, 0.03))
    P.add(_sides(geo.merge(*pen)), mat)
    lf, ef = [], []
    for u in rows(math.hypot(BA, ZR - BZ)):
        y = BA*(1 - u); xh = RH + (BA - RH)*(1 - u); z = BZ + (ZR - BZ)*u
        lf.append(geo.beam(_under((-xh, -y, z), ns, off), _under((xh, -y, z), ns, off), 0.04, 0.03))
    for u in rows(math.hypot(BA - RH, ZR - BZ)):
        x = RH + (BA - RH)*(1 - u); yh = BA*(1 - u); z = BZ + (ZR - BZ)*u
        if yh > 0.15: ef.append(geo.beam(_under((x, -yh, z), ne, off), _under((x, yh, z), ne, off), 0.04, 0.03))
    lf, ef = geo.merge(*lf), geo.merge(*ef)
    P.add(geo.merge(lf, geo.moved(lf, ang=math.pi), ef, geo.moved(ef, ang=math.pi)), mat)

def _dense_rings(rings, step=ROOF_STEP):
    """The rings with rings added between each pair, at most `step` apart (the same piecewise-straight shape)."""
    out = [rings[0]]
    for a, b in zip(rings, rings[1:]):
        n = max(1, math.ceil(max(math.dist(p, q) for p, q in zip(a, b))/step - 1e-6))
        out += [[tuple(p[i] + (q[i] - p[i])*k/n for i in range(3)) for p, q in zip(a, b)] for k in range(1, n + 1)]
    return out

def _roof_shell():
    """The thatch as an open shell (thickness comes from a Solidify modifier). The eave edge is a little uneven, like real thatch.
    Its faces are at most about ROOF_STEP across, for the thatch's relief (textures.thatch_relief)."""
    eave = []
    for (x, y, z) in geo.square_ring(EA, ZE, N_SIDE):
        j = geo.hash01(x, y); eave.append((x*(1 + 0.008*(j - 0.5)), y*(1 + 0.008*(geo.hash01(y, x) - 0.5)), z - 0.07*j))
    mid = geo.square_ring((EA + PA)/2, (ZE + PZ)/2, N_SIDE)
    top = geo.square_ring(PA, PZ, N_SIDE)
    penanggap = geo.loft(_dense_rings([eave, mid, top]))
    nb = math.ceil(2*BA/ROOF_STEP)                                      # the brunjung is smaller: fewer points per side
    b0 = []
    for (x, y, z) in geo.square_ring(BA, BZ, nb):
        j = geo.hash01(y, x, 1); b0.append((x, y, z - 0.05*j))
    ridge = [(x*RH/BA, 0.0, ZR) for (x, y, z) in geo.square_ring(BA, BZ, nb)]
    b1 = [tuple(a[i] + (b[i] - a[i])*0.5 for i in range(3)) for a, b in zip(b0, ridge)]
    brunjung = geo.loft(_dense_rings([b0, b1, ridge]))
    return geo.merge(penanggap, brunjung)

def pendapa(col, M, x=0.0, y=0.0, ang=0.0, z=0.0, stair_col=None, guru_col=None, pen_col=None, stone_file=None, soko_file=None):
    """Builds the pendopo, each part the user may want to handle on its own:
        Pendopo                the plinth, beams, tumpang sari, rafters and battens
        Pendopo_SokoGuru_1..4  the soko guru posts, with Pendopo_UmpakGuru_1..4 under them           (in guru_col if given)
        Pendopo_SokoPenanggap_1..12  the soko penanggap posts, with Pendopo_UmpakPenanggap_1..12     (in pen_col if given)
        Pendopo_Stair          the south steps                                                       (in stair_col if given)
        Pendopo_Roof           the thatch;  Pendopo_Ridge  the ridge cap
    The umpak and the stair are instances of the stone pieces in objects/stone.blend (stone_file), the posts of the soko in
    objects/soko.blend (soko_file); build those first.
    The user, 2026-10-18: "separate pendopo stair, and make it stone modelling like reference that extracted from video",
    "so we have 2 soko ? separate it", "separate stone in objects/stone"; 2026-10-21: "separate soko in other file"."""
    place = dict(loc=(x, y, z), rot_z=ang)
    guru_col, pen_col, stair_col = guru_col or col, pen_col or col, stair_col or col
    _linked(stone_file or paths.object_file("stone", built=True), soko_file or paths.object_file("soko", built=True),
            guru_col, pen_col, stair_col, x, y, z, ang)
    P = mesh.Parts()
    _base(P, M); _beams(P, M); _rafters(P, M)
    body = P.build("Pendopo", col, **place)
    roof = mesh.Parts().add(_roof_shell(), M["thatch"]).build("Pendopo_Roof", col, recalc=False, weld=1e-4, solidify=THICK,
                                                              smooth=False, **place)
    textures.thatch_relief(roof)                                        # the sasak thatch's real relief
    R = mesh.Parts()
    R.add(geo.beam((-RH - 0.4, 0, ZR + 0.06), (RH + 0.4, 0, ZR + 0.06), 0.44, 0.3), M["ridge"])
    for s in (-1, 1): R.add(geo.cylinder(s*(RH + 0.32), 0, ZR + 0.18, 0.12, 0.55, n=8, r1=0.0), M["ridge"])
    ridge = R.build("Pendopo_Ridge", col, **place)
    return body, roof, ridge
