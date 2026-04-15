---
name: Datastar _ prefix signals are frontend-only — not sent to backend
description: Signals prefixed with _ stay client-side; backend never receives them in @post/@get
type: feedback
---

Datastar signals prefixed with `_` (e.g. `_open`, `_notifOpen`, `_chatOpen`) are **not included** in the JSON body sent with `@post`/`@get` requests. Use them for pure UI state: modal open/closed, active tab, toggle.

**Why:** Reduces payload noise; backend doesn't need to know about purely visual state.

**How to apply:**
```html
data-signals="{_open:false, name:''}"
<!-- _open stays client-only; name is sent with every @post -->
data-show="$_open"
data-on:click="$_open=true"
```

Backend can still SET these signals by sending `data-signals` patches via SSE.

Rule of thumb: if the signal is never read by a Go handler (`ReadSignals`), prefix it with `_`.
