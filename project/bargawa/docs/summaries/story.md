# Bargawa, Chapter 1: Story Design

Claude's story design for Chapter 1, Jatisrana: the "Story Design" state of the workflow in [bargawa_game.md](../../bargawa_game.md). Written 2026-10-20 from:
- the accepted requirements in [general.md](../general.md)
- the user's answers: the classic Renuka story, and no critique in this first round
- the story research in [docs/stories/ramabargawa/](../../../../docs/stories/ramabargawa/models.md): the beats, the characters, and the place with its light states, symbols and key cameras ([environment.md](../../../../docs/stories/ramabargawa/environment.md))

The proposals in general.md hold unless the user changes them:
- the player is Ramabargawa throughout
- the funeral is a pyre by the lake
- on-screen text comes first, recorded voices later
- the screen is landscape

The models are in [models/](models/index.md); the overview, the decisions and the state are in [preview/design.html](../../preview/design.html).

## The chapter in one sentence

A son who obeys his father at any cost loses everything he obeyed for, and his grief turns into the vow that drives the rest of his life.

## How the game tells it

1. **One place, seven states.** Each scene is Jatisrana in one light state, S1 to S7. The same spots and the same paths come back, and only the light, the weather and the objects change. Between scenes the screen fades on a short narration line, in the manner of a dalang.
2. **Small acts, no fights.** The player:
   - walks
   - looks around
   - taps to do something (take, give, light, put down)
   - holds for heavy acts (take up the kapak, carry logs, gather the beads)
   - sits or kneels
3. **Each warm light belongs to someone, and the player tends it in S1:**
   - Renuka's hearth fire: he brings the wood.
   - Jamadagni's incense and lamp: he lights them.
   - The five mats: he sits among his brothers.

   Each loss takes one of them away. The last warm light is the pyre.
4. **Violence happens off-screen.** At the moment of violence the game cuts to the object that remembers it: the kendi breaking, the beads scattered, smoke.
5. **He cannot refuse.** In S3, after the order, the hermitage stops answering. The wind drops, the people look down, and nothing can be used except the kapak. The player can walk anywhere, but the only way on is the one he takes. The player should feel the obedience as the world closing in.
6. **Grief changes how it plays.** He walks slower in S5 and S6. In grief the camera pulls wider, not closer: a small man in a big empty place. The wind and the birds stop at the worst moments.
7. **No score, no failing, no counters.** A soft glow shows what can be used, and it fades in only when the player stands still for a while. The game is offline, and it saves at the start of each scene.

## Language

- Dialogue is in Javanese, with Indonesian subtitles under it. The few lines are short: in grief, people say little.
- **Registers**:
  - The parents speak ngoko to their sons.
  - The sons speak krama to their parents.
  - The brothers speak ngoko to each other, with krama inggil words for their father ("Rama wis lenggah").
- **Names**: in Javanese "Rama" is the krama word for father. So the family never calls the hero "Rama": his parents say "Bargawa" or "Le" (son), and he calls his father "Rama" and his mother "Ibu".
- **Narration**: one short line before some scenes, in Javanese, with the Indonesian under it.
- **Every Javanese line below is Claude's draft.** The user, who grew up with this story in Javanese, checks them before they go into the game.

## The scenes

| # | Scene | State | Beat | Where (cameras) | What the player does | Minutes |
|---|---|---|---|---|---|---|
| 1 | **Esuk** (morning) | S1 morning dew | 1 | the whole hermitage (K1, K2, K6) | morning tasks: wood, the hearth, the incense, the lamp, his mat | 8 |
| 2 | **Kendi** (the water jar) | S2 golden afternoon | 1 | the banyan, the lake steps, the bathing stone (K3, K4) | goes to fetch his mother from the lake and carries the kendi back | 5 |
| 3 | **Dhawuh** (the order) | S3 overcast | 2 | the pendopo, the five mats (K2) | sits through the order and the curses, then takes up the kapak | 5 |
| 4 | **Anugrah** (the boons) | S4 moonlit night | 2 | the pendopo at night (K2) | asks for his family back; sits on his mat, apart | 4 |
| 5 | **Awu** (ash) | S5 ash | 3 | the gate, the burned pendopo, the shrine (K1, K6) | comes home to smoke; finds his father; gathers the beads | 5 |
| 6 | **Udan** (rain) | S6 night rain | 3 | the woodpile, the pawon, the lakeside pyre, the banyan (K5) | builds the pyre, carries the last flame through the rain, kneels under the banyan | 6 |
| 7 | **Sumpah** (the vow) | S7 red dawn | 3 | the stump, the lake's edge, the gate (K7) | takes up the kapak, makes the vow, walks out | 2 |

About 35 minutes in all, within the 20–40 of general.md.

### 1. Esuk (S1, morning dew)

