"""Builds blender/buildings/pendopo.blend: the hermitage's pendopo (pendapa), an open joglo pavilion, in its own file
(the user, 2026-10-18: "separate pendopo to other blender file buildings/pendopo").

The file holds:
    Pendopo    the building at the origin, facing south (what places link)
    _Review    a ground disc, a sun and the sky, for looking at it here (not linked)
Run headless: scripts\\run.ps1 building pendopo. The model itself is common/joglo.py. Its umpak and stair are linked from
blender/objects/stone.blend: build that first (scripts\\run.ps1 object stone)."""
import os, sys; sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))   # scripts/
import common; common.reload()
import bpy
from common import paths, mesh, geo, joglo

def main():
    path = paths.building_file("pendopo")
    sc = mesh.fresh_file(path); M = mesh.mats()
    col = mesh.coll("Pendopo")
    joglo.pendapa(col, M, stair_col=mesh.coll("Pendopo_Stair", col),              # the stair and the two kinds of soko:
                  guru_col=mesh.coll("Pendopo_SokoGuru", col),                     # each its own object and collection
                  pen_col=mesh.coll("Pendopo_SokoPenanggap", col))
    col.asset_mark(); col.asset_data.description = "The hermitage's pendopo: an open joglo pavilion on a stone plinth, facing south"
    r = mesh.coll("_Review")
    mesh.obj("Review_Ground", geo.cylinder(0, 0, -0.3, 14.0, 0.3, n=48), r, mesh.material("M_review_ground", mesh.lin(86, 96, 62), 0.95))
    mesh.sun("Sun", r, elev=38.0, az=125.0, energy=3.0, rgb=(1.0, 0.95, 0.88))
    mesh.sky("Review_Sky", mesh.lin(150, 172, 200), 0.7)
    mesh.eevee(sc); sc.render.resolution_x, sc.render.resolution_y = 1920, 1080
    obs = [o for o in col.all_objects if o.type == "MESH"]
    print(f"STEP Pendopo: {len(obs)} objects ({', '.join(sorted(o.name for o in obs))}), "
          f"{sum(len(o.data.polygons) for o in obs)} faces", flush=True)
    bpy.ops.wm.save_mainfile()
    print("BUILDING saved", path, flush=True)

main()
