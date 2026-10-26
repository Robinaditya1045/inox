import * as React from "react";
import {
  EyeOff,
  Megaphone,
  MoreVertical,
  Pencil,
  Pin,
  PinOff,
  Plus,
  RefreshCw,
  Send,
  Trash2,
} from "lucide-react";
import { PageBody, PageHeader } from "@/components/layout/PageHeader";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { IconButton } from "@/components/ui/icon-button";
import { Menu, type MenuItem } from "@/components/ui/menu";
import { CATEGORIES } from "@/components/community/categories";
import { PostEditorDialog } from "@/components/community/PostEditorDialog";
import { inputOf, useCommunityPosts } from "@/hooks/useCommunityPosts";
import { useToast } from "@/hooks/useToast";
import { errorMessage } from "@/lib/api";
import { formatDateTime, formatRelative } from "@/lib/format";
import { cx } from "@/lib/utils";
import type { CommunityPost, CommunityPostInput } from "@/types/community";

type Filter = "all" | "published" | "drafts";

/** Posts for the viewer's Community page: new features, fixes and news. */
export function CommunityPage() {
  const {
    posts,
    isLoading,
    isDemoMode,
    error,
    refresh,
    create,
    update,
    remove,
  } = useCommunityPosts();
  const { toast } = useToast();
  const [filter, setFilter] = React.useState<Filter>("all");
  const [editor, setEditor] = React.useState<{
    open: boolean;
    post: CommunityPost | null;
  }>({ open: false, post: null });
  const [deleting, setDeleting] = React.useState<CommunityPost | null>(null);

  const drafts = posts.filter((p) => !p.published_at).length;
  const shown = posts.filter((p) =>
    filter === "all"
      ? true
      : filter === "drafts"
        ? !p.published_at
        : !!p.published_at,
  );

  const save = async (input: CommunityPostInput) => {
    const editing = editor.post;
    if (editing) await update(editing.id, input);
    else await create(input);
    toast(
      input.published
        ? editing?.published_at
          ? "Post updated"
          : "Post published"
        : editing?.published_at
          ? "Moved to drafts"
          : "Draft saved",
      { tone: "success" },
    );
  };

  /** One-click changes from a row's menu: pin, publish, unpublish. */
  const quick = async (
    post: CommunityPost,
    change: Partial<CommunityPostInput>,
    done: string,
  ) => {
    try {
      await update(post.id, { ...inputOf(post), ...change });
      toast(done, { tone: "success" });
    } catch (err) {
      toast(errorMessage(err, "Couldn't update the post"), { tone: "error" });
    }
  };

  const menuFor = (post: CommunityPost): MenuItem[] => {
    const published = !!post.published_at;
    return [
      {
        label: "Edit",
        icon: Pencil,
        onSelect: () => setEditor({ open: true, post }),
      },
      published
        ? {
            label: "Move to drafts",
            icon: EyeOff,
            onSelect: () =>
              quick(post, { published: false }, "Moved to drafts"),
            hint: "Hides it from viewers",
          }
        : {
            label: "Publish now",
            icon: Send,
            onSelect: () => quick(post, { published: true }, "Post published"),
          },
      post.pinned
        ? {
            label: "Unpin",
            icon: PinOff,
            onSelect: () => quick(post, { pinned: false }, "Post unpinned"),
          }
        : {
            label: "Pin to top",
            icon: Pin,
            onSelect: () => quick(post, { pinned: true }, "Post pinned"),
          },
      {
        label: "Delete",
        icon: Trash2,
        destructive: true,
        separated: true,
        onSelect: () => setDeleting(post),
      },
    ];
  };

  return (
    <PageBody className="max-w-5xl">
      <PageHeader
        title="Community"
        description="Updates every viewer sees on the Community page: new features, improvements, fixes and news."
        className="mb-6"
        actions={
          <>
            <IconButton
              label="Refresh"
              tooltip="bottom"
              onClick={() => refresh()}
              disabled={isLoading}
            >
              <RefreshCw
                className={cx("h-4 w-4", isLoading && "animate-spin")}
              />
            </IconButton>
            <Button
              variant="emerald"
              onClick={() => setEditor({ open: true, post: null })}
              disabled={!!error && !isDemoMode}
            >
              <Plus className="mr-1.5 h-4 w-4" />
              New post
            </Button>
          </>
        }
      />

      {error && (
        <p
          role={isDemoMode ? "status" : "alert"}
          className={cx(
            "mb-4 rounded-lg border px-4 py-3 text-sm",
            isDemoMode
              ? "border-amber-500/30 bg-amber-500/10 text-amber-300"
              : "border-rose-500/30 bg-rose-500/10 text-rose-300",
          )}
        >
          {error}
        </p>
      )}

      <div role="group" aria-label="Show" className="mb-4 flex flex-wrap gap-2">
        {(
          [
            ["all", `All ${posts.length}`],
            ["published", `Published ${posts.length - drafts}`],
            ["drafts", `Drafts ${drafts}`],
          ] as const
        ).map(([id, label]) => (
          <button
            key={id}
            type="button"
            aria-pressed={filter === id}
            onClick={() => setFilter(id)}
            className={cx(
              "cursor-pointer rounded-full border px-3 py-1 text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-emerald-400",
              filter === id
                ? "border-zinc-200 bg-zinc-100 text-black"
                : "border-zinc-700 text-zinc-300 hover:border-zinc-500",
            )}
          >
            {label}
          </button>
        ))}
      </div>

      {isLoading && posts.length === 0 ? (
        <ul
          aria-busy="true"
          aria-label="Loading posts"
          className="flex flex-col gap-3"
        >
          {[0, 1, 2].map((i) => (
            <li
              key={i}
              className="h-28 animate-pulse rounded-xl border border-zinc-800 bg-zinc-900/40"
            />
          ))}
        </ul>
      ) : shown.length === 0 ? (
        !error || isDemoMode ? (
          <div className="flex flex-col items-center rounded-xl border border-dashed border-zinc-800 px-6 py-14 text-center">
            <Megaphone className="h-8 w-8 text-zinc-600" aria-hidden="true" />
            <p className="mt-3 font-medium text-zinc-200">
              {filter === "drafts" ? "No drafts" : "Nothing posted yet"}
            </p>
            <p className="mt-1 max-w-sm text-sm text-zinc-500">
              Tell viewers what's new: a feature you shipped, a fix, or news
              about the platform.
            </p>
            <Button
              variant="outline-emerald"
              className="mt-5"
              onClick={() => setEditor({ open: true, post: null })}
            >
              <Plus className="mr-1.5 h-4 w-4" />
              Write the first post
            </Button>
          </div>
        ) : null
      ) : (
        <ul className="flex flex-col gap-3">
          {shown.map((post) => (
            <PostRow
              key={post.id}
              post={post}
              menu={menuFor(post)}
              onEdit={() => setEditor({ open: true, post })}
            />
          ))}
        </ul>
      )}

      <PostEditorDialog
        open={editor.open}
        post={editor.post}
        onClose={() => setEditor((e) => ({ ...e, open: false }))}
        onSave={save}
      />

      <ConfirmDialog
        open={!!deleting}
        title="Delete this post?"
        description={
          <>
            <span className="font-medium text-zinc-200">{deleting?.title}</span>{" "}
            will be removed from the Community page for everyone. This can't be
            undone.
          </>
        }
        confirmLabel="Delete post"
        destructive
        onConfirm={async () => {
          if (!deleting) return;
          try {
            await remove(deleting.id);
            toast("Post deleted", { tone: "success" });
          } catch (err) {
            toast(errorMessage(err, "Couldn't delete the post"), {
              tone: "error",
            });
          }
        }}
        onClose={() => setDeleting(null)}
      />
    </PageBody>
  );
}

