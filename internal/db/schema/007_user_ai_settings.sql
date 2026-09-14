-- +goose Up
CREATE TABLE user_ai_settings (
    user_id INT PRIMARY KEY REFERENCES users(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    provider_name VARCHAR(255) NOT NULL,
    model_name VARCHAR(255) NOT NULL,
    effort_level VARCHAR(255) NOT NULL,
    key TEXT,
    enabled BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_user_ai_settings_offering
        FOREIGN KEY (provider_name, model_name, effort_level)
        REFERENCES ai_providers (name, model, effort_level)
        ON DELETE RESTRICT
        ON UPDATE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_user_ai_settings_provider_model_effort
    ON user_ai_settings (provider_name, model_name, effort_level);

-- +goose Down
DROP TABLE IF EXISTS user_ai_settings;
