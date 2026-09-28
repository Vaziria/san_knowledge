# Forest Lake

Draft written from the user's two reference images ("Spring - Forest Lake": the detailed view with its numbered places, and the detailed assets and views with the asset sheet), for the user to correct. Refined on 2026-09-27 from the user's later reference sheets: the forest lake's objects in detail (per asset, and in low poly with their components), its key locations close up, its path variations, and the forest lake in its four seasons, "Spring, Summer, Autumn, Winter", which the user asked it to have ("ensure forest lake have 4 season like that reference"). The user asked for it "as is", in its own look, not the project's: its colours are the reference's (the forest lake's palette, [parts.ts](../../figures/ForestLake/parts.ts)), and it brings its own sky colour and lights. Picked as `forest_lake` in the Environments tab or with `?environment=forest_lake`; its season is the tab's Season setting (spring, summer, autumn or winter, `?season=`, spring by default), and a new season rebuilds it.

A valley round a lake, as the reference's overview lays it out, looking north from the south shore:

1. **The lake** in the middle, about 40 m across, clear turquoise in the shallows over a sandy bed, deepening to blue, 3 m deep at most. Bright lines of light wander over it in cells, the sky shows in it at a glance, the sun glints off its small waves, and white foam gathers at its shore and round the rocks in it. A ring of boulders lines its shore, their feet in the water. Lily pads with pink water lilies float in its shallows, and reeds with brown cattails stand in them.
2. **Two waterfalls** from the cliffs to the north: the main one about 10 m high, from a river running over the heights, and a smaller one, about 6 m, to the northwest. Each is a falling sheet of pale blue streaked with white, with white foam heaving where it lands, mist rising and boulders standing in its pool. A wooden footbridge crosses the main river just above its fall.
3. **The island**, off the middle of the lake toward its east shore: a plinth of rock rising steeply out of deep water, ringed with boulders, with a level grassy top. On it a big cherry tree in the middle, a pine behind it, a bush and flowers, a fence round its top on two sides, a double lamp (two lanterns on a crossbar) either side of the tree and a sign post; stone steps lead up from its jetty.
4. **The piers**: one from the southwest shore with a lamp at its landward corner and a rowboat moored beside it, a shorter one from the east shore, and the island's jetty.
5. **The outlet**: the lake runs out to the southeast in a stream that drops 1.9 m down a cascade just under an arched wooden bridge, then winds away southeast between rocky banks.
6. **The forest path**: a sandy dirt path from the camp along the south shore, past the landing, over the bridge and north along the east shore to the cave, with fences along stretches of it, lamp posts along it, flat stones and pebbles in it, and sign posts where paths meet (some with a lantern). Branches run from the camp north to the ruins and to the pier, and the way in comes from the south through **the forest entrance**, a wooden gate with a lantern hanging each side, fences running off from its posts.
7. **The camp**, in the southwest: a trodden clearing with a tent facing a campfire, a pot hanging over the fire from a tripod, log seats round it, crates and barrels by the tent, a lamp at its way in and a fence round its back.
8. **The cave**, in the cliffs to the east, as big as the reference draws it: its mouth glowing orange, torches either side, a campfire before it.
9. **The ruins**, on a terrace to the west: a stone arch, broken pillars and fallen blocks.
10. **The watch tower**, on the heights to the northeast, its flag waving.

Round all of it: cliffs of tall, faceted grey columns of rock under the heights to the north and east, capped with moss, with bushes and grass on their ledges and along their top, and boulders at their feet. A mixed forest stands on every side, pines, round broadleaf trees and cherries, thick right down to the lake's banks and sparser on the far hills. Bushes grow at the forest's edges, tall grass and flowers in the meadows, rocks lie along the shore and the stream, and logs, stumps and mushrooms in the forest. Beyond are forested hills and snowy mountains, tallest to the north, under a bright blue sky with big white clouds drifting, all hazed with distance.

## Seasons

The same valley in each, as the reference's four seasons show it; everything in it takes the season (every asset has a `season` option):

- **Spring**: fresh green grass, cherry trees in pink blossom, pink water lilies, flowers of every colour in the meadows; cherry petals fall round the trees by the landing and on the island.
- **Summer**: a deeper, denser green; the cherry trees in green leaf; white water lilies; white and yellow flowers.
- **Autumn**: the broadleaf trees orange, red and gold, the cherry trees red, some pines golden larches, the grass gold; fallen leaves lie thick on the ground and float on the lake by its shore, and leaves fall from the trees by the landing; the tent's canvas orange; more mushrooms; a warmer haze.
- **Winter**: snow on everything, the grass, the trees' tiers and branches, the rocks, the cliffs' ledges, the roofs and decks; icicles on the cliffs' top, the watch tower's eaves and over the cave's mouth; bare trees among the pines; red berries on the bushes and in sprigs out of the snow; the lake frozen, plates of pale ice with white cracks between them, but for a hole of open water where a floating figure goes; the rivers ice; the waterfalls nearly still; snow falling; a paler sky.

Each season brings its own light: a warm sun and a sky light pale blue from above and, from below, the ground's colour, softened (a little green off the grass, gold off autumn's leaves, white off the snow). Colours stay vivid in the shade, as the reference paints them.

Figures stand at the landing, a level clearing on the south shore, looking north across the lake to the island and the main waterfall; a figure in the water (the boat, the fish) goes to open water west of the island (in winter the hole in the ice). The views from the landing, north to the water and south behind the preview cameras, are kept clear of trees and bushes.

The walking camera walks the land, the water's surface (and its ice), the piers' and bridges' decks, the island's steps and the cave's floor, within 75 m of the lake's middle.

How it moves: the water's waves, light and foam; the waterfalls' streaks, foam and mist; the stream running; the flames and sparks of the campfires and torches; the lanterns flickering; the flag waving; the clouds drifting; the petals, leaves or snow falling; the moored boat riding the waves (frozen still in winter). No day and night, no wind.

Size: the valley about 120 m across inside its forest; land out to 340 m, hills and mountains to 1.1 km.

Everything in it is built from BufferGeometry made in code: the assets are figures in [src/figures/ForestLake](../../figures/ForestLake), each with its spec; the land, water, cliffs, sky and what grows are built here.
