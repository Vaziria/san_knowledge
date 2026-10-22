"""Builds blender/textures/<family>.blend: one family of the texture library (common/textures.FAMILIES: wood, stone, plants,
ground, metal, palette). Run headless from a factory startup: scripts\\run.ps1 texture <family> (or all); the family comes in
$TEXTURE_FAMILY. Every other file links its materials from these files (mesh.mats), so build them first.

Each material is an asset in the catalog Bargawa/<Family>. A sample of each stands in a row in the collection Samples (a block
and a post, so a texture shows on flat and round faces; the bare grounds a patch each), to look at in Material Preview; nothing
renders here."""
import os, sys; sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))   # scripts/
import common; common.reload()
import bpy
from common import paths, mesh, geo, terrain, textures

def step(msg): print("STEP", msg, flush=True)

def samples(M):
    col = mesh.coll("Samples")
    for i, (key, m) in enumerate(M.items()):
        x = 1.6*i
        P = mesh.Parts()
        if key in terrain.RELIEF_GROUNDS:                                               # a patch, its relief by GN
            terrain.relief_sample(col, m, x, key); continue
        if key == "thatch":                                                             # a piece of roof, its relief by GN
            textures.thatch_sample(col, m, x); continue
        if key == "water":
            P.add(geo.box(x, 0, 0, 1.0, 1.0, 0.02), m)
        else:
            P.add(geo.box(x, 0, 0, 0.6, 0.6, 0.6), m)                                  # a block
            P.add(geo.cylinder(x, 0.75, 0, 0.09, 1.2, n=16), m)                         # a post (timber gets its grain UVs)
        P.build("Sample_" + key, col, smooth=False)

def main():
    family = os.environ.get("TEXTURE_FAMILY")
    if family not in textures.FAMILIES:
        raise SystemExit(f"TEXTURE_FAMILY must be one of {', '.join(textures.FAMILIES)}")
    path = paths.texture_file(family)
    mesh.fresh_file(path)
    step(f"materials: {family}")
    M = textures.build_family(family)
    step("samples")
    samples(M)
    textures.write_catalogs()
    bpy.ops.file.make_paths_relative()          # the Poly Haven images, relative to this file, so links resolve from anywhere
    bpy.ops.wm.save_mainfile()
    print("TEXTURE saved", path, "materials:", ", ".join(M), flush=True)

main()
