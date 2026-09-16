# Development Lifecycle.

1. `state_report`, its live in `docs/development_state/[big_context]/[small_context].md`
2. for business requirements, its live in `docs/business/[big_context]/[small_context].md`, its human written.
3. for technical requirements, its live in `docs/technical/[big_context]/[small_context].md`, its human written.
4. when Agent need clarity or clarify, its writed in same location of file that need clarify with suffix `_clarify.md`
5. Agent save feedback decision from human to file. its writed in same location of file that need clarify with suffix `_decision.md`
6. `design_accept` is human previewing the story book prototype. it **blocks** — nothing after it runs until human accept.
7. summarize all clarity / clarify question in 7 biggest question in `docs/biggest_question.md`
8. `docs/biggest_question.md` is a **derived** file — regenerate it, never hand-edit it. rebuild it whenever any `_clarify.md` changes, and rank by what is BLOCKED. the 7 is a display cap, so the file must say how many questions are NOT shown and where they live.
9. build it from `_clarify.md` only, never from `_decision.md` — an answered question leaves the summary the moment it is recorded.

```mermaid
flowchart TD
s(("start"))
e(("end"))

s-->define[/"Human Define Requirements"/]
define-->|"docs/technical/"|define_technical[/"Define Technical Requirements"/]
define-->|"docs/business/"|define_bussiness[/"Define Business Requirements"/]

define_technical-->implementation_analysis["Doing Implementation Analysis"]
redefine_technical-->implementation_analysis
implementation_analysis-->implementation_clarity{"is Agent Need Clarity / Clarify"}

implementation_clarity-->|yes|technical_report["Technical Clarify File"]
    technical_report-->redefine_technical[/"Human Re Define Technical Requirements"/]

implementation_clarity-->|no|design_accept[/"Human Accept Design"/]
design_accept-->|"screen is wrong"|implementation_analysis
design_accept-->|"requirement was wrong"|redefine_business
design_accept-->|accept|backend_analysis["Backend Analysis"]

backend_analysis-->implementation["Writing Full Implementation"]
    implementation-->technical_test["Run Testing"]
    technical_test-->audit["Audit RPC"]
    audit-->state_report["Writing Summary Development State Report"]
    state_report-->add_requirements{"Change Or Add Requirements"}
    add_requirements-->|no|e
    add_requirements-->|yes|redefine_business

define_bussiness-->agent_analysis["Agent Doing Business Analysis"]
agent_analysis-->clarity{"Is Agent Need Clarity / Clarify"}

clarity-->|no|implementation_analysis
clarity-->|yes|business_clarity_report["Business Clarify File"]
    business_clarity_report-->redefine_business[/"Waiting Human Redefine Business Requirements"/]
    redefine_business-->agent_analysis

```

`Run Testing` is unit then integration then e2e. `Audit RPC` is the performance audit and, for a
write RPC, the concurrency audit — only a HEAVY or UNSAFE result gets written up.

## What Happen in `Implementation Analysis`
1. `implementation_analysis` is frontend first review in story book with complete wiring + mock
2. it produce a previewable prototype and nothing else — no backend, no migration — so a reject at `design_accept` only costs the prototype
```mermaid
flowchart TD
s(("start"))
e(("end"))

s-->frontend_analysis["Frontend Analysis"]
frontend_analysis-->pages_analysis["Page Analysis"]
pages_analysis-->components_analysis["Components Analysis"]
pages_analysis-->contract_analysis["Contract Analysis"]
contract_analysis-->write_storybook
components_analysis-->write_storybook["Write Frontend & mock wiring in Story Book (so it can be preview)"]
write_storybook-->e
```
