CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- users
CREATE TABLE users (
                       id         BIGINT PRIMARY KEY,
                       username   TEXT,
                       first_name TEXT        NOT NULL,
                       timezone   TEXT        NOT NULL DEFAULT 'UTC',
                       created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- messages
CREATE TABLE messages (
                          id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
                          user_id             BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                          telegram_message_id BIGINT      NOT NULL,
                          text                TEXT        NOT NULL,
                          processed           BOOLEAN     NOT NULL DEFAULT FALSE,
                          error               TEXT,
                          created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),

                          UNIQUE (user_id, telegram_message_id)
);

-- activity_logs
CREATE TABLE activity_logs (
                               id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
                               message_id       UUID        NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
                               user_id          BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                               description      TEXT        NOT NULL,
                               tag              TEXT        NOT NULL,
                               is_useful        BOOLEAN     NOT NULL,
                               duration_minutes INT         NOT NULL CHECK (duration_minutes > 0),
                               started_at       TIMESTAMPTZ NOT NULL,
                               created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- indexes
CREATE INDEX idx_messages_user_id          ON messages(user_id);
CREATE INDEX idx_messages_processed        ON messages(processed) WHERE processed = FALSE;
CREATE INDEX idx_activity_logs_user_id     ON activity_logs(user_id);
CREATE INDEX idx_activity_logs_started_at  ON activity_logs(started_at);
CREATE INDEX idx_activity_logs_tag         ON activity_logs(tag);
CREATE INDEX idx_activity_logs_message_id  ON activity_logs(message_id);