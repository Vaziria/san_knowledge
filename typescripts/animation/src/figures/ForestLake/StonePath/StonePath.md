# Stone Path Figure.

Draft written from the user's reference sheet ("add task," with [stone_path.png](../../../../docs/references/stone_path.png), 2026-10-01; task 19), for the user to correct. One of the forest lake's assets ([ForestLake.md](../../../environtments/ForestLake/ForestLake.md)): its paths are stone paths. Its colours are picked from the sheet (`STONE` in [parts.ts](parts.ts)); what grows in and along it takes the forest lake's own palette and season ([../parts.ts](../parts.ts)).

A path of flat, chunky flagstones, as the sheet's main picture draws it by the forest lake's fence, lamp post and waterfall, in one of the sheet's five variations:

1. **Regular**: flagstones laid close in rows, the joints between them narrow and green with grass, tufts along its edges.
2. **Mossy**: smaller stones, the joints wider and full of moss and small plants, moss on some of the stones.
3. **Dirty**: smaller stones spaced out in brown dirt, pebbles between them.
4. **Ruined**: stones broken, tilted and raised a little, some missing, rubble round them; in the forest lake, more broken the nearer it gets to the ruins.
5. **Stairs**: oblong stone blocks, two to a step, climbing, grass at their sides.

A flagstone is a slab whose top has five to eight corners (a square's, each corner a little off it, some clipped or a side bowed), a little domed and faceted round its middle, its edge bevelled and its sides tapering out to its foot, as the sheet's components and wireframe draw one: 35 to 56 triangles. A few shapes of each kind: whole ones, mossy ones, broken pieces, chips and pebbles, and the stairs' blocks. Grey with a cool tint, paler on top and darker down its bevel and sides, darkest at its foot.

The sheet's maps go into the colours at the corners, as everything in the forest lake is drawn: the albedo is the stones' and the joints' colours, the normal map is the facets, and the ambient occlusion is the darkness baked in at the joints (darker toward the path's middle) and round each stone's foot. No texture maps.

Seasons, as the sheet's Environment usage row shows them: in spring and summer green joints and verge, small flowers (more in spring); in autumn gold grass and orange and red leaves lying on the stones and between them; in winter snow filling the joints and banked along the sides, the stones' tops showing with a little snow on them.

Size: a path 1.4–1.8 m wide (the preview's strips 1.6 m wide and 4.2 m long); flagstones 25–78 cm across and 8–12 cm thick, their tops at most 5 cm over the ground, so what walks there walks over them. A step rises about 15 cm and is 34–80 cm deep, as the ground's slope takes to rise that.

Options: `variation` (regular by default), `length`, `width`, `season` (spring by default), `seed`; and, for a path laid along a way of its own (the forest lake's), its `stretches`, each with its course, width and variation, and the `light` on the ground to tint it by.

Parts: the stones (drawn instanced, a few shapes of each kind), what fills between them (the joints: the ground's fill and what grows or lies in it), the verge along its sides. Its preview shows the five variations side by side, as the sheet's Variation row lays them out.

The stairs' treads are ground: what stands on them stands on its tread.

Nothing moves. No behaviours.
