-- Stored procedures for the weather service database
-- These procedures encapsulate all database operations to keep SQL out of application code

-- =====================================================================
-- Procedure: sp_log_audit
-- Purpose: Records an audit entry for request tracking and compliance
-- =====================================================================
CREATE OR REPLACE PROCEDURE sp_log_audit(
    IN p_correlation_id VARCHAR(36),
    IN p_request_id VARCHAR(36),
    IN p_method VARCHAR(10),
    IN p_path VARCHAR(255),
    IN p_status_code INT,
    IN p_duration_ms BIGINT,
    IN p_user_agent TEXT,
    IN p_remote_addr VARCHAR(45),
    IN p_error_message TEXT,
    IN p_metadata JSONB
)
LANGUAGE plpgsql
AS $$
BEGIN
    INSERT INTO audit_logs (
        correlation_id, 
        request_id, 
        method, 
        path, 
        status_code,
        duration_ms, 
        user_agent, 
        remote_addr, 
        error_message, 
        metadata
    ) VALUES (
        p_correlation_id,
        p_request_id,
        p_method,
        p_path,
        p_status_code,
        p_duration_ms,
        p_user_agent,
        p_remote_addr,
        p_error_message,
        p_metadata
    );
END;
$$;

-- =====================================================================
-- Procedure: sp_log_weather_request
-- Purpose: Records weather request details for analytics
-- =====================================================================
CREATE OR REPLACE PROCEDURE sp_log_weather_request(
    IN p_request_id VARCHAR(36),
    IN p_latitude DECIMAL(10, 6),
    IN p_longitude DECIMAL(10, 6),
    IN p_temperature DECIMAL(5, 2),
    IN p_temperature_unit VARCHAR(1),
    IN p_forecast TEXT,
    IN p_category VARCHAR(20),
    IN p_response_time_ms INT,
    IN p_cache_hit BOOLEAN
)
LANGUAGE plpgsql
AS $$
BEGIN
    INSERT INTO weather_requests (
        request_id,
        latitude,
        longitude,
        temperature,
        temperature_unit,
        forecast,
        category,
        response_time_ms,
        cache_hit
    ) VALUES (
        p_request_id,
        p_latitude,
        p_longitude,
        p_temperature,
        p_temperature_unit,
        p_forecast,
        p_category,
        p_response_time_ms,
        p_cache_hit
    );
EXCEPTION
    WHEN unique_violation THEN
        -- If request_id already exists, update the record
        UPDATE weather_requests 
        SET 
            latitude = p_latitude,
            longitude = p_longitude,
            temperature = p_temperature,
            temperature_unit = p_temperature_unit,
            forecast = p_forecast,
            category = p_category,
            response_time_ms = p_response_time_ms,
            cache_hit = p_cache_hit,
            timestamp = CURRENT_TIMESTAMP
        WHERE request_id = p_request_id;
END;
$$;

-- =====================================================================
-- Function: fn_get_request_stats
-- Purpose: Retrieves aggregated statistics for monitoring and reporting
-- =====================================================================
CREATE OR REPLACE FUNCTION fn_get_request_stats(
    p_since TIMESTAMP WITH TIME ZONE
)
RETURNS TABLE (
    total_requests BIGINT,
    avg_response_time NUMERIC,
    min_response_time INT,
    max_response_time INT,
    cache_hit_rate NUMERIC
)
LANGUAGE plpgsql
AS $$
BEGIN
    RETURN QUERY
    SELECT 
        COUNT(*)::BIGINT as total_requests,
        ROUND(AVG(response_time_ms)::NUMERIC, 2) as avg_response_time,
        MIN(response_time_ms) as min_response_time,
        MAX(response_time_ms) as max_response_time,
        ROUND((SUM(CASE WHEN cache_hit THEN 1 ELSE 0 END)::NUMERIC / 
               NULLIF(COUNT(*)::NUMERIC, 0)), 4) as cache_hit_rate
    FROM weather_requests
    WHERE timestamp >= p_since;
END;
$$;

