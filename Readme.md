# GoDayLog

**Turn your daily notes into structured self-knowledge — powered by AI, owned by you.**

GoDayLog is a personal activity tracker that lives in Telegram. You describe your day in plain text, the AI extracts structured activity records, and you review everything before it gets saved. No apps to install, no forms to fill in. Just write.

> **Try it live → [@day_logbot](https://t.me/day_logbot)**
> Hosted on AWS EC2. Free to use.

---

## Why GoDayLog

Most productivity tools make you fill out forms. GoDayLog flips this — you describe your day the way you'd tell a friend, and the AI does the structuring.

No habits to build around the tool. No friction. You already know how to write a message.

---

## How it works

1. Send a message: `"Worked on the API refactor for 3 hours, then went for a run, watched YouTube for way too long"`
2. The AI splits this into individual activities, classifies each one, estimates duration, and extracts timestamps if you mentioned them
3. You see a structured review screen — modify any field (tag, type, start time) before accepting
4. Activities are saved to your personal log

**Activity types** give you honest clarity about where your time actually goes:

| Type | What it means |
|---|---|
| `growth` | Working toward your goals — learning, building, creating |
| `routine` | Necessary daily tasks — cooking, chores, errands |
| `rest` | Deliberate rest — walks, naps, time with people |
| `drain` | Passive consumption — social media, TV binges, mindless scrolling |

You can hint the AI directly: `"played guitar for an hour — growth"`. It respects your classification.

---

## Features

### Activity logging
- **Natural language input** — describe your day however feels natural, no structure required
- **Multi-activity extraction** — one message can describe five different things, each gets its own record
- **Human-in-the-loop review** — the AI never saves anything without your confirmation; modify tag, type, duration, or start time per activity before accepting
- **Flexible time input** — supports natural date/time: `"today 14:00"`, `"yesterday"`, `"3 days ago"`, `"28 april 10:00"`
- **Personalized AI context** — tell the bot about your goals and routine; it uses this context to classify your activities more accurately; editable anytime from settings

### Statistics
- **Today / this week / last 7 days / this month** — one tap to see any range
- **Custom date range** — enter any range you want
- **Breakdown by type** — time and percentage per activity type with visual progress bars
- **Top tags** — your most-used tags ranked
- **Daily breakdown** — see how activity was distributed across individual days within a range
- **Active period** — when you first logged and last completed an activity in the range

### AI analysis
- **Analyse any stats range** — ask the AI for insights on your patterns, not just raw numbers
- **Context-aware** — analysis is personalized to your goals and what you told the bot about yourself
- **Daily token budget** — per-user limits keep AI costs predictable; you see a clear message when the budget is reached

### Settings & personalization
- **Timezone** — auto-detect from location on onboarding, or set manually; all dates and stats respect it
- **Personal context** — update your context at any time from settings

---

## Architecture

GoDayLog is built so the Telegram bot is one transport layer. The core domain — users, messages, activity logs — knows nothing about Telegram. A future REST API or OAuth client would plug in without touching the schema or business logic.

```
Telegram update
    └── internal/telegram/bot           (routing, per-request middleware)
            └── internal/telegram/tghandlers  (thin adapters — translate updates to service calls)
                    └── internal/services     (business logic, interface-driven)
                            └── internal/storage/postgres  (only place that touches the DB)
```

**Key design choices:**

**Provider-agnostic schema.** The `users` table has no Telegram columns. Telegram is one row in `user_identities` with `provider = 'telegram'`. Adding Google login or API keys is a new row, not a schema change.

**Interface-driven services.** Every service declares its own small interfaces for its dependencies (`UserCreator`, `UserContext`, `MessageRepository`…). The concrete Postgres storage satisfies them — independently testable and swappable without touching handlers.

**No `ON DELETE CASCADE`.** Deletes are soft (`deleted_at TIMESTAMPTZ`). Activity logs trace back to the message they came from. Data is preserved for analytics and account restore.

**Token usage logging.** Every AI call records model, prompt tokens, completion tokens, and total against the originating message. Daily budget checks happen before the LLM call — not after.

**Prompt injection defense.** The system prompt explicitly defines what counts as injection versus valid user input. The model returns `valid: false` on detected injection without exposing prompt contents.

**Request-scoped logging.** Every Telegram update gets a `request_id` injected into its context on entry. All log lines for a single request share this ID — makes tracing across service and storage layers straightforward.

---

## Infrastructure & CI/CD

The bot runs on **AWS EC2** behind Docker Compose. A two-stage pipeline keeps deployments safe and fast:

**CI** — triggered on every push to any branch:
- `go build ./...` — fails fast on compilation errors
- `go vet ./...` — catches common correctness issues
- `staticcheck` — deep static analysis
- `govulncheck` — scans dependencies for known vulnerabilities

**CD** — triggered on push to `main` after CI passes:
- GitHub Actions builds the Docker image and pushes it to **GitHub Container Registry** (ghcr.io) — compilation happens in CI, not on the server
- SSH into EC2, pull the pre-built image, run any new migrations, restart the container
- Zero compilation on the VPS — deploys in under a minute

This means the production server never runs `go build`. It only pulls and runs a tested, pre-built image.

---

## Stack

| Layer | Technology |
|---|---|
| Language | Go |
| Bot framework | go-telegram/bot |
| Database | PostgreSQL (pgxpool) |
| Migrations | golang-migrate |
| AI | OpenAI-compatible chat completions API |
| Config | YAML + env vars (cleanenv + godotenv) |
| Logging | slog (structured JSON) |
| Hosting | AWS EC2 + Docker Compose |
| Registry | GitHub Container Registry (ghcr.io) |
| CI/CD | GitHub Actions |
| Dev tooling | Task runner, Docker Compose with hot reload |

The AI client is a thin HTTP wrapper over any OpenAI-compatible endpoint. Model, base URL, and timeout are all config — swap providers without touching code.

---

## Running it locally

**Requirements:** Docker, Task runner, a Telegram bot token, an LLM API key.

```bash
# First run — build and start dev compose (hot reload via bind mount)
task dev:up:build

# Watch logs
task dev:logs

# Run migrations
task migrate:up

# Open a psql shell into the DB
task db
```

Config is loaded from a YAML file pointed to by `CONFIG_PATH`. A `.env` file is auto-loaded if present.

Required env vars: `TELEGRAM_TOKEN`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `LLM_API_KEY`.

---

## About

Built by **Vadym Hushchin** — a backend developer focused on Go, clean service architecture, and tools that are actually worth using day-to-day.

- GitHub: [@Vadym-H](https://github.com/Vadym-H)
- Bot: [@day_logbot](https://t.me/day_logbot)
