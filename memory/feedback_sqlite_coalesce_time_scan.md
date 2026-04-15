---
name: SQLite COALESCE returns TEXT — can't scan into time.Time
description: GORM raw Scan fails when COALESCE/subquery returns a datetime column as TEXT string
type: feedback
---

SQLite's pure-Go driver (`glebarez/sqlite`) cannot auto-convert a TEXT value returned by `COALESCE(subquery, fallback)` into Go `time.Time` during GORM raw `Scan`. Direct column selects work; COALESCE results come back as `driver.Value type string`.

**Why:** COALESCE strips the column type metadata — the driver sees a plain string and doesn't know it's a timestamp.

**How to apply:** When raw SQL uses COALESCE or subqueries for datetime columns, scan them as `string` fields in the result struct and parse manually:

```go
type row struct {
    LastAt string `gorm:"column:last_at"` // NOT time.Time
}
// Then: parseTime(row.LastAt)
```

Use `CAST(col AS TEXT)` in SQL to ensure consistent format. Implement a `parseTime(s string) time.Time` helper that tries multiple SQLite datetime formats:
- `time.RFC3339Nano`
- `"2006-01-02 15:04:05.999999999-07:00"`
- `"2006-01-02 15:04:05.999999999"`
- `"2006-01-02 15:04:05"`
- `"2006-01-02"`
