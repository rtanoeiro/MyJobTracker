-- +goose Up
CREATE TYPE typ_application_status AS ENUM ('Applied', 'Not Applied', 'Processing', 'Interview', 'Offer', 'Rejected');
CREATE TYPE typ_work_model AS ENUM ('Remote', 'Hybrid', 'Onsite', 'Not Specified');

CREATE TABLE applications (
    id SERIAL PRIMARY KEY,
    user_id INT NOT NULL REFERENCES users(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    company_name VARCHAR(255) NOT NULL,
    company_website TEXT,
    role VARCHAR(255) NOT NULL,
    work_model VARCHAR(255) NOT NULL,
    sector VARCHAR(255),
    location VARCHAR(255),
    salary VARCHAR(255),
    date DATE NOT NULL,
    link VARCHAR(255),
    contact_linkedin_profile TEXT,
    status typ_application_status NOT NULL DEFAULT 'Applied',
    last_follow_up_contact_at DATE NULL DEFAULT NULL,
    interview_date DATE NULL,
    job_description TEXT NULL,
    chosen_cv_id INT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL
);

CREATE INDEX IF NOT EXISTS idx_applications_user_id ON applications (user_id);
CREATE INDEX IF NOT EXISTS idx_applications_date ON applications (date);
CREATE INDEX IF NOT EXISTS idx_applications_created_at ON applications (created_at);
CREATE INDEX IF NOT EXISTS idx_applications_updated_at ON applications (updated_at);
CREATE INDEX IF NOT EXISTS idx_applications_deleted_at ON applications (deleted_at);
CREATE INDEX IF NOT EXISTS idx_applications_interview_date ON applications (interview_date);
CREATE INDEX IF NOT EXISTS idx_applications_chosen_cv_id ON applications (chosen_cv_id);

-- +goose Down
DROP TABLE IF EXISTS applications;

DROP TYPE IF EXISTS typ_application_status;
DROP TYPE IF EXISTS typ_work_model;
