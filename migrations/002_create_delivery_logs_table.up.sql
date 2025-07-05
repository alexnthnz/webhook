-- Create delivery_logs table for tracking webhook delivery attempts
CREATE TABLE IF NOT EXISTS delivery_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    webhook_id UUID NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
    customer_id VARCHAR(50) NOT NULL,
    event_id VARCHAR(100) NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    attempt_number INTEGER NOT NULL DEFAULT 1,
    status VARCHAR(20) NOT NULL CHECK (status IN ('success', 'failed', 'retrying', 'expired', 'cancelled')),
    http_status_code INTEGER,
    request_payload JSONB,
    response_payload TEXT,
    request_headers JSONB DEFAULT '{}',
    response_headers JSONB DEFAULT '{}',
    latency_ms INTEGER,
    error_message TEXT,
    scheduled_at TIMESTAMP WITH TIME ZONE,
    attempted_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT valid_attempt_number CHECK (attempt_number > 0),
    CONSTRAINT valid_latency CHECK (latency_ms >= 0),
    CONSTRAINT valid_http_status CHECK (http_status_code IS NULL OR (http_status_code >= 100 AND http_status_code < 600))
);

-- Create indexes for efficient querying
CREATE INDEX IF NOT EXISTS idx_delivery_logs_webhook_id ON delivery_logs (webhook_id);
CREATE INDEX IF NOT EXISTS idx_delivery_logs_customer_id ON delivery_logs (customer_id);
CREATE INDEX IF NOT EXISTS idx_delivery_logs_event_id ON delivery_logs (event_id);
CREATE INDEX IF NOT EXISTS idx_delivery_logs_event_type ON delivery_logs (event_type);
CREATE INDEX IF NOT EXISTS idx_delivery_logs_status ON delivery_logs (status);
CREATE INDEX IF NOT EXISTS idx_delivery_logs_attempted_at ON delivery_logs (attempted_at);
CREATE INDEX IF NOT EXISTS idx_delivery_logs_created_at ON delivery_logs (created_at);

-- Create composite indexes for common queries
CREATE INDEX IF NOT EXISTS idx_delivery_logs_webhook_status ON delivery_logs (webhook_id, status);
CREATE INDEX IF NOT EXISTS idx_delivery_logs_customer_status ON delivery_logs (customer_id, status);
CREATE INDEX IF NOT EXISTS idx_delivery_logs_event_status ON delivery_logs (event_id, status);

-- Create partial index for failed deliveries
CREATE INDEX IF NOT EXISTS idx_delivery_logs_failed ON delivery_logs (webhook_id, attempted_at) 
WHERE status IN ('failed', 'retrying');

-- Add comments for documentation
COMMENT ON TABLE delivery_logs IS 'Stores webhook delivery attempt logs for observability';
COMMENT ON COLUMN delivery_logs.id IS 'Unique identifier for the delivery log entry';
COMMENT ON COLUMN delivery_logs.webhook_id IS 'Reference to the webhook that was delivered';
COMMENT ON COLUMN delivery_logs.customer_id IS 'Customer identifier for partitioning and filtering';
COMMENT ON COLUMN delivery_logs.event_id IS 'Unique identifier for the event being delivered';
COMMENT ON COLUMN delivery_logs.event_type IS 'Type of event being delivered';
COMMENT ON COLUMN delivery_logs.attempt_number IS 'Which attempt this is (1 for first attempt, 2+ for retries)';
COMMENT ON COLUMN delivery_logs.status IS 'Status of the delivery attempt';
COMMENT ON COLUMN delivery_logs.http_status_code IS 'HTTP status code returned by the webhook endpoint';
COMMENT ON COLUMN delivery_logs.request_payload IS 'The payload that was sent to the webhook';
COMMENT ON COLUMN delivery_logs.response_payload IS 'The response received from the webhook endpoint';
COMMENT ON COLUMN delivery_logs.request_headers IS 'Headers sent with the webhook request';
COMMENT ON COLUMN delivery_logs.response_headers IS 'Headers received in the webhook response';
COMMENT ON COLUMN delivery_logs.latency_ms IS 'Time taken for the delivery attempt in milliseconds';
COMMENT ON COLUMN delivery_logs.error_message IS 'Error message if the delivery failed';
COMMENT ON COLUMN delivery_logs.scheduled_at IS 'When the delivery was scheduled';
COMMENT ON COLUMN delivery_logs.attempted_at IS 'When the delivery attempt was made';
COMMENT ON COLUMN delivery_logs.completed_at IS 'When the delivery attempt completed';
COMMENT ON COLUMN delivery_logs.created_at IS 'When this log entry was created'; 