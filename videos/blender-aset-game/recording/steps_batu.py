"""Bagian 2, "Batu low poly": Ico Sphere, Randomize, made flatter, as driver.py runs it.

Screen positions were calibrated on the tutorial Blender (1920x1080 fullscreen, factory
settings, UI scale 1.25; screenshots in .state/b*.png). The Add menu opens around the cursor,
so its items hold only while Shift+A is pressed at "add_at".
"""

PARK = (1180, 780)

P = {
    "add_at": (1080, 520),            # Shift+A here: "Search..." lands under the cursor
    "add_mesh": (980, 551),
    "add_mesh_right": (1160, 551),    # into the submenu along the Mesh row
    "add_ico_sphere": (1200, 650),
    "viewport": (1100, 700),
    "menu_mesh": (446, 48),           # Edit Mode header
    "menu_transform": (500, 76),
    "menu_transform_right": (700, 76),
    "menu_randomize": (723, 416),
    "redo_panel": (150, 954),         # the collapsed "Randomize" panel, bottom left
    "redo_amount": (321, 865),        # its Amount field once expanded
    "viewport_low": (1100, 760),
}

blender_exec = None  # set by driver.take


def run(d):
    d.hold(1.0)

    d.step(1, "Batu low poly untuk game, mulai dari Ico Sphere.")
    d.move(*P["add_at"], dur=1.2)
    d.hold(2.6)

    d.step(2, "Shift+A, Mesh, Ico Sphere.")
    d.key("Shift+A", label="Shift + A")
    d.hold(0.9)
    d.move(*P["add_mesh"])
    d.hold(0.7)
    d.move(*P["add_mesh_right"], dur=0.35)
    d.click(*P["add_ico_sphere"], label="Ico Sphere")
    d.hold(2.2)

    d.step(3, "Tab: Edit Mode. Semua titik ikut terpilih.")
    d.move(*P["viewport"])
    d.hold(0.3)
    d.key("Tab")
    d.hold(2.4)

    d.step(4, "Mesh, Transform, Randomize: setiap titik digeser acak.")
    d.click(*P["menu_mesh"], label="Mesh")
    d.hold(0.6)
    d.move(*P["menu_transform"])
    d.hold(0.6)
    d.move(*P["menu_transform_right"], dur=0.35)
    d.click(*P["menu_randomize"], label="Randomize")
    d.hold(1.8)

    d.step(5, "Amount 0.25: bentuknya makin tidak beraturan.")
    d.click(*P["redo_panel"])
    d.hold(0.8)
    d.click(*P["redo_amount"], label="Amount")
    d.hold(0.4)
    d.type_text("0.25")
    d.key("Enter")
    d.hold(2.2)

    d.step(6, "S, Z, 0.6: batu jadi lebih pipih.")
    d.move(*P["viewport_low"])
    d.hold(0.4)
    d.key("S")
    d.hold(0.3)
    d.key("Z")
    d.hold(0.3)
    d.type_text("0.6")
    d.key("Enter")
    d.hold(2.2)

    d.step(7, "Tab kembali. Hanya 80 segitiga: ringan untuk game.")
    d.key("Tab")
    d.hold(3.6)
