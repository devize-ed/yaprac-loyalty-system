-- migrations/000001_create_metrics_tables.up.sql

-- Create table for store gauge metrics
CREATE TABLE IF NOT EXISTS gauges ( 
 id TEXT PRIMARY KEY NOT NULL,
 value DOUBLE PRECISION NOT NULL 
);
-- Create table for store counter metrics
CREATE TABLE IF NOT EXISTS counters ( 
 id TEXT PRIMARY KEY NOT NULL,
 delta BIGINT NOT NULL 
);


-- Create index for gauges
CREATE INDEX IF NOT EXISTS idx_gauges_id   ON gauges(id);
-- Create index for counters
CREATE INDEX IF NOT EXISTS idx_counters_id ON counters(id);