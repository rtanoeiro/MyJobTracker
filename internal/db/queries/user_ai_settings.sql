-- name: GetUserAISettings :one
SELECT
    user_id,
    provider_name,
    model_name,
    effort_level,
    key,
    enabled,
    created_at,
    updated_at
FROM user_ai_settings
WHERE user_id = $1;

-- name: UpsertUserAISettings :exec
INSERT INTO user_ai_settings (
    user_id,
    provider_name,
    model_name,
    effort_level,
    key,
    enabled
) VALUES (
    $1, $2, $3, $4, $5, $6
)
ON CONFLICT (user_id) DO UPDATE SET
    provider_name = EXCLUDED.provider_name,
    model_name = EXCLUDED.model_name,
    effort_level = EXCLUDED.effort_level,
    key = EXCLUDED.key,
    enabled = EXCLUDED.enabled,
    updated_at = CURRENT_TIMESTAMP;
