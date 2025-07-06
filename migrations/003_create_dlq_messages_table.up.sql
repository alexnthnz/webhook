-- Create DLQ messages table
CREATE TABLE dlq_messages (
    event_id VARCHAR(255) PRIMARY KEY,
    webhook_id VARCHAR(255) NOT NULL,
    customer_id VARCHAR(255) NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    original_event JSONB NOT NULL,
    failure_reason TEXT NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    first_failed_at TIMESTAMP NOT NULL,
    last_failed_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    headers JSONB,
    status VARCHAR(50) NOT NULL DEFAULT 'active',
    requeued_at TIMESTAMP
);

-- Create indexes for efficient querying
CREATE INDEX idx_dlq_messages_customer_id ON dlq_messages(customer_id);
CREATE INDEX idx_dlq_messages_webhook_id ON dlq_messages(webhook_id);
CREATE INDEX idx_dlq_messages_event_type ON dlq_messages(event_type);
CREATE INDEX idx_dlq_messages_status ON dlq_messages(status);
CREATE INDEX idx_dlq_messages_created_at ON dlq_messages(created_at);
CREATE INDEX idx_dlq_messages_last_failed_at ON dlq_messages(last_failed_at);

-- Add foreign key constraint to webhooks table
ALTER TABLE dlq_messages 
ADD CONSTRAINT fk_dlq_messages_webhook_id 
FOREIGN KEY (webhook_id) REFERENCES webhooks(id) ON DELETE CASCADE; 