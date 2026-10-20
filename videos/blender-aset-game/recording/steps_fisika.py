"""Bagian 1, "Kubus jatuh di Blender": Rigid Body Active vs Passive, as driver.py runs it.

Screen positions were calibrated on the tutorial Blender (1920x1080 fullscreen, factory
settings, UI scale 1.25; screenshots in .state/f*.png). Points in the scene are projected
at run time (`pt`), so they follow the start view in blender_session.py.
"""

PARK = (1180, 780)

P = {
    "tab_physics": (1598, 659),       # Properties tabs, once an object is active
    "rigid_body": (1776, 483),        # Physics tab: the Rigid Body button
    "type": (1803, 589),              # Rigid Body panel: Type
    "type_passive": (1775, 645),      # the Type menu's second item
    "viewport": (1100, 760),          # empty ground, for keys aimed at the 3D view
}

blender_exec = None  # set by driver.take


def pt(co):
    return tuple(blender_exec("result = project(%r)" % (tuple(co),))["result"])


def run(d):
    cube = pt((0.0, 0.0, 4.5))
    ground = pt((3.0, -3.0, 0.0))
    d.hold(1.0)

    d.step(1, "Di Blender tidak ada gravitasi otomatis. Kubus ini melayang.")
    d.move(cube[0] + 170, cube[1] + 70, dur=1.2)
    d.hold(3.2)

    d.step(2, "Klik kubus, lalu buka tab Physics.")
    d.click(*cube, label="Kubus")
    d.hold(0.8)
    d.click(*P["tab_physics"], label="Physics")
    d.hold(1.6)

    d.step(3, "Klik Rigid Body. Type Active: kubus jatuh karena gravitasi.")
    d.click(*P["rigid_body"], label="Rigid Body")
    d.hold(0.6)
    d.move(*P["type"])
    d.hold(2.8)

    d.step(4, "Klik tanah, beri Rigid Body juga.")
    d.click(*ground, label="Tanah")
    d.hold(0.8)
    d.click(*P["rigid_body"], label="Rigid Body")
    d.hold(1.4)

    d.step(5, "Ubah Type ke Passive: padat, tapi tidak bergerak.")
    d.click(*P["type"], label="Type")
    d.hold(0.7)
    d.click(*P["type_passive"], label="Passive")
    d.hold(2.0)

    d.step(6, "Tekan Space: kubus jatuh dan berhenti di tanah.")
    d.move(*P["viewport"])
    d.hold(0.5)
    d.key("Space")
    d.hold(3.2)
    d.key("Space")
    d.hold(0.6)

    d.step(7, "Active = bergerak, Passive = diam tapi padat.")
    d.key("Shift+Left", label="Shift + ←")
    d.hold(0.6)
    d.key("Space")
    d.hold(3.4)
