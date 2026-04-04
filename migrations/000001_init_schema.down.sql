DROP INDEX IF EXISTS idx_activity_logs_message_id;
DROP INDEX IF EXISTS idx_activity_logs_tag;
DROP INDEX IF EXISTS idx_activity_logs_started_at;
DROP INDEX IF EXISTS idx_activity_logs_user_id;
DROP INDEX IF EXISTS idx_messages_processed;
DROP INDEX IF EXISTS idx_messages_user_id;

DROP TABLE IF EXISTS activity_logs;
DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS users;