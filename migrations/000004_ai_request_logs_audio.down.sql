ALTER TABLE ai_request_logs DROP CONSTRAINT IF EXISTS ai_request_logs_request_type_check;
DELETE FROM ai_request_logs WHERE request_type = 'transcription';
ALTER TABLE ai_request_logs ADD CONSTRAINT ai_request_logs_request_type_check
    CHECK (request_type IN ('activity_extraction', 'stats_analysis'));
ALTER TABLE ai_request_logs DROP COLUMN IF EXISTS audio_seconds;
UPDATE ai_request_logs SET total_tokens = 0 WHERE total_tokens IS NULL;
ALTER TABLE ai_request_logs ALTER COLUMN total_tokens SET NOT NULL;
