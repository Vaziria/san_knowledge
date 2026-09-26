# Lake meeting state

Updated 2026-09-26 by every animal rebuilt from the model in its spec (asked for directly, no task file): the lion a new kind, the penguin an animal now, the host bear on all fours, the fish a salmon. Earlier that night by the swamps at the lakeside (the Scene line); before that by task 14 (the host roams), task 11 (the floating logs) and task 09 (the dock); first written by task 17, from the code as it was then.

## What works

The viewers of the YouTube live stream meet at the lake as animals. Spec: [lake_meeting.md](../../src/story/lake_meeting/lake_meeting.md), a draft. Rules: [rules.md, Stories](../../../../docs/3d_modelling/rules.md#stories), rules 10–22. Code: [lake_meeting.ts](../../src/story/lake_meeting/lake_meeting.ts) and [Meeting.ts](../../src/story/lake_meeting/Meeting.ts), listed in [stories.ts](../../src/stories.ts) after `piano_play`.

- **The dev server keeps who is there** ([meeting.ts](../../meeting.ts), plain logic with the clock and the random numbers passed in). The chat reader that feeds it is in [streaming.md](streaming.md).
  - A viewer's first message brings their animal, of a random kind of the nine in [events.ts](../../src/story/lake_meeting/events.ts) (the frog and, since 2026-09-26, the lion too). `!cat`, `!frog`, `!lion` and the rest switch it; the host can't switch. Every kind is an [animal](animals.md) now, the penguin too.
  - A message is a comment, cut to 300 characters, or a command: `!jump`, `!walk`, `!run`, `!stop`, or `!flap`, which only a penguin gets. Words after a command are said too.
  - The channel owner's messages go to the host, the Bear.
  - At most 12 viewers' animals: the next one destroys the one that joined first. A viewer quiet for 10 minutes walks away.
  - Bots of random kinds fill the meeting up to 7 animals. A viewer takes a bot's place, and a bot comes back when a viewer leaves. Bots never talk.
  - Supporters make the fish leap: a Super Chat or Super Sticker 0.3–1.5 m by YouTube's dollar tiers (rough rates; an unknown currency counts as dollars), a new member 0.6 m, a member milestone 0.45 m, gifted memberships 0.8 m. A Super Chat's text is said after the leap.
  - A deleted message is taken back. A viewer a moderator removes is gone at once.
- **Every page gets the same events**, numbered (the Vite event `meeting:event`, through [chat.ts](../../src/chat.ts)). A page that opens asks `GET /__chat/meeting` first and applies only later events. A dev server restart hands the meeting to the new code (`Meeting.save()`). A built page, with no dev server, makes 7 bots of its own.
- **The scene:** the lake built with a level, mown clearing 5.6 m round the landing ([rules.md, Environments](../../../../docs/3d_modelling/rules.md#environments) rule 20). The host ([Bear.md](../../src/figures/animals/Bear.md), on all fours now, as its model is) starts on the landing, where the opening shot shows it, and roams from there like the others; it never leaves the meeting. The fish ([Fish.md](../../src/figures/Fish/Fish.md)), a salmon, the fish's default kind, swims a square loop in the water.
  - **A dock** ([Dock.md](../../src/figures/objects/Dock.md), [DockSite.ts](../../src/environtments/Lake/DockSite.ts), `DOCK` in [Meeting.ts](../../src/story/lake_meeting/Meeting.ts); task 09).
    - **Where:** it runs 6.5 m out from the far right bank (−50°, turned 14° to fit between the boulders). It keeps 4.6 m from the fish's loop and clear of the corridors its leaps can carry it along.
    - **What it is:** 3 m wide, with a 1.2 m ramp at its landward end. Only the meeting's lake has it.
    - **Its site:** 15 stones and about 100 grass plants were cleared from it. Nothing else on the lake moved.
  - **Four logs float in the water** (`LOGS` in [Meeting.ts](../../src/story/lake_meeting/Meeting.ts), [Logs.ts](../../src/environtments/Lake/Logs.ts); task 11, Stories rule 22): 1.5–2.7 m long, three in the far left water across from the landing and one off the east bank. Only the meeting's lake has them.
    - They ride the waves 45% of their thickness down, rocking more in wind, and drift up to 0.5 m downwind and back, each in its own stretch of water.
    - They keep clear of the shore, the dock, the boulder in the west water, the fish's loop and its leaps, and each other.
    - They are solid to the meeting's cameras (`Sight.blocked()`).
  - **Two swamps at the lakeside** ([Swamps.ts](../../src/environtments/Lake/Swamps.ts); [environments.md](environments.md)): on the far shore to the southwest and right of the landing, 25 and 13 m from it. The clearing leaves no room for the one the default lake has left of the landing. Their pools lie under 12 cm, so the animals take them for water and walk round them (`WET` in Land.ts). Nothing else on the lake moved.
- **Every animal roams the lake's land, the host too** ([Roam.ts](../../src/story/lake_meeting/Roam.ts), [Land.ts](../../src/story/lake_meeting/Land.ts), [Member.ts](../../src/story/lake_meeting/Member.ts); Stories rule 17). It stands 3–15 s, sometimes jumps (a penguin may flap), then walks, or runs one time in five, to a spot within 12 m, or now and then anywhere on the land.
  - Its way goes round the water, the trunks, boulders, big stones and the other animals, off steep ground and short of the border.
  - **Out on the dock:** one spot in ten is out on the dock, and every kind, the deer and the wolf too, can walk out on it.
    - Its deck is ground to walk on (`Lake.deckAt`). Over its first 0.3 m the height eases from the land's to the deck's, so there is no step.
    - On the deck the room is measured exactly (`Land.ts`). A diagonal step from one deck cell to another is allowed, since the deck's room is a rectangle.
  - It keeps each spot, facing the way it came, and never goes back to its first place or its last 40 stands.
  - A new animal lands at a random spot with a jump. One that leaves walks to the land's end and runs the last part. One destroyed or removed goes at once; destroyed with a turn still waiting, after that turn.
  - A kind switch puts the new figure where the old one stood, moved up to 2.5 m when a bigger kind doesn't fit.
- **Turns, so nothing is missed:** comments, commands, kind switches and supporters' leaps wait their turn, in the order sent. The camera goes to the animal wherever it is (a 1.2 s glide, or a cut with more than 2 waiting or beyond 12 m). Then the bubble pops up, headed by the viewer's name, or the animal does the command.
  - The close-up comes from the first side with nothing in the way (trunk, boulder, ground, another animal, and since task 14 a crown or a stone, the camera in none of them: `Sight.ts`), never so far that the words are under about 24 px at 720p. An animal stops walking while it talks.
  - `!walk` and `!run` send an animal to a new spot near it, filmed from beside it. `!stop` holds it 4–10 s. The owner's commands do the same for the host. An animal facing another standing close in front of it waits for it to go (15 s at most), and of two waiting for each other the one not facing the other goes round (task 14: before, one could creep into the other).
  - A turn claims the camera back from a viewer who moved it, once they let go (Stories rule 10).
  - A supporter's leap is filmed from the shore's side of the fish, following the leap.
- **When the chat is quiet:** after 3.5 s the camera goes back to the host, wherever it is, and follows it as it roams, about as wide as the opening shot, from in front of it or the first clear side, never in or through anything ([HostView.ts](../../src/story/lake_meeting/HostView.ts); Stories rule 21). After 30 s the follow takes over, its first animal never the host. Then it follows a random animal (the host and the bots too, never the same one twice running) for 30 s each, in about four cut shots of 6–9 s from different angles, never through the ground, the water, a tree, a rock, the dock, a floating log or an animal ([Follow.ts](../../src/story/lake_meeting/Follow.ts), [Sight.ts](../../src/story/lake_meeting/Sight.ts); Stories rule 19).
- **The fish leaps of its own accord** every 20–60 s, 0.15–0.28 m, lower than any supporter's, with no turn and no camera. None while a supporter's leap waits or is under way, and none within 20 s after one ([FishLeaps.ts](../../src/story/lake_meeting/FishLeaps.ts); Stories rule 18).
- **The sun's shadow follows the view** (`followShadow`), so an animal far out keeps its shadow.
- **The Story tab's chat controls** ([ChatControls.tsx](../../src/panel/ChatControls.tsx), [control_panel.md](control_panel.md)): the channel to read, and made-up messages as a viewer, the owner, a Super Chat (Rp 200.000, a 0.8 m leap) or a new member, through `POST /__chat/say` ([chat-bridge.ts](../../chat-bridge.ts)).

## How it was last checked

- 2026-09-26, [task 11](../../../../tasks/done/11-lake-meeting-logs.md), the floating logs: typecheck clean at 00:26.
  - **The lake:** built with the dock alone and with the logs too, the stones, boulders, mud pits, trees, grass and deck were exactly the same.
  - **A 30 min run with 8 viewers and the bots**, the weather's own wind (over 0.5 in 24% of the frames):
    - every part of every log stayed over the water: at least 49 cm off the bed, 2.1 m from the shoreline, 1.8 m from the dock, 1.4 m from a boulder, 3.3 m from the fish's loop, never over where the fish goes, and 1.29 m from another log;
    - 45% of its thickness under the water on average, at most 0.37 cm of height and 0.39° of tilt in a frame, rocking more in wind;
    - they strayed up to 0.48 m from home, at up to 3 cm/s;
    - nothing was NaN; the stage's camera was never inside a log, and no log came between it and the animal followed.
  - **Headless Edge on the GPU (port 8142):** the opening shot calm, in wind and at night; logs close up at the waterline in felt and studio, filter off and on, by day and at night.
- 2026-09-26, [task 14](../../../../tasks/done/14-lake-meeting-host-roams.md): typecheck clean. Node, six runs of 30 min (7 bots, 3–4 viewers, 21–28 owner's turns): the host reached 49–71 spots a run all round the lake, never in the water, a solid, another animal or back at the landing; every walk started within 6 mm of the last one's end; in the host's view the camera was never in anything or under the ground and the host always in the picture; the follow never began with the host. The other animals: one footprint overlap of 4 cm in the six runs. Headless Edge on the GPU, day and night, Kuwahara on: the host leaving the landing, its view following it, an owner's comment 12–20 m out, and `!run`; no page errors.
- 2026-09-25, [task 09](../../../../tasks/done/09-lake-meeting-dock.md): typecheck (clean at 23:50).
  - **The lake:** built without and with the dock, the stones left, the boulders, mud pits and trees were exactly where they were.
  - **Access:** a way out onto the deck was found for all nine kinds.
  - **A 30 min run with 8 viewers of every kind:**
    - 14 walks out on the dock (the wolf, the deer and the host among them);
    - the animals' feet were exactly on the deck;
    - the most any animal's height changed in one frame, stepping on or off, was 0.22 cm;
    - no animal was ever over open water, and nothing was NaN.
  - **The quiet camera, 10 min with the dock among the solids:** in 2850 raycast checks it was never in the ground, the water, a solid or an animal, and nothing came between it and the animal.
  - **Headless Edge on the GPU:** the dock's preview in felt and studio, by day and at night; the walking camera 1.6 m over the deck; a deer out on the dock at the meeting.
- 2026-09-25, task 17: `npm run typecheck` is clean at 23:10, with other sessions' unfinished work in the tree (earlier that evening it failed for a few minutes on the dock's half-built lines, task 09). In Node with a fake clock, `meeting.ts` started with 7 bots, a viewer took a bot's place, `!flap` from a non-penguin was dropped, the owner's `!fox` did nothing, 14 viewers left 12 (two destroyed), 11 minutes of quiet brought the 7 bots back, and Rp 200.000 gave a 0.8 m leap.
- 2026-09-25, [task 08](../../../../tasks/done/08-lake-meeting-idle-camera.md), the latest browser check: typecheck. Node, 10 min with 7 bots and 4 viewers: the follow began at 30.0 s, 20 animals of 30 s, four shots each, and in 2850 raycast checks nothing came between the camera and the animal. Headless Edge on the GPU: angles on the bear and a bird, by day and night, with the Kuwahara filter; a test comment took the camera at once; a drag kept it until the next comment. Since then only the dock session has changed the story's code.
- 2026-09-25, [task 13](../../../../tasks/done/13-lake-meeting-fish-leaps.md): typecheck. Node, four runs of 30 min: 40–42 leaps of its own a run, 0.15–0.28 m, none near a supporter's. A leap caught in the air in headless Edge.
- 2026-09-25, [task 12](../../../../tasks/done/12-lake-meeting-keep-position.md): Node, six runs of 30 min: every walk started where the last ended, and the host never left the landing. A real page: 7 bots made 12 walks in 90 s, none back to an earlier stand.
- 2026-09-25, [task 07](../../../../tasks/done/07-lake-meeting-roaming.md): Node, twelve runs of 30 min: never in the water, at least 0.38 m from any obstacle, bodies never touched. Headless Edge by day and night: comments far out read, with shadows. 60 frames a second with 12 viewers, against 37 before.
- 2026-09-25, [task 10](../../../../tasks/done/10-frog-figure.md): on the GPU at 1280×720, a test viewer's `!frog` and comment; the bubble read at about 20 px.
- 2026-09-25, [task 05](../../../../tasks/done/05-stream-through-code-changes.md): a save of `meeting.ts` while streaming and reading a live chat; the viewers' animals and the event numbers carried on.

## Known issues and left for the user

- **Not in git.** `src/story/lake_meeting/`, `meeting.ts`, `chat-bridge.ts`, `src/chat.ts` and `ChatControls.tsx` are untracked: no commit holds the meeting.
- **Each page roams on its own.** Only who is there and what they say is shared. Each page picks its own spots, ways and followed animals with its own random numbers, so the panel's page and the stream's hidden page show the same animals in different places. Task 07 noted it for the first places; the code does it for all roaming. Only the stream matters to viewers.
- **A turn that follows the host walking or running** keeps its side and can pass a crown or a trunk: 0–24 frames a run with the camera in one and 0–24 with the host hidden, of 2,200–3,900 frames of the host's turns (task 14). Any animal's does the same.
- **The fish's own leaps are hardly seen.** From the opening shot its loop is some 16 m off, behind where the host starts, and lost in fog; the host's view goes wherever the host is. The quiet camera never follows the fish.
- **A supporter's leap starts wherever the fish is,** and can carry it 2–3.6 m off its loop for 15–23 s, onto the lake bed by the shore. Still so in the code (`pendingLeap` in `Meeting.update`).
- **Small in the picture:** the frog seen from high and the wolf from behind sometimes fill under 30% of the frame.
- **The quiet camera with the dock in:** a run of 10 min with the dock and the roaming host had one follow cut short at 22 s, 7 of 79 shots under 6 s, the animal's middle out of the picture in 4 frames, and one 50 ms frame of `follow.update`. The run without the dock had none of these.
- **After a supporter's leap the fish can wander** under the dock and through its posts, under the water. Its leaps themselves keep clear.
- **No kept tests.** Every numeric check was a script in its session's scratchpad. The project has no test files and no test script, so none can be run again.
- **The spec and the code differ.** The code also leaps for a member milestone and for gifted memberships, which item 7 doesn't name. The code wins.

## Open questions

- [lake_meeting.md](../../src/story/lake_meeting/lake_meeting.md) is Claude's draft; the user hasn't confirmed it.
- Task 08 left these for the user to adjust: the angles, the shot lengths (6–9 s), the framing (36–46% of the picture), and cutting rather than gliding.
- Should the fish's own leaps be seen? Task 13 offered: move its loop to one side of the host, let the quiet camera glance at it, or make its leaps bigger. Each would be a new task.
- Should every page show the animals in the same places?
- The dock's size (3 m wide, wider than a village jetty, so that the deer and the wolf fit on it) and its place are task 09's choices.
- The floating logs are task 11's defaults: four, 1.5–3 m long and 0.25–0.5 m thick, 45% under the water, drifting up to 0.5 m and a few centimeters a second. Seed 6 was picked because it puts all four where the cameras look; other seeds left room for three.

## History

- 2026-09-26, no task file, not committed: the animals rebuilt from their models ([animals.md](animals.md)): the lion is a kind (`!lion`), the penguin an animal like the others (`Figure` is `Animal`), the host bear on all fours, the fish a salmon. Checked with a test viewer's `!lion` on the GPU at 1280×720: the lion roamed and spoke with its named bubble (the close-up put a three-line bubble's top line at the frame's edge); the penguin, frog and snake agents checked theirs.
- 2026-09-26, asked for directly ("add swamps in lakeside", no task file): the meeting's lake has two swamps ([environments.md](environments.md)).
- 2026-09-26 [task 14, the host roams](../../../../tasks/done/14-lake-meeting-host-roams.md): the host roams like the others from the landing; the host's view follows it (HostView.ts); its fixed place, its room and its right of way are gone; the turns' close-ups keep out of crowns; waiting animals no longer creep into one another.
- 2026-09-26 [task 11, floating logs](../../../../tasks/done/11-lake-meeting-logs.md): four logs float and drift in the meeting's lake; the cameras treat them as solid.
- 2026-09-25 [task 09, dock](../../../../tasks/done/09-lake-meeting-dock.md): a dock out from the far bank, which the animals walk out on; the camera treats it as solid.
- 2026-09-25 [task 08, idle camera](../../../../tasks/done/08-lake-meeting-idle-camera.md): after 30 s of quiet the camera follows a random animal for 30 s at a time, from a few angles.
- 2026-09-25 [task 13, fish leaps](../../../../tasks/done/13-lake-meeting-fish-leaps.md): the fish leaps on its own now and then.
- 2026-09-25 [task 12, keep position](../../../../tasks/done/12-lake-meeting-keep-position.md): only the host has a place; the others keep each new spot and never go back.
- 2026-09-25 [task 07, roaming](../../../../tasks/done/07-lake-meeting-roaming.md): the two rows down an aisle gave way to roaming all round the lake; new animals land at random spots; the camera finds a clear side; the shadow follows the view.
- 2026-09-25 [task 10, frog](../../../../tasks/done/10-frog-figure.md): the frog is a viewer's kind, with `!frog`.
- 2026-09-25 [task 05, stream through code changes](../../../../tasks/done/05-stream-through-code-changes.md): a dev server restart hands the meeting and the chat reader to the new code.
- 2026-09-25, before the task queue, not committed: the story built. The dev server keeps the meeting from the live chat; the host and the animals stood in two rows down an aisle on the landing; bots, turns with the camera, supporters' leaps, the Story tab's chat controls. It uses the animals and the fish's `JumpOutFromWater` of commit 278ad67 (2026-09-24).
