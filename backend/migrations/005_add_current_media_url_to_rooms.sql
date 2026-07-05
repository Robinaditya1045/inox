-- Migration: Add current_media_url column to rooms table for Watch Party media persistence
ALTER TABLE rooms ADD COLUMN IF NOT EXISTS current_media_url VARCHAR(1000) NOT NULL DEFAULT 'https://media.w3.org/2010/05/bunny/movie.mp4';
