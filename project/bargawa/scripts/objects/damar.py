"""Builds blender/objects/damar.blend: Jamadagni's oil lamp (damar), a bronze pole lamp on a teak stand, in its own file.
The model is common/damar.py; the research docs/stories/ramabargawa/objects/damar.md.

The file holds:
    Damar        the lamp as lit in S1 (what places link), an asset:
                 Damar_Stand (teak), Damar_Lamp (the bronze head, oil, wicks), Damar_Flame_1..4 (instances of Damar_Flame)
    Damar_Flame  one flame and its small warm light, its root at the instance offset, an asset: light states can put one on
                 any wick (common/damar.FLAMES) to light only that one. The flame moves with the frame (common/fire.py), each
                 instance at its own moment; the light flickers gently
    _Review      a ground disc, a sun and the sky (not linked)
Run headless: scripts\\run.ps1 object damar."""
import os, sys; sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))   # scripts/
import common; common.reload()
import bpy
from common import paths, mesh, geo, damar, fire

FLAME_AT = (0.3, 0.0, damar.FLAMES[0][2])        # where the lone flame piece stands in this file, beside the lamp

def main():
    path = paths.object_file("damar")
    sc = mesh.fresh_file(path); M = mesh.mats()
    fc, fl, li = fire.flame_piece("Damar_Flame", damar.FLAME_H, damar.FLAME_W, 0.5, at=FLAME_AT, radius=0.02,   # at 1.5 W and
                                  desc="One flame of the damar with its light; its root is the instance's origin")  # 1 cm the
                                                                 # bronze glared copper-pink by day; the light radius is the flame's
                                                                 # size, so its glint is the flame's own (the video turns MIS off)

    col = mesh.coll("Damar")
    S = mesh.Parts()
    for vf in damar.stand(): S.add(vf, M["teak_turned"])
    S.build("Damar_Stand", col, smooth=True, sharp=40)
    B = mesh.Parts(); h = damar.head()
    for vf, wear in h["bronze"]: B.add(vf, M["bronze"], wear)
    for key in ("oil", "wick", "soot"):
        for vf in h[key]: B.add(vf, M[key])
    B.build("Damar_Lamp", col, smooth=True, sharp=40)
    for k, p in enumerate(damar.FLAMES):                                                       # all four lit (S1)
        mesh.place_collection(fc, f"Damar_Flame_{k + 1}", col, p)
    col.asset_mark(); col.asset_data.description = ("Jamadagni's damar: an Old Javanese bronze pole lamp with four oil dishes and "
                                                    "a seated ascetic, on a turned teak stand; all four flames lit")

    r = mesh.coll("_Review")
    mesh.obj("Review_Ground", geo.cylinder(0.1, 0.0, -0.05, 0.7, 0.05, n=48), r, M["mat"])     # a pandan mat's colour, as on the floor
    mesh.sun("Sun", r, elev=38.0, az=125.0, energy=3.0, rgb=(1.0, 0.95, 0.88))
    mesh.sky("Review_Sky", mesh.lin(150, 172, 200), 0.7)
    mesh.eevee(sc); sc.render.resolution_x, sc.render.resolution_y = 1920, 1080
    obs = [o for o in col.all_objects if o.type == "MESH"] + [fl]
    print(f"STEP Damar: {', '.join(o.name for o in obs)}; {sum(len(o.data.polygons) for o in obs)} faces; "
          f"flames at {', '.join('(%.3f, %.3f, %.3f)' % p for p in damar.FLAMES)}; light {li.data.energy} W, "
          f"flicker driver simple: {li.data.animation_data.drivers[0].driver.is_simple_expression}", flush=True)
    bpy.ops.wm.save_mainfile()
    print("OBJECT saved", path, flush=True)

main()
