# Grass Terrain.

Draft written from the code, for the user to correct.

Grassy ground, a base an environment's land is made from, like the lake's land beyond its bank. A lawn a shade darker than the grass, mottled lighter and darker in patches about a meter across, covered in tufts of pointed blades of three shapes (upright and broad, spreading, fine and floppy), 7–26 cm tall with most of them short, each turned and shaded on its own and leaning a little with the slope. Toward its edge the tufts thin out and the lawn sinks under the land it meets. The colour is the theme's grass colour.

It lies over the land it is given, uneven or flat. Size: 10 m by 8 m by default, its outline an uneven oval filling that footprint; it takes its size in meters, `new GrassTerrain({ width, depth })`, and an environment can give it any outline instead. `seed` grows another stretch of the same grass; `height` and `density` make it shorter, taller, thicker or sparser.

No animation behaviour: the tufts bend in the wind, when the environment has one.
