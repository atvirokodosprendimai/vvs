-- +goose Up
CREATE TABLE cron_jobs (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    schedule    TEXT NOT NULL,
    job_type    TEXT NOT NULL CHECK(job_type IN ('rpc','action','shell')),
    payload     TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'active'
                CHECK(status IN ('active','paused','deleted')),
    last_run    DATETIME,
    last_error  TEXT,
    next_run    DATETIME NOT NULL,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_cron_jobs_next_run ON cron_jobs(next_run) WHERE status='active';

-- +goose Down
DROP TABLE cron_jobs;
