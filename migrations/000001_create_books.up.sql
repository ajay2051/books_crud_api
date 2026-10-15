CREATE TABLE IF NOT EXISTS books (
    id         BIGSERIAL    PRIMARY KEY,
    title      TEXT         NOT NULL,
    author     TEXT         NOT NULL,
    year       INT          NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);