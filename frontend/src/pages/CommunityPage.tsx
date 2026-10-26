import React, { useEffect, useState } from "react";
import { Megaphone, RefreshCw } from "lucide-react";
import { useCommunityFeed } from "../hooks/useCommunityFeed";
import { Alert } from "../components/common/Alert";
import { Button } from "../components/common/Button";
import { EmptyState } from "../components/common/EmptyState";
import { Skeleton } from "../components/common/Skeleton";
import { Tabs } from "../components/common/Tabs";
import { PageHeader } from "../components/layout/PageHeader";
import { PostCard } from "../components/community/PostCard";
import { CATEGORIES, CATEGORY_ORDER } from "../components/community/categories";
import type { CommunityCategory } from "../types/community";
import { isUnread, markSeen, readLastSeen } from "../utils/communitySeen";
import { stagger } from "../utils/motion";
import { tabIds } from "../utils/tabs";
import styles from "./CommunityPage.module.css";

type Tab = "all" | CommunityCategory;

const TAB_PREFIX = "community";

const CardSkeletons: React.FC = () => (
  <div className={styles.feed} aria-busy="true" aria-label="Loading updates">
    {[0, 1, 2].map((i) => (
      <div key={i} className={styles.skeletonCard}>
        <Skeleton width={96} height={18} radius="var(--radius-full)" />
        <Skeleton width={`${62 - i * 10}%`} height={20} />
        <Skeleton width="100%" height={11} />
        <Skeleton width="88%" height={11} />
        <Skeleton width="40%" height={11} />
      </div>
    ))}
  </div>
);

/** Platform updates from the Inox team: new features, improvements, fixes and news. */
export const CommunityPage: React.FC = () => {
  const [tab, setTab] = useState<Tab>("all");
  const { posts, isLoading, isLoadingMore, hasMore, error, loadMore, refresh } =
    useCommunityFeed(tab === "all" ? null : tab);

  // What counted as seen when the page opened: posts newer than this keep their
  // "New" mark for the whole visit, even though opening the page clears the badge.
  const [lastSeen] = useState(readLastSeen);

  useEffect(() => {
    if (posts.length > 0) markSeen(posts.map((p) => p.published_at));
  }, [posts]);

  const panel = tabIds(TAB_PREFIX, tab);
  const filtered = tab !== "all";

  return (
    <div className={styles.page}>
      <PageHeader icon={<Megaphone size={20} />} title="Community">
        <Tabs<Tab>
          variant="pill"
          ariaLabel="Update categories"
          idPrefix={TAB_PREFIX}
          value={tab}
          onChange={setTab}
          items={[
            { id: "all", label: "All" },
            ...CATEGORY_ORDER.map((id) => ({
              id,
              label: CATEGORIES[id].plural,
            })),
          ]}
        />
      </PageHeader>

      <div className={styles.body}>
        <div
          key={tab}
          className={styles.panel}
          role="tabpanel"
          id={panel.panel}
          aria-labelledby={panel.tab}
        >
          <div className={styles.intro}>
            <h2 className={styles.introTitle}>What's new on Inox</h2>
            <p className={styles.introText}>
              New features, improvements, fixes and news from the team.
            </p>
          </div>

          {error && (
            <div className={styles.errorRow}>
              <Alert>{error}</Alert>
              <Button
                variant="secondary"
                size="sm"
                icon={<RefreshCw size={14} />}
                onClick={refresh}
              >
                Try again
              </Button>
            </div>
          )}

          {isLoading && posts.length === 0 ? (
            <CardSkeletons />
          ) : posts.length === 0 && !error ? (
            <EmptyState
              card
              as="h2"
              icon={<Megaphone size={24} />}
              title={
                filtered
                  ? `No ${CATEGORIES[tab].plural.toLowerCase()} yet`
                  : "No updates yet"
              }
              description="When the team ships something or has news to share, it'll be posted here."
            />
          ) : (
            <div className={styles.feed}>
              {posts.map((post, i) => (
                <PostCard
                  key={post.id}
                  post={post}
                  isNew={isUnread(post.published_at, lastSeen)}
                  style={stagger(i)}
                />
              ))}
            </div>
          )}

          {hasMore && (
            <div className={styles.more}>
              <Button
                variant="secondary"
                isLoading={isLoadingMore}
                onClick={loadMore}
              >
                Show older updates
              </Button>
            </div>
          )}
        </div>
      </div>
    </div>
  );
};
