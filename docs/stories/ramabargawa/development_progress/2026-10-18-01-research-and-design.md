# 2026-10-18: Research, model breakdown, environment design

The first session on the Ramabargawa story: from "do you know Bargawa?" to a full model list and the design of the first environment. Nothing has been built in Blender yet.

## Done

1. **Researched the story** (web search), with a focus on the Javanese version, which the user grew up hearing as folklore:
   - The Javanese wayang Ramabargawa differs from the Indian Parashurama: he is not Wisnu's incarnation, he descends from Batara Surya, he kills Prabu Arjunasasrabahu after Wisnu has left him, and he dies by Ramawijaya's hand and becomes Batara Ramaparasu.
   - Dalang tell it differently: Ki Purbo Asmoro (*Banjaran Ramabargawa*, 2015: Bargawa as Maespati's executioner, his father called Resi Wisanggeni), Bayu Ismayanto (*Bargawa*, 2013), Fani Rickyansyah (Renuka innocent, killed by Citrarata), and the UGM wayang orang and wayang kulit staging (2017).
   - In parts of Bantul there is a belief against performing plays from the Maespati era.
   - His grief: he lost his brothers (cursed), his mother (by his own hand) and his father (killed by Maespati warriors); later he could not die until Rama.
   - Main source: Rickyansyah, "Perancangan Lakon Ramabargawa", *Wayang Nusantara*, ISI Yogyakarta, 2019.
2. **Model breakdown**: [models.md](../models.md), with 7 story beats, 14 characters (C01–C14) with design sheets, 3 animals, 11 props, 8 sets and the shared kits. Synced to the knowledge graph with AI summaries (26 nodes; domain Game Development).
3. **Environment design** of Pertapan Jatisrana (sets E01–E03): [environment.md](../environment.md), with:
   - emotional principles
   - seven light states (S1 morning mist → S7 red dawn)
   - symbols tied to each person and loss
   - a layout map and seven key cameras
   - every element with its source (Poly Haven IDs checked against its public catalogue)
   - the folder plan for `project/bargawa/`
   - six milestones
4. **Checked Blender**: the Blender MCP's 3D generators (Hunyuan3D, Hyper3D, Tripo) and asset libraries (Poly Haven, Sketchfab, Poly Pizza) were all off.

## Decisions (the user's words)

| Decision | The user said |
|---|---|
| Story docs live in `docs/stories/<story-name>/` | "later we summaries in docs/stories/stories-name/*.md" |
| Cinematic, not origami | "we dont use origami, we use cinematic" |
| Break down and document before making anything | "dont create the model, we breakdown the model first and list all and save to the doc stories" |
| All production in `project/bargawa/` | "later, all of coding, blender and all, live in ./project/bargawa" |
| The environment comes first | "now we create environtment first" |
| No Meshy; Poly Haven and Blender | "forget meshy, polyhaven and blender" (read as: forget Meshy, use Poly Haven and Blender) |
| Stylized, focused on grief and sorrow | "stylized, we bring and focus on emotional, grief, and sorrow. make sure its developed well" |
| Progress notes in this folder | "every development progress note on docs/stories/ramabargawa/development_progress/*.md so knowledge recorded and you easyly remember" |

## Open questions

- Render engine: EEVEE or Cycles (the office render farm only runs Cycles).
- Renuka's version: classic (guilty, killed by her son on his father's order) or Rickyansyah's (innocent, killed by Citrarata).
- Funeral: a pyre by the lake, or grief under the banyan.
- Which of the proposed symbols to keep.
- Where the characters come from (Meshy is out for the environment; open for characters).
- Ramabargawa's skin: natural brown or ash-grey.
- Format: seven episodes or one film; narration language.

## Next

- The user reviews [environment.md](../environment.md) and answers the open questions.
- Then milestone 1, the **blockout**: set up `project/bargawa/`, build the terrain, the lake, grey boxes for buildings and trees, and the seven key cameras; review with small check renders.
- `environment.md`, `models.md` and these notes were synced into the knowledge graph with AI summaries (29 nodes in 4 docs, 8 references), with the user's OK.
