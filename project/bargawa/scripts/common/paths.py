# Project folders, found from this file's place (scripts/common/), so the project can move.
import os

ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))      # project/bargawa
SCRIPTS = os.path.join(ROOT, "scripts")
PLACES = os.path.join(ROOT, "blender", "places")
BUILDINGS = os.path.join(ROOT, "blender", "buildings")
OBJECTS = os.path.join(ROOT, "blender", "objects")
RENDERS = os.path.join(ROOT, "renders")

def place_file(name):
    """Where scripts/places/<name>.py writes its place: blender/places/<name>.blend, or $PLACE_OUT for a test build."""
    return os.environ.get("PLACE_OUT") or os.path.join(PLACES, name + ".blend")

def building_file(name, built=False):
    """blender/buildings/<name>.blend: where scripts/buildings/<name>.py writes it ($BUILDING_OUT for a test build), or with
    built=True the real file that places link."""
    real = os.path.join(BUILDINGS, name + ".blend")
    return real if built else (os.environ.get("BUILDING_OUT") or real)

def object_file(name, built=False):
    """blender/objects/<name>.blend: where scripts/objects/<name>.py writes it ($OBJECT_OUT for a test build), or with built=True
    the real file that buildings and places link."""
    real = os.path.join(OBJECTS, name + ".blend")
    return real if built else (os.environ.get("OBJECT_OUT") or real)

def checks_dir(name):
    """Where review images of `name` go."""
    return os.path.join(RENDERS, "checks", name)
