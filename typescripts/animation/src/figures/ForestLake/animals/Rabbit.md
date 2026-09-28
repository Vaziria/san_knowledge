# Rabbit Figure.

Draft written from the user's reference sheet for the rabbit (its picture on a rock by a waterfall, its front, side, back and top views, wireframes, colour variations, animation poses and close-ups), for the user to correct. One of the forest lake's animals ([ForestLake.md](../../../environtments/ForestLake/ForestLake.md)), built as the sheet draws it, not in the project's older animal style: its colours are the sheet's own, not the theme's.

A round rabbit of flat facets sitting up, warm tan. A white chest and underside, a white muzzle and chin, a round head with big dark eyes with a glint and a small pink nose, and long upright ears, pink inside, spreading apart a little toward their tips. Big haunches over long hind feet lying flat on the ground, white front legs and paws, and a white cotton tail. It stands still sitting, as the sheet draws it, and hops from there, its feet in pairs.

Size: 27 cm to the top of its head sitting (39 cm to its ear tips), 37 cm from nose to tail, as a European rabbit is.

Colour variations, as the sheet shows them: `brown` (the default), `white` (snow), `gray`, `black` and `spotted` (white with brown patches).

Parts: the head (with its ears), the body, the tail and the four legs.

## Animation Behavior.
1. `Idle()`

    Sits still, breathing, now and then looking to one side.

    Example:
    `rabbit.Idle()`
2. `Walk()`

    Hops slowly forward the way it faces until told otherwise.

    Example:
    `rabbit.Walk()`
3. `Run()`

    Bounds forward the way it faces until told otherwise, stretching out and gathering up.

    Example:
    `rabbit.Run()`
4. `Jump()`

    Leaps up and forward from its haunches, 30 cm high and 55 cm on, stretching out in the air, and lands and sits.

    Example:
    `rabbit.Jump()`
5. `Eat()`

    Lowers its head to nibble the grass until told otherwise (the sheet's "Eat / Forage").

    Example:
    `rabbit.Eat()`
6. `SetColor(variation string)`

    Paints it in one of the sheet's colour variations.

    Example:
    `rabbit.SetColor("spotted")`
