# C01 Ramabargawa, young (the player)

Part of the [model design](index.md). Look and costume: [models.md, C01](../../../../../docs/stories/ramabargawa/models.md).

## In Chapter 1

- He is in every scene, and he is the one the player controls.
- Most of the time the camera sees him from behind (third person). His face is seen close when:
  - he calls his mother (S2)
  - he answers his father (S3)
  - he kneels by his father (S5)
  - he kneels among the banyan's roots (S6)
  - he makes the vow (S7)

## Build

- **MPFB body**:
  - male, about 20, 1.80 m: the tallest of the mortals, 14 cm over the Indonesian average. He passes the huts' doors (1.89 m) upright, and bows under their porch beams (1.70 m).
  - high muscle, lean: a fighter hardened by tapa
  - Asian mix
- **Face**: strong jaw, intense eyes, a light beard (texture plus short hair cards).
- **Pieces**, each its own mesh weighted to the body:
  - hair in a high gelung tied with a plain band (hair cards)
  - upawita: a thin cord over the left shoulder to the right hip
  - a dark kain to the knee with a cloth sabuk, whose hanging ends get two chains of four bones
  - kelat bahu on both arms, and wristbands
  - barefoot
- **Not part of him**: the kapak ([props.md](props.md)). It attaches to a right-hand socket, or rests on the ground.
- **Skin**: natural brown or ash-grey (open). One material with both tones, so the preview can show both.

## States, as material settings, not new models

| Scenes | Look |
|---|---|
| S1–S4 | clean |
| S5 | ash on the skin and in the hair |
| S6 | wet: darker and glossier, wet hair; the ash runs |
| S7 | wet and ash, drying |

## Face shape keys

Blink, speech (open, round), calm, grief, rage, pain, and effort (for the hold actions).

## Animations

| Kind | Moves |
|---|---|
| Moving | idle, walk, slow grieving walk, walk under a load (wood bundle, logs), climb steps |
| Hands | split wood (lift, swing), carry the kendi, carry the burning brand (one hand shielding it), take up the kapak (slow), hold the kapak, light (reach forward), put down, pick up a bead, drop the bundle |
| Javanese | sit sila on the mat, stand up from sila, sembah, the "sendika dhawuh" bow |
| Grief | kneel, kneel weeping, look up |

## Budget

- About 20k triangles: the body about 12k after the hidden parts are cut, the hair about 4k, the kain about 2.5k, the ornaments about 1.5k.
- Textures: a 2K set for the body and face, and a 1K set for the costume.
- Bones: the game rig plus two cloth chains.
