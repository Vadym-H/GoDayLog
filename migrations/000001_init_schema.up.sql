CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- USERS
CREATE TABLE users (
                       id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
                       llm_context  TEXT,
                       timezone     TEXT        NOT NULL DEFAULT 'UTC',
                       deleted_at   TIMESTAMPTZ,
                       created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- USER EXTERNAL IDENTITIES (Telegram, future REST, etc.)
CREATE TABLE user_identities (
                                 id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
                                 user_id      UUID        NOT NULL REFERENCES users(id),
                                 provider     TEXT        NOT NULL,  -- e.g. 'telegram', 'google', 'api_key'
                                 external_id  TEXT        NOT NULL,  -- e.g. telegram user id
                                 created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                                 UNIQUE (provider, external_id)
);

-- MESSAGES
CREATE TABLE messages (
                          id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
                          user_id             UUID        NOT NULL REFERENCES users(id),
                          external_message_id TEXT,
                          text                TEXT        NOT NULL,
                          status              TEXT        NOT NULL DEFAULT 'pending'
                              CHECK (status IN ('pending', 'processing', 'done', 'failed')),
                          error               TEXT,
                          created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ACTIVITY LOGS
CREATE TABLE activity_logs (
                               id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
                               message_id       UUID        NOT NULL REFERENCES messages(id),
                               user_id          UUID        NOT NULL REFERENCES users(id),
                               description      TEXT        NOT NULL,
                               tag              TEXT        NOT NULL,
                               is_useful        BOOLEAN     NOT NULL,
                               duration_minutes INT         CHECK (duration_minutes > 0),
                               started_at       TIMESTAMPTZ,  -- intentionally nullable: user may omit
                               deleted_at       TIMESTAMPTZ,
                               created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- INDEXES
CREATE INDEX idx_user_identities_provider_external
    ON user_identities(provider, external_id);

CREATE INDEX idx_messages_user_id
    ON messages(user_id);

CREATE INDEX idx_messages_status
    ON messages(status) WHERE status IN ('pending', 'processing');

CREATE INDEX idx_messages_external_message_id
    ON messages(external_message_id) WHERE external_message_id IS NOT NULL;

CREATE INDEX idx_activity_logs_user_id
    ON activity_logs(user_id);

CREATE INDEX idx_activity_logs_started_at
    ON activity_logs(started_at);

CREATE INDEX idx_activity_logs_tag
    ON activity_logs(tag);

CREATE INDEX idx_activity_logs_message_id
    ON activity_logs(message_id);

-- Soft-delete filtering indexes
CREATE INDEX idx_users_deleted_at
    ON users(deleted_at) WHERE deleted_at IS NULL;

CREATE INDEX idx_activity_logs_deleted_at
    ON activity_logs(deleted_at) WHERE deleted_at IS NULL;