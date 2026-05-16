-- Migration: 000001_init
-- Purpose: Enable PostgreSQL extensions used throughout the system.
--
-- uuid-ossp  → gen_random_uuid() for primary keys
-- pgcrypto   → crypt() and gen_salt() for password hashing utilities

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
