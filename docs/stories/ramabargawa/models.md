# Ramabargawa: Model Breakdown

Every model the Ramabargawa story needs in Blender, listed before anything is made. **Look: cinematic, not origami** (the user, 2026-10-18). Nothing has been generated yet; every status is `todo`.

The story follows the Javanese wayang version (Ramabargawa / Ramaparasu / Rama Bargawa), not the Indian one. Costume and look details below are design proposals unless a source is given.

All production work (code, Blender files and the rest) will live in `project/bargawa/` in this repo (the user, 2026-10-18). The folder doesn't exist yet.

## Story beats (for the "Beats" columns)

| # | Beat | Where |
|---|---|---|
| 1 | The family at Pertapan Jatisrana. Renuka at the lake meets Prabu Citrarata | Hermitage, lake |
| 2 | Jamadagni orders Renuka's punishment. The four elder brothers refuse and are cursed. Ramabargawa obeys with his axe. His boons: long life, no equal, death only by Wisnu's incarnation, family made whole | Hermitage |
| 3 | Jamadagni is killed by Maespati warriors. Ramabargawa's grief and his vow to kill every satria | Hermitage |
| 4 | The purge of the satria | Battlefields |
| 5 | Weary of the world, he seeks death. He kills Prabu Arjunasasrabahu, then learns Wisnu had already left him. He feels deceived. Batara Narada tells him to wait | Maespati, Kahyangan |
| 6 | Years of waiting; he grows old | Wilderness |
| 7 | He meets Ramawijaya and Sinta on the road home from Mantili. The duel with the bow. He dies and becomes Batara Ramaparasu | Forest road, Kahyangan |

## General requirements

- **Cinematic look**: real-world scale, PBR materials (base colour, roughness, metallic, normal), enough detail for close-ups; no low-poly or folded-paper styling.
- **Characters** come in an **A-pose** (arms angled down, legs slightly apart), so they can be rigged, with weapons and hand props made as **separate models**.
- **Heights** (proposal): adult men 1.70–1.85 m, Ramabargawa the tallest of the mortals (about 1.90 m), women 1.55–1.65 m, Narada about 1.40 m.
- **Faces** carry the story (grief, rage, obedience): main characters need a face clean enough for close-ups and, later, expression shapes.
- **Long cloth** (kain, jarik, selendang, robes) will need extra bones or cloth simulation: note it per character.
- **Jewellery and crowns** stay part of the character model unless noted, so they follow the rig.

## Characters

| ID | Character | Variants | Beats | Phase | Status |
|---|---|---|---|---|---|
| C01 | Ramabargawa, young | — | 1–5 | 1 | todo |
| C02 | Ramabargawa, old | — | 6–7 | 2 | todo |
| C03 | Batara Ramaparasu (divine form) | costume variant of C02 | 7 | 3 | todo |
| C04 | Resi Jamadagni | — | 1–3 | 1 | todo |
| C05 | Dewi Renuka | — | 1–2 | 1 | todo |
| C06 | The four brothers | 4: Rumawan, Susena, Wasu, Wiswawasu | 1–2 | 1 | todo |
| C07 | Prabu Citrarata | — | 1 | 2 | todo |
| C08 | Maespati soldier | 3+: spearman, swordsman, captain | 3–4 | 2 | todo |
| C09 | Prabu Arjunasasrabahu | — | (3), 5 | 2 | todo |
| C10 | Batara Narada | — | 5 | 3 | todo |
| C11 | Ramawijaya | — | 7 | 3 | todo |
| C12 | Dewi Sinta | — | 7 | 3 | todo |
| C13 | Laksmana (optional) | — | 7 | 3 | todo |
| C14 | Ayodya escort (optional) | recolour of C08 | 7 | 3 | todo |

