# Swamp Terrain.

Draft written from the code, for the user to correct.

Waterlogged ground, a base an environment's land is made from. Still, murky water stands in the land's low places, dark with peat and tinged green, its waves hardly moving; duckweed floats on it in bright green patches, most of them in the shallows. Hummocks of peat stand out of the water, each topped by a tussock of long arching sedge, and stands of cattails grow in the shallow water and along its edge, 1.3–1.9 m tall, with long strap-like leaves and brown heads on their stems. Round the water the ground is dark, wet mud with a sheen; higher up it is peat and moss with short grass and patches of bare mud, until it meets the bare land at its edge. It is dark all over, much darker than the bare land round it: the mud is the bare land's colour turned halfway to the wood's brown and darkened, the moss is the grass colour darkened, and the water is the water colour turned toward the grass and darkened well. The duckweed, sedges and reeds are the grass colour, and dry blades and the cattails' heads are the wood's colour (straw, and dark brown).

Like the water, it needs the land shaped for it: the water lies at its level wherever the land is lower, and the land must rise above it all round, inside its footprint. Size: 10 m by 8 m by default, its outline an uneven oval filling that footprint; it takes its size in meters, `new SwampTerrain({ width, depth })`, and an environment can give it any outline instead. `seed` builds another stretch of the same swamp; `reeds` says how many stands of cattails (about one for every 6 m² by default), `duckweed` how much of the open water it covers (about a third by default), and `rim` the colour its ground fades to at its edge, the land's round it (the bare land's by default). An environment with day and night dulls its water's glint at night and in rain (`setNight`), as the lake does its own water.

The lake has up to three of them at its lakeside, dug into its land for their water (Lake/Swamps.ts).

No animation behaviour: on its own the water moves in its small waves, and the sedges and reeds bend in the wind, when the environment has one.
