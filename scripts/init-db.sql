-- Weather Service Database Initialization
-- This script is run automatically when PostgreSQL starts

-- Create weather service user if not exists
DO
$do$
BEGIN
   IF NOT EXISTS (
      SELECT FROM pg_catalog.pg_user
      WHERE usename = 'weather') THEN
      -- Create user with password from environment
      -- Password should be set via: CREATE USER weather WITH PASSWORD '<secure_password>';
      -- This file is for reference only. Use proper secret management in production.
      CREATE USER weather;
   END IF;
END
$do$;

-- Create database if not exists
SELECT 'CREATE DATABASE weather_service'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'weather_service')\gexec

-- Grant privileges
GRANT ALL PRIVILEGES ON DATABASE weather_service TO weather;

-- Connect to weather_service database
\c weather_service;

-- Create schema
CREATE SCHEMA IF NOT EXISTS weather;

-- Grant schema privileges
GRANT ALL ON SCHEMA weather TO weather;