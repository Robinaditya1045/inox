-- Migration 015: Admin portal admins
--
-- Admin portal access has two tiers. Owners are the addresses listed in ADMIN_EMAILS
-- and are never stored here: the env stays their single source of truth, so nobody
-- can remove an owner from the UI. Owners add admins from the admin portal, and each
-- admin is one row below. The role lives in its own table rather than on users, so
-- no query behind the main app ever reads it.

CREATE TABLE IF NOT EXISTS admin_users (
    user_id    UUID        PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    -- The owner who added them. Kept as history only; it grants nothing.
    granted_by UUID        REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
