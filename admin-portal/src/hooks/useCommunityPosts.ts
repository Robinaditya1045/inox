import * as React from "react";
import { ApiError, apiJson } from "@/lib/api";
import type { CommunityPost, CommunityPostInput } from "@/types/community";

export interface UseCommunityPostsResult {
  posts: CommunityPost[];
  isLoading: boolean;
  isDemoMode: boolean;
  error: string | null;
  refresh: () => Promise<void>;
  create: (input: CommunityPostInput) => Promise<CommunityPost>;
  update: (id: string, input: CommunityPostInput) => Promise<CommunityPost>;
  remove: (id: string) => Promise<void>;
}

const daysAgo = (d: number) =>
  new Date(Date.now() - d * 86_400_000).toISOString();

const MOCK_POSTS: CommunityPost[] = [
  {
    id: "demo-draft",
    category: "announcement",
    title: "Scheduled maintenance this weekend",
    body: "Rooms will be read-only for about ten minutes on Sunday while we upgrade the database.",
    pinned: false,
    published_at: null,
    author_id: "u-robin",
    author_username: "robin",
    author_avatar_url: "",
    created_at: daysAgo(0.2),
    updated_at: daysAgo(0.1),
  },
  {
    id: "demo-voice",
    category: "feature",
    title: "Invite friends to voice chat",
    body: "Online room members now have an invite button next to their name while you're in voice.\n\n- They get a toast with Accept and Reject\n- Invites to the same person are rate limited",
    pinned: true,
    published_at: daysAgo(2),
    author_id: "u-robin",
    author_username: "robin",
    author_avatar_url: "",
    created_at: daysAgo(2),
    updated_at: daysAgo(2),
  },
  {
    id: "demo-fix",
    category: "fix",
    title: "Kicked members now leave the room straight away",
    body: "Removing someone from a room closes their connection immediately instead of when they next reload.",
    pinned: false,
    published_at: daysAgo(5),
    author_id: "u-robin",
    author_username: "robin",
    author_avatar_url: "",
    created_at: daysAgo(5),
    updated_at: daysAgo(5),
  },
];

/** Drafts first, then pinned, then newest: the order the server lists them in. */
function sortPosts(posts: CommunityPost[]): CommunityPost[] {
  const key = (p: CommunityPost) => Date.parse(p.published_at ?? p.updated_at);
  return [...posts].sort((a, b) => {
    if (!a.published_at !== !b.published_at) return a.published_at ? 1 : -1;
    if (a.pinned !== b.pinned) return a.pinned ? -1 : 1;
    return key(b) - key(a);
  });
}

/**
 * Every Community post, drafts included, for the admin composer.
 *
 * As with the rooms list, sample posts stand in only when the backend can't be
 * reached at all; an error response (usually 403, not in ADMIN_EMAILS) is shown
 * as is. Writes in demo mode change the samples locally and nothing else.
 */
export function useCommunityPosts(): UseCommunityPostsResult {
  const [posts, setPosts] = React.useState<CommunityPost[]>([]);
  const [isLoading, setIsLoading] = React.useState(true);
  const [isDemoMode, setIsDemoMode] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);
  const demoRef = React.useRef(false);

  // Starts out loading; refresh() sets it again before reloading.
  const load = React.useCallback(async () => {
    try {
      const data = await apiJson<CommunityPost[] | null>(
        "/admin/community/posts",
      );
      setPosts(Array.isArray(data) ? data : []);
      setError(null);
    } catch (err) {
      if (err instanceof ApiError) {
        setPosts([]);
        setError(
          err.status === 401 || err.status === 403
            ? "This session is not an administrator, so posts can't be managed. Sign in with an address in ADMIN_EMAILS."
            : `Could not load posts (${err.status}).`,
        );
      } else {
        demoRef.current = true;
        setIsDemoMode(true);
        setPosts(MOCK_POSTS);
        setError(
          "Backend unreachable. Showing sample posts; changes won't be saved.",
        );
      }
    } finally {
      setIsLoading(false);
    }
  }, []);

  React.useEffect(() => {
    load();
  }, [load]);

  const refresh = React.useCallback(async () => {
    demoRef.current = false;
    setIsDemoMode(false);
    setIsLoading(true);
    await load();
  }, [load]);

  const create = React.useCallback(async (input: CommunityPostInput) => {
    let post: CommunityPost;
    if (demoRef.current) {
      const now = new Date().toISOString();
      post = {
        ...input,
        id: `demo-${Date.now()}`,
        published_at: input.published ? now : null,
        author_id: "u-demo",
        author_username: "you",
        author_avatar_url: "",
        created_at: now,
        updated_at: now,
      };
    } else {
      post = await apiJson<CommunityPost>("/admin/community/posts", {
        method: "POST",
        body: JSON.stringify(input),
      });
    }
    setPosts((prev) => sortPosts([post, ...prev]));
    return post;
  }, []);

  const update = React.useCallback(
    async (id: string, input: CommunityPostInput) => {
      let post: CommunityPost;
      if (demoRef.current) {
        const existing = posts.find((p) => p.id === id);
        if (!existing) throw new Error("post not found");
        const now = new Date().toISOString();
        post = {
          ...existing,
          ...input,
          published_at: input.published ? (existing.published_at ?? now) : null,
          updated_at: now,
        };
      } else {
        post = await apiJson<CommunityPost>(`/admin/community/posts/${id}`, {
          method: "PATCH",
          body: JSON.stringify(input),
        });
      }
      setPosts((prev) => sortPosts(prev.map((p) => (p.id === id ? post : p))));
      return post;
    },
    [posts],
  );

  const remove = React.useCallback(async (id: string) => {
    if (!demoRef.current) {
      try {
        await apiJson<void>(`/admin/community/posts/${id}`, {
          method: "DELETE",
        });
      } catch (err) {
        // Already gone is as good as deleted: the list was stale.
        if (!(err instanceof ApiError && err.status === 404)) throw err;
      }
    }
    setPosts((prev) => prev.filter((p) => p.id !== id));
  }, []);

  return {
    posts,
    isLoading,
    isDemoMode,
    error,
    refresh,
    create,
    update,
    remove,
  };
}

/** The fields of a post as the editor and quick actions send them back. */
export function inputOf(post: CommunityPost): CommunityPostInput {
  return {
    category: post.category,
    title: post.title,
    body: post.body,
    pinned: post.pinned,
    published: post.published_at !== null,
  };
}
