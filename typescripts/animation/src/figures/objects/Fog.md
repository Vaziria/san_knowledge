# Fog Figure.

Draft written from the code, for the user to correct.

A bank of low-lying fog standing in the scene (not the scene's own fog): many soft, see-through puffs layered together, thicker in the middle and near the ground, thinning out toward its edges and its top. Its colour is the theme's background, lightened toward the theme's light colour.

Size: about 8 m wide, 1.5 m tall and 5 m deep by default. It takes its size in meters, `new Fog({ width, height, depth })`, and its puffs stay inside that box. `seed` builds another bank of the same kind.

No animation behaviour. On its own it drifts and swirls slowly, for ever: the puffs drift sideways, circle, and swell and thin as they go (`fog.update(delta)` every frame moves it on).
