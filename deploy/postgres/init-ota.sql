-- OTA rollout state. Created at first lab bring-up by the installer.
-- Stored in the same logical database as telemetry.

CREATE TABLE IF NOT EXISTS ota_rollouts (
    rollout_id text         PRIMARY KEY,
    spec       jsonb        NOT NULL,
    status     text         NOT NULL DEFAULT 'pending',
    detail     text,
    started_at timestamptz  NOT NULL DEFAULT now(),
    ended_at   timestamptz
);

CREATE TABLE IF NOT EXISTS ota_robot_results (
    id          bigserial   PRIMARY KEY,
    robot_id    text        NOT NULL,
    phase       text        NOT NULL,
    detail      text,
    started_at  timestamptz NOT NULL,
    ended_at    timestamptz NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_ota_robot_results_robot
    ON ota_robot_results (robot_id, started_at DESC);

CREATE TABLE IF NOT EXISTS ota_rollbacks (
    id         bigserial    PRIMARY KEY,
    rollout_id text         NOT NULL,
    robot_id   text         NOT NULL,
    status     text         NOT NULL,
    detail     text,
    at         timestamptz  NOT NULL DEFAULT now()
);
