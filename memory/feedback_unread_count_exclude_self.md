---
name: Unread count must exclude messages sent by the reader themselves
description: SQL unread queries need AND cm.user_id != ? to avoid counting own messages as unread
type: feedback
---

If you send a message and your own `last_read_at` is before that message's timestamp, the message counts as unread — for you, the sender. The fix is to exclude the reader's own messages from the count.

**Why:** You can't have an unread message you sent yourself.

**How to apply:** Add `AND cm.user_id != ?` (with userID as parameter) to every unread count query:

```sql
SELECT COUNT(*) FROM chat_messages cm
LEFT JOIN chat_thread_reads r ON r.thread_id = cm.thread_id AND r.user_id = ?
WHERE cm.thread_id = t.id
  AND cm.user_id != ?   -- exclude own messages
  AND (r.last_read_at IS NULL OR cm.created_at > r.last_read_at)
```

Apply to both per-thread unread count (in `ListThreadsForUser`) and global unread count (`TotalUnread`).
