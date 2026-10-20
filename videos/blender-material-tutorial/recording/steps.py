"""The tutorial "Apa itu Material di Blender?", step by step, as driver.py runs it.

Screen positions were calibrated on the tutorial Blender (1920x1080 fullscreen, factory
settings, UI scale 1.25; screenshots in .state/). They hold as long as those stay the same.
"""

PARK = (1180, 780)          # start position: empty viewport, below and right of the sphere

P = {
    "near_sphere": (1060, 560),
    "pie_at": (1180, 560),               # Z opens the shading pie around the cursor
    "pie_material_preview": (1180, 703),  # 143 px below the pie centre
    "sphere": (860, 430),
    "tab_material": (1597, 765),
    "new": (1796, 427),
    "after_new": (1700, 470),
    "base_color": (1816, 581),
    "wheel_gold": (1680, 311),            # hue ~0.11, saturation ~0.86 on the picker that opens
    "close_picker": (1640, 597),          # empty panel space: a click outside closes the picker
    "metallic": (1816, 612),
    "roughness": (1816, 643),
    "roughness_slider": (1762, 109),     # left edge and width of the Roughness slider
    "tab_shading": (801, 16),
    "after_shading": (1300, 900),
    "node_principled": (720, 621),
    "node_base_color": (690, 674),
    "node_metallic": (700, 700),
    "node_roughness": (700, 726),
    "node_output": (1160, 621),
    "shading_sphere": (930, 340),
}


ROUGHNESS = ("next(n for n in bpy.data.materials['Material'].node_tree.nodes"
             " if n.type == 'BSDF_PRINCIPLED').inputs['Roughness'].default_value")


def run(d):
    d.hold(1.2)

    d.step(1, "Apa itu material? Material = tampilan permukaan objek.")
    d.move(*P["near_sphere"], dur=1.2)
    d.hold(3.2)

    d.step(2, "Tekan Z, pilih Material Preview agar material terlihat.")
    d.move(*P["pie_at"])
    d.hold(0.5)
    d.key("Z")
    d.hold(1.1)
    d.click(*P["pie_material_preview"], label="Material Preview")
    d.hold(2.6)

    d.step(3, "Klik objek untuk memilihnya.")
    d.click(*P["sphere"], label="Bola")
    d.hold(2.6)

    d.step(4, "Buka tab Material. Bola ini belum punya material.")
    d.click(*P["tab_material"], label="Material Properties")
    d.hold(3.4)

    d.step(5, "Klik New. Material baru dibuat (Principled BSDF).")
    d.click(*P["new"], label="New")
    d.hold(0.6)
    d.move(*P["after_new"])
    d.hold(2.8)

    d.step(6, "Base Color: warna dasar permukaan.")
    d.click(*P["base_color"], label="Base Color")
    d.hold(1.0)
    d.click(*P["wheel_gold"], label="Roda warna")
    d.hold(1.6)
    d.click(*P["close_picker"])
    d.hold(1.8)

    d.step(7, "Metallic 1: permukaan jadi seperti logam.")
    d.click(*P["metallic"], label="Metallic")
    d.hold(0.5)
    d.type_text("1")
    d.hold(0.4)
    d.key("Enter")
    d.hold(3.0)

    d.step(8, "Roughness kecil = mengkilap, besar = doff.")
    d.scrub(*P["roughness"], ROUGHNESS, [(0.05, 1.6, 1.6), (0.9, 2.2, 1.8), (0.2, 1.6, 0.4)],
            P["roughness_slider"], label="Geser Roughness")
    d.hold(2.0)

    d.step(9, "Di balik layar, material adalah susunan node.")
    d.click(*P["tab_shading"], label="Shading")
    d.hold(0.5)
    d.move(*P["after_shading"])
    d.hold(2.8)

    d.step(10, "Principled BSDF mengatur warna, logam, kekasaran.")
    d.move(*P["node_principled"])
    d.hold(1.0)
    for name in ("node_base_color", "node_metallic", "node_roughness"):
        d.move(*P[name], dur=0.45)
        d.hold(0.7)
    d.hold(0.6)

    d.step(11, "Hasilnya dikirim ke Material Output.")
    d.move(*P["node_output"], dur=1.0)
    d.hold(2.6)

    d.step(12, "Material = resep tampilan permukaan objek.")
    d.move(*P["shading_sphere"], dur=1.0)
    d.hold(3.6)
