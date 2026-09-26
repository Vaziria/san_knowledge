# Animation development state

Where each area of `typescripts/animation` stands. This is the "Writing Summary Development State Report" step of the [development lifecycle](../../../../docs/development_lifecycle.md), kept as the user asked: one file per area, rewritten when the area changes ([rules.md, Project rule 8](../../../../docs/3d_modelling/rules.md#project)).

Updated 2026-09-26 by every animal rebuilt from the low poly model the user pasted into its spec, the penguin moved into the animals, the lion added and the fish made three kinds (the Animals, Objects and Lake meeting lines); before that by the wolf rebuilt from the user's low poly model (the Animals line), and the swamps at the lakeside (the Environments and Lake meeting lines); earlier that day the swamp terrain made darker, and felt made the only theme (the Environments, Objects and Control panel lines). All four were asked for directly, no task file. Before that by task 14, the host roams (the Lake meeting line), task 11, the floating logs, task 09, the dock, and the swamp terrain; first written by task 17, from the code as it was then. `npm run typecheck` is clean after the animals and the fish became their models.

| Area | State | Updated |
|---|---|---|
| [Lake meeting](lake_meeting.md) | The chat's viewers meet at the lake as animals that roam the land, the host too, with turns for the camera, the host's view following the host, a quiet-time follow, the fish's leaps (a salmon), a dock, floating logs and two swamps; the lion is a viewer's kind too. The spec is a draft. | 2026-09-26 |
| [Piano play](piano_play.md) | The MIDI keyboard on the grass, played by OSC chords and notes or by clicking its keys. No check since it was built on 2026-09-24. | 2026-09-25 |
| [Environments](environments.md) | Floor, grass and lake work; the lake has day and night, fog, showers, a border and a sky, swamps at its lakeside (the newest), and a dock and floating logs when built with them (the lake meeting's). Five terrains (grass, mud, water, sand and swamp, now dark) are built for environments to be made from; only the swamp is used, by the lake. | 2026-09-26 |
| [Trees](trees.md) | Eight seeded kinds, two meshes each, with autumn leaves as an option. Their specs are empty; they have no behaviours and stand still. | 2026-09-25 |
| [Grasses](grasses.md) | Seven sized, seeded grasses and herbs, two meshes each, coarser on request for the meadow. Draft specs, no behaviours. | 2026-09-25 |
| [Animals](animals.md) | Ten animals, the penguin one of them now and the lion new, each the user's own low poly model from its spec, with its spec's behaviours and a speech bubble. The four-legged ones plant their feet on time now, the body sinking as far as the legs need; the bear stands on all fours. The lion's and the frog's specs are partly drafts. | 2026-09-26 |
| [Objects](objects.md) | Cliff, cloud, dock, firefly, fog, log (round, to float, as an option) and mud pit (draft specs), plus the boat, the fishing rod and the fish, now three kinds from the user's models (salmon, piranha, clownfish), each with its preview. | 2026-09-26 |
| [Pianos](pianos.md) | The MIDI keyboard plays chords and notes from buttons, OSC and clicks. The grand piano is built but in no preview. | 2026-09-25 |
| [Control panel](control_panel.md) | Ten tabs, the Terrains tab the newest, the camera's directions and walk, and every setting in the URL. A figure can be previewed part by part (`?part=`, task 16). Felt is the only theme, so Settings has only the Kuwahara filter. | 2026-09-26 |
| [Streaming](streaming.md) | Streams to YouTube in three Render modes, on the GPU or not, and carries on through code changes; the Audio tab's sound; the chat reader for the lake meeting. | 2026-09-25 |

**Across areas:**
- Most of the work since commit 278ad67 (2026-09-24) is not committed, the whole lake meeting included. Each file's History says which.
- There are no kept tests: every numeric check was a script in its session's scratchpad.
