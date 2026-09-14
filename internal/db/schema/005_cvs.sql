-- +goose Up
CREATE TABLE cvs (
    id SERIAL PRIMARY KEY,
    user_id INT NOT NULL REFERENCES users(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    title VARCHAR(255) NOT NULL,
    summary TEXT,
    profile JSONB NOT NULL DEFAULT '{}'::jsonb,
    experiences JSONB NOT NULL DEFAULT '{}'::jsonb,
    education TEXT,
    certifications TEXT,
    academic_contributions TEXT,
    skills TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL
);

CREATE INDEX IF NOT EXISTS idx_cvs_user_id ON cvs (user_id);
CREATE INDEX IF NOT EXISTS idx_cvs_title ON cvs (title);
CREATE INDEX IF NOT EXISTS idx_cvs_created_at ON cvs (created_at);
CREATE INDEX IF NOT EXISTS idx_cvs_updated_at ON cvs (updated_at);
CREATE INDEX IF NOT EXISTS idx_cvs_deleted_at ON cvs (deleted_at);

ALTER TABLE applications
    ADD CONSTRAINT fk_applications_chosen_cv_id
        FOREIGN KEY (chosen_cv_id)
        REFERENCES cvs (id)
        ON DELETE RESTRICT
        ON UPDATE CASCADE;

-- +goose Down
ALTER TABLE applications DROP CONSTRAINT IF EXISTS fk_applications_chosen_cv_id;

DROP TABLE IF EXISTS cvs;
