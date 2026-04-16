---
name: Spawn Codex review after writing code
description: After completing a coding task, automatically launch a Codex review agent in the background
type: feedback
---

After writing code (completing a feature, fixing a bug, or any meaningful implementation), spawn a Codex review in the background using `/codex:review` before ending the session.

**Why:** User explicitly requested this workflow on 2026-04-16.

**How to apply:** After committing code changes, run:
```
node "/Users/mind/.claude/plugins/cache/openai-codex/codex/1.0.3/scripts/codex-companion.mjs" review ""
```
with `run_in_background: true`. Tell the user "Codex review started in background — check `/codex:status` for results."
Do this automatically without being asked each time.
