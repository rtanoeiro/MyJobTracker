-- name: CreateUser :one
INSERT INTO users (
    email, username
) VALUES (
    $1, $2
)
RETURNING *;

-- name: GetUserByEmailOrUsername :one
SELECT * FROM users
WHERE (LOWER(email) = $1 OR LOWER(username) = $1)
    AND deleted_at IS NULL;

-- name: UserExists :one
SELECT EXISTS(SELECT 1 FROM users WHERE email = $1);

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: UpdateUsername :exec
UPDATE users
SET username = $2,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1;

-- name: UpdateUserEmail :exec
UPDATE users
SET email = $2,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1;