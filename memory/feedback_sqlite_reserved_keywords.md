---
name: SQLite reserved keywords as column names
description: Several common words are reserved in SQLite and cause parse errors when used as bare column names
type: feedback
---

Never use SQLite reserved keywords as bare column names in migrations or GORM `gorm:"column:..."` tags.

**Why:** `references` used as a column name in `email_messages` caused migration failure:
> `SQL logic error: near "references": syntax error (1)`

Fixed by renaming to `ref_ids` in both the SQL migration and the GORM struct tag.

**Common reserved words to avoid:**
- `references` → use `ref_ids` or `reference_ids`
- `order` → use `sort_order` or `order_num`
- `group` → use `group_name` or `group_id`
- `index` → use `idx` or `sort_index`
- `key` → use `key_name` or `api_key`
- `values` → use `val` or `value_data`
- `select`, `from`, `where`, `join` — obvious, but worth noting

**How to apply:** When defining a new table column, check if the name could be a SQL keyword. If in doubt, add a suffix (`_id`, `_name`, `_val`) or use an explicit GORM `column:` tag with a safe name.
