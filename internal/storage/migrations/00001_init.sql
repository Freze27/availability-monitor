-- +goose Up
CREATE TABLE targets (
    id               INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name             TEXT        NOT NULL CHECK (btrim(name) <> ''),
    url              TEXT        NOT NULL,
    interval_seconds INTEGER     NOT NULL CHECK (interval_seconds > 0),
    paused           BOOLEAN     NOT NULL DEFAULT false,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE checks (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    target_id   INTEGER     NOT NULL REFERENCES targets (id) ON DELETE CASCADE,
    duration_ms INTEGER     NOT NULL CHECK (duration_ms >= 0),
    checked_at  TIMESTAMPTZ NOT NULL,
    outcome     TEXT        NOT NULL CHECK (outcome IN ('up', 'http_error', 'timeout', 'network_error')),
    status_code INTEGER,
    error       TEXT,
    CONSTRAINT checks_outcome_status_code CHECK (
        (outcome = 'up' AND status_code BETWEEN 200 AND 299)
        OR (outcome = 'http_error' AND status_code IS NOT NULL AND status_code NOT BETWEEN 200 AND 299)
        OR (outcome IN ('timeout', 'network_error') AND status_code IS NULL)
    )
);

CREATE INDEX checks_target_id_checked_at_idx ON checks (target_id, checked_at DESC);

CREATE TABLE events (
    id          INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    target_id   INTEGER     NOT NULL REFERENCES targets (id) ON DELETE CASCADE,
    occurred_at TIMESTAMPTZ NOT NULL,
    kind        TEXT        NOT NULL CHECK (kind IN ('up', 'down'))
);

CREATE INDEX events_occurred_at_idx ON events (occurred_at DESC);
CREATE INDEX events_target_id_idx ON events (target_id);

-- +goose Down
DROP TABLE events;
DROP TABLE checks;
DROP TABLE targets;
