---
name: Datastar data-bind kebab normalizes to camelCase
description: data-bind:some-name and data-signals:{someName} refer to the same signal — Datastar normalizes kebab to camelCase
type: feedback
---

`data-bind:contact-first-name` and `data-signals="{contactFirstName: ''}"` refer to the **same** signal. Datastar normalizes `data-bind:*` suffixes to camelCase automatically.

**Why:** Codex flagged `data-bind:contact-first-name` + handler reading `contactFirstName` as a mismatch. It is not — this is correct Datastar behavior per spec: "Signalų apibrėžimo atributuose (data-signals:*, data-bind:*, data-computed:*) sufiksai normalizuojami į camelCase."

**How to apply:** When reviewing Datastar forms, `data-bind:foo-bar` and `$fooBar` are the same signal. Do not "fix" this. Only flag a real mismatch when the Go handler JSON tag differs from the camelCase form of the kebab name.
