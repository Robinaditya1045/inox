-- Migration 016: Caption tracks for library videos
--
-- A video has at most one track per language. Uploading a second file for a
-- language replaces the first rather than stacking alternatives the player would
-- have to choose between. Tracks are stored as WebVTT objects under captions/,
-- each save under a fresh key so the stream proxy can let browsers cache them;
-- storage_key is what to purge when a track is replaced or deleted.
--
-- is_default marks the track a viewer gets on turning captions on before they
-- have picked a language. At most one per video, enforced by the partial index.

CREATE TABLE IF NOT EXISTS media_captions (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    media_asset_id UUID          NOT NULL REFERENCES media_assets(id) ON DELETE CASCADE,
    language       VARCHAR(35)   NOT NULL,
    label          VARCHAR(100)  NOT NULL,
    url            VARCHAR(1024) NOT NULL,
    storage_key    VARCHAR(1024) NOT NULL,
    cue_count      INT           NOT NULL DEFAULT 0,
    is_default     BOOLEAN       NOT NULL DEFAULT FALSE,
    created_at     TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    CONSTRAINT media_captions_unique_language UNIQUE (media_asset_id, language)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_media_captions_one_default
    ON media_captions (media_asset_id) WHERE is_default;
