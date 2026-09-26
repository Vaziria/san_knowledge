# Mud Terrain.

Draft written from the code, for the user to correct.

A stretch of wet, muddy ground, a base an environment's land is made from. Where it is wettest the mud is dark, smooth and has a sheen, and shallow puddles of standing water lie in it; in drier patches, and toward its edge, it dries paler and rougher, its grit, small stones and cracks showing, until it is the colour of the bare land it meets. It is the mud pit's mud as open ground: level with the land, with no rim. The mud is the bare land's colour, darker where it is wet; the puddles are the water's colour, with a glint like the lake's.

It lies over the land it is given, uneven or flat. Size: 8 m by 6 m by default, its outline an uneven oval filling that footprint; it takes its size in meters, `new MudTerrain({ width, depth })`, and an environment can give it any outline instead. `seed` builds another stretch of the same mud; `puddles` says how many puddles (about one for every 8 m² by default).

No animation behaviour: it lies still.
