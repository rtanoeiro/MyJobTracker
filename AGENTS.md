# Agent Instructions

## Backend Ownership + Socratic Method (CRITICAL)

## This is the most important rule from a design perspective. It is **non-negotiable**.
**Human owns all Go implementation logic.** For any backend logic or design decision, you do not just hand over an answer — it asks guiding questions first (trade-offs, edge cases, "what should happen if X?") to help the human reach their own design, then implements only the agreed skeleton.

This applies to: choosing an approach/pattern, structuring a handler or query, error-handling strategy, data modeling changes. It does **not** apply to templates, CSS, or answering direct factual/lookup questions (docs, syntax, "what does this error mean") — answer those normally.

When writing code for a backend feature:
- Skeleton only — signatures, structs, method stubs, interface additions, route registrations
- Non-trivial logic bodies → `// TODO: implement`
- Templates and CSS are fully implementable, no stub needed

**Write this:**
```go
// PUT /applications/{id}/follow-up
func (h *ApplicationsHandler) UpdateFollowUp(w http.ResponseWriter, r *http.Request) {
    // TODO: parse id from URL param
    // TODO: parse follow_up_date from form
    // TODO: call h.Store.UpdateFollowUpDate(...)
    // TODO: render follow-up-badge template
}
```

**Not this:**
```go
func (h *ApplicationsHandler) UpdateFollowUp(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    idInt, err := utils.ConvertFromStrToInt32(id)
    // ... full implementation
}
```

## Project: MyJobTracker

Full-stack job-application tracker. Users register, log applications, track pipeline status, take notes, and keep CVs.

### Stack

| Layer | Tech |
|---|---|
| Language | Go 1.27+ |
| Router | chi v1.5.5 |
| DB | PostgreSQL 17 via pgx/v5 |
| Query gen | sqlc |
| Migrations | Goose |
| Frontend | Go `html/template` + HTMX v1.9.12 |
| Auth | Passwordless (email/username) + session cookies |
| Dev/reload | mise / air |
| Container | Docker + Compose |

### Layout

```
main.go
internal/
  database/       # pgx pool
  db/             # sqlc models + queries, schema/ (Goose 001–007), queries/, seed/
  handlers/       # one file per feature (home, login, register, applications, notes, cv, user_settings)
  middlewares/    # auth (chi)
  session/
  server/         # init + routes
  templates/      # renderer wrapper
templates/        # HTML
static/           # CSS + logo
docker/           # Dockerfiles + compose (dev/test/prod)
```

### Features

- **Auth**: passwordless register/login/logout via email or username, 24h session, extended on activity
- **Applications**: CRUD + filter — company, role, work model (Remote/Hybrid/Onsite), salary, location, status, dates, recruiter contact
- **Statuses**: `Applied`, `Not Applied`, `Processing`, `Interview`, `Offer`, `Rejected`
- **Notes**: per-user, header + body, full CRUD
- **CVs**: per-user, editable — profile, experiences (with bullets), education, certifications, academic contributions, others; optional per-application `chosen_cv_id`
- **AI settings**: per-user provider/model/effort selection + API key
- **Stats bar**: live total/interview/pending counts (HTMX)
- **Soft deletes**: `deleted_at`, never hard-deleted

### DB Schema

- `users` — id, email, username, timestamps
- `sessions` — id (VARCHAR 64), user_id, ip, user_agent, last_activity, expires_at
- `applications` — id, user_id, company_name, company_website, role, location, sector, work_model (ENUM), salary, date, link, contact_linkedin_profile, status (ENUM), last_follow_up_contact_at, interview_date, job_description, chosen_cv_id (FK → cvs), timestamps
- `notes` — id, user_id, note_header (UNIQUE), note_text, timestamps
- `cvs` — id, user_id, title, profile (JSONB), experiences (JSONB), education (JSONB), certifications (JSONB), academic_contributions (JSONB), others (JSONB), timestamps
- `ai_providers` — name, model, effort_level (UNIQUE combo), description, base_url, enabled
- `user_ai_settings` — user_id (PK), provider_name/model_name/effort_level (FK → ai_providers), key (BYTEA), enabled

### Routes

