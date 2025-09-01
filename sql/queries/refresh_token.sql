-- name: CreateRefreshToken :one
INSERT INTO refresh_token (token, user_id, expires_at)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetValidToken :one
SELECT * FROM refresh_token
WHERE token = $1 AND expires_at > now() AND revoked_at IS NULL
LIMIT 1;

-- name: RevokeToken :exec
UPDATE refresh_token
SET revoked_at = now(), updated_at = now()
WHERE token = $1 AND revoked_at IS NULL;
