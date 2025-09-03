-- migrations/000001_create_metrics_tables.down.sql
-- Drop tables and indexes created in the up migration
DROP INDEX IF EXISTS idx_gauges_id;
DROP TABLE IF EXISTS gauges; 
DROP INDEX IF EXISTS idx_counters_id;
DROP TABLE IF EXISTS counters;