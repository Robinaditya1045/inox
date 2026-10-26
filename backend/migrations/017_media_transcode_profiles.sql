-- Migration 017: Choose which qualities an upload is transcoded to
--
-- An upload is encoded once per quality, so choosing fewer finishes sooner.
-- The choice is kept on the asset rather than only in the queued task because
-- the worker re-reads the asset on every retry, and ReconcileOrphanedAssets
-- re-queues interrupted transcodes at boot with nothing but the asset to go on.
-- Empty means the full ladder, which is what every asset before this got.

ALTER TABLE media_assets ADD COLUMN IF NOT EXISTS transcode_profiles TEXT[] NOT NULL DEFAULT '{}';
