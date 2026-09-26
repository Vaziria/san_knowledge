# Sand Terrain.

Draft written from the code, for the user to correct.

Dry, loose sand, a base an environment's land is made from, as on a beach or a sandbank. It lies a few centimeters deep on the land, heaped a little higher and lower in soft drifts, and thins out toward its edge until it sinks under the land it meets. Its top is covered in the ripples the wind raises on dry sand: long wavy crests about 17 cm apart and about a centimeter high, high in some patches and all but gone in others, with darker grains gathered in their troughs, over fine grain. It is matte, and mottled a little lighter and darker in patches a couple of meters across. The colour is the theme's sand colour.

It lies over the land it is given, uneven or flat. Size: 10 m by 8 m by default, its outline an uneven oval filling that footprint; it takes its size in meters, `new SandTerrain({ width, depth })`, and an environment can give it any outline instead. `seed` builds another stretch of the same sand, its ripples facing another way; `ripples` says which way the wind blew that raised them.

No animation behaviour: it lies still.
