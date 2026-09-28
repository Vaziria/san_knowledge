# Fence Figure.

Draft written from the user's two reference images ("Spring - Forest Lake": the Fence asset, front, side and top, and the fences along the forest path and round the camp area), for the user to correct. One of the forest lake's assets (the forest_lake environment, [ForestLake.md](../../environtments/ForestLake/ForestLake.md)), the first built, which the user approved on 2026-09-26 before the rest; it stands along stretches of the forest path and round the camp. Refined on 2026-09-27 from the user's later reference sheets (the forest lake's objects in detail, and the forest lake in its four seasons, "Spring, Summer, Autumn, Winter"), which the user asked the forest lake to follow, in all four seasons.

A rustic wooden fence: chunky square posts standing in the ground, and two thick planks nailed across the posts' fronts, running well past the end posts. The planks of two sections meet halfway across the post between them. The posts' tops are cut off, with a slight slant. It is hand made: the posts lean a little, the planks sag and tilt a little, their edges are cut off so each catches the light, and the grain shows as dark lines split open along the wood. A dark nail head shows where each plank meets a post. A clump of broad grass blades grows round each post's foot, and now and then a tuft along the fence.

Look, as in the reference, not the project's theme: low poly with every face flat, bright and saturated. The wood is a warm orange-brown, the posts a little darker and more weathered than the planks; each face has its own shade of it, drifting a little lighter and darker along the grain; the cut-off edges are lighter; the grain lines are dark brown; the sawn ends, the posts' tops and the planks' ends, are pale end grain, darker toward the middle. The nails are dark iron. The grass is fresh green, lightest at the tips and darkest at the feet, each blade folded along its middle. Its colours are in the forest lake's palette ([parts.ts](parts.ts)).

Size: the posts stand 0.95 m over the ground, give or take 3 cm, 0.15 m square, and go 0.2 m into it. The planks are 0.14 m tall and 5.5 cm thick, their middles 0.38 m and 0.73 m over the ground; they run 0.185 m past the end posts' middles. By default three posts in a row, 1.15 m apart, as the asset sheet draws it.

Seasons: Its grass is the season's; in winter snow lies along its rails and on its posts' tops.

Options: `posts`, where the posts stand in order, (x, z), for a fence along a path, a section between each two; `groundAt`, the ground's height, so the posts stand on uneven land and the planks follow; `side`, which side of the posts the planks are nailed to; `tufts`, the grass (on by default); `season` (spring by default); `seed`, another fence of the same kind.

Parts: the posts, the rails (with their nails), and the tufts.

No animation behaviour: it stands still.
