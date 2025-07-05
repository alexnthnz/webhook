-- Drop trigger
DROP TRIGGER IF EXISTS update_webhooks_updated_at ON webhooks;

-- Drop function
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop indexes
DROP INDEX IF EXISTS idx_webhooks_customer_active;
DROP INDEX IF EXISTS idx_webhooks_created_at;
DROP INDEX IF EXISTS idx_webhooks_active;
DROP INDEX IF EXISTS idx_webhooks_event_types;
DROP INDEX IF EXISTS idx_webhooks_customer_id;

-- Drop table
DROP TABLE IF EXISTS webhooks; 