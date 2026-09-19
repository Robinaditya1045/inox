-- Migration 013: Friendships
--
-- One row per relationship between two users, holding both states it can be in:
-- a pending request (requester_id asked addressee_id) and, once answered, an
-- accepted friendship. Direction is kept after acceptance but stops mattering --
-- every read below treats an accepted pair symmetrically.
--
-- Uniqueness is on (LEAST, GREATEST) of the two ids rather than on the column pair
-- itself: a plain UNIQUE(requester_id, addressee_id) would still admit A -> B and
-- B -> A as two separate rows, so two people who add each other at the same moment
-- would each be left holding a request that can never collapse into one friendship.

CREATE TABLE IF NOT EXISTS friendships (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    requester_id UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    addressee_id UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status       VARCHAR(20) NOT NULL DEFAULT 'pending', -- 'pending' | 'accepted' | 'declined'
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT friendships_no_self_link CHECK (requester_id <> addressee_id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_friendships_pair
    ON friendships (LEAST(requester_id, addressee_id), GREATEST(requester_id, addressee_id));

-- Both list queries filter one side of the pair by status, so each side gets its own index.
CREATE INDEX IF NOT EXISTS idx_friendships_requester ON friendships (requester_id, status);
CREATE INDEX IF NOT EXISTS idx_friendships_addressee ON friendships (addressee_id, status);
