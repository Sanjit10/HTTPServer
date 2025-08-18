-- +goose Up

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE TABLE users (
    id UUID PRIMARY KEY
        DEFAULT uuid_generate_v4(),
    created_at TIMESTAMPTZ NOT NULL
        DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL
        DEFAULT now(),
    email TEXT NOT NULL
        UNIQUE
);

-- +goose Down
DROP TABLE IF EXISTS users;