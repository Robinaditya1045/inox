export type CommunityCategory =
  "feature" | "improvement" | "fix" | "announcement";

/** A platform update posted by the Inox team (see backend domain.CommunityPost). */
export interface CommunityPost {
  id: string;
  category: CommunityCategory;
  title: string;
  /** Plain text: blank lines split paragraphs, "- " lines are list items */
  body: string;
  pinned: boolean;
  /** Always set in the viewer feed, which never includes drafts */
  published_at: string | null;
  author_id: string;
  author_username: string;
  author_avatar_url: string;
  created_at: string;
  updated_at: string;
}

export interface CommunityFeedPage {
  posts: CommunityPost[];
  has_more: boolean;
}
