---
tldr: Consolidate all SSE endpoints into global /sse + 1 page-level SSE (max 2 open at once)
status: completed
---

# Plan: Consolidate SSE Endpoints

## Context

HTTP/1.1 allows 6 concurrent connections per domain. Current chat page opens 5 simultaneously:

| SSE endpoint | Where opened |
|---|---|
| `/sse/clock` | layout (every page) |
| `/sse/notifications` | layout (every page) |
| `/sse/chat` | layout (chat widget) |
| `/sse/chat/threads` | /chat page |
| `/sse/chat/messages/{threadID}` | /chat page (on thread select) |

Module pages (customers, products, etc.) add a 6th via `/api/customers` etc.

**Target:** max 2 SSE connections on any page — global + page-level.

## Design

### Global `/sse` (replaces `/sse/clock` + `/sse/notifications` + `/sse/chat` widget)

Single long-lived SSE opened in the layout. Multiplexes:
- **Clock tick** — patches `#clock` every second
- **Notification badge** — patches `#notif-badge` on `isp.notifications.*` events
- **Chat unread badge** — patches `#chat-unread` on `isp.chat.message.*` events (no message content, just count)

One goroutine per connected user, subscribes `isp.notifications.*` + `isp.chat.>` + clock ticker.

### Chat page `/sse/chat-page` (replaces `/sse/chat/threads` + `/sse/chat/messages/{threadID}`)

Reads `threadid` signal from query params. Manages both thread list and message stream in one connection.

When user selects a new thread: Datastar reconnects `@get('/sse/chat-page')` with updated `$threadid` signal → backend switches message subscription.

In the template:
```html
<div data-init="@get('/sse/chat-page')" data-on:$threadid__case__changed="@get('/sse/chat-page')">
```
(or use a dedicated signal watcher pattern — exact Datastar syntax TBD)

### Module pages

`/api/customers`, `/api/routers`, etc. already serve one SSE per page. Keep as-is — they're on separate pages so at most 1 is open at a time.

**Result: global SSE (1) + page SSE (1) = 2 max.**

## Phases

### Phase 1 — Global `/sse` endpoint — status: completed

1. [x] GlobalSSE handler in `internal/infrastructure/http/global_sse.go`
2. [x] `GET /sse` registered in router.go
3. [x] Layout uses single `data-init="@get('/sse')"`

### Phase 2 — Consolidated chat SSE — status: completed

1. [x] `chatPageSSE` handler — multiplexes `isp.chat.>` (thread list) + `isp.chat.message.{threadID}` (messages)
2. [x] `chat.templ` — single `data-init="@get('/sse/chat-page')"` on outer wrapper
3. [x] Thread row click: `($threadid='X') && @get('/sse/chat-page')` — reconnects with new thread
4. [x] Removed `/sse/chat/threads` and `/sse/chat/messages/{threadID}` routes + dead handler code

## Verification

- Chat page: DevTools Network → max 2 SSE connections at once
- Notifications still update in real time
- Clock still ticks in layout
- Thread selection loads messages correctly
- Unread badge in nav updates when messages arrive
- No connection queueing in browser (all SSE connect immediately)

## Progress Log

- 2604151841 — Plan created after observing 5 SSE connections on chat page
- 2604162100 — Phase 1 was already done; Phase 2 implemented: chatPageSSE consolidates 2→1 connection
