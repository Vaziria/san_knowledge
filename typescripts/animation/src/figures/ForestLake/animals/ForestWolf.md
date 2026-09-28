# Forest Wolf Figure.

Draft written from the user's reference sheet for the wolf (its picture on a rock by a waterfall, its front, side, back and top views, wireframes, colour variations, animation poses and close-ups), for the user to correct. One of the forest lake's animals ([ForestLake.md](../../../environtments/ForestLake/ForestLake.md)), built as the sheet draws it, not in the project's older animal style: its colours are the sheet's own, not the theme's. Named ForestWolf since the project already has a Wolf.

A big grey wolf of large flat facets, like folded paper. A mane of fur in shingles over its neck and shoulders, pale over the throat and chest. A broad face: a grey crown and nose bridge, white brows, cheeks and muzzle sides, a cheek ruff of pale points framing it, big pointed ears standing up from the crown's corners, paler inside, narrow amber eyes slanting up at their outer corners, and a black nose. A pale chest, belly and legs, a darker saddle, big pale paws with dark pads, and a bushy tail hanging behind, pale underneath and at its tip.

Size: 78 cm at the shoulder, as a grown grey wolf is, 1.5 m from nose to tail tip, 1.12 m to its ear tips.

Colour variations, as the sheet shows them: `gray` (the default), `white` (winter), `brown`, `black` and `snow`.

Parts: the head (with its lower jaw), the body (with the mane), the tail and the four legs.

## Animation Behavior.
1. `Idle()`

    Stands still, breathing, now and then looking to one side.

    Example:
    `wolf.Idle()`
2. `Walk()`

    Walks forward the way it faces until told otherwise.

    Example:
    `wolf.Walk()`
3. `Run()`

    Gallops forward the way it faces until told otherwise, its tail streaming out behind.

    Example:
    `wolf.Run()`
4. `Howl()`

    Sits on its haunches, as the sheet draws it, lifts its head to the sky and howls, its mouth open, for about four seconds, then stands again.

    Example:
    `wolf.Howl()`
5. `SetColor(variation string)`

    Paints it in one of the sheet's colour variations.

    Example:
    `wolf.SetColor("white")`
