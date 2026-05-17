ALTER TABLE ai_request_logs ALTER COLUMN total_tokens DROP NOT NULL;
ALTER TABLE ai_request_logs ADD COLUMN audio_seconds INT;
ALTER TABLE ai_request_logs DROP CONSTRAINT IF EXISTS ai_request_logs_request_type_check;
ALTER TABLE ai_request_logs ADD CONSTRAINT ai_request_logs_request_type_check
    CHECK (request_type IN ('activity_extraction', 'stats_analysis', 'transcription'));
