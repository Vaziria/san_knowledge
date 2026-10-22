"""The Poly Haven textures the project uses (all CC0, polyhaven.com), and where their files live (no bpy).
Fetched by scripts\\run.ps1 fetch into blender/assets/polyhaven/<id>/<id>_<map>_1k.<ext> (git-ignored; this list is the record).

TEXTURES: palette material key -> (Poly Haven id, tile size in metres, how much of the palette colour to mix in, bump strength)."""
import os
from . import paths

DIR = os.path.join(paths.ROOT, "blender", "assets", "polyhaven")
RES = "1k"
MAPS = ("Diffuse", "Rough", "Displacement")

TEXTURES = {
    # "thatch" was reed_roof_04 (1.0 m tiles); since 2026-10-20 it is the user's sasak photo (textures.thatch_material)
    "woven":      ("bamboo_wall_03", 1.0, 0.15, 0.3),       # gedhek
    "teak":       ("teak_veneer", 1.2, 0.25, 0.15),          # real teak, straight grain (wood_table_worn was a swirly
    "teak_dark":  ("teak_veneer", 1.2, 0.5, 0.15),           # polished tabletop that read as leather on the posts)
    "teak_turned": ("teak_veneer", 0.4, 0.55, 0.05),         # small turned pieces (the damar's stand): finer grain, oiled
    "stone":      ("rock_surface", 2.0, 0.3, 0.4),
    "stone_kit":  ("rock_surface", 1.4, 0.35, 0.35),        # chipped stone blocks; the wear tint is added in textures.py
    "stone_dark": ("rock_wall_08", 1.6, 0.25, 0.5),         # andesite blocks: shrine, umpak, foot courses
    "bark":       ("tree_bark_03", 1.8, 0.55, 0.5),         # the banyan's grey bark (at 0.2 of the palette colour it glowed)
    "bark_dark":  ("bark_platanus", 1.4, 0.2, 0.4),
    "firewood":   ("bark_brown_01", 0.4, 0.3, 0.4),         # the fire samples' logs (environment.md lists it for the pyre); at
                                                            # 0.8 m its plates looked like wrinkled tubes on 2-3 cm sticks
    "earth":      ("forest_ground_04", 2.5, 0.25, 0.3),     # paths, kitchen floor
    "ground":     ("brown_mud_leaves_01", 4.0, 0.35, 0.3),  # under the grass
}

UV_KEYS = {"teak", "teak_dark", "teak_turned"}   # textured through UVs along each piece's length (mesh.Parts makes them)
CARED_KEYS = {"teak_turned"}      # handled, oiled household wood: no weathering layers (on a 2 cm turned stand every face read
                                  # as a worn edge, and the dirt from the foot covered all of it)

def ids():
    return sorted({t[0] for t in TEXTURES.values()})

def file(asset, kind):
    """The downloaded file of one map (any extension), or None."""
    d = os.path.join(DIR, asset)
    for ext in ("jpg", "png"):
        p = os.path.join(d, f"{asset}_{kind}_{RES}.{ext}")
        if os.path.exists(p): return p
    return None
