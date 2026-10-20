---
domain: Game Development
keyword: blender, game asset, low poly, keyframe animation, rigid body, collision shape, mirror modifier, torii
---
# Blender Basics for Game Assets

Summary of a chat session (2026-10-16 to 2026-10-17) where the user started learning Blender by hand to make game assets. Checked against Blender 5.2.2 LTS. The lessons ran hands-on: the user worked in their open Blender, said "done", and Claude checked the scene through the Blender MCP. Every step was explained as what it is, why, where it is in the menus (shortcut as an extra), and what you should see; the user asked for that instead of bare shortcut keys.

Practice files: `D:\blender_project\game.blend` (ground, cube, stone, the torii pillar `Tiang`); `D:\blender_project\assets\origami manual.blend` was left untouched.

## The screen

| Area | Where | What it is for |
|---|---|---|
| 3D Viewport | middle | The scene; its header has the View, Select, Add, Object menus |
| Outliner | top right | List of every object; double-click a name to rename it |
| Properties | bottom right | Settings of the selected object, in tabs down its left edge (wrench = Modifiers, blue circle with orbit = Physics, red ball = Material, cone and sphere = Scene) |
| Timeline | bottom | Frames of the animation, playhead, Start/End, play buttons |
| Sidebar | right edge of the 3D view, **View → Sidebar** (N) | Exact numbers: Item tab has Location, Rotation, Scale, Dimensions; View tab has the 3D cursor |
| Adjust Last Operation | bottom-left of the 3D view, after an action | Change the last action's settings (plane size, cylinder sides, cuts); it disappears after the next action |

Moving the view: middle mouse drag orbits, Shift + middle mouse pans, scroll zooms, Numpad . frames the selection, Numpad 1/3/7 is front/side/top. Without a middle mouse or numpad, drag the axis gizmo at the top right, or turn on *Edit → Preferences → Input → Emulate 3 Button Mouse / Emulate Numpad*.

Moving objects: **Object → Transform → Move / Rotate / Scale** (G / R / S), then X, Y or Z locks to one axis, a typed number gives an exact amount, Enter or left-click confirms, right-click or Esc cancels. Undo is **Edit → Undo** (Ctrl + Z).

## Core ideas

- **Axes:** X is left–right (red line), Y is front–back (green line), Z is up. 1 Blender unit = 1 metre.
- **Origin:** the orange dot of an object. "Location" is where the origin is, not where the shape is. A 2 m cube with its origin in the middle needs Location Z = 1 to stand on the ground.
- **Object Mode vs Edit Mode:** the mode dropdown at the top-left of the 3D view (Tab). Object Mode moves the whole object with its origin; Edit Mode changes the shape (vertices, edges, faces; 1 / 2 / 3 on the top number row) and leaves the origin where it is.
- **3D cursor:** the red-and-white circle. New objects, and their origins, appear there. **Object → Snap → Cursor to World Origin** (Shift + S) puts it in the centre; Shift + right-click places it on a spot.
- **Apply scale:** **Object → Apply → Scale** (Ctrl + A) bakes the stretch into the shape so Scale reads 1, 1, 1. Games, physics and exact Edit Mode numbers all need it. Forgetting it is the most common game-asset mistake.
- **Modifiers:** non-destructive effects stacked on an object (Properties → Modifiers tab → Add Modifier). The real shape is untouched until you Apply.

## What makes a game asset different

| Rule | Why |
|---|---|
| Low poly count | The game draws everything about 60 times a second |
| Real size and applied scale | It arrives in the engine at the right size |
| UV unwrap | The engine needs to know how to wrap a flat image round the model |
| Image textures, not procedural | Procedural node materials only work inside Blender; for a game they are baked into an image |
| Export as glTF (`.glb`) | Godot, Unity and Unreal all import it |
| Clear names | "Icosphere.003" means nothing among hundreds of objects, and names travel with the export |
| One asset per `.blend` file | Easy to export, reuse and fix |

## Procedural wood material

Built on a 2 × 6.3 × 0.78 m plank as a first example (`Wood_Procedural`, in a file that was not saved). The idea: build a virtual tree trunk of rings and let the object cut a board out of it, so the ends show rings and the top shows long grain.

