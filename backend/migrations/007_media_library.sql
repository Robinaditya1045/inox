-- Migration 007: Create media library assets and HLS adaptive bitrate renditions
CREATE TABLE IF NOT EXISTS media_assets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title VARCHAR(255) NOT NULL,
    description TEXT,
    source_url VARCHAR(1024) NOT NULL,
    status VARCHAR(32) DEFAULT 'pending',
    duration_seconds INT DEFAULT 0,
    progress INT DEFAULT 0,
    thumbnail_url VARCHAR(1024),
    hls_master_url VARCHAR(1024),
    created_by UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS media_renditions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    media_asset_id UUID NOT NULL REFERENCES media_assets(id) ON DELETE CASCADE,
    resolution VARCHAR(32) NOT NULL,
    bitrate_kbps INT NOT NULL,
    playlist_url VARCHAR(1024) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_media_assets_created_at ON media_assets(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_media_renditions_asset_id ON media_renditions(media_asset_id);
