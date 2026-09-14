-- name: CreateSession :execresult
INSERT INTO sessions (id, user_id, ip_address, user_agent, created_at, last_activity, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: GetSessionByID :one
SELECT * FROM sessions WHERE id = $1 AND expires_at > NOW();

-- name: UpdateSessionActivity :exec
UPDATE sessions SET last_activity = $1, expires_at = $2 WHERE id = $3;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE id = $1;
