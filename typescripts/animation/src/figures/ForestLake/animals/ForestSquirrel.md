# Forest Squirrel Figure.

Draft written from the user's reference sheet for the squirrel (its picture on a rock by a waterfall, its front, side, back and top views, wireframes, colour variations, animation poses and close-ups), for the user to correct. One of the forest lake's animals ([ForestLake.md](../../../environtments/ForestLake/ForestLake.md)), built as the sheet draws it, not in the project's older animal style: its colours are the sheet's own, not the theme's. Named ForestSquirrel since the project already has a Squirrel.

A red squirrel of flat facets, bright orange-red. A white chest and belly, a pale muzzle and cheeks, big dark eyes with a glint, a small dark nose, and pointed ears, pale inside, each with a dark tuft at its tip. Dark little hands and feet, and a huge plume of a tail rising behind its back and curling back over at the top, paler at its tip. It stands still sitting up on its haunches with an acorn in its hands, as the sheet draws it, nibbling it now and then; on the move it goes on all fours with the acorn in its mouth.

Size: 20 cm from nose to rump, and a tail as long again, as a red squirrel is; 27 cm tall sitting up.

Colour variations, as the sheet shows them: `brown` (the default, its orange-red), `gray`, `red`, `black` and `white` (albino).

Parts: the head, the body, the tail, the four legs and the acorn.

## Animation Behavior.
1. `Idle()`

    Sits up with its acorn, nibbling it now and then.

    Example:
    `squirrel.Idle()`
2. `Walk()`

    Walks forward on all fours the way it faces until told otherwise.

    Example:
    `squirrel.Walk()`
3. `Run()`

    Bounds forward the way it faces until told otherwise, its tail streaming out behind.

    Example:
    `squirrel.Run()`
4. `Jump()`

    Leaps forward, 18 cm high and 45 cm on, and sits.

    Example:
    `squirrel.Jump()`
5. `Climb()`

    Climbs the trunk of a tree whose face is 22 cm ahead of it: onto it, 60 cm up, turns round and comes down head first, then lunges off onto the ground where it started, facing away, and sits. Its preview puts two trees there.

    Example:
    `squirrel.Climb()`
6. `SetColor(variation string)`

    Paints it in one of the sheet's colour variations.

    Example:
    `squirrel.SetColor("gray")`
