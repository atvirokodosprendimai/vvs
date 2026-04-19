---
tldr: Portal floating chat widget — rule-based FAQ → AI (Ollama) fallback → "talk to human" live handoff → auto-create ticket on close
status: active
---

# Plan: Customer Portal Chat Bot + Live Human Handoff

## Context

New feature (not in Consilium top-20 but user-requested). Hybrid bot:
1. **Rule-based FAQ** — instant answers to common questions (balance, services, invoice status)
2. **AI fallback (Ollama)** — LLM answers with customer context injected (name, services, overdue invoices)
3. **Live human handoff** — customer says "talk to human" → creates a support chat thread in vvs-core → staff picks it up
4. **Auto-ticket** — on conversation close without resolution → pre-filled support ticket

**Existing infrastructure:**
- `internal/infrastructure/chat/store.go` — `Thread`, `Message`, `CreateThread`, `AddMember`, `SendMessage`
- `isp.chat.message.{threadID}` NATS subject — staff chat real-time updates
- Ollama at `http://localhost:11434` (already running for gomine)
- Portal NATS RPC bridge pattern (`isp.portal.rpc.*`)
- Portal ticket submission plan: [[plan - 2604191738 - customer portal support ticket submission]]

**Architecture decision:**
- Bot logic runs on **vvs-core** (has DB access, Ollama access, chat store)
- Portal sends messages via NATS RPC → core bot processes → returns reply
- Live chat: core creates a real `chat.Thread` of type `"portal-support"` → routes to existing staff chat UI
- AI model: use Ollama chat API (`/api/chat` with `messages` array) — model configurable via env

## Phases

### Phase 1 — Bot Session + Rule-Based FAQ (core side) — status: open

1. [ ] Define bot session store (in-memory, keyed by customerID)
   - `internal/infrastructure/bot/session.go`
   - `BotSession{CustomerID, Messages []BotMessage, State string, ThreadID string}`
   - States: `"bot"`, `"handoff_pending"`, `"live"`, `"closed"`
   - TTL: 30min inactivity → auto-close

2. [ ] Implement rule-based FAQ matcher
   - `internal/infrastructure/bot/rules.go`
   - Rules checked in order, keyword/pattern match on lowercase message
   - Built-in rules:
     | Trigger keywords | Data needed | Response template |
     |-----------------|-------------|-------------------|
     | balance, how much, owe, unpaid | ListOverdue for customer | "You have {N} unpaid invoices totalling {amount}" |
     | invoice, bill | ListInvoicesForCustomer (last 3) | Latest invoice status |
     | service, internet, active, connected | ListServicesForCustomer | Active service list |
     | ip, address, connection | Customer.IPAddress | "Your IP address is {ip}" |
     | pay, payment | last paid invoice | Last payment date |
     | human, agent, person, staff, help | — | Trigger handoff flow |

3. [ ] Add NATS RPC subject: `isp.portal.rpc.bot.message`
   - Request: `{customerID, message, sessionID?}`
   - Response: `{reply, sessionID, state, suggestHandoff bool}`
   - Core handler: check rules → if match return rule response; else → AI fallback (Phase 2)

4. [ ] Register handler in `PortalBridge`

5. [ ] Unit tests for rule matcher: each rule triggers correctly

### Phase 2 — AI Fallback (Ollama) — status: open

1. [ ] Create Ollama chat client
   - `internal/infrastructure/bot/ollama.go`
   - `OllamaChat(ctx, model, systemPrompt, messages []ChatMessage) (string, error)`
   - POST `{OLLAMA_BASE_URL}/api/chat` with `{"model": model, "messages": [...], "stream": false}`
   - Model configurable via `VVS_BOT_MODEL` env (default: `"llama3.2"` or first available)

2. [ ] Build system prompt with customer context
   - `internal/infrastructure/bot/context.go`
   - `BuildSystemPrompt(customer, services, recentInvoices) string`
   - Template: "You are a helpful support assistant for {CompanyName} ISP. Customer: {name}. Active services: {list}. Overdue invoices: {N}. Answer concisely. If you can't help, say so."

