-- name: GetAllUserCVs :many
SELECT
    id,
    user_id,
    title,
    summary,
    profile,
    experiences,
    education,
    certifications,
    academic_contributions,
    skills,
    created_at,
    updated_at
FROM cvs
WHERE user_id = $1 AND deleted_at IS NULL
ORDER BY created_at DESC;

-- name: GetCVByID :one
SELECT
    id,
    user_id,
    title,
    summary,   
    profile,
    experiences,
    education,
    certifications,
    academic_contributions,
    skills,
    created_at,
    updated_at
FROM cvs
WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL;

-- name: CreateCV :one
INSERT INTO cvs (
    user_id,
    title,
    summary,
    profile,
    experiences,
    education,
    certifications,
    academic_contributions,
    skills
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
) RETURNING id;

-- name: UpdateCV :exec
UPDATE cvs SET
    title = $2,
    summary = $3,
    profile = $4,
    experiences = $5,
    education = $6,
    certifications = $7,
    academic_contributions = $8,
    skills = $9,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND user_id = $10 AND deleted_at IS NULL;

-- name: DeleteCV :exec
UPDATE cvs SET
    deleted_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND user_id = $2;

-- name: IsCVOwner :one
SELECT EXISTS(
    SELECT 1 FROM cvs
    WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
);
