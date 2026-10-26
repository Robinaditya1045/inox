-- Migration 018: Community posts
--
-- Platform updates written by admins and shown to every viewer on the
-- Community page: new features, improvements, fixes and general announcements.
--
-- A post with no published_at is a draft and only admins see it. Publishing
-- stamps published_at once; editing a published post keeps its date so a typo
-- fix does not bump it back to the top of everyone's feed. The author is kept
-- as a reference only: deleting an admin's account leaves their posts up.

CREATE TABLE IF NOT EXISTS community_posts (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    author_id    UUID         REFERENCES users(id) ON DELETE SET NULL,
    category     VARCHAR(20)  NOT NULL DEFAULT 'announcement'
                 CHECK (category IN ('feature', 'improvement', 'fix', 'announcement')),
    title        VARCHAR(140) NOT NULL,
    body         TEXT         NOT NULL,
    pinned       BOOLEAN      NOT NULL DEFAULT FALSE,
    published_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- The viewer feed: published posts, pinned first, newest first.
CREATE INDEX IF NOT EXISTS idx_community_posts_feed
    ON community_posts (pinned DESC, published_at DESC)
    WHERE published_at IS NOT NULL;
