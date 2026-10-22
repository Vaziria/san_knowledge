"""Builds blender/places/jatisrana.blend: Pertapan Jatisrana, the hermitage by the lake (sets E01-E03).
Run headless from a factory startup: scripts\\run.ps1 place jatisrana. Design: docs/stories/ramabargawa/environment.md."""
import os, sys; sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))   # scripts/
import common; common.reload()
import bpy
from common import paths, mesh, terrain, buildings, plants, grass, dressing, jatisrana as J

def step(msg): print("STEP", msg, flush=True)

def main():
    path = paths.place_file("jatisrana")
    sc = mesh.fresh_file(path); M = mesh.mats()
    root = mesh.coll("Jatisrana")

    step("ground"); g = mesh.coll("Ground", root)
    t = terrain.terrain(g); terrain.lake(g); terrain.paths(g, M); terrain.mountains(g, M)
    terrain.bare_ground(g, M, t)                               # the user's soil (terrain.BARE): the yard and the far bank

    step("buildings"); b = mesh.coll("Buildings", root)
    px, py, pa = J.PENDAPA                                     # the pendopo is its own file, linked (blender/buildings/pendopo.blend)
    mesh.place_collection(mesh.link_collection(paths.building_file("pendopo", built=True), "Pendopo"), "Pendopo", b,
                          (px, py, J.height(px, py)), pa)
    buildings.hut(b, M, "Hut_Elders", *J.HUT_ELDERS)
    buildings.hut(b, M, "Hut_Sons", *J.HUT_SONS, w=6.0, d=4.5)
    buildings.pawon(b, M, *J.PAWON)
    buildings.sanggar(b, M, *J.SANGGAR)
    buildings.gate(b, M, *J.GATE)
    buildings.kandhang(b, M, *J.KANDHANG)
    buildings.lake_steps(b, M)

    step("plants"); p = mesh.coll("Plants", root)
    n = plants.all_plants(p, M); step(f"forest trees: {n}")
    grass.grass(p, p, M); step("grass")

    step("dressing"); d = mesh.coll("Dressing", root)
    dressing.pendapa_dressing(d, M); dressing.yard_dressing(d, M)
    s = mesh.coll("_ScaleFigures", root); dressing.scale_figures(s, M); s.hide_render = True

    step("light"); l = mesh.coll("Lights", root)
    mesh.sun("Sun", l, elev=38.0, az=125.0, energy=3.0, rgb=(1.0, 0.95, 0.88))
    mesh.sky("Jatisrana_Sky", mesh.lin(150, 172, 200), 0.7)
    mesh.eevee(sc)
    sc.render.resolution_x, sc.render.resolution_y = 1920, 1080

    faces = sum(len(o.data.polygons) for o in bpy.data.objects if o.type == "MESH" and not o.data.users > 1)
    step(f"objects: {len(bpy.data.objects)}, unique-mesh faces: {faces}")
    bpy.ops.wm.save_mainfile()
    print("PLACE saved", path, flush=True)

main()
