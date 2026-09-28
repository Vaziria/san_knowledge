# Forest Firefly Figure.

Draft written from the user's reference sheet for the firefly (its picture over a pond in the forest at night, its front, side, back and top views, wireframes, colour variations, animation poses and close-ups), for the user to correct. One of the forest lake's animals ([ForestLake.md](../../../environtments/ForestLake/ForestLake.md)), built as the sheet draws it, not in the project's older style: its colours are the sheet's own, not the theme's. Named ForestFirefly since the project already has a Firefly.

A low poly beetle of flat facets, orange-brown, its lit faces paler. A round head with big dark glossy eyes on its sides, each with a glint, and a small pointed snout. Long antennae of little segments, thickening to a club at the tip. A humped thorax, and six thin jointed legs with knobs at their knees. Two pairs of big see-through wings, pale with a dark leading edge and pale veins, glowing faintly where the lantern lights them. Three brown plates on its abdomen with dark lines between them, then a big faceted lantern at its tail that glows in its colour, pulsing brighter and dimmer, with a soft halo round it.

Size: 2 cm from the front of its head to the tip of its lantern, as a firefly is (the sheet draws it big), its front wings 1.6 cm long. It hovers with the middle of its body 4.5 cm above the ground.

Colour variations, as the sheet shows them: `yellow` (the default), `green`, `blue`, `purple`, `red` and `white`. Each colours its lantern, its halo and the tint of its wings; its body stays orange-brown.

Parts: the head (with its eyes), the two antennae, the body (its thorax), the abdomen, the lantern, the four wings and the six legs. Its lantern can carry a small light in its colour, off by default.

## Animation Behavior.
1. `Idle()`

    Rests in the air, bobbing gently, its wings beating slowly, now and then looking about.

    Example:
    `firefly.Idle()`
2. `Hover()`

    Hovers in place, its wings a blur of quick beats.

    Example:
    `firefly.Hover()`
3. `Fly()`

    Flies forward the way it faces until told otherwise, a little nose down, its legs drawn back and its antennae swept back.

    Example:
    `firefly.Fly()`
4. `Turn()`

    Banks and comes round, half a circle 2 cm in radius, the way it is already turning (to its right when it flies straight or hovers), slowing as it goes, then hovers.

    Example:
    `firefly.Turn()`
5. `Land()`

    Comes down onto what is under it, its legs reaching for it, folds its wings back over its body and rests there three seconds, its lantern still glowing, then takes off again, back up to where it was, and idles. The sheet's "Land on Leaf": its preview puts a leaf under where it lands.

    Example:
    `firefly.Land()`
6. `SetColor(variation string)`

    Paints it in one of the sheet's colour variations.

    Example:
    `firefly.SetColor("green")`
