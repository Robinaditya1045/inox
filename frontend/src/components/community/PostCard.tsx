import React, { type CSSProperties } from "react";
import { Pin } from "lucide-react";
import { Avatar } from "../common/Avatar";
import { Badge } from "../common/Badge";
import { PostBody } from "./PostBody";
import { CATEGORIES } from "./categories";
import type { CommunityPost } from "../../types/community";
import styles from "./PostCard.module.css";

const dateFormat = new Intl.DateTimeFormat(undefined, {
  month: "short",
  day: "numeric",
  year: "numeric",
});
const fullFormat = new Intl.DateTimeFormat(undefined, {
  dateStyle: "full",
  timeStyle: "short",
});

interface PostCardProps {
  post: CommunityPost;
  /** Published since the viewer last opened the page */
  isNew?: boolean;
  style?: CSSProperties;
}

export const PostCard: React.FC<PostCardProps> = ({ post, isNew, style }) => {
  const category = CATEGORIES[post.category] ?? CATEGORIES.announcement;
  const Icon = category.icon;
  const published = post.published_at ? new Date(post.published_at) : null;
  const author = post.author_username || "Inox team";
  const titleId = `post-${post.id}-title`;

  return (
    <article
      id={`post-${post.id}`}
      className={styles.card}
      data-new={isNew || undefined}
      data-pinned={post.pinned || undefined}
      aria-labelledby={titleId}
      style={style}
    >
      <div className={styles.tags}>
        <Badge variant={category.badge}>
          <Icon size={11} aria-hidden="true" />
          {category.label}
        </Badge>
        {post.pinned && (
          <span className={styles.pinned}>
            <Pin size={12} aria-hidden="true" />
            Pinned
          </span>
        )}
        {isNew && (
          <Badge variant="danger" pop>
            New
          </Badge>
        )}
      </div>

      <h2 id={titleId} className={styles.title}>
        {post.title}
      </h2>

      <PostBody body={post.body} />

      <footer className={styles.byline}>
        <Avatar
          src={post.author_avatar_url || null}
          username={author}
          size="xs"
        />
        <span className={styles.author}>{author}</span>
        {published && (
          <>
            <span aria-hidden="true">·</span>
            <time
              dateTime={post.published_at ?? undefined}
              title={fullFormat.format(published)}
            >
              {dateFormat.format(published)}
            </time>
          </>
        )}
      </footer>
    </article>
  );
};
