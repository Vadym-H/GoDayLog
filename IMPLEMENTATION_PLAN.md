# GoDayLog Implementation Plan

## ✅ Completed

### 1. **Config System**
   - Created `internal/config/config.go` - Config struct with all required sections:
     - Telegram (bot token)
     - Database (SQLite path)
     - LLM (AI model configuration)
     - Logger (logging level)
     - Server (port configuration)

### 2. **Config Parser**
   - Created `internal/config/parser.go` with:
     - `Load()` - Loads YAML and overrides with env variables
     - `GetConfigPath()` - CLI flag support for custom config path

### 3. **Config File**
   - Updated `config/local.yaml` with all necessary configuration sections

### 4. **Main Integration**
   - Updated `cmd/bot/main.go` to:
     - Load config on startup
     - Log configuration values
     - Include token masking for security

### 5. **Dependencies**
   - Updated `go.mod` with `gopkg.in/yaml.v3` for YAML parsing

---

## 📋 Proposed Next Steps

### Phase 1: Infrastructure & Database (Priority: HIGH)
1. **Create Logger Package**
   - Implement `internal/lib/logger/` with structured logging
   - Use Go's `log/slog` or similar
   - Configure log levels from config

2. **Setup Database Layer**
   - Create `internal/storage/sqlite.go` for database initialization
   - Create database migrations in `migrations/`
   - Schema should include:
     - `users` table (Telegram user tracking)
     - `activities` table (activities log with tags, duration, classification)
     - `daily_reports` table (aggregated stats)

3. **Create Data Models**
   - `internal/models/activity.go` - Activity struct
   - `internal/models/user.go` - User struct
   - `internal/models/report.go` - Daily report struct

### Phase 2: Telegram Bot Integration (Priority: HIGH)
1. **Setup Telegram Bot**
   - Use library like `github.com/go-telegram-bot-api/telegram-bot-api/v5`
   - Create `internal/telegram/bot.go`
   - Implement message handlers in `internal/telegram/handlers/`

2. **Message Processing Pipeline**
   - `internal/telegram/handlers/message_handler.go` - Entry point
   - Route messages to LLM for parsing

### Phase 3: AI/LLM Integration (Priority: MEDIUM)
1. **Create LLM Client**
   - `internal/llm/client.go` - Abstract LLM interface
   - `internal/llm/openai/` - OpenAI implementation
   - `internal/llm/ollama/` - Ollama implementation (local)

2. **Activity Parsing**
   - `internal/llm/parser.go` - Parse activity text into structured format
   - Prompt engineering for consistent JSON output

### Phase 4: Business Logic (Priority: MEDIUM)
1. **Activity Service**
   - `internal/service/activity.go` - Save, retrieve, filter activities
   - Implement activity validation and enrichment

2. **Report Generation**
   - `internal/service/report.go` - Daily/weekly/monthly reports
   - Statistics calculation (useful vs useless activities)

3. **Reminder System**
   - `internal/service/reminder.go` - Scheduled reminders
   - Use Go's `time` or background job library

### Phase 5: API & Testing (Priority: MEDIUM)
1. **HTTP API (Optional - for web dashboard)**
   - REST endpoints for analytics
   - Create `internal/http/handlers/`

2. **Unit & Integration Tests**
   - Write tests for all business logic
   - Mock LLM responses for testing

---

## 🚀 Recommended Execution Order

1. **Day 1**: Logger + Database Setup + Migrations
2. **Day 2**: Data Models + Database CRUD operations
3. **Day 3**: Telegram Bot integration + basic message handling
4. **Day 4**: LLM client setup + activity parsing
5. **Day 5**: Business logic (Activity service + Reports)
6. **Day 6**: Testing + Polish
7. **Day 7**: Deployment setup

---

## 📦 Dependencies to Add

```bash
go get github.com/go-telegram-bot-api/telegram-bot-api/v5
go get github.com/mattn/go-sqlite3
go get gopkg.in/yaml.v3
```

For LLM (choose based on provider):
```bash
go get github.com/openai/openai-go  # For OpenAI
```

---

## 🔧 Configuration Best Practices

- **Local Development**: Use `config/local.yaml` with test credentials
- **Production**: Use `config/prod.yaml` with env variable overrides
- **Run with custom config**: `go run cmd/bot/main.go -config config/prod.yaml`
- **Run with defaults**: `go run cmd/bot/main.go`

---

## ✨ Current Project Structure Ready For

```
GoDayLog/
├── cmd/bot/main.go ✅ (Ready to add bot initialization)
├── config/
│   └── local.yaml ✅ (Configuration template)
├── internal/
│   ├── config/ ✅ (Complete)
│   ├── telegram/ (To be created)
│   ├── llm/ (To be created)
│   ├── storage/ (To be created)
│   ├── service/ (To be created)
│   ├── models/ (To be created)
│   └── lib/logger/ (To be created)
├── migrations/ (To be created)
└── go.mod ✅ (Updated with dependencies)
```

