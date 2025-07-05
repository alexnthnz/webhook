-- Create webhooks table
CREATE TABLE IF NOT EXISTS webhooks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id VARCHAR(50) NOT NULL,
    url TEXT NOT NULL,
    event_types TEXT[] NOT NULL,
    retry_policy JSONB NOT NULL DEFAULT '{"max_attempts": 5, "backoff_type": "exponential", "initial_delay_ms": 1000, "max_delay_ms": 60000, "multiplier": 2.0}',
    secret TEXT NOT NULL,
    headers JSONB DEFAULT '{}',
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT valid_url CHECK (url ~ '^https?://'),
    CONSTRAINT valid_customer_id CHECK (length(customer_id) > 0),
    CONSTRAINT valid_secret CHECK (length(secret) >= 16),
    CONSTRAINT valid_event_types CHECK (array_length(event_types, 1) > 0)
);

-- Create indexes for efficient querying
CREATE INDEX IF NOT EXISTS idx_webhooks_customer_id ON webhooks (customer_id);
CREATE INDEX IF NOT EXISTS idx_webhooks_event_types ON webhooks USING GIN (event_types);
CREATE INDEX IF NOT EXISTS idx_webhooks_active ON webhooks (active);
CREATE INDEX IF NOT EXISTS idx_webhooks_created_at ON webhooks (created_at);

-- Create composite index for common queries
CREATE INDEX IF NOT EXISTS idx_webhooks_customer_active ON webhooks (customer_id, active);

-- Create function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Create trigger to automatically update updated_at
CREATE TRIGGER update_webhooks_updated_at 
    BEFORE UPDATE ON webhooks 
    FOR EACH ROW 
    EXECUTE FUNCTION update_updated_at_column();

-- Add comments for documentation
COMMENT ON TABLE webhooks IS 'Stores webhook configurations for customers';
COMMENT ON COLUMN webhooks.id IS 'Unique identifier for the webhook';
COMMENT ON COLUMN webhooks.customer_id IS 'Customer identifier who owns this webhook';
COMMENT ON COLUMN webhooks.url IS 'Target URL for webhook delivery';
COMMENT ON COLUMN webhooks.event_types IS 'Array of event types this webhook subscribes to';
COMMENT ON COLUMN webhooks.retry_policy IS 'JSON configuration for retry behavior';
COMMENT ON COLUMN webhooks.secret IS 'Secret key for HMAC signature generation';
COMMENT ON COLUMN webhooks.headers IS 'Custom headers to include in webhook requests';
COMMENT ON COLUMN webhooks.active IS 'Whether this webhook is active and should receive events';
COMMENT ON COLUMN webhooks.created_at IS 'Timestamp when the webhook was created';
COMMENT ON COLUMN webhooks.updated_at IS 'Timestamp when the webhook was last updated'; 