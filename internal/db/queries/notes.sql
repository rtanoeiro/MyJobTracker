-- name: GetAllUserNotes :many
SELECT * FROM notes
WHERE user_id = $1
AND deleted_at IS NULL
ORDER BY created_at DESC;

-- name: GetNoteByID :one
SELECT * FROM notes
WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL;

-- name: CreateNote :one
INSERT INTO notes (user_id, note_header, note_text)
VALUES ($1, $2, $3)
RETURNING *;

-- name: UpdateNote :exec
UPDATE notes
SET note_header = $2,
    note_text = $3,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND user_id = $4;

-- name: DeleteNote :exec
UPDATE notes
SET deleted_at = CURRENT_TIMESTAMP
WHERE id = $1 AND user_id = $2;