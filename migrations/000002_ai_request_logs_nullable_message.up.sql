ALTER TABLE ai_request_logs ALTER COLUMN message_id DROP NOT NULL;
ALTER TABLE ai_request_logs ADD COLUMN request_type TEXT NOT NULL DEFAULT 'activity_extraction';
