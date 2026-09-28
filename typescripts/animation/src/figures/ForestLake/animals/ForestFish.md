# Forest Fish Figure.

Draft written from the user's reference sheet for the fish (its picture in the lake among rocks and water weed, its front, side, back and top views, wireframes, colour variations, animation poses and close-ups), for the user to correct. One of the forest lake's animals ([ForestLake.md](../../../environtments/ForestLake/ForestLake.md)), built as the sheet draws it, not in the project's older animal style: its colours are the sheet's own, not the theme's. Named ForestFish since the project already has a Fish.

A trout of flat facets, like folded paper. Olive green along its back, fading through a golden band along its side to a cream belly, with small dark spots of six or seven sides over its back and sides. A rounded head with a big dark eye in a ring of golden skin, a white glint up toward its tail, a small nostril, and a dark crease along the back edge of its gill cover. A pale rounded bar of an upper lip along the jaw's edge, arched over the mouth, and a pale lower jaw; its mouth hangs a little open, red inside. Orange fins, each a pleated fan of rays, a deeper orange toward the body: a tall dorsal fin, a little adipose fin behind it, two pectoral fins behind the gills, two pelvic fins under its middle, the anal fin and a broad forked tail.

Size: 40 cm from its snout to the tips of its tail, as a grown trout in a lake is. Its body is 13 cm deep and 9 cm wide; 23 cm from the tips of its pelvic fins to the top of its dorsal fin.

It swims in the lake, its middle 15 cm under the surface. Out of the water it rests on its fins where it is put, and its poses still play.

Colour variations, as the sheet shows them: `trout` (the default), `salmon` (red), `carp` (gold), `koi` (white with orange patches) and `blue`.

Parts: the head, the jaw (the lower jaw and the inside of the mouth), the body, the tail fin, the dorsal fin, the adipose fin, the anal fin, the two pectoral fins and the two pelvic fins.

## Animation Behavior.
1. `Idle()`

    Hovers where it is, its pectoral fins sculling, its body swaying slowly and its mouth working as it breathes.

    Example:
    `fish.Idle()`
2. `Swim()`

    Swims forward the way it faces until told otherwise, a wave running along its body from its head to its tail, its fins folded back.

    Example:
    `fish.Swim()`
3. `Turn(degrees number)`

    Turns sharply where it is, its body bent into a C, then hovers. 180 degrees (the default) turns it round to face the other way; more than 0 turns it to its left.

    Example:
    `fish.Turn(90)`
4. `OpenMouth()`

    Opens its mouth wide and shuts it, three times, then hovers.

    Example:
    `fish.OpenMouth()`
5. `Jump(height number)`

    Leaps out of the water in an arc, clearing it by `height` meters (0.3 by default, 1 at most), and dives back in ahead of where it left, then hovers at its depth. The water splashes where it breaks the surface, going out and coming back in. Out of the water it flops: a hop where it lies.

    Example:
    `fish.Jump(0.5)`
6. `SetColor(variation string)`

    Paints it in one of the sheet's colour variations.

    Example:
    `fish.SetColor("koi")`
