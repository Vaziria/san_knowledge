"""Bagian 3, "Mirror modifier": one pillar becomes two, as driver.py runs it.

Screen positions were calibrated on the tutorial Blender (1920x1080 fullscreen, factory
settings, UI scale 1.25; screenshots in .state/m*.png). Points in the scene are projected at
run time (`pt`), so they follow the start view in blender_session.py.
"""

PARK = (1180, 780)

P = {
    "tab_modifier": (1598, 590),          # Properties tabs, once an object is active
    "add_modifier": (1777, 321),
    "generate": (1690, 377),
    "generate_left": (1560, 377),         # the Generate submenu opens to the left
    "mirror": (1433, 601),
    "viewport": (1100, 760),
}

blender_exec = None  # set by driver.take


def pt(co):
    return tuple(blender_exec("result = project(%r)" % (tuple(co),))["result"])


def run(d):
    pillar = pt((0.0, 0.0, 1.8))
    origin = pt((0.0, 0.0, 0.0))
    d.hold(1.0)

    d.step(1, "Satu tiang. Gerbang butuh dua tiang yang sama persis.")
    d.move(pillar[0] + 160, pillar[1] + 40, dur=1.2)
    d.hold(3.0)

    d.step(2, "Klik tiang, lalu buka tab Modifier.")
    d.click(*pillar, label="Tiang")
    d.hold(0.8)
    d.click(*P["tab_modifier"], label="Modifier")
    d.hold(1.6)

    d.step(3, "Add Modifier, Generate, Mirror.")
    d.click(*P["add_modifier"], label="Add Modifier")
    d.hold(0.8)
    d.move(*P["generate"])
    d.hold(0.6)
    d.move(*P["generate_left"], dur=0.35)
    d.click(*P["mirror"], label="Mirror")
    d.hold(1.5)

    d.step(4, "Belum terlihat: cerminnya ada di origin, tepat di tengah tiang.")
    d.move(origin[0] + 14, origin[1] + 12, dur=0.9)
    d.hold(3.4)

    d.step(5, "Tab: Edit Mode. Tekan A untuk memilih semua titik.")
    d.move(*P["viewport"])
    d.hold(0.3)
    d.key("Tab")
    d.hold(1.0)
    d.key("A")
    d.hold(1.6)

    d.step(6, "G, X, 1.5: geser bentuknya, origin tetap di tengah.")
    d.key("G")
    d.hold(0.3)
    d.key("X")
    d.hold(0.3)
    d.type_text("1.5")
    d.key("Enter")
    d.hold(2.4)

    d.step(7, "Tab kembali. Jarak antar tiang = 2 × jarak ke origin.")
    d.key("Tab")
    d.hold(3.8)
