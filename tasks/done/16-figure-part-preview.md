# Figures: preview a figure part by part

The user asked (2026-09-25): "in figures, figure can be preview by part". That line is the user's; the rest of this file was written by Claude from it. Choices marked **(default)** are decided, not open questions: build them as written unless the user has changed this file.

"In figures" is the panel's Figures tab. Read rules.md first, especially Project rules 4–6 (a figure's parts, and the preview) and Control panel rules 1–9. The tab's tree is `src/panel/figureTree.ts` and `SearchTree.tsx`; the preview is `src/previews.ts` and `Stage.show()`.

A figure with distinct parts already exposes each one as a `readonly` field whose origin is its pivot (Project rule 4): `penguin.head`, `penguin.leftWing`, a four-legged animal's `leftFrontLeg`, a tree's `trunk` and `crown`, a grass's `leaves` and `stems`, the boat's `hull` and `rudder`, the fish's `leftFin` and `rightFin`. This task lets you pick one of them and see it on its own.

## What changes

- **Parts in the tree (default):** a figure with parts can be opened in the Figures tree like a folder, and lists its parts under it.
  - Clicking the figure's row still picks the whole figure, as now. Its arrow, or the right and left arrow keys, open and close its parts.
  - A part is named from its field in the figures' own style, lowercase with dashes: `head`, `left-wing`, `left-front-leg`.
  - Each part the figure exposes is listed, both of a pair (`left-wing`, `right-wing`). A leg built of several groups (the frog's) is one part.
  - A figure with no parts, or only one (the cloud's `body`), stays a plain row as now. So do the pianos, unless their code has parts worth a look.
  - The tree lists the parts without building the figures: building every figure when the page loads would be slow. The names are type-checked against the figure's fields, so a renamed part can't be missed.
- **Search:** a part is found by its figure and its name: "bear head" finds the bear's head, and "head" every figure's head. A figure comes before its parts, so Enter on "bear" still picks the bear.
- **Picking a part shows only that part (default):**
  - The rest of the figure is hidden, and so are the parts attached to it: the bird's body holds its head, wings, tail and legs, which don't show with it. The speech bubble and anything held don't show either.
  - The part stays where it is on the figure, in the figure's resting pose, in the environment shown. It is not moved to the ground or the origin.
  - It stands still: the demo doesn't run, and the Behaviours tab says the behaviours are the whole figure's, to pick the figure to run them. Restart demo builds the part again.
- **The camera frames the part (default):**
  - The figure's own view looks at the part from the direction the figure's preview looks from, at the middle of the part's box, from the distance that fits the part.
  - The Camera tab's directions (`left`, `top`, …) go round the part as they go round a figure, and `walk` walks on where it is, as with a new figure.
  - Small parts (the frog's hind leg, the bird's leg, a few centimeters) are framed close without the camera cutting into them.
  - The sun's shadow fits the part (Checking the result, rule 5).
- **Kept in the URL:** `?part=<name>` next to `?figure=` (Control panel rules 3 and 5). A reload shows the same part.
  - Picking another figure, or the whole figure, drops the part.
  - An unknown part shows the whole figure and leaves the URL without it.
  - A story (`?story=`) shows its own scene as now, and the part is ignored.
- **Leaving a part** for its whole figure or another figure moves the camera to that figure's view, as picking a new figure does now.
- **Nothing else changes:** a figure picked whole looks and moves exactly as now, with its demo and behaviours. No part changes shape. Where a part isn't a field of its own (the frog's legs are lists of segment groups), the figure may gain a way to name it, without changing how it is built or moves.

Other sessions are changing `src/previews.ts` and `src/Stage.ts` (task 09, the dock). Change only the lines you need, and never restore any of them from a copy.

## Checks

- `npm run typecheck` in `typescripts/animation`.
- **Numeric checks,** in Node (Checking the result, rule 8): build every figure and check that
  - each part the tree lists is on the figure, and each readonly part field is listed;
  - with a part shown, nothing else of the figure is visible;
  - the part is in its resting pose and nothing is NaN;
  - a figure picked whole is exactly as before.
- **In a browser session** through the remote-debugging protocol (Control panel rule 6), on a spare port, never 8087:
  - open a figure's parts with a click and with the arrow keys, pick a part, and read back `aria-expanded`, `aria-selected` and the URL;
  - search "bear head", "head" and "bear", and press Enter;
  - load `?figure=penguin&part=left-wing`, a bad `?part=`, and a part with `?camera=top`;
  - from a part, pick the whole figure and another figure: the part leaves the URL and the camera goes to the figure's view;
  - no console exception.
- **Screenshots** with headless Edge: the penguin's head, the bear's left front leg, the oak's crown, the frog's hind leg (small), and the bird's body (without its head and wings). Take them with the Kuwahara filter on and off, and one in `studio`.
- Bring rules.md in line (Project rule 6 and Control panel rules 6 and 8: the tree's rows, what a part row is), add what you learned, then call `knowledge_sync`.
- Don't commit.

Taken: 2026-09-25 22:46

## Done

**What changed** (in `typescripts/animation`, nothing committed):
- `src/parts.ts` (new): each figure's parts, in tree order, and `partPreview(figure, part)`. 31 figures have 112 parts between them: the penguin, the eight animals, the boat, the fish, the rod, the trees, the grasses, and the cliff, log, mud pit and firefly (its 4 wings and 6 legs). The pianos, the cloud and the fog have none: the grand piano exposes only its lid and isn't in a preview, the MIDI keyboard exposes none, and the cloud and the fog are one part each.
  - The table is type-checked: every field a figure has beyond a `THREE.Group`'s that holds an Object3D, or a list of them, must be listed, as a part or as `false`. This caught the `speechBubble` getter on the animals and the penguin, which is listed as no part.
  - A list is split into equal runs, so the frog's `hindLegs` (a group per segment, the left leg first) makes `left-hind-leg` and `right-hind-leg`. Side −1 is left in every figure.
  - The part preview builds the figure's own preview and poses it as its first frame (`update(0, 0)`). Everything but the part is taken off every camera layer, and so are the parts inside it; lights stay. It has no demo and no behaviours. The camera looks from the figure's view direction, 2.7 × the radius of the ball round the part, a little right of its middle. `bounds` fits the shadow to the part.
- `settings.ts`: `part` (`?part=`); `setFigure(figure, part = null)`. An unknown part is taken out of the URL on load. `main.tsx` shows `partPreview()` when a part is picked. `Stage.ts` is unchanged.
- `panel/figureTree.ts`: parts are children of their figure, with the value `figure/part` (`treeValue`, `fromTreeValue`).
- `panel/SearchTree.tsx`:
  - A leaf can hold leaves of its own. Its row picks it, and its arrow (or → and ←) opens it. Parts have a puzzle icon.
  - The search now needs every word typed to be in the path, so "oak tree" finds `oak-tree` too. A leaf's own leaves also need a word in their own name.
  - Enter picks the first real match, not a figure shown only for its parts. Folder counts still count figures.
- `panel/Panel.tsx`: the Figures tab passes the part through, and the Behaviours tab has a note for a part. The terrain session (working at the same time) kept both, and `setFigure` also clears its `?terrain=`.
- rules.md: Project rule 6 (one part at a time), and Control panel rule 3 (`?part=`), rule 6 (the tree's rows) and rule 8 (`SearchTree` and the Figures tree). The state report `docs/states/control_panel.md` and its index are updated too.

**How it was checked:**
- `npm run typecheck` is clean, with the terrain session's work in the tree.
- **In Node,** every figure is built with each of its 112 parts. For each part:
  - exactly the part's objects show (less the parts inside it);
  - the pose equals the whole figure's first frame;
  - the camera is outside the part and past the near plane;
  - the part is inside the view at 1000×700 and 1280×720;
  - there are no behaviours and nothing is NaN.

  0 failures.
- **In headless Edge** through the remote-debugging protocol, on port 8131, 34 checks with 0 failures:
  - the arrow opens the penguin's 7 parts without picking anything;
  - a click on `head` writes `?part=head`, and the camera goes to the part's view;
  - the Behaviours note; the penguin's row drops `?part=` and brings the camera back to the penguin's view and its 7 behaviours;
  - ←, →, →, ↓ and Enter pick `body`, and ← goes up to the penguin;
  - the searches:
    - "bear head" gives Animals, bear, head, and Enter picks it;
    - "head" gives 8 heads, and Enter picks the cat's;
    - "bear" gives the bear alone, closed, and Enter picks it whole;
    - "front leg" and "oak tree" find what they should;
  - `?figure=penguin&part=left-wing` loads picked, with its figure open;
  - `?part=nope` leaves the URL and shows the whole penguin;
  - `?figure=bear&part=head&camera=top` looks straight down;
  - from the bear's head, picking the fox drops the part and goes to the fox's view;
  - no console exception.
- **Screenshots** (1200×800) of the penguin's head, the bear's left front leg, the oak's crown, the frog's left hind leg and the bird's body (without its head, wings, tail and legs), each with Kuwahara on and off, plus the penguin's head in `studio` and from the top. Each part shows alone in its place, left of the panel, casting only its own shadow.

**Left for the user:**
- The spec said Restart demo builds the part again. With a part shown, the Behaviours tab shows only its note, so it has no Restart button. `stage.restart()` still rebuilds the part, which the stream's hidden page relies on.
- A part keeps its place on the figure. So an animal's part is where the animal starts its demo (the bear's head is 1.6 m off to one side), and the oak's crown hangs in the air over its shadow. Picking a part moves the camera to it, so this is only visible in the shadow.
- The framing (2.7 × the part's radius) is a guess that fits every part at 1000×700; say if you want parts closer or farther.
