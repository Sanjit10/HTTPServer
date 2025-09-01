-- +goose Up

CREATE TABLE refresh_token (
    token TEXT PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL DEFAULT now() + interval '1 hour',
    revoked_at TIMESTAMPTZ NULL
);

-- +goose Down
DROP TABLE IF EXISTS refresh_token;