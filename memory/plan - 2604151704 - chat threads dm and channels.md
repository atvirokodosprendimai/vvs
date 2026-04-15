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

### Phase 1 — DB Schema — status: open

1. [ ] Migration `002_add_threads.sql`
   - `chat_threads(id, type CHECK('direct'|'channel'), name, is_private, created_by, created_at)`
   - `chat_thread_members(thread_id FK, user_id, joined_at) PK(thread_id, user_id)`
   - `chat_thread_reads(thread_id, user_id, last_read_at) PK(thread_id, user_id)` — unread tracking
   - `ALTER TABLE chat_messages ADD COLUMN thread_id TEXT NOT NULL DEFAULT ''`
   - Seed: INSERT `#general` channel (id='general', type='channel', is_private=0)
   - Backfill: UPDATE chat_messages SET thread_id='general' WHERE thread_id=''

2. [ ] Update `chat.Store` with new types and methods
   - Add types: `Thread`, `ThreadMember`
   - `CreateThread(ctx, Thread) error`
   - `FindDirectThread(ctx, userA, userB string) (Thread, error)` — find existing DM
   - `AddMember(ctx, threadID, userID string) error`
   - `ListThreadsForUser(ctx, userID string) ([]ThreadSummary, error)` — includes last message + unread count
   - `Recent(ctx, threadID string, limit int) ([]Message, error)` — filter by thread_id
   - `Save(ctx, Message) error` — Message now has ThreadID field
   - `MarkRead(ctx, threadID, userID string) error`
   - `UnreadCount(ctx, userID string) (int, error)` — total unread across all threads

3. [ ] Seed `#general` channel on app startup in `app.go`
   - If thread 'general' doesn't exist, create it with all existing users as members

### Phase 2 — Backend HTTP Layer — status: open

1. [ ] SSE: `/sse/chat/threads` — thread list for current user
   - Initial render: `ThreadList(threads, currentUserID)`
   - NATS `isp.chat.>` events → re-query `ListThreadsForUser` → diff → patch if changed
   - Shows unread badge per thread

2. [ ] SSE: `/sse/chat/messages/{threadID}` — message stream for a thread
   - Auth check: user must be member of thread
   - Initial render: `ChatMessages(msgs, currentUserID, threadID)`
   - NATS `isp.chat.message.{threadID}` → append new message + scroll sentinel
   - Mark thread as read on connect

3. [ ] POST `/api/chat/send` — extend to include `threadID` signal
   - Publish to `isp.chat.message.{msg.ThreadID}` (not generic `isp.chat.message`)

4. [ ] POST `/api/chat/threads/direct` — create or open 1:1 DM
   - Signal: `{targetUserID}`
   - Find existing direct thread; if none, create + add both members
   - Response: redirect to `/chat?thread={id}` or patch `$threadID` signal

5. [ ] POST `/api/chat/threads/channel` — create named channel
   - Signals: `{channelName, isPrivate}`
   - Create thread, add creator as member
   - Redirect or patch `$threadID`

6. [ ] POST `/api/chat/threads/{id}/members` — add member to channel
   - Auth: only channel members can add

7. [ ] POST `/api/chat/threads/{id}/read` — mark thread as read

### Phase 3 — /chat Full Page — status: open

1. [ ] Chat page template `ChatPage(threads []ThreadSummary, currentUser)`
   - Two-panel layout: left sidebar (260px) + right message area (flex-1)
   - Left: thread list with SSE (`data-init="@get('/sse/chat/threads')"`)
   - Right: message panel, initially empty (select a thread to load)
   - Signals: `{threadid:'', newchannel:false, newdmtarget:''}`

2. [ ] Thread list component `ThreadList(threads []ThreadSummary, currentUserID)`
   - Section headers: "Direct Messages" / "Channels"
   - Each row: avatar/icon, name, last message preview, unread badge
   - Click → `($threadid='<id>') && @get('/sse/chat/messages/'+$threadid)`
   - "New DM" button → opens user picker modal
   - "New Channel" button → opens channel create modal

3. [ ] Message panel component: thread header + `#chat-messages` + input
   - Reuse `ChatMessageItem` with thread context
   - Input: same `data-bind:chatmsg` + send with threadID signal

4. [ ] New DM modal: list all users, click → `@post('/api/chat/threads/direct')`

5. [ ] New Channel modal: name input + private toggle + create button

6. [ ] Register `/chat` GET route → renders `ChatPage`

### Phase 4 — Integration — status: open

1. [ ] Add "Chat" nav item to sidebar in `layout.templ`
   - Route: `/chat`
   - Badge: unread count via SSE or periodic refresh

2. [ ] Wire floating widget to `#general` thread
   - Widget SSE changes from `/sse/chat` → `/sse/chat/messages/general`
   - Widget send POST includes `threadID: 'general'`

3. [ ] "Message" button on `/users` page user rows
   - `@post('/api/chat/threads/direct')` with `{targetUserID}`
   - On success: redirect to `/chat?thread={id}`

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

<!-- Updated after every completed action -->
