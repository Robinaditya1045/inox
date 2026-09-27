-- Migration 014: why a live channel last resolved the way it did.
--
-- last_error says what went wrong in a sentence. These say which kind of result
-- the latest resolve had -- STREAM_FOUND, DRM_PROTECTED, NO_STREAM_FOUND,
-- RESOLUTION_TIMEOUT or RESOLUTION_FAILED -- and what the resolver saw, for the
-- admin portal. Diagnostics hold hosts, counts and DRM system names only: never
-- URLs, header values, tokens, key IDs or licence data.

ALTER TABLE live_channels
    ADD COLUMN IF NOT EXISTS last_outcome     VARCHAR(32),
    ADD COLUMN IF NOT EXISTS last_diagnostics JSONB;
