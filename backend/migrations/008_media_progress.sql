-- Migration 008: Add progress tracking column to media_assets
ALTER TABLE media_assets ADD COLUMN IF NOT EXISTS progress INT DEFAULT 0;
