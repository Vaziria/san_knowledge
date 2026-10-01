# Forest Spirit Dragon Figure.

Draft written from the user's ten reference sheets ("Forest Spirit Dragon: Majestic · Friendly · Low Poly · Game Ready": the overview, and the head, eye, snout, horn, neck and scale, wing, back spike, tail and claw breakdowns, kept in [docs/references/forest_spirit_dragon](../../../../../docs/references/forest_spirit_dragon/)), for the user to correct, when the user asked for it ("im add dragon in forest lake, this image for reference, create as is, dont let previous style influence you, first create the head and i will preview", 2026-10-01). One of the forest lake's animals ([ForestLake.md](../../../../environtments/ForestLake/ForestLake.md)), built as the sheets draw it, in their own look and colours, not the theme's and not the other animals'.

**All of it is built**: first the head, which the user previewed, then the rest ("okay build at rest", 2026-10-01): the neck, body, wings, legs, tail and back spikes. Then it was given its moves, for the forest lake meeting ("in forest lake meeting, its fly and roar around our cristal tree, and sometimes its walk calm in ground", 2026-10-01): it walks calmly, takes off, flies, lands and roars (below).

A big, friendly dragon of the forest, on four legs, with wings: its body covered in overlapping leaf-shaped scales in greens and yellow-greens, its belly, throat, muzzle and jaw in broad cream plates, tan horns like branching wood, and blue crystals growing from its head, along its back and at the end of its tail, which glow at night. Low poly, every face flat.

- **Head**: big and rounded, with a short snout; green above and cream below. Leaf scales over the forehead and the bridge of the nose, pointing back. Big amber eyes with vertical slit pupils, set forward, a dark line round them, green lids, and cream brow scales over them. Small nostrils at the yellow-green tip of the snout, and a mouth line turning up in a smile. Two branching horns rising up and back from the top of the head, a tine forking from each, an upper branch behind each, and the long middle horns sweeping straight back below them; a crown of crystals between them. A tan leaf of an ear on either side, a ruff of leaf scales round the back of the head (green, gold, cream and crystal blue), gold and cream cheek scales, and cream fur under the chin. The lower jaw opens on a red mouth with white teeth and a tongue.
- **Neck**: long and thick, rising from the chest and leaning forward into the head; cream plates down its front, each overlapping the one below like a snake's, layered leaf scales on its back and sides, larger lower down, side frills of bigger gold and green leaves low on it, and crystals along the back of it, a cluster of big ones at its foot.
- **Body**: heavy and rounded, green leaf scales above lying back like shingles; cream below, the throat's plates carrying on down the front of the chest and broad, uneven plates over the belly; crystal spikes along the spine, each in a cup of leaves.
- **Wings**: bat-like, raised and half open as every picture of it stands it: an olive green arm rising from the back to a wrist higher than the head, a pale thumb claw there, and two fingers, the first along the arcing leading edge to the tip; a cream membrane between them and down to the body, cut in a deep scallop between the fingers' tips; big leaf plates on the membrane where the wing meets the body, and a crystal at its root.
- **Back spikes**: crystals in cups of leaves along the spine, large at the shoulders and smaller toward the tail, flatter on the tail.
- **Tail**: long and tapering, falling from the rump to just above the ground and sweeping round to its left, green leaf scales over its top and sides pointing to the tip, cream plates beneath, and at its end a plume of crystal blades and leaf blades (green, gold and cream) fanning up and back.
- **Legs**: four sturdy legs of leaf scales pointing down, a ring of bigger leaves hanging over each ankle, the back of each foreleg cream with cream tufts at the elbow and the wrist; paws green on top and cream beneath, three cream toes and a small inner one, each with an ivory claw, the front claws larger and more curved.

Where the sheets disagree, the overview's picture of the whole dragon is followed, and the breakdowns give the detail it is too small to show:
- The wing sheet's bones are tan with lilac tips, the overview's green: green here.
- The claw sheet's claws are crystal, the overview's ivory: ivory here.
- Seen from the side the wings reach straight back, seen from behind they spread out to the sides: here they spread halfway, 6.1 m across.
- The overview's big picture sweeps the tail round to the dragon's left, as here; its side and back views can't show that.
- The sheets draw it mid-stride; here it stands square on all four feet, as an animal at rest.

Size: 4.5 m to the top of its head standing (the overview's size comparison, beside a 1.7 m player), its crown at 4.4 m and its horns' tips at 5.1 m, 7.9 m from its snout to the end of its tail's plume. Its head is about 1.3 m from the back of the skull to the tip of the snout and 1 m from chin to crown. Its back is 2.45 m up at the shoulders, its belly 1 m off the ground.

Colour variations, as the sheets show them: `forest` (the default), `autumn`, `winter`, `crystal` and `desert`. Only `forest` is built so far.

Parts, the overview's modular ones: the head (the head sheet's exploded view names its own: its top, horns, crystals, ears, eyes, upper and lower jaw, side, cheek and back scales, chin fur), the neck, the body, the left and right wings, the four legs and the tail. The neck, the tail and the legs bend; the wings beat about the line where they join the body; the lower jaw opens.

## Animation Behavior.
Draft by Claude, from the sheets' poses the user picked for the forest lake meeting (fly, roar, walk calmly), for the user to correct. It moves itself forward the way it faces, and is turned to steer; flying, it climbs or comes down to the height it is given and banks into its turns. On the ground its feet stay where they are set down.

1. `Idle()`

    Stands still, breathing, now and then looking about. Flying, it lands first.

    Example:
    `dragon.Idle()`

2. `Walk()`

    Walks calmly forward, 0.7 m a second, until told otherwise, the slow four-beat walk of a big animal, its tail swaying. Flying, it lands and then walks.

    Example:
    `dragon.Walk()`

3. `Stop()`

    Stands still where it is (as `Idle()`).

    Example:
    `dragon.Stop()`

4. `Fly()`

    Crouches and springs up on a downstroke of its wings, its legs folding under it, and flies on forward at 8.5 m a second, climbing to its flying height, its tail streaming out behind and its neck reaching forward: beating its wings as it climbs, gliding as it comes down. Landing, it flies on again.

    Example:
    `dragon.Fly()`

5. `Land()`

    Comes down to the ground ahead, slowing to a stand as it comes, braking with its wings and reaching for the ground with its legs, and takes the landing in them.

    Example:
    `dragon.Land()`

6. `Roar()`

    Draws its head back, then thrusts it up and roars, its jaws wide, for about 3 s, on the ground or flying, going on with what it was doing.

    Example:
    `dragon.Roar()`

The sheets also show run, glide, attack, tail attack, hit and sleep, and the head's expressions (neutral, happy, roar, angry, surprised, blink), for the user to choose from later.