- **Purpose**: the family is whole, and the player learns to love the place and the controls. Everything lost later is shown here first.
- **Opens** on K1, the hermitage in the morning mist, with the narration.
- **He wakes** in the sons' hut. The tasks can be done in any order:
  1. **Split wood** at the woodcutter's stump with the ordinary axe: hold to lift, release to swing, three times. This is the same stump and the same lift as the kapak later.
  2. **Bring the wood** to the pawon. Renuka, with jasmine in her hair, feeds it into the luweng, and her cooking fire takes.
  3. **Light the incense** at the sanggar pamujan. Jamadagni sits there in semedi and does not open his eyes.
  4. **Light the damar** in the pendopo: its four flames, one tap each.
- **Ends** when he sits on his mat with his four brothers. The five mats are full. Jamadagni's lesson is one line, and then the scene fades.
- **To find** (optional, never pointed at): the kamboja's few white flowers by the shrine; the goats bleating in the kandhang; the dakon board on the hut's porch.

| Who | Javanese | Indonesian |
|---|---|---|
| Narration | Ing pertapan Jatisrana, ing lereng gunung, Resi Jamadagni urip tentrem karo garwa lan putra lima. | Di pertapaan Jatisrana, di lereng gunung, Resi Jamadagni hidup tenteram bersama istri dan kelima putranya. |
| Renuka | Le, wis tangi? Gawakna kayu menyang pawon, ya. | Nak, sudah bangun? Bawakan kayu ke dapur, ya. |
| Bargawa | Inggih, Ibu. | Ya, Ibu. |
| Rumawan | Bargawa, Rama wis lenggah ing pendapa. | Bargawa, Ayah sudah duduk di pendapa. |

### 2. Kendi (S2, golden afternoon)

- **Purpose**: the moment everything turns on, seen through a son's eyes. It is beautiful, and it is uneasy.
- Jamadagni, in the pendopo, says Renuka has not come back from the lake. The player walks past the banyan and down the stone steps.
- **Halfway down, a man is singing** across the water. Citrarata's voice is beautiful, and it is heard before he is seen.
- **Renuka** stands still on the bathing stone with the full kendi, looking south-west. On the far bank a figure stands against the low sun: a dark shape with a bright edge, unreadable. For a moment the camera takes the far bank's view (K4): Renuka small on her stone, the pendopo above her.
- **When he calls her**, she starts. The kendi tips, and a little water spills on the stone. He carries the kendi up for her. She says nothing on the way, and the singing stops.
- **Ends** at the top of the steps: Jamadagni is watching from the pendopo (K3, looking up).

| Who | Javanese | Indonesian |
|---|---|---|
| Jamadagni | Bargawa, ibumu durung bali saka telaga. | Bargawa, ibumu belum kembali dari telaga. |
| Bargawa | Ibu… | Ibu… |
| Renuka | Ayo mulih, Le. | Ayo pulang, Nak. |

### 3. Dhawuh (S3, overcast)

- **Purpose**: the order and the obedience. This is the heart of the chapter.
- **The setting**: a grey sky and the wind rising in the bamboo. The five sons sit on their mats in the pendopo (K2). Renuka kneels on the ground below the pendopo's steps, head bowed, the kendi beside her. Jamadagni knows what happened at the lake through the power of his tapa.
- **He orders his sons one by one**, eldest first. Each refuses in his own way (traits from models.md) and is cursed:
  - Rumawan answers politely.
  - Susena stands up in anger.
  - Wasu weeps.
  - Wiswawasu turns his face away.
- **The curse**: the cursed son goes still, as if made of stone. The colour drains out of him, and he stays like that until S4.
- **The player cannot leave his mat until his turn.** Then the order comes. Now he can stand, and the scene changes:
  - the wind stops
  - the brothers do not move
  - every other thing that could be used goes dark
- **The kapak** leans against a soko of the pendopo. The player holds to take it up and answers. The walk to his mother is long and silent: the player must walk it.
- **When he reaches her, the screen cuts to black.** Then a still shot: the kendi on its side, broken, its water soaking into the dry ground. The scene fades.

| Who | Javanese | Indonesian |
|---|---|---|
| Jamadagni | Rumawan, pateni ibumu. | Rumawan, bunuh ibumu. |
| Rumawan | Nyuwun pangapunten, Rama. Kula mboten saged. | Mohon ampun, Ayah. Saya tidak sanggup. |
| Jamadagni | Yen mangkono, ilanga budimu. | Kalau begitu, hilanglah akalmu. |
| Jamadagni | Bargawa, pateni ibumu. | Bargawa, bunuh ibumu. |
| Bargawa | Sendika dhawuh, Rama. | Saya laksanakan, Ayah. |

### 4. Anugrah (S4, moonlit night)

