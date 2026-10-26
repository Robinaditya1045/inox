import * as React from "react";
import { Loader2, Pin } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { CATEGORIES, CATEGORY_ORDER } from "@/components/community/categories";
import { PostBody } from "@/components/community/PostBody";
import { inputOf } from "@/hooks/useCommunityPosts";
import { errorMessage } from "@/lib/api";
import { cx } from "@/lib/utils";
import type { CommunityPost, CommunityPostInput } from "@/types/community";

// Mirrors the limits in backend community.Service.
const MAX_TITLE = 140;
const MAX_BODY = 10_000;

const BLANK: CommunityPostInput = {
  category: "feature",
  title: "",
  body: "",
  pinned: false,
  published: false,
};

interface PostEditorDialogProps {
  open: boolean;
  /** The post being edited; null writes a new one */
  post: CommunityPost | null;
  onClose: () => void;
  /** Saves and resolves once the server has the post; rejects to keep the dialog open */
  onSave: (input: CommunityPostInput) => Promise<void>;
}

export function PostEditorDialog({
  open,
  post,
  onClose,
  onSave,
}: PostEditorDialogProps) {
  const initial = React.useMemo(() => (post ? inputOf(post) : BLANK), [post]);
  const [draft, setDraft] = React.useState<CommunityPostInput>(initial);
  const [view, setView] = React.useState<"write" | "preview">("write");
  const [saving, setSaving] = React.useState<"draft" | "publish" | null>(null);
  const [error, setError] = React.useState<string | null>(null);
  const [confirmDiscard, setConfirmDiscard] = React.useState(false);

  // Each opening starts from the post as it is now.
  const [openedFor, setOpenedFor] = React.useState<{
    open: boolean;
    initial: CommunityPostInput;
  }>({ open, initial });
  if (openedFor.open !== open || openedFor.initial !== initial) {
    setOpenedFor({ open, initial });
    if (open) {
      setDraft(initial);
      setView("write");
      setError(null);
    }
  }

  const isPublished = post?.published_at != null;
  const dirty =
    draft.title !== initial.title ||
    draft.body !== initial.body ||
    draft.category !== initial.category ||
    draft.pinned !== initial.pinned;

  const titleLength = draft.title.trim().length;
  const bodyLength = draft.body.trim().length;
  const valid =
    titleLength > 0 &&
    titleLength <= MAX_TITLE &&
    bodyLength > 0 &&
    bodyLength <= MAX_BODY;

  const set = <K extends keyof CommunityPostInput>(
    key: K,
    value: CommunityPostInput[K],
  ) => setDraft((d) => ({ ...d, [key]: value }));

  const save = async (published: boolean) => {
    setSaving(published ? "publish" : "draft");
    setError(null);
    try {
      await onSave({ ...draft, published });
      onClose();
    } catch (err) {
      setError(errorMessage(err, "Couldn't save the post"));
    } finally {
      setSaving(null);
    }
  };

  const requestClose = () => {
    if (saving) return;
    if (dirty) setConfirmDiscard(true);
    else onClose();
  };

  const category = CATEGORIES[draft.category];
  const CategoryIcon = category.icon;

  return (
    <>
      <Dialog open={open} onOpenChange={(o) => !o && requestClose()}>
        <DialogContent className="sm:max-w-3xl">
          <DialogHeader>
            <DialogTitle>{post ? "Edit post" : "New post"}</DialogTitle>
            <DialogDescription>
              {isPublished
                ? "This post is live on the Community page. Edits keep its original date."
                : "Viewers see it on the Community page once it's published."}
            </DialogDescription>
          </DialogHeader>

          <fieldset>
            <legend className="mb-2 text-xs font-medium text-zinc-400">
              Category
            </legend>
            <div className="flex flex-wrap gap-2">
              {CATEGORY_ORDER.map((id) => {
                const meta = CATEGORIES[id];
                const Icon = meta.icon;
                const selected = draft.category === id;
                return (
                  <label
                    key={id}
                    className={cx(
                      "inline-flex cursor-pointer items-center gap-2 rounded-full border px-3 py-1.5 text-sm transition-colors has-[:focus-visible]:ring-2 has-[:focus-visible]:ring-emerald-400",
                      selected
                        ? "border-emerald-500/50 bg-emerald-500/10 text-emerald-300"
                        : "border-zinc-700 text-zinc-300 hover:border-zinc-500",
                    )}
                  >
                    <input
                      type="radio"
                      name="post-category"
                      value={id}
                      checked={selected}
                      onChange={() => set("category", id)}
                      className="sr-only"
                    />
                    <Icon className="h-4 w-4" aria-hidden="true" />
                    {meta.label}
                  </label>
                );
              })}
            </div>
          </fieldset>

          <div>
            <div className="mb-1.5 flex items-baseline justify-between">
              <label
                htmlFor="post-title"
                className="text-xs font-medium text-zinc-400"
              >
                Title
              </label>
              <span
                className={cx(
                  "text-xs tabular-nums",
                  draft.title.length > MAX_TITLE
                    ? "text-rose-300"
                    : "text-zinc-500",
                )}
              >
                {draft.title.length}/{MAX_TITLE}
              </span>
            </div>
            <Input
              id="post-title"
              value={draft.title}
              onChange={(e) => set("title", e.target.value)}
              placeholder="e.g. Playlists are here"
              autoComplete="off"
              className="h-10 text-base"
            />
          </div>

          <div>
            <div className="mb-1.5 flex items-center justify-between gap-3">
              <span
                id="post-body-label"
                className="text-xs font-medium text-zinc-400"
              >
                Post
              </span>
              <div
                role="tablist"
                aria-label="Editor view"
                className="inline-flex rounded-md border border-zinc-800 p-0.5 text-xs"
              >
                {(["write", "preview"] as const).map((v) => (
                  <button
                    key={v}
                    type="button"
                    role="tab"
                    aria-selected={view === v}
                    onClick={() => setView(v)}
                    className={cx(
                      "cursor-pointer rounded px-2.5 py-1 font-medium capitalize transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-emerald-400",
                      view === v
                        ? "bg-zinc-800 text-white"
                        : "text-zinc-400 hover:text-zinc-200",
                    )}
                  >
                    {v}
                  </button>
                ))}
              </div>
            </div>

            {view === "write" ? (
              <>
                <textarea
                  aria-labelledby="post-body-label"
                  aria-describedby="post-body-hint"
                  value={draft.body}
                  onChange={(e) => set("body", e.target.value)}
                  rows={10}
                  placeholder={
                    "What changed, and why it matters.\n\n- Start a line with a dash for a bullet\n- Links like https://… become clickable"
                  }
                  className="block min-h-48 w-full resize-y rounded-md border border-zinc-800 bg-zinc-900/50 px-3 py-2 text-sm leading-relaxed text-zinc-100 placeholder:text-zinc-600 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-emerald-400"
                />
                <div className="mt-1.5 flex items-start justify-between gap-3 text-xs text-zinc-500">
                  <p id="post-body-hint">
                    Plain text. A blank line starts a new paragraph; lines
                    starting with “- ” become a list.
                  </p>
                  <span
                    className={cx(
                      "shrink-0 tabular-nums",
                      draft.body.length > MAX_BODY && "text-rose-300",
                    )}
                  >
                    {draft.body.length}/{MAX_BODY}
                  </span>
                </div>
              </>
            ) : (
              <div
                role="tabpanel"
                className="min-h-48 rounded-lg border border-zinc-800 bg-zinc-950 p-5"
              >
                <div className="flex flex-wrap items-center gap-2">
                  <Badge variant={category.badge} className="gap-1">
                    <CategoryIcon className="h-3 w-3" aria-hidden="true" />
                    {category.viewerLabel}
                  </Badge>
                  {draft.pinned && (
                    <span className="inline-flex items-center gap-1 text-[11px] font-bold uppercase tracking-wide text-emerald-400">
                      <Pin className="h-3 w-3" aria-hidden="true" />
                      Pinned
                    </span>
                  )}
                </div>
                <h3 className="mt-3 text-xl font-bold tracking-tight text-white [overflow-wrap:anywhere]">
                  {draft.title.trim() || "Untitled post"}
                </h3>
                <div className="mt-3">
                  {draft.body.trim() ? (
                    <PostBody body={draft.body} />
                  ) : (
                    <p className="text-sm text-zinc-600">
                      Nothing written yet.
                    </p>
                  )}
                </div>
              </div>
            )}
          </div>

          <label className="flex cursor-pointer items-start gap-3 rounded-lg border border-zinc-800 bg-black/30 p-3 text-sm text-zinc-300">
            <input
              type="checkbox"
              checked={draft.pinned}
              onChange={(e) => set("pinned", e.target.checked)}
              className="mt-0.5 h-4 w-4 cursor-pointer accent-emerald-500"
            />
            <span>
              Pin to the top of the feed
              <span className="block text-xs text-zinc-500">
                For something every viewer should see first, like maintenance or
                a big launch.
              </span>
            </span>
          </label>

          {error && (
            <p
              role="alert"
              className="rounded-md border border-rose-500/30 bg-rose-500/10 px-3 py-2 text-sm text-rose-300"
            >
              {error}
            </p>
          )}

          <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
            <Button variant="ghost" onClick={requestClose} disabled={!!saving}>
              Cancel
            </Button>
            <Button
              variant="outline"
              onClick={() => save(false)}
              disabled={!valid || !!saving}
            >
              {saving === "draft" && (
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              )}
              {isPublished ? "Move to drafts" : "Save draft"}
            </Button>
            <Button
              variant="emerald"
              onClick={() => save(true)}
              disabled={!valid || !!saving || (isPublished && !dirty)}
            >
              {saving === "publish" && (
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              )}
              {isPublished ? "Save changes" : "Publish"}
            </Button>
          </div>
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={confirmDiscard}
        title="Discard changes?"
        description="What you've written in this post since opening it will be lost."
        confirmLabel="Discard"
        destructive
        onConfirm={() => {
          setConfirmDiscard(false);
          onClose();
        }}
        onClose={() => setConfirmDiscard(false)}
      />
    </>
  );
}
