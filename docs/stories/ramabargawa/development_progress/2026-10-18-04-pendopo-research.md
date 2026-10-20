# 2026-10-18: Pendopo research

Web research on the pendopo, written to [buildings/pendopo.md](../buildings/pendopo.md), next to the model `project/bargawa/blender/buildings/pendopo.blend`.

## Decisions (the user's words)

| Decision | The user said |
|---|---|
| Focus on the pendopo, research first | "lets focus on pendopo, lets doing research about pendopo" |
| Write it in a doc named and placed like the Blender file | "write it in pendopo.md same as blender file" |

## Done

- Researched what a pendopo is: name and origin (Sanskrit *mandapa*), its place in the Javanese house (regol, pendopo, pringgitan, dalem...), history (Borobudur reliefs, Ratu Boko, Trowulan), notable pendopo (Mangkunegaran, Ambarrukmo), uses, roof types and status, the joglo's parts, roof layers, types, earthquake behaviour, materials, ornament, meaning, orientation.
- Compared the model with the research in the doc's "Our pendopo" section.

## Findings that matter for the model

- The **tumpang sari has two bands**. The outer band steps outward and carries the rafters; the model has this. The **inner band is a carved, inverted stepped-pyramid ceiling**, and the space under it is the most sacred spot. The model lacks it.
- Facing south matches Yogyakarta and Mataram practice; the model faces south.
- Two roof tiers match a simple **Joglo Lawakan**, which suits a hermitage.
- Umpak are traditionally **padma** (lotus) shaped. Typical ornaments are tlacapan and praba on the posts, and a nanasan pendant locking the tumpang sari.

## Open questions (in pendopo.md)

- Which proposed changes to make:
  - the inner tumpang sari ceiling and nanasan
  - the sacred centre in the story
  - padma umpak
  - modest ornament
  - joints for close-ups
  - two tiers or three
- Joglo or the humbler limasan for a hermitage.

## Next

The user decides the changes; then update `common/joglo.py`, rebuild with `run.ps1 building pendopo`, and review with `run.ps1 views pendopo -Building`.
