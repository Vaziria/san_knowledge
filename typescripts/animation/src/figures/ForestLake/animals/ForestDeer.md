# Forest Deer Figure.

Draft written from the user's reference sheet for the deer (its picture on a rock by a waterfall, its front, side, back and top views, wireframes, colour variations, animation poses and close-ups), for the user to correct. One of the forest lake's animals ([ForestLake.md](../../../environtments/ForestLake/ForestLake.md)), built as the sheet draws it, not in the project's older animal style: its colours are the sheet's own, not the theme's. Named ForestDeer since the project already has a Deer.

A stag of flat facets, warm tan, a few facets darker. A white throat and chest, a pale belly and a white patch on its rump. A long neck held high, and a slim head: pale brows over the eyes, a cream muzzle round a black nose, a darker bridge of the nose, dark gentle eyes, and big leaf-shaped ears standing out to the sides, turned forward, pink inside a rim of its coat. Antlers of pale bone, a lyre from the front: each main beam leaves the head nearly flat, turns up and ends in two points, with a brow tine reaching forward over the forehead and two tines rising from its bend, their tips paler. Long slender legs, pale down their fronts and insides, on black cloven hooves. A short tail, white underneath, that flicks up when it runs.

Size: 1 m at the shoulder, as a grown stag is, 1.62 m from nose to tail, 1.87 m to its antler tips.

Colour variations, as the sheet shows them: `brown` (the default), `winter` (light), `autumn`, `dark` and `snow` (white).

Parts: the head (with its antlers and lower jaw), the body, the tail and the four legs.

## Animation Behavior.
1. `Idle()`

    Stands still, breathing, now and then looking to one side.

    Example:
    `deer.Idle()`
2. `Walk()`

    Walks forward the way it faces until told otherwise.

    Example:
    `deer.Walk()`
3. `Run()`

    Gallops forward the way it faces until told otherwise, its tail up.

    Example:
    `deer.Run()`
4. `LookBack()`

    Turns its head to look back over its shoulder for a few seconds, then stands again.

    Example:
    `deer.LookBack()`
5. `SetColor(variation string)`

    Paints it in one of the sheet's colour variations.

    Example:
    `deer.SetColor("autumn")`
