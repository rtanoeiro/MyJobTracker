-- name: ListEnabledProviders :many
SELECT
    name,
    model,
    effort_level,
    description,
    base_url,
    enabled
FROM ai_providers
WHERE enabled = true
ORDER BY name, model, effort_level;

-- name: ListModelsByProviderName :many
SELECT DISTINCT model
FROM ai_providers
WHERE enabled = true AND name = $1
ORDER BY model;

-- name: ListEffortsByProviderAndModel :many
SELECT DISTINCT effort_level
FROM ai_providers
WHERE enabled = true AND name = $1 AND model = $2
ORDER BY effort_level;

-- name: GetModelInformationByNameAndEffort :one
SELECT
    name,
    model,
    effort_level,
    base_url
FROM ai_providers
WHERE name = $1 AND model = $2 AND effort_level = $3;
