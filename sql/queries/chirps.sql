-- name: CreateChirps :one
INSERT INTO chirps (body, user_id)
VALUES ($1, $2)
RETURNING *;

-- name: GetAllChirps :many
SELECT * FROM chirps
ORDER BY
    CASE WHEN $1 = 'asc' THEN created_at END ASC,
    CASE WHEN $1 = 'desc' THEN created_at END DESC;

-- name: GetChirp :one
SELECT * FROM chirps WHERE id = $1;

-- name: DeleteChirp :exec
DELETE FROM chirps WHERE id = $1;

-- name: GetChirpsByUserID :many
SELECT * FROM chirps
WHERE user_id = $1
ORDER BY
    CASE WHEN $2 = 'asc' THEN created_at END ASC,
    CASE WHEN $2 = 'desc' THEN created_at END DESC;