Node chain: Texture Coordinate (Object) → Mapping (scale = object scale × (1, 0.08, 1), rotation X 4°, location (0.3, 0, -0.25)) → Wave Texture (Rings, **rings direction Y**, scale 4, distortion 2, detail 2) plus Noise Texture (scale 80, pores) mixed by Math Multiply Add (pores × 0.2 + rings) → Color Ramp (dark 0.08/0.035/0.012 → light 0.45/0.24/0.09) to Base Color, Map Range (0.45–0.6) to Roughness, Bump (strength 0.1) to Normal.

- The Wave Texture's rings direction defaults to X, which makes the grain run across the board; Y runs it along the length.
- Multiplying the Mapping scale by the object scale stops a non-uniformly scaled object from stretching the grain.
- A slight Mapping tilt makes the grain drift along the board; too much turns it into cross stripes.
- For a game this material has to be baked into an image texture.

## Ground surface

1. **Add → Mesh → Plane**, Size 20 in the Adjust Last Operation panel (or scale it and apply the scale).
2. A plane has only 4 corners, so it cannot bend: in Edit Mode, **Select → All**, right-click → **Subdivide**, Number of Cuts 20 (about 1 m squares, light enough for a game).
3. Hills: vertex select, click one point, turn on **Proportional Editing** (O) so nearby points follow softly, then Move along Z. The scroll wheel sets the influence circle; a sharp spike means it is too small.
4. **Object → Shade Smooth** removes the faceted look.

Next for the ground: a repeating grass/dirt texture, export, and collision in the engine.

## Keyframe animation

- An animation is frames shown quickly; Blender plays 24 frames per second (frame 120 = 5 s). The Timeline's playhead is the current frame; set End to the last frame so it loops.
- A **keyframe** saves a value at one frame; Blender calculates the frames in between. **Object → Animation → Insert Keyframe** (I, mouse over the 3D view) saves location, rotation and scale.
- Field colours in the Sidebar: **yellow** = keyframe on this frame, **orange** = changed but not keyed (lost if you change frame; the most common beginner trap), **green** = animated, value calculated from keyframes.
- Blender eases in and out at each keyframe by default; game-style constant speed needs Linear interpolation.
- Exercise used: keys at frames 1, 60 (X + 8 m) and 120 (Y + 8 m), plus a jump key at frame 30 (Z + 2 m).

## Physics: Rigid Body

**Nothing is automatic.** Objects have no gravity and no collision until you add them, so they can overlap and keyframes can push them through each other.

- **Physics** tab → **Rigid Body** (or **Object → Rigid Body → Add Active / Add Passive**). Not the **Collision** button, which is for cloth and particles.
- **Active:** moved by gravity and collisions (falling cube, loose pebble). **Passive:** solid but never moves, and does not fall (ground, big rock, wall).
- The first rigid body creates the **Rigid Body World** (Scene tab), which holds gravity (−9.81 m/s²).
- Play from frame 1; the result is cached, so after a change go back to frame 1 and play again.
- **Physics and keyframes fight:** an Active body ignores its keyframes. A keyframed obstacle that should push things is **Passive + Animated** (Rigid Body → Settings → Animated).

### Which objects get a rigid body

Only objects something must touch. Ask: does anything need to hit it, land on it, or does it fall? No → nothing (grass, distant mountains, clouds, camera, lights). Yes → should physics move it? Yes = Active, no = Passive. Each rigid body costs calculation time every frame.

### Collision shapes

