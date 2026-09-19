-- Migration 012: Live channels
--
-- A live channel is a media_assets row of kind 'live' plus the ingest configuration
-- needed to keep a playable manifest URL alive behind a stable proxy URL. The asset
-- row is what makes live channels appear in the existing media library, room
-- CHANGE_MEDIA flow, and rooms.current_media_url with no changes to those paths.

ALTER TABLE media_assets
    ADD COLUMN IF NOT EXISTS kind VARCHAR(16) NOT NULL DEFAULT 'vod'; -- 'vod' | 'live'

CREATE TABLE IF NOT EXISTS live_channels (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    media_asset_id      UUID NOT NULL REFERENCES media_assets(id) ON DELETE CASCADE,
    slug                VARCHAR(64)   NOT NULL UNIQUE,
    resolver            VARCHAR(32)   NOT NULL,
    source_url          VARCHAR(2048) NOT NULL,
    resolver_config     JSONB         NOT NULL DEFAULT '{}'::jsonb,
    fallback_sources    JSONB         NOT NULL DEFAULT '[]'::jsonb,
    protocol            VARCHAR(8)    NOT NULL DEFAULT 'hls',
    is_dvr              BOOLEAN       NOT NULL DEFAULT false,
    dvr_window_seconds  INT           NOT NULL DEFAULT 0,
    status              VARCHAR(16)   NOT NULL DEFAULT 'idle',

    -- Resolved upstream. Never leaves the backend: the proxy seals it into an
    -- encrypted ref before it appears in any manifest served to a client.
    upstream_url        VARCHAR(2048),
    upstream_headers    JSONB,
    upstream_expires_at TIMESTAMPTZ,
    last_resolved_at    TIMESTAMPTZ,
    last_error          TEXT,

    created_by          UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    created_at          TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at          TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_live_channels_status   ON live_channels(status);
CREATE INDEX IF NOT EXISTS idx_live_channels_asset_id ON live_channels(media_asset_id);
CREATE INDEX IF NOT EXISTS idx_media_assets_kind      ON media_assets(kind);
