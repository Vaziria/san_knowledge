"""Builds blender/objects/soko.blend: the pendopo's teak posts (soko), in their own file (the user, 2026-10-21: "separate soko in
other file"). Buildings link them as collection instances, each post on its umpak from objects/stone.blend; edit the posts here
(sizes in common/joglo.py), rebuild this file, then the buildings that link it.

The file holds:
    Soko       one collection per post, each marked as an asset (what buildings link):
               Soko_Guru_1..4 (the soko guru, 3.28 m), Soko_Penanggap_1..4 (the soko penanggap, 2.19 m)
    _Review    a ground disc, a sun and the sky, for looking at them here (not linked)
The posts stand in two rows for review, each on z 0 where its umpak's top would be; each collection's instance offset is its
post's foot, so an instance shows the post at the instancing empty. Run headless: scripts\\run.ps1 object soko."""
import os, sys; sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))   # scripts/
import common; common.reload()
import bpy
from common import paths, mesh, geo, joglo

def layout(name):
    """Where a post stands in this file: the soko guru in a row at y 0.6, the shorter soko penanggap in front of the gaps."""
    k = int(name.rpartition("_")[2]) - 1
    return (k*0.7, 0.6, 0.0) if name.startswith("Soko_Guru") else (0.35 + k*0.7, -0.6, 0.0)

def main():
    path = paths.object_file("soko")
    sc = mesh.fresh_file(path); M = mesh.mats()
    root = mesh.coll("Soko")
    faces = 0
    for name, P, build in joglo.soko_pieces(M):
        c = mesh.coll(name, root)
        ob = P.build(name, c, loc=layout(name), **build)
        c.instance_offset = tuple(ob.location)
        kind = "soko guru" if name.startswith("Soko_Guru") else "soko penanggap"
        c.asset_mark(); c.asset_data.description = f"Teak post {name}: a {kind} of the pendopo, its foot at the origin"
        faces += len(ob.data.polygons)
    r = mesh.coll("_Review")
    mesh.obj("Review_Ground", geo.cylinder(1.2, 0.0, -0.3, 2.4, 0.3, n=48), r, mesh.material("M_review_ground", mesh.lin(86, 96, 62), 0.95))
    mesh.sun("Sun", r, elev=38.0, az=125.0, energy=3.0, rgb=(1.0, 0.95, 0.88))
    mesh.sky("Review_Sky", mesh.lin(150, 172, 200), 0.7)
    mesh.eevee(sc); sc.render.resolution_x, sc.render.resolution_y = 1920, 1080
    print(f"STEP Soko: {len(root.children)} posts ({', '.join(c.name for c in root.children)}), {faces} faces", flush=True)
    bpy.ops.wm.save_mainfile()
    print("OBJECT saved", path, flush=True)

main()
