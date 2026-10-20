"""Builds blender/objects/fire.blend: fire samples, one collection (an asset) for each kind of fire the story needs, which places
and light states link (the user, 2026-10-18: "can we create objects/fire.blend to create fire collection sample ?").
The method is Max Hay's fire video (references/video-fire-max-hay.md); sizes and rhythm from docs/stories/ramabargawa/objects/fire.md.
Code: common/fire.py (flames, light, sparks, embers), common/fuel.py (logs, coals, the sizes).

The file holds, each with its root at the instance origin:
    Fire_Lamp     the damar's flame, 3.4 cm, and its small light (as in damar.blend)
    Fire_Hearth   Renuka's cooking fire (S1): three sticks pushed into one fire mouth of the luweng, coals, flames 0.2 x 0.24 m,
                  a few sparks
    Fire_Wood     a small campfire: six logs leaning together over two laid flat, coals, flames 0.45 x 0.5 m, sparks
    Fire_Embers   smouldering, no flame (S5): a bed of coals and ash, three charred stubs glowing in their cracks
    Fire_Large    a big log fire (the pyre, S6): four big logs crossed on a wide bed of coals, flames 1.4 x 1.8 m, many sparks
    _Review       the ground, a sun and the sky (not linked)
Run headless: scripts\\run.ps1 object fire."""
import os, sys; sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))   # scripts/
import common; common.reload()
import bpy
from common import paths, mesh, geo, damar, fire, fuel

SPARKS = {   # name: count, radius, height (m over a spark's life), life (s), size (m), swirl (m)
    "Hearth": dict(count=14, radius=0.06, height=0.5, life=1.5, size=0.0025, swirl=0.08),
    "Wood":   dict(count=60, radius=0.15, height=1.6, life=2.5, size=0.003, swirl=0.25),
    "Large":  dict(count=220, radius=0.5, height=5.0, life=4.0, size=0.005, swirl=0.8),
}
DESC = {
    "Hearth": "A cooking fire for one fire mouth of the luweng: three sticks pushed in from the front (-y), coals, flames, sparks",
    "Wood":   "A small campfire: logs leaning together, charred and glowing towards the middle, coals, flames, sparks",
    "Embers": "Smouldering embers, no flame: coals and ash with three charred stubs glowing in their cracks, a dim light",
    "Large":  "A big log fire, the size of a pyre's: four big logs crossed on a wide bed of coals, flames 1.8 m tall, sparks",
}

def sample(name, M, logs, coals, cards=3):
    """One fire sample as a collection: its logs (charred firewood), its coals, the flames and their light (or, with no flame,
    a dim light in the coals), its sparks; the root at fuel.SAMPLES[name]['at'], the instance offset."""
    s = fuel.SAMPLES[name]; at = s["at"]; c = mesh.coll("Fire_" + name); obs = []
    W = mesh.Parts()
    for vf, wear in logs: W.add(vf, M["firewood"], wear)
    obs.append(W.build(f"Fire_{name}_Logs", c, loc=at, smooth=True, sharp=50))
    E = mesh.Parts()
    for vf, wear in coals: E.add(vf, fire.ember_material(), wear)
    obs.append(E.build(f"Fire_{name}_Coals", c, loc=at, smooth=True, sharp=45))
    root = (at[0], at[1], at[2] + s["z0"])
    if s["L"]:
        fl, li = fire.fire_piece(f"Fire_{name}_Flames", c, s["D"], s["L"], s["energy"], at=root, cards=cards,
                                 strength=s["strength"])
        obs.append(fl)
    else:                                                        # embers: a dim light low in the coals, slow
        li = mesh.point_light(f"Fire_{name}_Light", c, (root[0], root[1], root[2] + 0.05), s["energy"], mesh.lin(255, 110, 40),
                              radius=0.4*s["D"])
        fire.flicker(li, s["energy"], amount=0.15, hz=0.4)
    if name in SPARKS: fire.sparks(f"Fire_{name}_Sparks", c, root, seed=len(name), **SPARKS[name])
    c.instance_offset = at
    c.asset_mark(); c.asset_data.description = DESC[name]
    return c, obs, li

def main():
    path = paths.object_file("fire")
    sc = mesh.fresh_file(path); M = mesh.mats()
    fire.charred(M["firewood"])
    lc, lfl, lli = fire.flame_piece("Fire_Lamp", damar.FLAME_H, damar.FLAME_W, 0.5, at=fuel.LAMP_AT, radius=0.02,
                                    desc="The damar's flame (3.4 cm) with its small light; its root is the instance's origin")
    built = {"Hearth": fuel.hearth(fuel.SAMPLES["Hearth"]["D"]), "Wood": fuel.campfire(fuel.SAMPLES["Wood"]["D"]),
             "Embers": fuel.smoulder(fuel.SAMPLES["Embers"]["D"]), "Large": fuel.bonfire(fuel.SAMPLES["Large"]["D"])}
    report = []
    for name, (logs, coals) in built.items():
        c, obs, li = sample(name, M, logs, coals, cards=4 if name == "Large" else 3)
        report.append(f"{c.name}: {sum(len(o.data.polygons) for o in obs)} faces, light {li.data.energy:.0f} W")

    r = mesh.coll("_Review")
    mesh.obj("Review_Ground", geo.cylinder(1.2, 0.0, -0.05, 9.0, 0.05, n=64), r, M["earth"])
    mesh.sun("Sun", r, elev=38.0, az=125.0, energy=3.0, rgb=(1.0, 0.95, 0.88))
    mesh.sky("Review_Sky", mesh.lin(150, 172, 200), 0.7)
    mesh.eevee(sc); sc.render.resolution_x, sc.render.resolution_y = 1920, 1080
    print("STEP Fire:", "; ".join(report), flush=True)
    print("STEP puffing:", ", ".join(f"{n} {fuel.puff_hz(s['D']):.1f} Hz" for n, s in fuel.SAMPLES.items() if s["L"]), flush=True)
    bpy.ops.wm.save_mainfile()
    print("OBJECT saved", path, flush=True)

main()
