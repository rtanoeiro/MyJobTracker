-- name: GetAllUserApplications :many
SELECT
    id,
    user_id,
    company_name,
    company_website,
    role,
    work_model,
    sector,
    location,
    salary,
    date,
    link,
    contact_linkedin_profile,
    status,
    last_follow_up_contact_at,
    interview_date
FROM applications
WHERE user_id = $1 AND deleted_at IS NULL;

-- name: GetAllUserApplicationsByStatus :many
SELECT
    id,
    user_id,
    company_name,
    company_website,
    role,
    work_model,
    sector,
    location,
    salary,
    date,
    link,
    contact_linkedin_profile,
    status,
    last_follow_up_contact_at,
    interview_date
FROM applications
WHERE user_id = $1
AND status = $2
AND deleted_at IS NULL;

-- name: GetAllUserApplicationsByNameOrCompany :many
SELECT
    id,
    user_id,
    company_name,
    company_website,
    role,
    work_model,
    sector,
    location,
    salary,
    date,
    link,
    contact_linkedin_profile,
    status,
    last_follow_up_contact_at,
    interview_date
FROM applications
WHERE user_id = $1
AND (LOWER(company_name) LIKE '%' || $2 || '%' OR role LIKE '%' || $2 || '%')
AND deleted_at IS NULL;

-- name: GetAllUserApplicationsByAllFilters :many
SELECT
    id,
    user_id,
    company_name,
    company_website,
    role,
    work_model,
    sector,
    location,
    salary,
    date,
    link,
    contact_linkedin_profile,
    status,
    last_follow_up_contact_at,
    interview_date
FROM applications
WHERE user_id = $1
AND status = $2
AND (LOWER(company_name) LIKE '%' || $3 || '%' OR role LIKE '%' || $3 || '%')
AND deleted_at IS NULL;

-- name: GetApplicationByID :one
SELECT
    id,
    user_id,
    company_name,
    company_website,
    role,
    work_model,
    sector,
    location,
    salary,
    date,
    link,
    contact_linkedin_profile,
    status,
    last_follow_up_contact_at,
    interview_date
FROM applications
WHERE id = $1 AND deleted_at IS NULL;


-- name: CreateApplication :one
INSERT INTO applications (
    user_id, company_name, company_website, role, work_model, sector, location, salary, date, link, contact_linkedin_profile, status, interview_date
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
) RETURNING id;

-- name: UpdateApplication :exec
UPDATE applications SET
    company_name = $2,
    company_website = $3,
    role = $4,
    work_model = $5,
    sector = $6,
    location = $7,
    salary = $8,
    date = $9,
    link = $10,
    contact_linkedin_profile = $11,
    status = $12,
    last_follow_up_contact_at = $13,
    interview_date = $14
WHERE id = $1 AND user_id = $15;

-- name: UpdateFollowUpDate :exec
UPDATE applications SET last_follow_up_contact_at = $2 WHERE id = $1 AND user_id = $3;

-- name: DeleteApplication :exec
UPDATE applications SET deleted_at = NOW() WHERE id = $1 AND user_id = $2;

-- name: IsOwner :one
SELECT EXISTS(SELECT 1 FROM applications WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL);

-- name: CountNumberApplications :one
SELECT COUNT(*) FROM applications WHERE user_id = $1 AND deleted_at IS NULL;

-- name: CountNumberInterviews :one
SELECT COUNT(*) FROM applications WHERE user_id = $1 AND status = 'Interview' AND deleted_at IS NULL;

-- name: CountNumberPending :one
SELECT COUNT(*) FROM applications WHERE user_id = $1 AND status = 'Applied' AND deleted_at IS NULL;