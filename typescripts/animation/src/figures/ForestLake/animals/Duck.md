# Duck Figure.

Draft written from the user's reference sheet for the duck (its picture on a rock by a lake, its front, side, back and top views, wireframes, colour variations, animation poses and close-ups), for the user to correct. One of the forest lake's animals ([ForestLake.md](../../../environtments/ForestLake/ForestLake.md)), built as the sheet draws it, not in the project's older animal style: its colours are the sheet's own, not the theme's.

A mallard drake of flat facets, like folded paper. A glossy green head with dark round eyes, each with a glint, and a broad yellow bill with dark nostrils and a small dark nail at its tip. A white ring round its neck over a full, round, deep chestnut breast, pale grey-beige flanks and belly, and a brown back. Its wings lie folded on its sides in a few large facets: cream to tan coverts, a brown band along their lower edge, a clean blue speculum edged in white, and dark wing tips meeting over the rump. A black rump, glossy green on top, with the drake's curl feathers over it, and a white tail under it, a white smile from behind. Orange legs and big webbed orange feet with grey claws.

Size: 52 cm from bill to tail and 45 cm to its crown, standing, and 89 cm across its spread wings, as a mallard is. On the water it floats a third of the way up its body.

Colour variations, as the sheet shows them: `mallard` (the default), `white`, `brown`, `mandarin` (the sheet's colourful one) and `black`.

Parts: the head, the neck, the body, the two wings, the tail and the two legs.

On the water: it floats when told to swim, or when its `afloat` is set; its preview on the lake keeps its `surface` at the water's height under it and hands its splashes (`onSplash`) to the water.

## Animation Behavior.
1. `Idle()`

    Stands still, or floats still on the water, breathing, now and then looking to one side and wagging its tail. On the water its feet paddle slowly.

    Example:
    `duck.Idle()`
2. `Walk()`

    Waddles forward the way it faces until told otherwise, rocking onto each foot, its head bobbing and its tail wagging. On the water it swims.

    Example:
    `duck.Walk()`
3. `Run()`

    Runs forward the way it faces until told otherwise, leaning ahead with its neck stretched out. On the water it paddles hard.

    Example:
    `duck.Run()`
4. `Swim()`

    Floats low in the water and swims forward the way it faces, paddling, its tail up, until told otherwise. Its preview offers it only on the lake.

    Example:
    `duck.Swim()`
5. `FlapWings()`

    Rears up on its tail and beats its wings half a dozen times, as the sheet draws it, then folds them and settles, about 3 seconds. On the water its wings splash it.

    Example:
    `duck.FlapWings()`
6. `TakeOff()`

    Runs a few steps beating its wings (on the water it patters over it), lifts off, climbs to 90 cm, flies on, glides down and lands, then folds its wings, about 6 seconds and 8 m on.

    Example:
    `duck.TakeOff()`
7. `SetColor(variation string)`

    Paints it in one of the sheet's colour variations.

    Example:
    `duck.SetColor("mandarin")`
