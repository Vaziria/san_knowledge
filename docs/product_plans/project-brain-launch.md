# project-brain launch

To-do list for launching project-brain, our skill on agensi.io: a living knowledge graph for you and your AI agents, powered by san_knowledge. Its files live in `project/sell_to_agensi/harness_skill/`, which the knowledge graph leaves out on purpose (the user, 2026-10-19: "dont add skill that we generated to our knowledge, its for agensi.io"). This list is the one place in the graph that tracks the launch.

Status on 2026-10-20: the listing is live on Agensi: https://www.agensi.io/skills/project-brain ($10 a month, or yearly at -17%; category Agents & Orchestration). The promotion is under way.

## Done

- [x] Skill package `project-brain.zip`: SKILL.md, references, examples, and the san_knowledge source the buyer builds (`harness_skill/package.ps1` rebuilds it after any change to san_knowledge).
- [x] Logo: origami crane, 512x512, navy tile (`harness_skill/logo/project-brain-logo.png`).
- [x] Listing copy: one-line description, full description, compatibility note.
- [x] Five listing screenshots from the Paperboat sample project (`harness_skill/screenshots/`).
- [x] Demo video, 60 s, 16:9, English, on YouTube (Heri Santoso Dev): https://youtu.be/HJUhnFcaTDc, public from 2026-10-19 22:50. Source: `harness_skill/demo-video/`.
- [x] Listing submitted to Agensi; in preview.

## Waiting on Agensi

- [x] Listing approved and live (2026-10-20): https://www.agensi.io/skills/project-brain
- [ ] Put the listing URL into the demo video's description in YouTube Studio (it links to https://agensi.io until then).
- [ ] In the explainer's description (https://studio.youtube.com/video/aTElvMX4R7c/edit), replace "Contoh kami, project-brain, sedang direview Agensi." with "Contoh kami, project-brain, sudah tayang di Agensi: https://www.agensi.io/skills/project-brain".
- [ ] Upload the rebuilt `project-brain.zip` as a new version on Agensi: the live listing has the zip from before the Board view (and its paging, 2026-10-20).
- [ ] Still open from the build, if not settled at submission: the license for the shipped source, the listing's tags and price, and a test of the skill inside an agent session (Agensi asks for one).

## Promotional video for TikTok and YouTube

Starts when the listing is live, so the caption can carry its URL.

- [x] Decide the language: Indonesian (the user, 2026-10-20). Angle: the opportunity of selling skills on Agensi, with project-brain as our example.
- [x] YouTube explainer, 2 min, 16:9, Indonesian, sound effects and no music (`videos/agensi-jual-skill/`): https://youtu.be/aTElvMX4R7c, scheduled for 2026-10-21 19:00. Kept as uploaded (the user, 2026-10-20), so scene 9 still says "sedang direview"; a corrected render saying "sudah tayang" is in `videos/agensi-jual-skill/renders/video.mp4`.
- [x] TikTok cut, 30 s, 1080x1920, Indonesian, sound effects only (`videos/agensi-jual-skill-tiktok/`).
- [x] Preview the video, title and caption with the user before any upload: no emoji or icons, hashtags first, a catchy title.
- [x] Check what is already published and scheduled on both platforms, and pick a free day for each: one video a day per platform ([social posting plan](../social_posting.md#rules)). Free days on 2026-10-20: YouTube 2026-10-21, TikTok 2026-10-22.
- [x] TikTok: scheduled with `san_tiktok` on profile basic (@archetipe_) for 2026-10-22 19:00, Everyone.
- [x] YouTube: the 2-minute explainer above, scheduled with `san_youtube` on profile basic (Heri Santoso Dev), instead of a Short.

## Articles on Medium and LinkedIn

- [ ] Decide what kind of article it is. Our [article guideline](../articles/guideline.md#educational-only) allows only educational articles that never name our products, and asks for Indonesian. Either:
  - an educational article about the idea (why AI agents need a shared, cited, living knowledge graph of a project) that follows the guideline, or
  - a promotional article that names project-brain and links the listing, as a written exception to the guideline.
- [ ] Decide the language (the guideline says Indonesian; the listing is English).
- [ ] Write it as one markdown file under `docs/articles/<topic>/`, with its images in that topic's `assets/`.
- [ ] Medium: make the draft with `san_medium` (profile basic); the user adds tags and publishes ([publishing to Medium](../articles/guideline.md#publishing-to-medium)).
- [ ] LinkedIn: draft and schedule with `san_linkedin` on profile basic, with a cover image and the post text ([publishing to LinkedIn](../articles/guideline.md#publishing-to-linkedin)).
- [ ] Link the listing and the demo video from both, once the listing is live.
