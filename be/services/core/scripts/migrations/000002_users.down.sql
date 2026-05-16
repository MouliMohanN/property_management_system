-- Migration: 000002_users (rollback)
DROP INDEX IF EXISTS idx_users_email;
DROP TABLE IF EXISTS users;
