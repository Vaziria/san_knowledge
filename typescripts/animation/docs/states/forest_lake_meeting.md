# Forest lake meeting state

Written 2026-09-28, when the story was made (asked for directly: "make story ForestLake Metting like lake meeting"; no task file). It answers the open question the forest lake's animals left ("Should the animals come to the forest lake itself, roaming it as the lake meeting's animals do?").

## What works

The viewers of the YouTube live stream meet at the forest lake as its own animals, as they meet at the lake in the [lake meeting](lake_meeting.md). Spec: [forest_lake_meeting.md](../../src/story/forest_lake_meeting/forest_lake_meeting.md), a draft. Code: [forest_lake_meeting.ts](../../src/story/forest_lake_meeting/forest_lake_meeting.ts) and [ForestMeeting.ts](../../src/story/forest_lake_meeting/ForestMeeting.ts), listed in [stories.ts](../../src/stories.ts) after `lake_meeting`; `?story=forest_lake_meeting`.

- **It plays as the lake meeting plays, on its modules** in `src/story/lake_meeting/`: the roaming ([Roam.ts](../../src/story/lake_meeting/Roam.ts), [Land.ts](../../src/story/lake_meeting/Land.ts), [Member.ts](../../src/story/lake_meeting/Member.ts)), the host's view ([HostView.ts](../../src/story/lake_meeting/HostView.ts)), the quiet camera's follow ([Follow.ts](../../src/story/lake_meeting/Follow.ts)) and the fish's own leaps ([FishLeaps.ts](../../src/story/lake_meeting/FishLeaps.ts)). ForestMeeting.ts is Meeting.ts's turns and close-ups, copied, with the forest lake's cast, land, sight and fish: a fix to one of the two may be wanted in the other.
- **The dev server keeps it from the same chat as the lake meeting** ([meeting.ts](../../meeting.ts) with the forest's rules, [events.ts](../../src/story/forest_lake_meeting/events.ts); [chat-bridge.ts](../../chat-bridge.ts), [streaming.md](streaming.md)). Every chat message goes to both meetings; each has its own seven bots and its own viewers' animals. Its events are the Vite event `forest-meeting:event`, and a page asks `GET /__chat/forest-meeting` first ([chat.ts](../../src/chat.ts)). A built page, with no dev server, makes 7 bots of its own.
  - **Its kinds:** the forest lake's wolf, deer, fox, boar, rabbit, squirrel, duck, otter and frog ([forest_lake_animals.md](forest_lake_animals.md)); `!wolf` and the rest switch. A lake kind (`!cat`) is only a comment there.
  - **Its commands:** `!jump`, `!walk`, `!run` and `!stop`, as at the lake, and each kind's tricks, from its sheet's poses (`TRICKS`): `!howl` (a wolf), `!flap` (a duck), `!sit` and `!look` (a fox), `!look` (a deer), `!eat` (a boar or a rabbit), `!stand` (an otter), `!wave` (the host). The dev server sends a trick only for a kind that has it.
  - **The host is the explorer**, the channel owner's. The owner can't switch him.
- **Its season is the Environments tab's Season** (`?season=`, spring by default): the story gives its environment in each season (`Story.environmentIn`, used by [main.tsx](../../src/main.tsx)), and a new season builds it again, the host back on the landing and the others at new spots. The forest lake itself is in [forest_lake.md](forest_lake.md).
- **The cast** ([guests.ts](../../src/story/forest_lake_meeting/guests.ts)): the forest lake's animals and the explorer as their sheets made them, each in a small subclass that gives it what the meeting asks of a figure (`Figure` in [kinds.ts](../../src/story/lake_meeting/kinds.ts)); nothing in their own files changed.
  - A speech bubble just over its head, headed by the viewer's name, sized for its height as the lake's animals' are.
  - `Walk()` and `Run()` start it going only when it isn't already (the meeting asks every frame, and the frog's hop would start over each time); a jump or a flap under way ends first.
  - `Jump()` is a hop where it stands, so it never lands anywhere the meeting didn't put it: the four-legged ones spring straight up (their rig's leap, 18–40 cm), the frog leaps as its sheet has it, the duck flaps its wings, the explorer jumps.
  - Its colour is one of its sheet's variations, picked from the viewer's id, so the same on every page; its season's coat half the time where its sheet has one (snow, winter, autumn).
  - Standing at a spot, it now and then does one of its tricks rather than hop (`Pastime`, through `Roam`'s new pastime hook): the wolf howls, the fox sits or looks up, the boar and the rabbit eat, and so on. The squirrel and the frog only hop.
- **The land** ([ground.ts](../../src/story/forest_lake_meeting/ground.ts)): the land round the lake out to 39 m from its middle (the camp, the ruins, the gate and the cave within it), the piers' and bridges' decks and the cave's floor its ground (`ForestLake.landAt`), a shore pier's first 0.6 m eased up from the bank.
  - **Water:** the lake, the rivers and the stream, and their banks up to 12 cm over them (`Land`'s new `wet` option).
  - **Not the island:** only land the smallest animal can walk to from the landing counts (`Land`'s new `home`).
  - **What stands on it:** the trees' trunks and the rocks as circles (`Scatter.obstacles`), and the footprints of the rest found from their meshes: whatever of the landmarks, the cliffs, the bushes, the logs and the stumps is 6 cm to 1.3 m over the ground under it, each 25 cm square of it a circle. That takes in the tent, the fences, the lamps, the piers' posts, the bridges' rails, the ruins and the cave's rock.
  - **The two shore piers** are decks, as the lake meeting's dock is: one walk in ten may go out on one, for the animals that fit.
- **The cameras' sight** ([ForestSight.ts](../../src/story/forest_lake_meeting/ForestSight.ts), on the new `Seeing` interface in [Sight.ts](../../src/story/lake_meeting/Sight.ts)):
  - **The floor:** the ground mesh sampled every 25 cm, or the water now: the lake's waves (its ice in winter), the rivers and the stream, 8 cm more over water.
  - **What is solid:** cubes of 25 cm, kept as bits, from 1 m under the ground to 17 m over it, over a square 58 m each way round the lake's middle. A cube is marked where the landmarks, the trees, bushes, rocks, logs and stumps, or the cliffs pass through it. A path's pebbles, under 30 cm across, are left out.
- **The fish,** a trout, swims round 1.4 m from the open water's middle north of the landing (in winter the hole in the ice). For a supporter's leap it first turns to face its circle's middle, then leaps across the open water. The lake meeting's heights, 0.3–1.5 m by the Super Chat's tier, become 0.3–1 m, the most the forest fish leaps. Its own leaps (0.15–0.28 m, every 20–60 s) go the way it swims. The camera films its leap from the shore's side, nearer than the lake's, the trout being 40 cm long.
- **Turns** as at the lake. A trick's animal stops as its turn begins, so the camera finds it facing the way it chose to film it from, and its turn lasts the trick (2.6–6 s).
- **The Story tab** lists it (hinted "in the Season picked in Environments") and shows the chat controls under it, with its host and commands ([ChatControls.tsx](../../src/panel/ChatControls.tsx), [control_panel.md](control_panel.md)).

## How it was last checked

- 2026-09-28, when it was made:
  - `npm run typecheck`: clean.
  - **Node, the dev server's rules** (a script in the session's scratchpad): all 14 checks passed.
    - A wolf may `!howl`, not `!flap`; a duck may `!flap`; a fox may `!sit` and `!look`, not `!eat`.
    - The owner's explorer may `!wave`, not `!howl`, and can't be switched; `!cat` is a comment.
    - Rp 200.000 asks for a 0.8 m leap.
    - A saved meeting carries on after a restart; a saved seat of a kind it lacks becomes one of its kinds.
    - Quiet viewers walk away after 10 minutes and bots come back.
    - At the lake, a penguin still flaps, and `!howl` is only a comment there.
  - **Headless Edge on the GPU** (RTX 3050, 1280×720 as the stream, the painterly filter on; a test server on port 8151): made-up chat in spring and in winter, each screenshot once its turn began.
    - The turns: a viewer's comment in its bubble, a switch to a wolf, the wolf howling (from in front, once its animal stopped at its turn's start), the explorer waving and speaking, a Super Chat's leap followed across the water (in winter across the ice hole), a duck talking and flapping, a fox sitting, and the quiet camera following a duck side-on and from behind.
    - Every animal's room, sampled every half second over both runs, 538 samples: none stood more than 5 cm into anything or over water.
    - In winter the fish's leap took it at most about 2 m from the hole's middle (the open water is 3.2 m round it).
    - The season changed in the Environments tab while it played (autumn, winter, summer): each built it again with its animals, and the URL followed.
    - Building it adds about 0.8 s to the forest lake's own: the page's first long task 2.2–2.3 s, against 1.45 s for the forest lake alone. In all, about 3 s of long tasks, against 3.5 s for the lake meeting. Then 60 frames a second.
    - No page errors.

## Known issues and left for the user

- **The big animals can't cross the outlet's bridge or walk the piers.** The wolf, the deer and the boar take up half their length round them, more than a 1.7 m bridge or a 1.9 m pier leaves room for. The fox, otter, rabbit, squirrel, duck, frog and the explorer fit. A big one that lands past the bridge (on the east shore, by the cave) stays on that side.
- **Each page roams on its own**, as at the lake: only who is there and what they say is shared.
- **A new season starts it over:** the host back on the landing, the others at new spots.
- **The squirrel and the frog have no trick**; the boar's charge, the squirrel's climb and the duck's take-off would carry them where the meeting didn't send them, so they aren't used.
- **Building it takes a moment,** about 0.8 s more than the forest lake alone: the land's footprints and the cameras' sight are measured from the meshes.
- **No kept tests.** The checks were scripts in the session's scratchpad.

## Open questions

- [forest_lake_meeting.md](../../src/story/forest_lake_meeting/forest_lake_meeting.md) is Claude's draft; the user hasn't confirmed it.
- Is the explorer the host you want, rather than one of the animals?
- Claude's picks, yours to change: the tricks and their commands, how long each lasts, how often an animal does one of its own accord, the fish's circle, and how high a supporter's leap goes (0.3–1 m).
- Should the duck, the otter and the frog swim in the lake (their sheets have them swimming), or the butterflies and fireflies fly about?
- Should the animals walk out on the ice in winter?
- Should the big animals get over the bridge (a wider bridge, or a narrower footprint for them)?

## History

- 2026-09-28, asked for directly (no task file, not committed): the story built.
  - Its folder `src/story/forest_lake_meeting/`: the scene, the cast, the land, the sight, the events and the spec.
  - The lake meeting's modules opened to it:
    - `Figure` an interface;
    - `Member` handed its figure;
    - `Roam`'s pastime;
    - `Land`'s `wet`, `home` and obstacle buckets;
    - `Seeing`;
    - `FishLeaps` taking any fish that leaps;
    - the events' types generic, with `MeetingRules`.
  - The dev server's `Meeting` taking rules, and the chat bridge keeping both meetings (handover 2, which takes 1).
  - `onMeeting`/`fetchMeeting` for either story, `Story.environmentIn`, and the Story tab's chat controls for both.
  - On the forest lake: `ForestLake.landAt`, `Scatter.solid` and `low`, `Landmarks.solid` and `piers`.
