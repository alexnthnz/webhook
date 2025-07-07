-- Create DLQ messages table
CREATE TABLE IF NOT EXISTS dlq_messages (
    id BIGSERIAL PRIMARY KEY,
    event_id VARCHAR(255) NOT NULL UNIQUE,
    webhook_id UUID NOT NULL,
    customer_id VARCHAR(50) NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    original_event JSONB NOT NULL,
    failure_reason TEXT NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    first_failed_at TIMESTAMP WITH TIME ZONE NOT NULL,
    last_failed_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    headers JSONB DEFAULT '{}',
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    requeued_at TIMESTAMP WITH TIME ZONE,
    CONSTRAINT valid_event_id CHECK (length(event_id) > 0),
    CONSTRAINT valid_attempts CHECK (attempts >= 0),
    CONSTRAINT valid_status CHECK (status IN ('active', 'requeued', 'deleted'))
);

-- Create indexes for efficient querying
CREATE INDEX IF NOT EXISTS idx_dlq_messages_customer_id ON dlq_messages (customer_id);
CREATE INDEX IF NOT EXISTS idx_dlq_messages_webhook_id ON dlq_messages (webhook_id);
CREATE INDEX IF NOT EXISTS idx_dlq_messages_event_type ON dlq_messages (event_type);
CREATE INDEX IF NOT EXISTS idx_dlq_messages_status ON dlq_messages (status);
CREATE INDEX IF NOT EXISTS idx_dlq_messages_created_at ON dlq_messages (created_at);
CREATE INDEX IF NOT EXISTS idx_dlq_messages_last_failed_at ON dlq_messages (last_failed_at);

-- Create composite indexes for common queries
CREATE INDEX IF NOT EXISTS idx_dlq_messages_customer_status ON dlq_messages (customer_id, status);
CREATE INDEX IF NOT EXISTS idx_dlq_messages_webhook_status ON dlq_messages (webhook_id, status);

-- Add comments for documentation
COMMENT ON TABLE dlq_messages IS 'Stores failed webhook delivery messages for analysis and requeuing';
COMMENT ON COLUMN dlq_messages.id IS 'Auto-incrementing primary key';
COMMENT ON COLUMN dlq_messages.event_id IS 'Unique identifier for the failed event';
COMMENT ON COLUMN dlq_messages.webhook_id IS 'Reference to the webhook that failed';
COMMENT ON COLUMN dlq_messages.customer_id IS 'Customer identifier who owns the webhook';
COMMENT ON COLUMN dlq_messages.event_type IS 'Type of event that failed to deliver';
COMMENT ON COLUMN dlq_messages.original_event IS 'Original event payload that failed to deliver';
COMMENT ON COLUMN dlq_messages.failure_reason IS 'Reason for the delivery failure';
COMMENT ON COLUMN dlq_messages.attempts IS 'Number of delivery attempts made';
COMMENT ON COLUMN dlq_messages.first_failed_at IS 'Timestamp of first failure';
COMMENT ON COLUMN dlq_messages.last_failed_at IS 'Timestamp of last failure';
COMMENT ON COLUMN dlq_messages.created_at IS 'Timestamp when the DLQ record was created';
COMMENT ON COLUMN dlq_messages.headers IS 'HTTP headers that were sent with the request';
COMMENT ON COLUMN dlq_messages.status IS 'Current status: active, requeued, or deleted';
COMMENT ON COLUMN dlq_messages.requeued_at IS 'Timestamp when the message was requeued'; 