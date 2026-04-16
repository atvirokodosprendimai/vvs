-- +goose Up
CREATE TABLE tasks (
    id          TEXT PRIMARY KEY,
    customer_id TEXT NOT NULL DEFAULT '',
    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'todo' CHECK(status IN ('todo','in_progress','done','cancelled')),
    priority    TEXT NOT NULL DEFAULT 'normal' CHECK(priority IN ('low','normal','high')),
    due_date    DATETIME,
    assignee_id TEXT NOT NULL DEFAULT '',
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_tasks_customer ON tasks(customer_id) WHERE customer_id != '';
CREATE INDEX idx_tasks_status ON tasks(status);

-- +goose Down
DROP INDEX IF EXISTS idx_tasks_status;
DROP INDEX IF EXISTS idx_tasks_customer;
DROP TABLE IF EXISTS tasks;
