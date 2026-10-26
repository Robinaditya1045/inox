import { useCallback, useEffect, useRef, useState } from "react";
import { communityService } from "../services/community/community.service";
import { APIError } from "../api/client";
import type { CommunityCategory, CommunityPost } from "../types/community";

const PAGE_SIZE = 20;

interface UseCommunityFeedReturn {
  posts: CommunityPost[];
  isLoading: boolean;
  isLoadingMore: boolean;
  hasMore: boolean;
  error: string | null;
  loadMore: () => void;
  refresh: () => void;
}

interface FeedState {
  /** Which category these posts are for; null while (re)loading from scratch */
  key: string | null;
  posts: CommunityPost[];
  hasMore: boolean;
  error: string | null;
}

const EMPTY: FeedState = { key: null, posts: [], hasMore: false, error: null };

const messageOf = (err: unknown) =>
  err instanceof APIError ? err.message : "Couldn't load updates";

/**
 * One category's Community feed, a page at a time. Switching category starts
 * over, and a response for a category no longer selected is dropped.
 */
export const useCommunityFeed = (
  category: CommunityCategory | null,
): UseCommunityFeedReturn => {
  const key = category ?? "all";
  const [feed, setFeed] = useState<FeedState>(EMPTY);
  const [isLoadingMore, setIsLoadingMore] = useState(false);
  const [reloads, setReloads] = useState(0);
  const currentKey = useRef(key);

  useEffect(() => {
    currentKey.current = key;
    let cancelled = false;
    communityService
      .feed({ category: category ?? undefined, limit: PAGE_SIZE })
      .then((page) => {
        if (!cancelled)
          setFeed({
            key,
            posts: page.posts,
            hasMore: page.has_more,
            error: null,
          });
      })
      .catch((err) => {
        if (!cancelled) setFeed({ ...EMPTY, key, error: messageOf(err) });
      });
    return () => {
      cancelled = true;
    };
  }, [category, key, reloads]);

  const current = feed.key === key ? feed : EMPTY;

  const loadMore = useCallback(async () => {
    if (isLoadingMore || !current.hasMore) return;
    setIsLoadingMore(true);
    try {
      const page = await communityService.feed({
        category: category ?? undefined,
        limit: PAGE_SIZE,
        offset: current.posts.length,
      });
      if (currentKey.current !== key) return;
      setFeed((prev) => {
        // A post published between pages shifts the offsets by one.
        const known = new Set(prev.posts.map((p) => p.id));
        return {
          ...prev,
          posts: [...prev.posts, ...page.posts.filter((p) => !known.has(p.id))],
          hasMore: page.has_more,
          error: null,
        };
      });
    } catch (err) {
      if (currentKey.current === key)
        setFeed((prev) => ({ ...prev, error: messageOf(err) }));
    } finally {
      setIsLoadingMore(false);
    }
  }, [category, current.hasMore, current.posts.length, isLoadingMore, key]);

  const refresh = useCallback(() => {
    setFeed(EMPTY);
    setReloads((n) => n + 1);
  }, []);

  return {
    posts: current.posts,
    isLoading: feed.key !== key,
    isLoadingMore,
    hasMore: current.hasMore,
    error: current.error,
    loadMore,
    refresh,
  };
};
