# GoDayLog

**Turn your daily notes into structured self-knowledge — powered by AI, owned by you.**

GoDayLog is a personal activity tracker that lives in Telegram. You describe your day in plain text, the AI extracts structured activity records, and you review everything before it gets saved. No apps to install, no forms to fill in. Just write.

---

## What it does

Most productivity tools make you fill out forms. GoDayLog flips this — you describe your day as naturally as you would to a friend, and the AI does the structuring.

**The flow:**

1. You send a message like: `"Worked on the API refactor for about 3 hours, then went for a run, watched YouTube for way too long"`
2. The AI splits this into individual activities, classifies each one, estimates duration, and extracts timestamps if you mentioned any
3. You see a structured review screen and can modify any field — tag, type, start time — before accepting
4. Once you accept, the activities are saved to your personal log

**Activity types** give you honest clarity about how you spend your time:
- `growth` — learning, working toward your goals, self-development
- `routine` — necessary daily tasks: cooking, chores, errands
- `rest` — low-effort leisure, relaxation, short breaks
- `drain` — passive entertainment: social media, gaming, TV binges

You can hint the AI directly in your message: `"played guitar for an hour — growth"`. It respects your classification.

---

## Features

**Natural language input**
No dropdowns, no categories to pick upfront. Describe your day in whatever way feels natural. The AI handles the parsing.

**Multi-activity extraction**
A single message can describe five different things. Each gets its own record with its own tag, type, duration, and timestamps.

**Human-in-the-loop review**
The AI never saves anything without your confirmation. After extraction, you see what it found and can modify tag, activity type, or start time before accepting — or cancel the whole thing.

**Personalized AI context**
You can provide a short personal context (your goals, routine, what "useful" means to you). The AI uses it to better classify your specific activities. It stays editable and is applied on every future request.

**Today stats** *(in progress)*
A summary of your logged day: total activities, breakdown by type.

**Token budget controls**
Per-user daily token limits and max input size — keeps AI costs predictable when running for multiple users.

**Audit trail**
Every message and its processing status (`pending → done / failed`) is preserved. Activity logs trace back to the originating message. Nothing is hard-deleted — soft deletes let you restore data.

---

## Architecture

GoDayLog is built so the Telegram bot is just one transport layer. The core domain — users, messages, activity logs — knows nothing about Telegram. A future REST API or OAuth client would plug in without touching the schema or business logic.

```
Telegram update
    └── internal/telegram/bot           (routing, per-request middleware)
            └── internal/telegram/tghandlers  (thin adapters — translate updates to service calls)
                    └── internal/services     (business logic, interface-driven)
                            └── internal/storage/postgres  (only place that touches the DB)
```

**Key design choices:**

**Provider-agnostic schema.** The `users` table has no Telegram columns. Telegram is one row in `user_identities` with `provider = 'telegram'`. Adding Google login or API keys is a new row, not a schema change.

**Interface-driven services.** Every service declares its own small interfaces for its dependencies (`UserCreator`, `UserContext`, `MessageRepository`…). The concrete Postgres storage satisfies them. This makes services independently testable and swappable without touching handlers.

**No `ON DELETE CASCADE`.** Deletes are soft (`deleted_at TIMESTAMPTZ`). Activity logs trace back to the message they came from. Data is preserved for analytics and account restore.

**Token usage logging.** Every AI call records model, prompt tokens, completion tokens, and total tokens against the originating message. Daily budget checks happen before calling the LLM — not after.

**Prompt injection defense.** The system prompt explicitly defines what counts as injection (changing output schema, leaking the prompt, changing format) versus valid user input (classifying their own activities). The model returns `valid: false` on detected injection without exposing prompt contents.

**Request-scoped logging.** Every Telegram update gets a `request_id` injected into its context on entry. All log lines for a single request share this ID — makes tracing across service and storage layers straightforward.

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
| Dev tooling | Task runner, Docker Compose with hot reload |

The AI client is a thin HTTP wrapper over any OpenAI-compatible endpoint. Model, base URL, and timeout are all config — swap providers without touching code.

---

## Running it

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

## Project status

Core loop is complete: register user → log activity text → AI extraction → human review → save to DB. Full stats/reporting and a REST interface are the next planned additions.

The foundation is intentionally built to support non-Telegram clients without schema or service changes.
