# Forest Fox Figure.

Draft written from the user's reference sheet for the fox (its picture on a rock by a waterfall, its front, side, back and top views, wireframes, colour variations, animation poses and close-ups), for the user to correct. One of the forest lake's animals ([ForestLake.md](../../../environtments/ForestLake/ForestLake.md)), built as the sheet draws it, not in the project's older animal style: its colours are the sheet's own, not the theme's. Named ForestFox since the project already has a Fox.

A slender red fox of flat facets, bright orange, a few facets a deeper orange. A white bib from its chin down its throat and chest, white cheeks and muzzle sides with white tufts flaring out below the eyes, and a pale line along its belly. Black stockings below its elbows and hocks, and dark paws. A wide face narrowing fast to a long pointed snout with a small black nose, big dark amber eyes, and big pointed ears, orange at their edges, black behind and white inside. A huge brush of a tail hanging behind, white for its last third.

Size: 39 cm at the shoulder, as a red fox is, 82 cm from nose to tail tip (the sheet's proportions: its body is shorter than a real fox's), 60 cm to its ear tips.

Colour variations, as the sheet shows them: `orange` (the default), `white` (arctic), `brown` and `black` (black and silver).

Parts: the head (with its lower jaw), the body, the tail and the four legs.

## Animation Behavior.
1. `Idle()`

    Stands still, breathing, now and then looking to one side.

    Example:
    `fox.Idle()`
2. `Walk()`

    Walks forward the way it faces until told otherwise.

    Example:
    `fox.Walk()`
3. `Run()`

    Gallops forward the way it faces until told otherwise, its brush streaming out behind.

    Example:
    `fox.Run()`
4. `Sit()`

    Sits on its haunches, front legs straight, its brush curled round to one side, looking about, until told otherwise.

    Example:
    `fox.Sit()`
5. `LookUp()`

    Sits and lifts its head to look up at the sky for a few seconds, then sits on.

    Example:
    `fox.LookUp()`
6. `SetColor(variation string)`

    Paints it in one of the sheet's colour variations.

    Example:
    `fox.SetColor("white")`
