-- +goose Up
-- SQLite cannot ALTER CHECK constraints; recreate the table with url added.
CREATE TABLE cron_jobs_new (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    schedule    TEXT NOT NULL,
    job_type    TEXT NOT NULL CHECK(job_type IN ('rpc','action','shell','url')),
    payload     TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'active'
                CHECK(status IN ('active','paused','deleted')),
    last_run    DATETIME,
    last_error  TEXT,
    next_run    DATETIME NOT NULL,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO cron_jobs_new SELECT * FROM cron_jobs;
DROP TABLE cron_jobs;
ALTER TABLE cron_jobs_new RENAME TO cron_jobs;
CREATE INDEX idx_cron_jobs_next_run ON cron_jobs(next_run) WHERE status='active';

-- +goose Down
CREATE TABLE cron_jobs_new (
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
INSERT INTO cron_jobs_new SELECT * FROM cron_jobs WHERE job_type != 'url';
DROP TABLE cron_jobs;
ALTER TABLE cron_jobs_new RENAME TO cron_jobs;
CREATE INDEX idx_cron_jobs_next_run ON cron_jobs(next_run) WHERE status='active';
