# Forest Frog Figure.

Draft written from the user's reference sheet for the frog (its picture on a lily pad by a waterfall, its front, side, back and top views, wireframes, colour variations, animation poses and close-ups), for the user to correct. One of the forest lake's animals ([ForestLake.md](../../../environtments/ForestLake/ForestLake.md)), built as the sheet draws it, not in the project's older animal style: its colours are the sheet's own, not the theme's. Named ForestFrog since the project already has a Frog.

A chunky pond frog of flat facets, sitting up on its front legs, its back rising toward its head. A lime-green back with darker green spots, yellow-green flanks, and a cream throat, chest and belly. A big head with a wide mouth: a pale band of upper lip over a thin dark line that runs from the snout back to below the eyes, and two small dark nostrils. Big round eyes bulging from green lids on top of its head, dark brown round a black pupil, with a white glint. Straight front legs under its chest with four splayed fingers, big hind legs folded at its sides, and peach-orange feet with round toe tips, the hind toes webbed.

Size: 8 cm from snout to rump sitting, as a grown pond frog is, 5.8 cm to the top of its eyes, 6.3 cm across its spread feet.

Colour variations, as the sheet shows them: `green` (the default), `brown`, and the poison dart frogs `blue`, `yellow` and `red`, with bold black patches.

Parts: the body (with the head and mouth), the two eyes and the four legs.

It lives by the water. Put in the water, with its origin at the surface, it floats with its back, head and eyes out, and swims rather than hops.

## Animation Behavior.
1. `Idle()`

    Sits still, its throat pulsing, blinking now and then. In the water it floats at the surface, its legs spread.

    Example:
    `frog.Idle()`
2. `Walk()`

    Creeps forward in small low hops, about 2 cm each, the way it faces, until told otherwise. In the water it swims instead.

    Example:
    `frog.Walk()`
3. `Jump()`

    One leap forward: it crouches, leaps about 20 cm on and 7 cm up with its hind legs stretched out behind, lands on its front feet, sits and idles. From the water it leaps out and dives back in.

    Example:
    `frog.Jump()`
4. `Swim()`

    Swims forward at the water's surface, its body level, kicking its hind legs together with its front legs tucked back, until told otherwise. On dry ground it does the same on its belly.

    Example:
    `frog.Swim()`
5. `SetColor(variation string)`

    Paints it in one of the sheet's colour variations.

    Example:
    `frog.SetColor("blue")`