3. [ ] Wire AI fallback in bot handler
   - If no rule matches → call `OllamaChat` with conversation history + system prompt
   - If Ollama unavailable/error → fallback message: "I'm having trouble answering that. Would you like to speak with a staff member?"
   - Append AI reply to session history (for context window)

4. [ ] Test: known question → rule wins; unknown question → AI response; Ollama down → fallback message

### Phase 3 — "Talk to Human" Live Handoff — status: open

1. [ ] Create portal-support chat thread in core
   - When customer requests human (rule trigger or explicit): 
   - `chat.CreateThread(ctx, Thread{ID: "portal-{sessionID}", Type: "portal-support", Name: "Portal: {customerName}", ...})`
   - Add system member + post context message: "Customer {name} ({IP}) connected via portal chat. Conversation history: ..."

2. [ ] Add NATS RPC subject: `isp.portal.rpc.bot.handoff`
   - Core creates thread, notifies staff via `isp.chat.message.{threadID}` (existing NATS subject)
   - Returns `{threadID, accepted: false}` — pending until staff joins

3. [ ] Add NATS RPC subject: `isp.portal.rpc.bot.livemessage`
   - Customer message → stored in chat thread via `chat.Store.SendMessage`
   - Publishes `isp.chat.message.{threadID}` → staff sees in existing chat UI
   - Returns staff reply if available (last message in thread from non-system user)

4. [ ] Staff-side: portal-support threads appear in staff chat list
   - Thread type `"portal-support"` shown with special badge in chat sidebar
   - Staff replies via existing chat UI → NATS → customer polling picks it up

5. [ ] Unit test: handoff creates thread; messages route correctly both ways

### Phase 4 — Auto-Ticket on Close — status: open

1. [ ] Add NATS RPC subject: `isp.portal.rpc.bot.close`
   - Closes session; if state != `"live"` (unresolved by bot) → offer ticket creation
   - If customer accepts: calls `OpenTicketHandler` with subject from conversation summary + body from last few messages

2. [ ] Session timeout handler (in-memory TTL checker)
   - Background goroutine: every 5min scan sessions, close + auto-create ticket if 30min inactive with unresolved `"bot"` state

### Phase 5 — Portal Widget (UI) — status: open

1. [ ] Create floating chat widget component (`portal_chat.templ`)
   - Fixed bottom-right button: "Chat" → expands to chat panel
   - Panel: message history, input box, send button, "Talk to human" quick-action button
   - Datastar signals: `_chatOpen`, `_chatMessages`, `_chatState`, `_sessionID`

2. [ ] Wire chat via SSE + `@post`
   - Send message: `@post('/portal/bot/message')` → SSE fragment appends reply bubble
   - Request human: `@post('/portal/bot/handoff')` → state badge changes to "Connecting..."
   - Live mode: `@get('/sse/portal/bot/live/{sessionID}', {openWhenHidden: false})` — polls for staff replies

3. [ ] Add portal HTTP routes
   - `POST /portal/bot/message` — send to bot, returns reply fragment
   - `POST /portal/bot/handoff` — request human
   - `POST /portal/bot/close` — close session, optionally create ticket
   - `GET  /sse/portal/bot/live/{sessionID}` — long-poll for staff replies in live mode

4. [ ] Inject chat widget into portal base layout (all portal pages)

### Phase 6 — Wiring — status: open

1. [ ] Wire bot handler in `wire_infra.go` / `PortalBridge`:
   - pass `listInvoices`, `listServices`, `getCustomer`, `openTicket`, `chatStore`, `ollamaClient`

2. [ ] Add `VVS_BOT_MODEL` to config + `deploy/` env templates

3. [ ] `go build ./... && go test ./internal/infrastructure/bot/...`

## Verification

```bash
go test ./internal/infrastructure/bot/... -v
go build ./cmd/portal/ ./cmd/vvs-core/
# Open portal → chat button visible bottom-right
# "what is my balance?" → rule answer
# "explain WireGuard" → Ollama AI answer
# "talk to human" → thread appears in staff chat
# Staff replies → customer sees message in widget
# Close chat with unresolved question → offered ticket creation
```

## Adjustments

## Progress Log
