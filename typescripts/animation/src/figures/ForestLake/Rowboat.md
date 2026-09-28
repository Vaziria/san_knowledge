# Rowboat Figure.

Draft written from the user's two reference images ("Spring - Forest Lake": the "Boat" asset and the boat moored at the pier), for the user to correct. One of the forest lake's assets (the forest_lake environment, [ForestLake.md](../../environtments/ForestLake/ForestLake.md)); its colours are the forest lake's own palette ([parts.ts](parts.ts)), picked from the images, not the theme's. Named Rowboat because the project already has a Boat. Refined on 2026-09-27 from the user's later reference sheets (the forest lake's objects in detail, and the forest lake in its four seasons, "Spring, Summer, Autumn, Winter"), which the user asked the forest lake to follow, in all four seasons.

A wooden rowing boat, its bow (+x) pointed and its stern cut square. It is built of strakes, five a side, in alternating shades of orange-brown, paler toward the rim, with a pale rim along its top, a keel under it and a stem up its bow. It is darker inside, with three seats across it, four ribs, and two oars lying on the seats. Its keel is at y 0.

Low poly, every face flat shaded, bright and saturated, as the reference is.

Size: 3 m long and 1.1 m wide; afloat it sits 15 cm deep.

Seasons: In winter snow lies on its seats and gunwales.

Options: `season` (spring by default), `seed`.

Parts: the hull (its outside, keel and rim), the inside (with the seats and ribs), the oars.

Afloat, its inside is kept dry of the water (`keepDry()`, environtments/water.ts). In an environment with water its preview floats, rising and falling with the waves and tipping with them; it starts in the forest lake.