The physics does not use the visible model; it uses a simpler invisible **collision shape** (like a phone's case: the case is what touches the table). The shape decides how the object bumps and rolls, and how fast the simulation runs.

| Shape | Speed | Good for |
|---|---|---|
| Box | fastest | cubes, crates, walls |
| Sphere | fastest | balls |
| Capsule | fast | characters (slides over bumps) |
| Cylinder / Cone | fast | barrels, poles |
| Convex Hull | medium | stones, odd shapes; shrink-wrap that fills dents |
| Mesh | slowest | ground and terrain; exact surface, only for objects that don't move |

- **Convex** means no dents or holes. A Convex Hull fills dents, so a ball can never fall into a cup with that shape.
- Mesh on Active objects jitters, sinks or sticks.
- For the practice scene: cube Box, stone Convex Hull, ground Mesh (Convex Hull, Blender's default, works while the ground is flat).
- Experiment: set the cube's shape to Sphere and play; it rolls and sinks like a ball.

### Physics in a game engine

Blender's rigid body physics is not exported. The engine runs physics live, but you still tell it manually which objects are solid and how, in the engine:

| Blender | Godot |
|---|---|
| Active rigid body | RigidBody3D |
| Passive rigid body | StaticBody3D |
| Collision shape | CollisionShape3D (Box, Sphere, Capsule, Convex, Concave/Trimesh for static only) |
| player moved by keys | CharacterBody3D + a script |

Name tricks that create collision on import: Godot `-col` suffix (StaticBody3D with collision), `-rigid` (RigidBody3D); Unity "Generate Colliders" or a Mesh Collider; Unreal auto collision or a `UCX_` shape. Games give collision only to what can be touched, often simplified (a tree gets a capsule on the trunk only).

The user's engine is not confirmed yet (possibly Godot).

## Low-poly stone

1. **Add → Mesh → Ico Sphere** (even triangles, no pinched poles, better for rocks than a UV Sphere), Subdivisions 2 (80 faces), Radius 0.5.
2. Scale it wider than tall (e.g. 1.4 × 1.0 × 0.7), then **Apply → Scale**.
3. Edit Mode, **Select → All**, **Mesh → Transform → Randomize**, Amount about 0.1; Random Seed gives a different stone.
4. Flatten the bottom: front view, **X-Ray** on (Alt + Z) to select hidden points, box-select the bottom row, Scale Z 0.
5. **Shade Flat** (crisp low-poly) or **Shade Smooth** (realistic), put it on the ground, name it `Stone`, give it a grey material with Roughness about 0.8.
6. Rigid body only if something should hit it: Passive, Convex Hull. A Passive object does not fall, so place it on the ground first.

## Mirror modifier

Good for anything symmetrical, such as a torii gate: model one half, Blender shows the other. Both sides always match, edits update live, and later both halves can share texture space.

- **Add:** Properties → Modifiers → **Add Modifier → Generate → Mirror**. Axis X = left ↔ right, Y = front ↔ back, Z = up ↔ down.
- **The rule:** it flips around the object's **origin**. With the origin in the middle of the shape the copy lands on top of the original and nothing seems to happen.
- **Move the shape, not the object:** in Edit Mode, **Select → All** and Move; the copy moves the opposite way. In Object Mode the origin moves too, so the gap never changes.
- **Clipping:** points cannot cross the centre line, so halves touch but never overlap. **Merge:** joins the points that meet at the centre into one piece. Both matter for parts that cross the middle (beams).
- **Header buttons:** monitor icon shows or hides the copy; the down-arrow menu → **Apply** makes it real geometry (edits stop mirroring); X deletes the modifier.

### Changing the centre line

1. **Origin to 3D cursor** (recommended): put the cursor where the line should be (**Snap → Cursor to World Origin**, Shift + right-click, or Sidebar → View → 3D Cursor), then **Object → Set Origin → Origin to 3D Cursor**. Only the dot moves.
2. **Move the origin by hand:** 3D view header **Options → Transform → Affect Only: Origins**, then Move. Untick it afterwards, or nothing will move normally.
3. **Mirror Object:** add **Add → Empty → Plain Axes** (an invisible helper) and pick it in the modifier's Mirror Object field; the line follows the Empty. Fine for experiments; for a game asset the origin way is cleaner.

### Setting the distance

Distance between the two halves = 2 × the shape's distance from the centre line. Apply the scale first, then in Edit Mode select all; the Sidebar's Item tab shows **Median** (the middle of the selection). Switch it to **Global** and type X: -1.5 puts the pillar 1.5 m left and the copy 1.5 m right, 3 m centre to centre.

## Torii gate plan

Parts: **hashira** (two pillars, slightly thinner at the top), **kamebara** (black base), **nuki** (lower beam through the pillars), **gakuzuka** (centre post or name plaque), **shimaki** (beam under the top), **kasagi** (top beam with ends curving up). Colours: vermilion body, black top of the kasagi and black bases.

Mirror covers every part: one pillar and base make two; half of each beam is completed across the centre with Clipping and Merge; the centre post's halves join.

Planned steps: file and first pillar with Mirror (a 12-sided cylinder, radius 0.2, depth 4, moved in Edit Mode to X 1.5 and Z 2) → pillar taper and base → nuki → kasagi and shimaki → gakuzuka → colours, names and export clean-up.

Where the user stands (2026-10-17): pillar `Tiang` modelled with a wider top and base, but its scale is not applied (0.48 × 0.57 × 0.16), the Mirror was removed, its origin sits in the middle of the pillar, and it floats 1.745 m above the ground. Next: apply scale, origin to world centre, Mirror again, Median X -1.5, move down 1.745 m.
