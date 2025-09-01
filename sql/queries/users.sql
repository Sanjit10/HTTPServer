-- name: CreateUser :one
INSERT INTO users (id, created_at, updated_at, email)
VALUES (
    gen_random_uuid(),
    NOW(),
    NOW(),
    $1
)
RETURNING *;

-- name: DeleteAllUsers :exec
DELETE FROM users;

-- name: SetUserPassword :exec
UPDATE users
SET hashed_password = $1,
    updated_at = now()
WHERE id = $2;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = $1;

-- name: GetUserById :one
SELECT * FROM users
WHERE id = $1;

-- name: UpdateUser :exec
UPDATE users
SET email = $1,
    updated_at = now(),
    hashed_password = $2
WHERE id = $3;


-- name: SetUserIsChirpyRed :exec
UPDATE users
SET is_chirpy_red = $1
WHERE id = $2;