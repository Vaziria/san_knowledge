"""Shared code for the Ramabargawa Blender scripts. A script starts with:

    import os, sys; sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))   # scripts/
    import common; common.reload()                       # a live Blender (MCP) keeps old copies of these modules

Modules:
    paths      project folders (ROOT, PLACES, place_file, building_file, object_file, checks_dir)
    geo        vertex/face lists for simple solids: box, taper, cylinder, ellipsoid, loft, beam (no bpy)
    mesh       bpy side: fresh_file, coll, material palette, Parts (pieces -> one mesh), instance, sun, sky
    jatisrana  the hermitage place's measures: terrain height, lake, where everything stands (no bpy)
    terrain    terrain, lake water, paths, the mountains
    joglo      the pendopo (open joglo pavilion); built into its own file by buildings/pendopo.py
    buildings  huts, pawon, sanggar pamujan, gate, kandhang, lake steps
    stone      stylized chipped stone blocks with a 'wear' value per point, umpak, padma, and stair() (the pendopo's steps);
               built into blender/objects/stone.blend by objects/stone.py, which buildings link
    plants     banyan, bamboo, banana, kamboja, jasmine, lotus, glagah, reeds, tree ferns, the far forest
    grass      grass over the clearing (hair particles on a hidden emitter with a density map)
    damar      Jamadagni's oil lamp: a bronze pole lamp with four dishes and an ascetic, on a teak stand (objects/damar.py)
    fire       flames drawn on cards by their shader, moving with the frame, and a flickering light (after Max Hay's video);
               wood fires, sparks (Geometry Nodes), embers and charred wood; objects/fire.py builds the samples
    fuel       what the fire samples burn: firewood and beds of coals, each point's 'wear' how burned it is; the fires' sizes (no bpy)
    dressing   mats, bale, the damar (linked from blender/objects/damar.blend), stump and axe, pots, scale figures
    polyhaven  the Poly Haven textures used and where their files are (no bpy)
    textures   those textures on the palette materials (box projection, bump)
"""

def reload():
    """Forget the loaded common.* modules, so the next import reads the files again (needed in a long-running Blender)."""
    import sys
    for m in [m for m in sys.modules if m.startswith(__name__ + ".")]:
        del sys.modules[m]
        globals().pop(m.split(".", 1)[1], None)
