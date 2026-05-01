ALTER TABLE ai_request_logs DROP COLUMN request_type;
ALTER TABLE ai_request_logs ALTER COLUMN message_id SET NOT NULL;