- **Purpose**: getting everything back does not make it whole. The family is together, but he is apart.
- **The boons**: Jamadagni is pleased with his son and tells him to ask for anything. Bargawa asks for his mother's life and his brothers' minds back. Jamadagni grants them, and adds his own boons: a long life, no one his equal, and death only by Wisnu's incarnation.
- **Then the night.** Renuka and the brothers are back, but they do not remember. The only light is one flame of the damar; the other three are out.
  - Renuka offers him a bowl of food. He takes it, and he can only put it down.
  - The jasmine's flowers have fallen.
- **Ends** when he sits on his mat, which stands apart from the other four.

| Who | Javanese | Indonesian |
|---|---|---|
| Jamadagni | Bargawa, njaluka apa wae. | Bargawa, mintalah apa saja. |
| Bargawa | Rama, kula nyuwun Ibu gesang malih, lan kangmas sedaya wangsul kados wingi. | Ayah, saya mohon Ibu hidup kembali, dan semua kakak saya kembali seperti kemarin. |
| Jamadagni | Kalakona. Umurmu bakal dawa, ora ana sing bisa madhani kowe, lan mung titisane Wisnu sing bisa mungkasi uripmu. | Terjadilah. Umurmu akan panjang, tak ada yang dapat menandingimu, dan hanya titisan Wisnu yang dapat mengakhiri hidupmu. |
| Renuka | Le, mangan dhisik. | Nak, makanlah dulu. |

### 5. Awu (S5, ash)

- **Purpose**: shock and silence. The violence is shown only by what it left behind.
- **He comes home from the forest** with firewood on his back. At the gate he sees smoke and drops the bundle. A broken tombak lies in the gate: the only sign of the Maespati warriors.
- **Inside**:
  - a white sun through the smoke, ash falling
  - the pendopo burned, the five mats black
  - the damar knocked over and out
  - the kandhang's fence broken, and no goats
  - no wind, no birds
- **The beads**: japamala beads lie scattered across the ground towards the sanggar pamujan. On its steps lies Jamadagni. The incense is burned out and cold, and kamboja flowers fall around him.
- **Ends** when the player kneels by his father. Before that he may gather the beads, one hold for each, as many as he wants.

| Who | Javanese | Indonesian |
|---|---|---|
| Narration | Nalika Bargawa lunga menyang alas, prajurit Maespati teka ing Jatisrana. | Ketika Bargawa pergi ke hutan, prajurit Maespati datang ke Jatisrana. |
| Bargawa | Rama… | Ayah… |

### 6. Udan (S6, night rain)

- **Purpose**: grief at its peak, carried out as the work of a funeral.
- **The pyre**: rain on the dark lake. He sets the kapak down on the woodcutter's stump. Then he carries logs from the woodpile to the shore below the steps, one hold each, slowly, four times. After a cut, the body lies on the pyre, wrapped in white cloth.
- **The last flame**: the only warm light left is the embers in the pawon's luweng.
  - He lights a brand there and carries it through the rain to the pyre.
  - Walking fast makes it gutter. If it goes out, he walks back to the embers: the game's only way to fail, and it costs only time.
  - The pyre takes. The fire rises, with sparks.
- **Ends** under the banyan (K5). He kneels among the hanging roots, which close around him like bars. The wind stops, and only the rain is left.

| Who | Javanese | Indonesian |
|---|---|---|
| Narration | Udan tumurun, kaya langit melu nangis. | Hujan turun, seakan langit ikut menangis. |

### 7. Sumpah (S7, red dawn)

- **Purpose**: grief turns to rage. The red is not warmth coming back.
- The rain has stopped and the pyre smoulders. Clouds burn red over the mountain, and the hermitage stands in silhouette.
- He takes up the kapak from the stump, the same spot as the ordinary axe in S1. He walks to the lake's edge (K7: very wide, a tiny figure at the edge of the dark water) and makes the vow.
- **Ends** when he walks out through the gate onto the road (towards beat 4), on the chapter's title.

| Who | Javanese | Indonesian |
|---|---|---|
| Bargawa | Kabeh satria ing jagad iki bakal tak tumpes. | Semua satria di dunia ini akan kutumpas. |
| Title | Ramabargawa. Bab 1: Jatisrana | Ramabargawa. Bab 1: Jatisrana |

## Open questions (for the user)

1. **The curse**: what happens to the brothers in the Javanese telling you know? This design has them go still as stone until S4. The Mahabharata says they lost their reason.
2. **Renuka brought back**: does she remember? The design says no: only Bargawa remembers.
3. **Beat 3**: where are Renuka and the brothers when Jamadagni is killed? models.md has them in beats 1–2 only, so this design has Bargawa come home to an empty hermitage. In the Indian telling Renuka is there, grieving over the body.
4. **Why Jamadagni is killed**: the game shows only the broken kandhang, which fits the telling where the king's men take his livestock ([caritawayang](https://caritawayang.blogspot.com/2015/06/arjuna-sasrabahu.html)). Keep it unexplained, or add a line?
5. **The Javanese lines** above: the user checks every one.
6. **The title**: "Ramabargawa. Bab 1: Jatisrana", or another name for the game?
