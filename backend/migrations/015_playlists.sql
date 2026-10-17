-- Migration 015: Playlists
--
-- A playlist is an ordered list of library videos owned by one user. Rooms play
-- them as a queue (see ws.Hub), but the playlist itself belongs to its owner and
-- not to any room: the same playlist can be started in several rooms.
--
-- A video appears in a playlist at most once. "Save to playlist" is a toggle in
-- the library, and a toggle has nothing sensible to do with a second copy.
-- Position is dense from 0 and rewritten wholesale on reorder, so it is not
-- unique-constrained: a reorder would otherwise have to dodge its own
-- intermediate states.

CREATE TABLE IF NOT EXISTS playlists (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id    UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name        VARCHAR(100) NOT NULL,
    description TEXT         NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS playlist_items (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    playlist_id    UUID        NOT NULL REFERENCES playlists(id) ON DELETE CASCADE,
    media_asset_id UUID        NOT NULL REFERENCES media_assets(id) ON DELETE CASCADE,
    position       INT         NOT NULL,
    added_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT playlist_items_unique_media UNIQUE (playlist_id, media_asset_id)
);

CREATE INDEX IF NOT EXISTS idx_playlists_owner       ON playlists (owner_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_playlist_items_order  ON playlist_items (playlist_id, position);
CREATE INDEX IF NOT EXISTS idx_playlist_items_media  ON playlist_items (media_asset_id);
