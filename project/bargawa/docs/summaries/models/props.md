# Props for Chapter 1

Part of the [model design](index.md). Film-level list: [models.md, Props](../../../../../docs/stories/ramabargawa/models.md) (P01–P11).

**Sources**: **BL** = built by script in Blender (`project/bargawa/scripts/`), **PH** = Poly Haven (CC0), and **built** = already exists in `blender/objects/`.

| Prop | Scenes and states | Source | Budget |
|---|---|---|---|
| **P01 Kapak**, the hero prop: an iron blade with engraved motifs, a long wrapped wooden handle | S3 leaning on a soko, then taken up; S6 set down on the woodcutter's stump; S7 taken up again | BL | 3k tris, 1K; seen close |
| **P02 Kendi** | S2 full, carried; S3 beside Renuka; at the end of S3 broken, its water soaking into the ground (a darkening decal and a few drops) | BL (turned profile); a whole and a broken version | 1.5k, 512 |
| **P03 Japamala** | S1–S4 in Jamadagni's hand; S5 about 20 loose beads on the way to the shrine, and the broken string; a bead disappears when it is picked up | BL; the string, and one bead copied many times | 1k, 256 |
| **Woodcutter's axe and stump** | S1 the player splits wood; the stump stays in every scene, with the kapak on it in S6–S7 | PH `wooden_axe`, `tree_stump_01` | from PH, textures cut to 1K |
| **Firewood** | S1 split pieces and a small pile; S5 the bundle on his back, dropped at the gate; S6 the woodpile and the four logs he carries | BL (logs from `common/fuel.py`) | 2k in all |
| **Five mats** (tikar pandan) | S1 side by side; S4 his mat apart from the others; S5 burned | BL; texture candidate PH `tatami_mat`; a whole and a burned version | 200 each, one 1K set |
| **Damar**, Jamadagni's lamp | S1 the player lights its four flames; S4 one left; S5 knocked over and out | built: `blender/objects/damar.blend`; game version with flame cards and one light | measure at export; cut down if above 4k |
| **Incense burner** (pedupaan) and sticks | S1 lit by the player, smoke rising straight; S5 cold | BL | 1k |
| **Offering tray** (sesaji) | S1 at the sanggar pamujan | BL | 1k |
| **Bowl of food** | S4 Renuka offers it, he puts it down | PH `wooden_bowl_01` | from PH |
| **Dakon board** | S1 on the sons' porch | PH `sungka_board` | from PH |
| **Pots and baskets** | dressing, S1–S4 | PH `ceramic_pot`, `brass_pot_01`, `wicker_basket_01` | from PH |
| **Broken tombak** | S5 in the gate: the only sign of the Maespati warriors | BL; a spear broken in two | 1k |
| **Burning brand** | S6 carried from the luweng to the pyre | BL stick + game flame | 300 |
| **Pyre** | S6 grows in four stages as the logs are brought, then burns (big fire); S7 smoulders | BL (logs from `common/fuel.py`); fire as in `fire.blend` | 4k |
| **Shroud** | S6 the body wrapped in white cloth on the pyre | BL; a simple wrapped shape | 2k |

## Fire, smoke and water in the game

- **Fire** follows the method already used for `blender/objects/fire.blend` ([objects/fire.md](../../../../../docs/stories/ramabargawa/objects/fire.md)), rebuilt as Godot shaders:
  - flames drawn on cards, with rising noise, fading with height
  - a flickering point light the size of the fire
  - sparks as GPU particles
- **Fires in Chapter 1**: the damar's flames, Renuka's hearth (S1), the brand (S6), the pyre (S6), and the embers in the luweng (S5–S6) and in the pyre (S7).
- **Smoke, ash and rain** are GPU particles with soft textures.
- **The spilled and soaking water** is a decal that darkens the ground, plus a few drops.
