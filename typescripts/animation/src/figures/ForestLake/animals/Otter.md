# Otter Figure.

Draft written from the user's reference sheet for the otter (its picture on a rock by a lake and a waterfall, its front, side, back and top views, wireframes, colour variations, animation poses and close-ups), for the user to correct. One of the forest lake's animals ([ForestLake.md](../../../environtments/ForestLake/ForestLake.md)), built as the sheet draws it, not in the project's older animal style: its colours are the sheet's own, not the theme's.

A Eurasian river otter of flat facets, long, low and sleek, a soft greyish brown, a few facets darker, with a darker crown. A cream muzzle, cheeks and chin, running down the throat and chest as a bib to between the fore legs, and a pale patch on the belly. A broad, flat head with small round ears, dark inside; big round glossy eyes, dark brown round a black pupil, with a glint; a broad black nose over two puffed whisker pads; a thin dark smile; and long pale whiskers fanned out and back. Short, thick legs, dark below the elbows and knees, on big dark paws of four thick toes. A long, thick tail, broad where it leaves the rump, tapering to a point along the ground.

Size: 97 cm from nose to tail tip (a Eurasian otter's length, in the sheet's proportions), 34 cm at the shoulder, 46 cm to the top of its head as it stands.

Colour variations, as the sheet shows them: `brown` (the default), `dark` (a warmer, richer brown), `light` (caramel), `gray` and `albino` (white, with a pink nose, pink ears inside and red eyes).

Parts: the head, the body, the tail, the four legs and the whiskers.

On land it stands on the ground. In the water (afloat) it floats with its back, the top of its sides and its head out of the water; the water's surface is where it is put. Swim() puts it in the water, and a preview on the lake starts it there. Afloat, Walk() and Run() swim.

## Animation Behavior.
1. `Idle()`

    On land, sits up on its haunches, its fore legs straight and its tail along the ground, looking about. Afloat, treads water, its head up, looking about.

    Example:
    `otter.Idle()`
2. `Walk()`

    Walks forward the way it faces until told otherwise. Afloat, it swims.

    Example:
    `otter.Walk()`
3. `Run()`

    Bounds forward the way it faces until told otherwise, its back flexing and its tail streaming out behind. Afloat, it swims fast.

    Example:
    `otter.Run()`
4. `Swim()`

    Swims forward at the water's surface the way it faces, low in the water, paddling, its tail waving behind, until told otherwise.

    Example:
    `otter.Swim()`
5. `Dive()`

    In the water, arches over headfirst and dives, its tail flicking up out of the water, swims on under the water and comes back up nose first, then swims on (about 3.4 seconds). The water splashes where it goes in and comes out. On land it does nothing.

    Example:
    `otter.Dive()`
6. `StandUp()`

    Rears up on its hind legs, its fore paws held before its chest and its tail on the ground behind, looks about, then drops back to Idle (about 4.6 seconds). Afloat, it rears up out of the water.

    Example:
    `otter.StandUp()`
7. `SetColor(variation string)`

    Paints it in one of the sheet's colour variations.

    Example:
    `otter.SetColor("albino")`
