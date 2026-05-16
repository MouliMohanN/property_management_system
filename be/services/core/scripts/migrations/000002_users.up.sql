-- Migration: 000002_users
-- Purpose: Create the users table, which is the central identity entity for
-- the entire platform. All other domain entities (properties, leases, payments)
-- will eventually reference this table via foreign keys.

CREATE TABLE users (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    email         VARCHAR(255) NOT NULL UNIQUE,
    phone_number  VARCHAR(20)  UNIQUE,
    password_hash TEXT         NOT NULL,
    first_name    VARCHAR(100) NOT NULL,
    last_name     VARCHAR(100) NOT NULL,
    role          VARCHAR(50)  NOT NULL,
    status        VARCHAR(50)  NOT NULL DEFAULT 'active',
    version       INTEGER      NOT NULL DEFAULT 1,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Index on email to support O(log n) lookups on the login hot path.
CREATE INDEX idx_users_email ON users (email);