### C01 Ramabargawa, young
- **Who**: youngest son of Resi Jamadagni and Dewi Renuka; firm, loving, eager to learn, very obedient to his parents (character as in [Rickyansyah 2019](https://journal.isi.ac.id/index.php/wayang/article/download/3054/1202)).
- **Age and build**: about 20; tall, broad, a fighter's body hardened by tapa (ascetic practice).
- **Face**: strong jaw, intense eyes, light beard.
- **Hair**: long black hair in a high gelung (topknot) tied with a plain band.
- **Costume**: bare torso with an upawita (sacred thread) over the left shoulder; dark kain wrapped to the knee with a cloth sabuk (sash); simple armbands (kelat bahu) and wristbands; barefoot.
- **Carries**: the kapak (P01).
- **Skin**: open decision, natural brown or ash-grey (the wayang puppet is grey, see [MOIFA](https://collection.moifa.org/objects/24775/ramabargawa)).
- **Needs**: close-up face (grief, rage); the kain needs cloth or extra bones.

### C02 Ramabargawa, old
- **Same man** as C01, aged decades: leaner and harder, scars, weathered skin.
- **Hair**: long grey hair and beard, loose or in an untidy gelung.
- **Costume**: C01's costume worn out, plus a rough outer cloth.
- **Feel**: a frightening brahmin ("brahmana yang berparas mengerikan", after Sindhunata's novel, in Rickyansyah 2019).
- **Carries**: the kapak (P01) and a great bow (P08); in the Prambanan relief story he asks Rama to kill him with his own bow ([SEAsite](https://seasite.niu.edu/Indonesian/ramayana/templerelief.htm)).

### C03 Batara Ramaparasu (divine form)
- C02's body after death, raised as a god: calm face, white and gold garments, a crown or a light behind the head.
- Can be a costume and material variant of C02 rather than a new model.

### C04 Resi Jamadagni
- **Who**: a powerful brahmin of Pertapan Jatisrana, much older than his wife, more devoted to heaven than to the world (Rickyansyah 2019). Also called Resi Wisanggeni in Ki Purbo Asmoro's telling.
- **Build**: old, thin, upright; often sits in semedi (meditation).
- **Hair**: long white hair in a gelung, long white beard.
- **Costume**: plain white or ochre robe over one shoulder, bare feet.
- **Carries**: japamala (P03).
- **Needs**: a seated meditation pose that rigs well; a death scene in beat 3.

### C05 Dewi Renuka
- **Who**: daughter of Prabu Prasnajid, wife of Jamadagni after he won the contest for her; a princess living as a hermit's wife.
- **Age**: much younger than Jamadagni; mother of five grown sons, so in her late 30s.
- **Hair**: sanggul (bun) with jasmine.
- **Costume**: kemben (chest cloth), long jarik to the ankles, selendang (scarf), a few pieces of modest gold jewellery left from her royal life.
- **Carries**: kendi (P02).
- **Needs**: close-up face; the jarik and selendang need cloth or extra bones; scenes at the lake.

### C06 The four brothers
- **Who**: Rumawan (eldest, firm, polite, responsible), Susena (ambitious, rebellious, emotional), Wasu (the middle one), Wiswawasu (proud, indifferent); traits from Rickyansyah 2019. The names match the Mahabharata's Rumanvat, Sushena, Vasu and Vishvavasu.
- **Model**: one base hermit's son, plain white or undyed kain and gelung, no jewellery; four variants by face, age, hair and cloth colour.
- **Needs**: their curse in beat 2.

### C07 Prabu Citrarata
- **Who**: young king of Martikawata: rich, handsome, with a beautiful voice; ambitious and cunning (Rickyansyah 2019).
- **Costume**: gold crown (jamang or makutha), sumping (ear ornaments), kalung (necklace), kelat bahu, rich batik kain with gold, keris worn at the back (P04).
- **Needs**: lakeside scene with Renuka.

### C08 Maespati soldier
- **Who**: the satria of Maespati who kill Jamadagni, and the warriors Ramabargawa cuts down.
- **Model**: an old-Javanese warrior with headcloth or helmet, light armour, kain, carrying a spear and shield (P05, P06) or a sword.
- **Variants**: spearman, swordsman, captain; recolours stand in for other kingdoms in the purge (beat 4).
- **Needs**: several instances in one shot.

### C09 Prabu Arjunasasrabahu
- **Who**: king of Maespati, Wisnu's incarnation until Wisnu leaves him; in beat 5 he has lost the will to live after the deaths of Sumantri and Dewi Citrawati.
- **Look**: mature, tall, majestic; high gold makutha, rich jewellery, regal kain; a tired, grieving face.
- **Needs**: a duel with C01/C02; the "Wisnu leaves him" moment is an effect, not a model.

### C10 Batara Narada
- **Who**: messenger of the gods who tells Ramabargawa to wait for Rama.
- **Look**: short, round, old; white hair; crown; white and gold robe; in wayang his face turns upward.
- **Needs**: appears in Kahyangan (E07), maybe on a cloud.

### C11 Ramawijaya
- **Who**: son of Prabu Dasarata, Wisnu's new incarnation, returning to Ayodya after winning Sinta.
- **Look**: young, slim, calm and refined; gold crown; royal kain.
- **Carries**: bow and quiver (P07); in the duel he takes Ramabargawa's bow (P08).

### C12 Dewi Sinta
- **Who**: princess of Mantili, newly won by Rama.
- **Look**: young, elegant, bridal finery: crown or sanggul with flowers, kemben, long jarik, selendang.
- **Needs**: cloth like C05.

### C13 Laksmana (optional)
- Rama's younger brother, travelling with him: like C11 but younger, with his own bow.

### C14 Ayodya escort (optional)
- The procession around Rama and Sinta: recoloured C08 soldiers.

## Animals

| ID | Animal | Count | Beats | Phase | Status |
|---|---|---|---|---|---|
| A01 | Cattle or goats of the hermitage (Jamadagni's livestock, killed by the king's guards in one telling, [caritawayang](https://caritawayang.blogspot.com/2015/06/arjuna-sasrabahu.html)) | 2–3 | 1, 3 | 2 | todo |
| A02 | Horses (for the chariot P10) (optional) | 2 | 7 | 3 | todo |
| A03 | Lake birds: herons, kingfisher (optional, set dressing) | 2–3 | 1 | 3 | todo |

## Props

| ID | Prop | Used by | Beats | Phase | Status |
|---|---|---|---|---|---|
| P01 | **Kapak**: Ramabargawa's great axe; ancient iron blade with engraved motifs on a long wrapped wooden handle. The hero prop | C01, C02 | 2–7 | 1 | todo |
| P02 | Kendi: clay water jar | C05 | 1 | 1 | todo |
| P03 | Japamala: prayer beads | C04 | 1–3 | 1 | todo |
| P04 | Keris, with sheath | C07, C09 | 1, 5 | 2 | todo |
| P05 | Tombak: spear | C08 | 3–4 | 2 | todo |
| P06 | Tameng: shield, and a sword | C08 | 3–4 | 2 | todo |
| P07 | Rama's bow, arrows and quiver (endhong) | C11, C13 | 7 | 3 | todo |
| P08 | Ramabargawa's great bow | C02, C11 | 7 | 2 | todo |
| P09 | Teken: walking staff (optional) | C02 | 6 | 3 | todo |
| P10 | Chariot of Rama and Sinta (optional) | C11, C12 | 7 | 3 | todo |
| P11 | Hermitage set dressing: pandan mat (tikar), oil lamp (damar; built: [objects/damar.md](objects/damar.md)), incense burner (pedupaan), offering trays (sesaji), clay pots, wooden bench (lincak) | E01 | 1–3 | 1 | todo |

## Sets and environments

| ID | Set | Contents | Beats | Phase | Status |
|---|---|---|---|---|---|
| E01 | **Pertapan Jatisrana** (the hermitage) | open pendapa with a thatched (ijuk) roof on wooden pillars, small huts (pondok), a stone sanggar pamujan (shrine for prayer), stone paths, a simple gate | 1–3 | 1 | todo |
| E02 | Lake near Jatisrana | water with lotus and reeds, rocks, a beringin (banyan) on the bank | 1–2 | 1 | todo |
| E03 | Javanese forest | the vegetation kit below, around the hermitage | 1–3 | 1 | todo |
| E04 | Maespati | palace front: candi bentar (split gate), alun-alun (palace square), the duel ground | 5 | 2 | todo |
| E05 | Battlefields | open fields, a burned village, a river crossing; built from the kits | 4 | 2 | todo |
| E06 | Wilderness of the waiting years | mountain, cliffs, a cave or rock seat | 6 | 3 | todo |
| E07 | Kahyangan (heaven) | clouds, a golden gate (Selamatangkep in wayang), a hall of the gods | 5, 7 | 3 | todo |
| E08 | Forest road | a road through the forest where Rama's procession meets Ramabargawa | 7 | 3 | todo |

### Shared kits
- **Vegetation**: beringin (banyan), teak, bamboo clumps, banana, coconut palm, ferns, lotus, reeds, grass, fallen leaves.
- **Rocks**: boulders, river stones, cliff pieces, carved stone blocks for the shrine and gates.
- **Sky and light**: HDRIs for dawn, day, dusk and night.
- **Terrain**: ground materials for soil, mud, moss and dry grass.

## Not models (listed so nothing is forgotten)

These come later as effects or animation, not as models:
- Wisnu's light leaving Arjunasasrabahu
- The curse on the brothers
- Renuka brought back to life
- The apotheosis
- Implied violence (never shown directly)
- Dust and wind
- Facial expressions

## Counts

- **Characters**: 14 entries; about 11 unique models to make (C03 and C14 are variants, C13 is optional), plus 4 brother and 3+ soldier variants.
- **Animals**: 3 entries (2 optional).
- **Props**: 11 entries (2 optional).
- **Sets**: 8, plus the shared kits.
- **Phase 1** (beats 1–3, the core of the grief story): C01, C04, C05, C06, P01, P02, P03, P11, E01, E02, E03.

## Open decisions

1. ~~Where the Blender files live~~: decided, `project/bargawa/` (see the top of this doc).
2. ~~How models are generated~~: decided for the environment, Poly Haven and Blender, no Meshy (the user, 2026-10-18). Still open for the characters.
3. ~~Which cinematic look~~: decided, **stylized**, focused on emotion: grief and sorrow. The environment design is in [environment.md](environment.md).
4. **Ramabargawa's skin**: natural brown or ash-grey.
5. **Which Renuka story**: the classic one (she is guilty and her son kills her on his father's order) or Rickyansyah's (she is innocent and Citrarata kills her). This decides whether C07 needs a keris fight and whether C01 needs the axe in beat 2.
6. **Format**: seven episodes or one film; narration in Indonesian, Javanese, or none.

## Sources

- [Parasurama, Wikipedia bahasa Indonesia](https://id.wikipedia.org/wiki/Parasurama): the wayang section (from *Serat Arjunasasrabahu*).
- [Rickyansyah, "Perancangan Lakon Ramabargawa", Wayang Nusantara, ISI Yogyakarta, 2019](https://journal.isi.ac.id/index.php/wayang/article/download/3054/1202): characters, the brothers' names and traits, Jatisrana, Citrarata, other dalang versions.
- [Kumpulan Cerita Wayang: Arjuna Sasrabahu](https://caritawayang.blogspot.com/2015/06/arjuna-sasrabahu.html): Jamadagni's livestock and his death.
- [MOIFA: Ramabargawa puppet, Ki Pringgo Satata, Boyolali, 1994](https://collection.moifa.org/objects/24775/ramabargawa): the grey-skinned puppet.
- [NIU SEAsite: Ramayana temple relief, "Rama Memanah Ramaparasu"](https://seasite.niu.edu/Indonesian/ramayana/templerelief.htm): Rama kills him with his own bow.
