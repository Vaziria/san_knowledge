# Wild Boar Figure.

Draft written from the user's reference sheet for the wild boar (its picture on a rock by a waterfall, its front, side, back and top views, wireframes, colour variations, animation poses and close-ups), for the user to correct. One of the forest lake's animals ([ForestLake.md](../../../environtments/ForestLake/ForestLake.md)), built as the sheet draws it, not in the project's older animal style: its colours are the sheet's own, not the theme's.

A heavy boar of flat facets, brown, patched with darker facets, darker along its spine and underneath. A hump at its shoulders with a crest of dark bristles along its spine, tallest over the neck. A big wedge of a head, as broad as its shoulders, sloping down to a flat grey snout disc with two dark nostrils; curved white tusks from its lower jaw, out and up past the snout, and small upper ones; small dark eyes with a glint; pointed ears standing up and out, dark inside. Short, thick legs darkening to black-brown below, on dark cloven hooves. A thin tail hanging behind, ending in a dark tuft.

Size: 85 cm at the shoulder, as a grown boar is, 1.38 m from snout to tail.

Colour variations, as the sheet shows them: `brown` (the default), `dark`, `gray` and `snow`.

Parts: the head (with its tusks), the body (with its bristles), the tail and the four legs.

## Animation Behavior.
1. `Idle()`

    Stands still, breathing, now and then looking to one side.

    Example:
    `boar.Idle()`
2. `Walk()`

    Walks forward the way it faces until told otherwise.

    Example:
    `boar.Walk()`
3. `Run()`

    Gallops forward the way it faces until told otherwise.

    Example:
    `boar.Run()`
4. `Charge()`

    Lowers its head, tusks forward, and charges forward at a gallop for three seconds, then slows and stands.

    Example:
    `boar.Charge()`
5. `Eat()`

    Lowers its snout to the ground and roots about for food, its tail swishing, until told otherwise (the sheet's "Eat / Forage").

    Example:
    `boar.Eat()`
6. `SetColor(variation string)`

    Paints it in one of the sheet's colour variations.

    Example:
    `boar.SetColor("gray")`
