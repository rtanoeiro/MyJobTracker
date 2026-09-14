# MyJobTracker

A full-stack job-application tracker. Register once, log every application you send, watch it move through your pipeline, and keep the details that matter close at hand — all in one place.

**Passwordless.** There are no passwords. You sign in with your email or username; the app issues a 24h session cookie (extended while you're active).

---

## Table of Contents

- [What it does](#what-it-does)
- [Core concepts](#core-concepts)
  - [Statuses](#statuses)
  - [Follow-ups](#follow-ups)
  - [Notes](#notes)
  - [CVs](#cvs)
  - [Stats bar](#stats-bar)
  - [Search & filter](#search--filter)
- [Tech stack](#tech-stack)
- [Getting started](#getting-started)
  - [Prerequisites](#prerequisites)
  - [Configuration](#configuration)
  - [Run locally (dev)](#run-locally-dev)
  - [Run the checks](#run-the-checks)
  - [Database](#database)
- [Contributing](#contributing)
- [Project layout](#project-layout)
- [Roadmap: AI-assisted matching (future)](#roadmap-ai-assisted-matching-future)

---

## What it does

For each job application you record the essentials:

- **Company** — name, website, sector, location, and the recruiter's LinkedIn profile
- **Role** — title, salary, and the original listing link
- **Work model** — Remote, Hybrid, or Onsite
- **Dates** — application date and scheduled interview date
- **Status** — where it sits in your pipeline
- **Last follow-up** — when you last nudged the recruiter

Every application belongs to its user, so two people can use the same instance without ever seeing each other's data. Nothing is ever hard-deleted; removing an application is a soft delete that keeps your history intact.

## Core concepts

### Statuses

Every application moves through a status pipeline. Pick the one that matches reality:

| Status | Meaning |
|---|---|
| `Not Applied` | Saved the listing, haven't sent it yet |
| `Applied` | Application submitted |
| `Processing` | Under review, waiting to hear back |
| `Interview` | Got an interview, actively interviewing |
| `Offer` | Received an offer |
| `Rejected` | This one didn't work out |

Changing a status updates the dashboard live — no page reload needed.

### Follow-ups

A good application sometimes needs a nudge. Each application tracks a **last follow-up contact date** so you always know whether you've already pinged the recruiter (and when). The badge on the list and the inline edit form make it quick to record a follow-up right where you're looking.

### Notes

Applications aren't the whole story. Attach free-form notes (header + body) to remember prep, talking points, salary conversations, or anything else — fully searchable and editable.

### CVs

Keep one or more CVs and edit them in place: profile, experiences (each with achievement bullets and current/previous roles), education, certifications, academic contributions, and extra sections. Every CV is per-user, and any application can point at the CV you used for it.

### Stats bar

At a glance you see live totals: **total applications, interviews in flight, and pending** items. The counts update automatically as your data changes (HTMX).

### Search & filter

Instead of scrolling, filter the list by the criteria that matter right now — company, role, work model, location, status — and narrow in on what needs your attention.

## Tech stack

| Layer | Tech |
|---|---|
| Language | Go 1.27+ |
| Router | chi |
| Database | PostgreSQL 17 (pgx/v5) |
| Query generation | sqlc |
| Migrations | Goose |
| Frontend | Go `html/template` + HTMX |
| Auth | Passwordless (email/username) + session cookies |
| Dev tooling | mise / air |
| Deployment | Docker + Compose |

## Getting started

### Prerequisites

- [mise](https://mise.jdx.dev) (installs Go, sqlc, goose, golangci-lint, air for you)
- Docker + Docker Compose

### Configuration

Copy the template and fill in your values:

```bash
cp .env.example .env
```

Key variables (see `.env.example` for the full list):

| Variable | Purpose |
|---|---|
| `PORT` | HTTP port the app listens on (default `9000`) |
| `POSTGRES_DB_STRING` | Single full PostgreSQL connection string per environment |
| `POSTGRES_DB` / `POSTGRES_USER` / `POSTGRES_PASSWORD` | Used by Docker Compose to spin up Postgres |
| `POSTGRES_HOST_PORT` | Host port Postgres is published on (dev only; prod exposes no host port) |
| `ADMINER_PORT` | Host port for Adminer |
| `GOOSE_DBSTRING` / `GOOSE_MIGRATION_DIR` | Used by host-run Goose to apply dev migrations |

### Run locally (dev)

```bash
mise run up-dev      # start PostgreSQL (and Adminer) containers
mise run migrate     # apply schema migrations (goose up)
air                  # start the app with hot reload
```

`air` (installed for you by mise) watches your Go and template files and rebuilds/restarts the app on every change, so you never restart manually during development. Stop it with `Ctrl-C` and bring the database down with `mise run down-dev` when you're done.

Then open <http://localhost:9000>, register an account, and start logging applications.

### Deploy (prod)

In prod both Postgres and the app run as containers (self-contained; Postgres exposes no host port). All prod values come from a single `.env` file — see [Configuration](#configuration).

#### With Docker Compose (Recommended)

`docker/docker-compose.yml` defines `postgresql-job-applications`, `migrate`, and `job-applications` (plus Adminer). The `migrate` service depends on Postgres being healthy; the app waits on `migrate` completing successfully.

```bash
docker compose -f docker/docker-compose.yml --env-file .env up -d
docker compose -f docker/docker-compose.yml --env-file .env down
```

#### With plain Docker commands

With plain `docker run` there is no Compose to create the network for you. Containers must share a user-defined bridge network to resolve each other by name (Docker's built-in `bridge` network does not do DNS name resolution). This is the one manual step Compose does automatically:

```bash
docker network create jobtracker
```

1. **Postgres**

```bash
docker run -d --name postgresql-job-applications \
  --network jobtracker \
  --restart unless-stopped \
  -v "${LOCAL_DATA_DIR}:/var/lib/postgresql/data" \
  -e POSTGRES_USER="${POSTGRES_USER}" \
  -e POSTGRES_PASSWORD="${POSTGRES_PASSWORD}" \
  -e POSTGRES_DB="${POSTGRES_DB}" \
  -e TZ=UTC \
  postgres:17.9-alpine3.23
```

2. **Run migrations** (one-shot; the image bundles the `goose` binary and `internal/db/schema/`). Let this finish before starting the app — it returns when migrations are applied.

```bash
docker run --rm --network jobtracker \
  -e GOOSE_DRIVER=postgres \
  -e GOOSE_DBSTRING="${POSTGRES_DB_STRING}" \
  -e GOOSE_MIGRATION_DIR=/app/schema \
  job-applications:latest ./goose up
```

3. **Start the app**

```bash
docker run -d --name job-applications \
  --network jobtracker \
  --restart unless-stopped \
  -p 9000:9000 \
  -e POSTGRES_DB_STRING="${POSTGRES_DB_STRING}" \
  -e PORT=9000 \
  -e TZ=UTC \
  job-applications:latest
```

> `mise run up-prod` / `down-prod` are convenience wrappers for the Compose commands above.

## Contributing

### Building

The app is packaged as a Docker image named `job-applications:latest` (the name `docker/docker-compose.yml` expects). Use the `mise run` wrappers so the consistent platform flags are applied:

```bash
mise run build          # build the amd64 image (linux/amd64)
mise run build-arm64    # build the arm64 image
```

The Dockerfile (`docker/Dockerfile`) builds the Go binary, bundles the `goose` migration runner and `internal/db/schema/`, then copies in `static/` and `templates/` for a self-contained runtime image.

### Checks

Before opening a change, run the full suite:

```bash
mise run checks         # clean + fmt + lint + sec + govul + test
```

This runs `gofmt`, `golangci-lint`, `gosec`, `govulncheck`, and the unit tests with coverage — all through the `mise run` wrappers (never run the underlying tools directly).

### Database

Schema changes are handled with Goose migrations in `internal/db/schema/`. Generated query code (sqlc) lives under `internal/db/`.

```bash
mise run migrate          # apply migrations (up)
mise run migrate-down     # WARNING: dev/test only, drops all data
mise run sqlc-generate    # regenerate sqlc code after editing queries
```

## Project layout

```
main.go
internal/
  database/       # pgx pool
  db/             # sqlc models + queries, schema/, queries/, seed/
  handlers/       # one file per feature
  middlewares/    # auth
  session/        # session manager + cookies
  server/         # init + routes
  templates/      # renderer wrapper
templates/        # HTML
static/           # CSS + logo
docker/           # Dockerfiles + compose (dev/test/prod)
```

## Roadmap: AI-assisted matching

A planned enhancement is to use an AI model to help you assess how well you fit the jobs you've applied to — and to spot where you don't yet match the **selection criteria** so you can close the gap before an interview.

**Shipped so far — the groundwork.** You can already choose your AI provider, model, and effort level in **Settings** and store your API key. Only DeepSeek works for now. The data such a feature builds on is in place too: each application stores its listing link and job description, and **CVs** give you a structured profile.

The scoring itself is still future work:

1. **Capture the criteria.** The listing link and job description are already stored per application. A future step would extract the posting's stated requirements (must-haves, nice-to-haves, skills, years of experience, domain) — either by fetching the linked page or from the stored description.

2. **Build your profile.** We'd assemble a profile from your stored data (roles applied to, companies, sector) plus your stored CVs, rather than re-typing per job.

3. **Score the match.** For each application, the AI compares your profile against the posting's selection criteria and returns:
   - an overall fit score (e.g. strong / partial / weak),
   - a per-criterion breakdown of what you meet vs. what's missing,
   - suggested talking points or evidence to strengthen weak areas.

4. **Prioritise your pipeline.** Combine the fit score with your current status so you spend effort where it counts — e.g. prepare more for high-fit interviews, or adjust/customise applications that fall short of must-have criteria.

The exact model, whether criteria are fetched automatically or pasted, and how results are surfaced in the UI are all open design questions to be worked out when the feature is scoped.
