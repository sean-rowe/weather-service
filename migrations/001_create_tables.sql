-- Create tables for the weather service database
-- This migration creates the base tables needed for audit logging and analytics

-- Create the audit_logs table for request tracking and compliance
CREATE TABLE IF NOT EXISTS audit_logs (
    id SERIAL PRIMARY KEY,
    correlation_id VARCHAR(36),
    request_id VARCHAR(36),
    timestamp TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    method VARCHAR(10),
    path VARCHAR(255),
    status_code INT,
    duration_ms BIGINT,
    user_agent TEXT,
    remote_addr VARCHAR(45),
    error_message TEXT,
    metadata JSONB
);

-- Create indexes for audit_logs
CREATE INDEX IF NOT EXISTS idx_audit_logs_correlation_id ON audit_logs(correlation_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_timestamp ON audit_logs(timestamp);
CREATE INDEX IF NOT EXISTS idx_audit_logs_request_id ON audit_logs(request_id);

-- Create the weather_requests table for analytics
CREATE TABLE IF NOT EXISTS weather_requests (
    id SERIAL PRIMARY KEY,
    request_id VARCHAR(36) UNIQUE,
    timestamp TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    latitude DECIMAL(10, 6),
    longitude DECIMAL(10, 6),
    temperature DECIMAL(5, 2),
    temperature_unit VARCHAR(1),
    forecast TEXT,
    category VARCHAR(20),
    response_time_ms INT,
    cache_hit BOOLEAN DEFAULT FALSE
);

-- Create indexes for weather_requests
CREATE INDEX IF NOT EXISTS idx_weather_requests_timestamp ON weather_requests(timestamp);
CREATE INDEX IF NOT EXISTS idx_weather_requests_coordinates ON weather_requests(latitude, longitude);
CREATE INDEX IF NOT EXISTS idx_weather_requests_request_id ON weather_requests(request_id);