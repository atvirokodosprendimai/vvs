# Memory Index

- [SQLite COALESCE → TEXT scan bug](feedback_sqlite_coalesce_time_scan.md) — COALESCE strips type metadata; scan datetime subquery results as string + parseTime helper
- [SQLite writer contention in SSE loops](feedback_sqlite_writer_contention.md) — WriteTX per-event in SSE goroutines starves the single writer; only write on relevant event types
- [MarkRead needs NATS publish](feedback_sse_mark_read_nats.md) — Silent DB writes don't wake live SSE views; always publish after state changes visible in SSE
- [Datastar _ prefix = FE-only signals](feedback_datastar_underscore_signals.md) — `_signal` not sent to backend; use for modal/toggle/tab state
- [HTTP/1.1 SSE connection limit](feedback_sse_http1_connection_limit.md) — 6-connection cap; design for 1 global /sse + 1 page-level SSE max
- [Unread count excludes own messages](feedback_unread_count_exclude_self.md) — Add `cm.user_id != ?` to all unread count SQL queries
- [Go error handling](feedback_go_error_handling.md) — Never discard with `_`; HTTP handlers return error responses, SSE loops log and continue
