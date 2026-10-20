# Example: explain output

A project with `docs/pricing.md`, `docs/plans/launch.md` and `NOTES.md`, linked to each other. The pricing doc and its Discounts section have summaries; the launch timeline doesn't yet.

Command (the MCP tool `knowledge_explain` returns the same markdown):

```
.claude/skills/project-brain/bin/knowledge explain "how much is the ramadan discount" --limit 3
```

Output:

```markdown
# Knowledge context: how much is the ramadan discount

## 1. Discounts (doc_section `docs/pricing.md#discounts`)
- loc: docs/pricing.md:9
- keyword: ramadan, discount, annual plan

Ramadan discount is 25% on annual plans only; monthly plans get none.

Related:
- Pricing (doc `docs/pricing.md`) [section_of, depth 1]: Pricing for the Indonesian market: three monthly tiers, a Ramadan discount on annual plans, and why the free tier was dropped in February.
- Timeline (doc_section `docs/plans/launch.md#timeline`) [reference, depth 1]

## 2. Pricing (doc `docs/pricing.md`)
- loc: docs/pricing.md
- domain: Sales
- keyword: pricing, tiers, free tier, ramadan discount

Pricing for the Indonesian market: three monthly tiers, a Ramadan discount on annual plans, and why the free tier was dropped in February.

Related:
- Sales (domain `sales`) [domain_of, depth 1]
- Tiers (doc_section `docs/pricing.md#tiers`) [section_of, depth 1]
- Discounts (doc_section `docs/pricing.md#discounts`) [section_of, depth 1]: Ramadan discount is 25% on annual plans only; monthly plans get none.
- Notes (doc `NOTES.md`) [reference, depth 1]

## 3. Timeline (doc_section `docs/plans/launch.md#timeline`)
- loc: docs/plans/launch.md:3

_(no summary yet)_ Public launch on 3 November. The Ramadan offer from [pricing](../pricing.md#discounts) runs in the first campaign.

Related:
- Launch plan (doc `docs/plans/launch.md`) [section_of, depth 1]
- Discounts (doc_section `docs/pricing.md#discounts`) [reference, depth 1]: Ramadan discount is 25% on annual plans only; monthly plans get none.
```

The agent then reads `docs/pricing.md` around line 9 and answers:

> 25%, on annual plans only; monthly plans get no Ramadan discount (`docs/pricing.md:9`). It runs in the first campaign after the 3 November launch (`docs/plans/launch.md:3`).
