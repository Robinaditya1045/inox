import { apiClient } from "../../api/client";
import type {
  CommunityCategory,
  CommunityFeedPage,
} from "../../types/community";

/** The Community feed: platform updates, pinned first, then newest first. */
export const communityService = {
  async feed(
    options: {
      category?: CommunityCategory;
      limit?: number;
      offset?: number;
    } = {},
  ): Promise<CommunityFeedPage> {
    const params: Record<string, string | number> = {};
    if (options.category) params.category = options.category;
    if (options.limit) params.limit = options.limit;
    if (options.offset) params.offset = options.offset;
    const page = await apiClient.get<CommunityFeedPage>("/community/posts", {
      params,
    });
    return {
      posts: Array.isArray(page?.posts) ? page.posts : [],
      has_more: !!page?.has_more,
    };
  },
};
