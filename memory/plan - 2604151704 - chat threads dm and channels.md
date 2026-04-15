---
tldr: Upgrade chat from single global room to threaded DMs + named channels with full /chat page
status: active
---

# Plan: Chat Threads — DMs and Channels

## Context

Current state: single global chat room (floating widget, `chat_messages` table, no thread concept).

Decisions from planning:
- Keep floating widget (wire to #general channel) + add full `/chat` page
- Channels: public + private (invite-only)
- DMs: sidebar user picker + "Message" button on `/users` page

## Phases

### Phase 1 — DB Schema — status: completed

1. [x] Migration `002_add_threads.sql`
   - => `chat_threads`, `chat_thread_members`, `chat_thread_reads` created
   - => `ALTER TABLE chat_messages ADD COLUMN thread_id`; backfill to 'general'
   - => #general seeded in migration SQL

2. [x] Update `chat.Store` with new types and methods
   - => Thread, ThreadSummary, Message (with ThreadID) types
   - => CreateThread, ThreadExists, FindDirectThread, AddMember, IsMember
   - => ListThreadsForUser (with last message + unread count via SQL)
   - => ListPublicChannels, Recent(threadID), MarkRead, TotalUnread

3. [x] Seed `#general` channel on app startup in `app.go`
   - => seedGeneralChannel() checks ThreadExists before creating
   - => chat.go SSE/send wired to isp.chat.message.{threadID}; widget defaults to 'general'

### Phase 2 — Backend HTTP Layer — status: completed

1. [x] SSE: `/sse/chat/threads` — thread list for current user
   - => threadsSSE with reflect.DeepEqual diff, subscribes isp.chat.>

2. [x] SSE: `/sse/chat/messages/{threadID}` — message stream for a thread
   - => threadMessagesSSE; auto-join public channels; marks read on connect
   - => shared streamMessages loop handles both widget and full-page

3. [x] POST `/api/chat/send` — extended with threadID signal
   - => publishes to isp.chat.message.{threadID}

4. [x] POST `/api/chat/threads/direct` — create or open 1:1 DM
   - => FindDirectThread → create if not found → AddMember both → redirect /chat?thread=id

5. [x] POST `/api/chat/threads/channel` — create named channel
   - => prefix # normalised, creator auto-added as member → redirect

6. [x] POST `/api/chat/threads/{id}/members` — add member to channel
7. [x] POST `/api/chat/threads/{id}/read` — mark thread as read

### Phase 3 — /chat Full Page — status: completed

1. [x] ChatPage template — two-panel, signals {threadid, chatmsg, newdm, newchannel, channelname, isprivate, newdmtarget}
2. [x] ChatThreadList / ChatThreadRow — channels + DMs sections, unread badges
3. [x] ChatMessages / ChatMessageItem — message panel with input bar
4. [x] New DM modal (user ID input), New Channel modal (name + private toggle)
5. [x] /chat GET route registered in router.go

### Phase 4 — Integration — status: completed

1. [x] Chat nav item in sidebar — chatNavIcon(), links to /chat
2. [x] Widget wired to #general (isp.chat.message.general subject)
3. [x] Message button on user rows → POST /api/chat/threads/direct → redirect
4. [ ] Update AGENTS.md with new chat routes and SSE patterns

## Verification

- Create a DM with another user, send messages, verify only those two users see them
- Create a public channel, multiple users join and chat in real time
- Create a private channel, verify non-members cannot see or join it
- Floating widget still works (posts to #general, receives #general messages)
- Unread badge appears on nav item when messages arrive while on another page
- "Message" button on /users page opens the correct DM thread
- reflect.DeepEqual diff on thread list suppresses unnecessary SSE patches

## Progress Log

- 2604151704 — Phase 1 complete: DB schema, Store, seed #general, widget wired to thread subjects
- 2604151730 — Phases 2-4 complete: all HTTP endpoints, /chat page, nav item, Message button
