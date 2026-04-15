---
tldr: Consolidate all SSE endpoints into global /sse + 1 page-level SSE (max 2 open at once)
status: active
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

### Phase 1 — Global `/sse` endpoint — status: open

1. [ ] Create `GlobalSSE` handler in `internal/infrastructure/http/global_sse.go`
   - Fan-in: clock ticker (1s), `isp.notifications.*`, `isp.chat.>`
   - Patches: `#clock` (time), `#notif-badge` (count), `#chat-unread` (total unread)
   - Requires `chat.Store.TotalUnread` (already exists)

2. [ ] Register `GET /sse` route in router.go

3. [ ] Update layout.templ
   - Replace 3 separate `data-init="@get('/sse/clock')"` etc. with single `data-init="@get('/sse')"`
   - Remove `/sse/clock`, `/sse/notifications`, `/sse/chat` `data-init` elements

4. [ ] Remove old handlers: `clockSSE`, `notificationsSSE` (or keep as dead code temporarily)

5. [ ] Remove old routes from router.go

### Phase 2 — Consolidated chat SSE — status: open

1. [ ] Create `chatPageSSE` handler: reads `threadid` from query signal, runs both thread-list loop and message loop in one goroutine (or two goroutines sharing the SSE writer)
   - Thread list: subscribes `isp.chat.>`, patches `#chat-thread-list`
   - Messages: subscribes `isp.chat.message.{threadID}`, patches `#chat-messages`
   - On `threadid` change: handled by client reconnect

2. [ ] Update chat.templ to use single `@get('/sse/chat-page')` instead of two separate inits

3. [ ] Wire thread row click to reconnect SSE:
   ```
   data-on:click="($threadid='X') && @get('/sse/chat-page')"
   ```
   (replacing current separate `@get('/sse/chat/messages/X')`)

4. [ ] Remove `/sse/chat/threads` and `/sse/chat/messages/{threadID}` routes

## Verification

- Chat page: DevTools Network → max 2 SSE connections at once
- Notifications still update in real time
- Clock still ticks in layout
- Thread selection loads messages correctly
- Unread badge in nav updates when messages arrive
- No connection queueing in browser (all SSE connect immediately)

## Progress Log

- 2604151841 — Plan created after observing 5 SSE connections on chat page
