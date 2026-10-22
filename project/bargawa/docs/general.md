# Bargawa: General Requirements

The acceptable requirements for the Bargawa game: the first state of the workflow in [bargawa_game.md](../bargawa_game.md). The design (the story and model summaries, then the HTML preview) starts from this file and from the critique in [story.md](story.md).

**Status: draft.** Written by Claude on 2026-10-20 from the user's answers. The user edits it and changes this line to "accepted" when it is right.

## The game

- An Android game that adapts the Ramabargawa story in its Javanese wayang version, as researched in [docs/stories/ramabargawa/](../../../docs/stories/ramabargawa/models.md).
- **A narrative adventure** (the user's choice): the player walks and explores in third person, interacts with places, objects and people, and the story plays out in scenes between. Few or no fights.
- **Its subject is grief and sorrow**, as for the whole production. The place, the light and small acts carry it, after the emotional principles in [environment.md](../../../docs/stories/ramabargawa/environment.md#emotional-intent):
  - one place in changing states
  - each warm light belongs to someone and goes out with their loss
  - the violence happens off-screen and is shown by what it leaves behind

## First release: Chapter 1, Jatisrana

- Beats 1–3 of the story ([models.md](../../../docs/stories/ramabargawa/models.md), "Story beats"):
  1. The family at Pertapan Jatisrana. Renuka at the lake meets Prabu Citrarata.
  2. Jamadagni orders Renuka's punishment. The four elder brothers refuse and are cursed. Ramabargawa obeys, and is given his boons.
  3. Jamadagni is killed by Maespati warriors. Ramabargawa's grief and his vow.
- One place, Pertapan Jatisrana, in its seven light states, from S1 (morning dew) to S7 (red dawn).
- About 20–40 minutes of play.
- Beats 4–7 are later chapters. Nothing in Chapter 1 should block them.

## Look

- 3D, the same stylized cinematic look as the Blender production: no origami, no low-poly styling.
- Built from our Blender files in `project/bargawa/blender/` (the place, the pendopo, the damar, the fire, the stone), exported for the game.
- Phones need lighter versions: fewer and simpler plants, smaller textures, baked light. The Blender scripts stay the one source: the game versions are generated from them, not edited by hand.
- Sources stay Poly Haven (CC0) and Blender, with no Meshy for the environment. Where the characters come from is still open.

## Language

- Dialogue in **Javanese**, the way a dalang tells it, with **Indonesian subtitles** (the user's choice).
- Javanese names and terms stay as they are (pendopo, damar, resi, satria).

## Engine and platform

- **Godot 4**, as for Endless Samurai (`project/game/endless_samurai`: Godot 4.3, Mobile renderer). The exact version is picked in the design.
- **Android**, with preview builds to try on a phone, and a production build at the end of the workflow.
- Touch controls.
- The game project lives in `project/bargawa/` with the rest of the production (the user, 2026-10-18: "all of coding, blender and all, live in ./project/bargawa").

## Workflow files

| Step | Who | File |
|---|---|---|
| Requirements | the user | `docs/general.md` (this file) |
| Story critique | the user | `docs/story.md` |
| Story design | Claude | `docs/summaries/story.md` |
| Model design | Claude | `docs/summaries/models/*.md` |
| Design summary, decisions and state | Claude | `preview/design.html` |

## Open: for the user to decide

The proposal is in brackets. The design can start from it, and it changes if the user says so.

1. **Renuka's version**: the classic one (she is guilty, and her son kills her on his father's order), or Rickyansyah's (she is innocent, and Citrarata kills her). It decides beat 2, the heart of Chapter 1, and what the player does in it. [no proposal: the user's call]
2. **The funeral**: a pyre by the lake (S6), or grief under the banyan. [the pyre]
3. **Voice**: recorded Javanese voices with Indonesian subtitles (who records them?), or Javanese text on screen with the Indonesian under it. [text first, voices later]
4. **Who the player is**: Ramabargawa throughout, or other family members in beat 1 (for example Renuka at the lake). [Ramabargawa throughout]
5. **Characters**: where the models come from (Blender by hand, a generator, a library), and Ramabargawa's skin (natural brown or ash-grey).
6. **Test phone**: which Android phone the preview builds run on, and the lowest phone the game must run on.
7. **Screen**: landscape or portrait. [landscape, for the cinematic framing]
8. **Price**: free, paid, or free with ads.
