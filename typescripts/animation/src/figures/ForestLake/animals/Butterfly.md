# Butterfly Figure.

Draft written from the user's reference sheet for the butterfly (its picture flying over flowers by a lake and a waterfall, its front, side, back and top views, wireframes, colour variations, animation poses and close-ups), for the user to correct. One of the forest lake's animals ([ForestLake.md](../../../environtments/ForestLake/ForestLake.md)), built as the sheet draws it, not in the project's older animal style: its colours are the sheet's own, not the theme's.

A monarch butterfly of flat facets. Bright orange wings, fore and hind, split into cells of deep orange, orange and pale yellow by dark veins, with broad dark borders carrying rows of white spots, and a dark forewing tip with a long white bar. A dark brown body: a round head with big black eyes, each with a glint and a tan ring round it, a deep thorax with a tan collar, and a slim tapering abdomen with a few pale dots. Six long thin dark legs, and long thin antennae ending in tan clubs shaped like long diamonds. The wings are thin sheets seen from both sides, their pattern the same on both.

Size: 10 cm across its wings, as a monarch is, and 4 cm from its head to the tip of its abdomen. It hovers with its body 30 cm above the ground under it.

Colour variations, as the sheet shows them: `monarch` (the default), `blue`, `purple`, `yellow`, `white` and `pink`.

Parts: the head (with its eyes and antennae), the body (its thorax), the abdomen, the left and right wings (each a forewing and a hind wing), the two antennae and the six legs.

## Animation Behavior.
1. `Idle()`

    Hovers where it is, its wings beating slowly between the sheet's Flap Up (nearly together over its back) and Flap Down (pressed down past its sides), bobbing a little with each beat.

    Example:
    `butterfly.Idle()`
2. `Fly()`

    Flies forward the way it faces until told otherwise, nose up, its legs tucked, beating faster on a path that rises and falls, as the sheet's Fly (Side) draws it. On each upstroke its hind wings trail its forewings. It banks into turns.

    Example:
    `butterfly.Fly()`
3. `Land(height number)`

    Glides down onto what is `height` m above the ground under it, its six legs reaching down to stand, and rests there for six seconds, opening and closing its wings slowly over its back, as the sheet's Land on Flower draws it. Then it rises back to hovering. Without a height it lands on the ground, or on the flower a preview has put under it.

    Example:
    `butterfly.Land(0.16)`
4. `SetColor(variation string)`

    Paints it in one of the sheet's colour variations.

    Example:
    `butterfly.SetColor("blue")`
