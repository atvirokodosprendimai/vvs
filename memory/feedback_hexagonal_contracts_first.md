---
name: Hexagonal arch — write contracts first, then spawn agents
description: Define ports/interfaces before implementation; parallelize adapters across agents
type: feedback
---

Hexagonal arch = contracts (ports) are source of truth. Implementation (adapters) flows from them.

**Why:** Agents can work in parallel once interfaces are locked. No dependency conflicts. Each agent gets clear scope: "implement this interface".

**How to apply:**
1. Write domain interfaces + types first (ports, commands, queries, events)
2. Spawn up to 5 agents in parallel, each owning one adapter:
   - Agent 1: persistence adapter (GORM impl)
   - Agent 2: HTTP handler
   - Agent 3: domain logic / command handlers
   - Agent 4: tests
   - Agent 5: migrations / infra
3. Agents compile against contract — no need to wait on each other

**Rule:** Never spawn agents on ambiguous scope. Contract must be written and committed before parallelizing.
