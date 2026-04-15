---
name: HTTP/1.1 allows only 6 concurrent connections per domain — consolidate SSE endpoints
description: Multiple SSE endpoints per page hit the browser's 6-connection cap; merge into 1 global + 1 page-level
type: feedback
---

HTTP/1.1 browsers allow 6 concurrent connections per origin. Long-lived SSE connections eat these up. The chat page had 5 open simultaneously (3 layout + 2 chat-specific) — close to the limit, leaving almost no room for regular API calls.

**Why:** Each `data-init="@get('/sse/...')"` opens a persistent connection. Layout SSEs are open on every page.

**How to apply:** Design for max 2 SSE connections at any time:
- **1 global `/sse`** in the layout outer div (`data-init="@get('/sse')"`) — handles clock, notifications, widget chat. One goroutine per user, fan-in from clock ticker + NATS subscriptions.
- **1 page-level SSE** per page type — handles the page-specific live data (e.g. `/sse/chat-page` handles both thread list and messages).

Global SSE patches by element ID (`#server-clock`, `#notif-badge`, `#widget-messages`). Each page's SSE patches its own element IDs.

Avoid naming conflicts: widget elements use unique IDs (`#widget-messages`) separate from full-page elements (`#chat-messages`).