function PostRow({
  post,
  menu,
  onEdit,
}: {
  post: CommunityPost;
  menu: MenuItem[];
  onEdit: () => void;
}) {
  const category = CATEGORIES[post.category] ?? CATEGORIES.announcement;
  const Icon = category.icon;
  const published = !!post.published_at;
  // The body's first line is enough to tell posts apart in the list.
  const excerpt = post.body
    .replace(/^\s*[-*]\s+/gm, "")
    .replace(/\s+/g, " ")
    .trim();

  return (
    <li className="group flex animate-enter gap-3 rounded-xl border border-zinc-800 bg-zinc-900/40 p-4 transition-colors hover:border-zinc-700 sm:p-5">
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-2">
          <Badge variant={category.badge} className="gap-1">
            <Icon className="h-3 w-3" aria-hidden="true" />
            {category.label}
          </Badge>
          {!published && <Badge variant="outline">Draft</Badge>}
          {post.pinned && (
            <span className="inline-flex items-center gap-1 text-[11px] font-bold uppercase tracking-wide text-emerald-400">
              <Pin className="h-3 w-3" aria-hidden="true" />
              Pinned
            </span>
          )}
        </div>
        <button
          type="button"
          onClick={onEdit}
          className="mt-2 block max-w-full cursor-pointer text-left text-base font-semibold text-white hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-emerald-400 [overflow-wrap:anywhere]"
        >
          {post.title}
        </button>
        <p className="mt-1 line-clamp-2 text-sm text-zinc-400">{excerpt}</p>
        <p className="mt-2 text-xs text-zinc-500">
          {published ? (
            <time
              dateTime={post.published_at ?? undefined}
              title={formatDateTime(post.published_at ?? undefined)}
            >
              Published {formatRelative(post.published_at ?? undefined)}
            </time>
          ) : (
            <time
              dateTime={post.updated_at}
              title={formatDateTime(post.updated_at)}
            >
              Edited {formatRelative(post.updated_at)}
            </time>
          )}
          {post.author_username && <> · by {post.author_username}</>}
        </p>
      </div>
      <div className="shrink-0">
        <Menu
          label={`Actions for ${post.title}`}
          items={menu}
          trigger={(props) => (
            <IconButton
              {...props}
              label="Post actions"
              size="sm"
              tooltip="none"
            >
              <MoreVertical className="h-4 w-4" />
            </IconButton>
          )}
        />
      </div>
    </li>
  );
}
