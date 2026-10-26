export type CommunityCategory =
  "feature" | "improvement" | "fix" | "announcement";

/** A platform update shown on the viewer's Community page (backend domain.CommunityPost). */
export interface CommunityPost {
  id: string;
  category: CommunityCategory;
  title: string;
  /** Plain text: blank lines split paragraphs, "- " lines are list items */
  body: string;
  pinned: boolean;
  /** null while the post is a draft */
  published_at: string | null;
  author_id: string;
  author_username: string;
  author_avatar_url: string;
  created_at: string;
  updated_at: string;
}

/** What the editor sends; the server stamps the publish date. */
export interface CommunityPostInput {
  category: CommunityCategory;
  title: string;
  body: string;
  pinned: boolean;
  published: boolean;
}
