# Water Terrain.

Draft written from the code, for the user to correct.

Water filling a hollow in the land, a base an environment's land is made from, like the lake's water. The environment shapes the hollow; the water lies in it up to its surface, and meets the land in a crisp line where the bank slopes up out of it. It is see-through and moves in small waves, 1–3 cm high, that grow higher and roughen in the wind. Its bed is the bare land's colour, fading toward the water's colour with depth, so deep water looks deeper; the bank is darker and smoother just above the water, where it is wet. The water is the theme's water colour.

Size: 10 m by 8 m by default, its outline an uneven oval filling that footprint, which must reach past the water's edge all round; it takes its size in meters, `new WaterTerrain({ width, depth })`, and an environment can give it any outline instead. Its origin is on the still water's surface.

No animation behaviour: on its own the water moves in its waves.
