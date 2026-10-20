"""Builds blender/objects/stone.blend: the chipped stone pieces, in their own file (the user, 2026-10-18: "separate stone in
objects/stone"). Buildings link them as collection instances; edit the pieces here (common/stone.py, sizes in common/joglo.py),
rebuild this file, then the buildings that link it.

The file holds:
    Stone      one collection per piece, each marked as an asset (what buildings link):
               Umpak_Guru_1..4 (padma umpak of the soko guru), Umpak_Penanggap_1..4 (plain umpak), Stair_Pendopo
    _Review    a ground disc, a sun and the sky, for looking at them here (not linked)
The pieces stand in rows for review; each collection's instance offset is its piece's position, so an instance shows the piece at
the instancing empty. Run headless: scripts\\run.ps1 object stone."""
import os, sys; sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))   # scripts/
import common; common.reload()
import bpy
from common import paths, mesh, geo, joglo

def layout(name):
    """Where a piece stands in this file, small to tall from the front: plain umpak, the stair, the padma umpak at the back."""
    k = name.rpartition("_")[2]
    if name.startswith("Umpak_Guru"): return ((int(k) - 1)*0.9, 1.2, 0.0)
    if name.startswith("Umpak_Penanggap"): return (0.45 + (int(k) - 1)*0.6, -2.3, 0.0)
    return (1.35, -0.6, 0.0)

def main():
    path = paths.object_file("stone")
    sc = mesh.fresh_file(path); M = mesh.mats()
    root = mesh.coll("Stone")
    faces = 0
    for name, P, build in joglo.stone_pieces(M):
        c = mesh.coll(name, root)
        ob = P.build(name, c, loc=layout(name), **build)
        c.instance_offset = tuple(ob.location)
        c.asset_mark(); c.asset_data.description = f"Chipped stone piece {name} (common/stone.py, after the modular stone video)"
        faces += len(ob.data.polygons)
    r = mesh.coll("_Review")
    mesh.obj("Review_Ground", geo.cylinder(1.35, -0.5, -0.3, 3.6, 0.3, n=48), r, mesh.material("M_review_ground", mesh.lin(86, 96, 62), 0.95))
    mesh.sun("Sun", r, elev=38.0, az=125.0, energy=3.0, rgb=(1.0, 0.95, 0.88))
    mesh.sky("Review_Sky", mesh.lin(150, 172, 200), 0.7)
    mesh.eevee(sc); sc.render.resolution_x, sc.render.resolution_y = 1920, 1080
    print(f"STEP Stone: {len(root.children)} pieces ({', '.join(c.name for c in root.children)}), {faces} faces", flush=True)
    bpy.ops.wm.save_mainfile()
    print("OBJECT saved", path, flush=True)

main()
