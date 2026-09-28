# Explorer Figure.

Draft written from the user's character sheet for the explorer (his front, three-quarter, side, back and three-quarter back views, a face close-up, the colour palette, the accessories, six expressions, the optional tools and an animation preview), for the user to correct. One of the forest lake's figures ([ForestLake.md](../../../environtments/ForestLake/ForestLake.md)), kept with its animals, built as the sheet draws him, not in the project's older style: his colours are the sheet's own, not the theme's.

A young explorer boy of about ten, in chibi proportions: a big head, a third of his height. Messy brown hair in big pointed locks: a fringe hanging to the brows, locks flaring at the sides and down to the nape, and a tuft on the crown. Big brown eyes with a thick dark line over each and a white glint, a small nose, a smile and rosy cheeks, round ears. A cream shirt with short, puffy sleeves rolled at the cuffs, its hem dipping to a point in front. A thick green scarf rolled round his neck, falling to a point on his chest, a gold button on it; behind, it hangs down over his backpack as a short cape with a pale leaf on it and two little leaves at its corners. A brown belt with a gold buckle, a leather pouch on it at his left and a small one with a glowing leaf at his right. Dark baggy shorts with pockets, bare shins, brown boots with thick turned-down cuffs and a gold buckle, and leather bracers. A brown backpack with side pockets and a cream bedroll strapped on top, its straps over his shoulders, and a small lantern glowing on a hook at its right side. Flat facets in the sheet's colours.

Size: 1.2 m tall to the tip of his hair's tuft, a ten-year-old's height; his head is 39 cm of it. His hips are 39 cm up, his shoulders 70 cm.

Colour variations, as the sheet shows them: one, `default` (the sheet's Color Palette: the scarf's greens, the shirt's cream, the leather's browns, the skin's salmon and the buckles' gold).

Expressions, as the sheet shows them: `happy` (the default), `laugh`, `surprised`, `angry`, `wink` and `smirk`.

Tools, as the sheet shows them: a wooden `sword`, a `fishing-rod` with a red and white float, a `pickaxe`, an `axe` and the `lantern`.

Parts: the head (with the hair and the face), the body (the shirt, scarf, cape, belt and shorts), the two arms, the two legs, the backpack and the lantern.

## Animation Behavior.
1. `Idle()`

    Stands, breathing, blinking, now and then looking to one side.

    Example:
    `explorer.Idle()`
2. `Walk()`

    Walks forward the way he faces until told otherwise, swinging his arms.

    Example:
    `explorer.Walk()`
3. `Run()`

    Runs forward the way he faces until told otherwise, leaning into it, his fists pumping.

    Example:
    `explorer.Run()`
4. `Jump()`

    Crouches and jumps 35 cm straight up, his arms flung wide and one knee drawn up, lands and stands again.

    Example:
    `explorer.Jump()`
5. `Interact()`

    Steps forward, kneels on one knee, reaches down and touches something low in front of him (a sapling), then stands up again.

    Example:
    `explorer.Interact()`
6. `Wave()`

    Raises a hand and waves it, then puts it down.

    Example:
    `explorer.Wave()`
7. `Sit()`

    Sits down, as on a log 17 cm high behind him, his hands on his knees, looking about, until told otherwise; then stands up first.

    Example:
    `explorer.Sit()`
8. `SetExpression(name string)`

    Shows one of the sheet's expressions: `happy`, `laugh`, `surprised`, `angry`, `wink` or `smirk`.

    Example:
    `explorer.SetExpression("laugh")`
9. `Hold(tool string)`

    Takes one of the sheet's tools in his left hand: `sword`, `fishing-rod`, `pickaxe`, `axe` or `lantern` (the one from his side); `nothing` puts it away.

    Example:
    `explorer.Hold("fishing-rod")`
10. `SetColor(variation string)`

    Paints him in one of the sheet's colour variations.

    Example:
    `explorer.SetColor("default")`