Public (no auth):

| Method | Path | Description |
|---|---|---|
| GET | `/health` | Health check |
| GET | `/register` | Register page |
| POST | `/register` | Create account |
| GET | `/` | Login page |
| POST | `/login` | Authenticate |

Authenticated (all other routes):

| Method | Path | Description |
|---|---|---|
| GET | `/account/home` | Dashboard / applications page |
| POST | `/logout` | Logout |
| GET | `/stats/bar` | Stats bar fragment (HTMX) |
| GET | `/applications/list` | Applications list fragment (HTMX) |
| GET | `/applications/filter` | Search & filter |
| POST | `/applications` | Create application |
| DELETE | `/applications/{id}` | Soft-delete application |
| GET | `/applications/{id}/edit` | Edit form |
| PUT | `/applications/{id}` | Update application |
| GET | `/applications/new` | New application form |
| GET | `/applications/{id}/follow-up` | Follow-up badge (HTMX fragment) |
| GET | `/applications/{id}/follow-up/edit` | Inline follow-up form (HTMX fragment) |
| PUT | `/applications/{id}/follow-up` | Update `last_follow_up_contact_at`, returns badge |
| GET | `/account/notes` | Notes page |
| GET | `/notes/list` | Notes list fragment (HTMX) |
| GET | `/notes/new` | New note form |
| POST | `/notes` | Create note |
| GET | `/notes/{id}/edit` | Edit form |
| PUT | `/notes/{id}` | Update note |
| DELETE | `/notes/{id}` | Delete note |
| GET | `/account/cvs` | CVs page |
| GET | `/cvs/list` | CV list fragment (HTMX) |
| GET | `/cvs/new` | New CV form |
| POST | `/cvs` | Create CV |
| GET | `/cvs/{id}/edit` | Edit CV form |
| PUT | `/cvs/{id}` | Update CV |
| DELETE | `/cvs/{id}` | Delete CV |
| POST | `/cvs/snippets/{section}/add` | Add snippet block (HTMX fragment) |
| DELETE | `/cvs/snippets/{section}/{index}` | Remove snippet block |
| GET | `/account/settings` | Settings page |
| PUT | `/account/username` | Update username |
| PUT | `/account/email` | Update email |
| PUT | `/account/settings/ai` | Update AI settings |
| DELETE | `/account/settings/ai/key` | Remove saved AI API key |
| GET | `/account/settings/ai/config` | AI config fields fragment (HTMX) |
| GET | `/account/settings/ai/efforts` | AI effort options fragment (HTMX) |

### Env Vars

```
PORT                    # default 9000
POSTGRES_DB_STRING      # full connection string
POSTGRES_DB / POSTGRES_USER / POSTGRES_PASSWORD
GOOSE_DRIVER            # always "postgres"
GOOSE_DBSTRING
GOOSE_MIGRATION_DIR     # ./internal/db/schema/
```

Config: `.env`, `.env.example` (template).

---

## Development

Run all `mise run` commands from project root.

```bash
mise run test             # unit tests
```

Never run `go build`, `go test`, `golangci-lint`, `gosec`, `staticcheck`, `templ`, `sqlc`, etc. directly — always the `mise run` wrapper, for consistent config/flags.

## Testing Standards

Write tests for **behaviour**, not snapshots of the code.

**Skip:**
- Struct-field-matches-hardcoded-default tests
- Tests that mirror implementation line-by-line
- Standard library behavior (`os.Getenv`, `time.ParseDuration`, etc.)
- Simple passthrough/assignment with no branching
- Log-output field assertions

**Write:**
- Branching logic, edge cases, invalid/missing input
- Shared helpers tested directly, not only via call sites
- Error paths, boundary conditions
- Business rules, precedence, override logic
- Smoke tests for panic-prone input

**Rule of thumb:** if this test didn't exist, would a real bug slip through? If no, skip it.

## MCP Servers

**Context7**: current library docs. Add "use context7" to prompt when you need up-to-date docs.

## Git Rules (CRITICAL)

**Stealth mode enabled** — no git network operations.

- `git commit` only (clear message, no Claude references)
- Never `git push` / `git pull` — human handles remote sync