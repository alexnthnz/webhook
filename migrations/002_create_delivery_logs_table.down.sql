-- Drop indexes
DROP INDEX IF EXISTS idx_delivery_logs_failed;
DROP INDEX IF EXISTS idx_delivery_logs_event_status;
DROP INDEX IF EXISTS idx_delivery_logs_customer_status;
DROP INDEX IF EXISTS idx_delivery_logs_webhook_status;
DROP INDEX IF EXISTS idx_delivery_logs_created_at;
DROP INDEX IF EXISTS idx_delivery_logs_attempted_at;
DROP INDEX IF EXISTS idx_delivery_logs_status;
DROP INDEX IF EXISTS idx_delivery_logs_event_type;
DROP INDEX IF EXISTS idx_delivery_logs_event_id;
DROP INDEX IF EXISTS idx_delivery_logs_customer_id;
DROP INDEX IF EXISTS idx_delivery_logs_webhook_id;

-- Drop table
DROP TABLE IF EXISTS delivery_logs; 