-- =====================================================================
-- Function: fn_get_popular_locations
-- Purpose: Returns the most frequently requested locations
-- =====================================================================
CREATE OR REPLACE FUNCTION fn_get_popular_locations(
    p_limit INT DEFAULT 10,
    p_since TIMESTAMP WITH TIME ZONE DEFAULT NOW() - INTERVAL '7 days'
)
RETURNS TABLE (
    latitude DECIMAL(10, 6),
    longitude DECIMAL(10, 6),
    request_count BIGINT,
    avg_temperature NUMERIC,
    most_common_category VARCHAR(20)
)
LANGUAGE plpgsql
AS $$
BEGIN
    RETURN QUERY
    SELECT 
        wr.latitude,
        wr.longitude,
        COUNT(*)::BIGINT as request_count,
        ROUND(AVG(wr.temperature)::NUMERIC, 2) as avg_temperature,
        MODE() WITHIN GROUP (ORDER BY wr.category) as most_common_category
    FROM weather_requests wr
    WHERE wr.timestamp >= p_since
    GROUP BY wr.latitude, wr.longitude
    ORDER BY request_count DESC
    LIMIT p_limit;
END;
$$;

-- =====================================================================
-- Function: fn_get_audit_logs
-- Purpose: Retrieves audit logs with optional filtering
-- =====================================================================
CREATE OR REPLACE FUNCTION fn_get_audit_logs(
    p_correlation_id VARCHAR(36) DEFAULT NULL,
    p_request_id VARCHAR(36) DEFAULT NULL,
    p_since TIMESTAMP WITH TIME ZONE DEFAULT NOW() - INTERVAL '1 day',
    p_limit INT DEFAULT 100
)
RETURNS TABLE (
    id INT,
    correlation_id VARCHAR(36),
    request_id VARCHAR(36),
    timestamp TIMESTAMP WITH TIME ZONE,
    method VARCHAR(10),
    path VARCHAR(255),
    status_code INT,
    duration_ms BIGINT,
    user_agent TEXT,
    remote_addr VARCHAR(45),
    error_message TEXT,
    metadata JSONB
)
LANGUAGE plpgsql
AS $$
BEGIN
    RETURN QUERY
    SELECT 
        al.id,
        al.correlation_id,
        al.request_id,
        al.timestamp,
        al.method,
        al.path,
        al.status_code,
        al.duration_ms,
        al.user_agent,
        al.remote_addr,
        al.error_message,
        al.metadata
    FROM audit_logs al
    WHERE 
        al.timestamp >= p_since
        AND (p_correlation_id IS NULL OR al.correlation_id = p_correlation_id)
        AND (p_request_id IS NULL OR al.request_id = p_request_id)
    ORDER BY al.timestamp DESC
    LIMIT p_limit;
END;
$$;

-- =====================================================================
-- Procedure: sp_cleanup_old_data
-- Purpose: Removes old records to manage database size
-- =====================================================================
CREATE OR REPLACE PROCEDURE sp_cleanup_old_data(
    IN p_audit_retention_days INT DEFAULT 30,
    IN p_weather_retention_days INT DEFAULT 90,
    OUT deleted_audit_logs INT,
    OUT deleted_weather_requests INT
)
LANGUAGE plpgsql
AS $$
BEGIN
    -- Delete old audit logs
    DELETE FROM audit_logs 
    WHERE timestamp < NOW() - (p_audit_retention_days || ' days')::INTERVAL;
    GET DIAGNOSTICS deleted_audit_logs = ROW_COUNT;
    
    -- Delete old weather requests
    DELETE FROM weather_requests 
    WHERE timestamp < NOW() - (p_weather_retention_days || ' days')::INTERVAL;
    GET DIAGNOSTICS deleted_weather_requests = ROW_COUNT;
    
    -- Vacuum analyze to reclaim space and update statistics
    ANALYZE audit_logs;
    ANALYZE weather_requests;
END;
$$;

-- =====================================================================
-- Function: fn_get_error_summary
-- Purpose: Provides a summary of errors for monitoring
-- =====================================================================
CREATE OR REPLACE FUNCTION fn_get_error_summary(
    p_since TIMESTAMP WITH TIME ZONE DEFAULT NOW() - INTERVAL '1 hour'
)
RETURNS TABLE (
    status_code INT,
    error_count BIGINT,
    paths TEXT[],
    sample_errors TEXT[]
)
LANGUAGE plpgsql
AS $$
BEGIN
    RETURN QUERY
    SELECT 
        al.status_code,
        COUNT(*)::BIGINT as error_count,
        ARRAY_AGG(DISTINCT al.path) as paths,
        ARRAY_AGG(DISTINCT al.error_message) FILTER (WHERE al.error_message IS NOT NULL) as sample_errors
    FROM audit_logs al
    WHERE 
        al.timestamp >= p_since
        AND al.status_code >= 400
    GROUP BY al.status_code
    ORDER BY error_count DESC;
END;
$$;