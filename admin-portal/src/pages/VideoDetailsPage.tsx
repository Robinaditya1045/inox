import * as React from "react"
import { Link, useBlocker, useNavigate } from "react-router-dom"
import { Captions, Copy, EllipsisVertical, ExternalLink, Loader2, Plus, Star, Trash2 } from "lucide-react"
import type { MediaAsset, MediaDetailsUpdate, MediaRendition } from "@/types/media"
import { PageBody } from "@/components/layout/PageHeader"
import { VideoPlayer } from "@/components/content/VideoPlayer"
import { StatusBadge } from "@/components/content/StatusBadge"
import { ThumbnailSection, type PendingThumbnail } from "@/components/content/ThumbnailSection"
import { Button } from "@/components/ui/button"
import { IconButton } from "@/components/ui/icon-button"
import { Menu } from "@/components/ui/menu"
import { ConfirmDialog } from "@/components/ui/confirm-dialog"
import { useCatalog, useVideoAsset } from "@/lib/contexts"
import { useToast } from "@/hooks/useToast"
import { errorMessage } from "@/lib/api"
import { fileNameOf, formatDateTime, formatDuration, formatRelative, hostOf } from "@/lib/format"
import { mediaOrigin, playbackUrl } from "@/lib/media"
import { copyText, cx } from "@/lib/utils"

const TITLE_MAX = 255
const DESCRIPTION_MAX = 5000

export function VideoDetailsPage() {
  const asset = useVideoAsset()
  // Keyed so that moving to another video starts a fresh form.
  return <DetailsForm key={asset.id} asset={asset} />
}

function DetailsForm({ asset }: { asset: MediaAsset }) {
  const { updateDetails, uploadThumbnail, resetThumbnail, deleteMedia, fetchAsset } = useCatalog()
  const { toast } = useToast()
  const navigate = useNavigate()

  const [title, setTitle] = React.useState(asset.title)
  const [description, setDescription] = React.useState(asset.description ?? "")
  const [pendingThumb, setPendingThumbState] = React.useState<PendingThumbnail | null>(null)
  const [saving, setSaving] = React.useState(false)
  const [confirmDelete, setConfirmDelete] = React.useState(false)
  const [renditions, setRenditions] = React.useState<MediaRendition[] | null>(asset.renditions ?? null)

  // Object URLs of staged images are released as soon as they are replaced.
  const setPendingThumb = (next: PendingThumbnail | null) => {
    setPendingThumbState((prev) => {
      if (prev?.kind === "image" && prev !== next) URL.revokeObjectURL(prev.previewUrl)
      return next
    })
  }

  // The list endpoint carries no renditions; the single-asset read does.
  React.useEffect(() => {
    let cancelled = false
    fetchAsset(asset.id)
      .then((full) => !cancelled && setRenditions(full?.renditions ?? []))
      .catch(() => !cancelled && setRenditions([]))
    return () => {
      cancelled = true
    }
  }, [asset.id, asset.status, fetchAsset])

  const titleChanged = title.trim() !== asset.title
  const descriptionChanged = description.trim() !== (asset.description ?? "").trim()
  const dirty = titleChanged || descriptionChanged || pendingThumb !== null
  const titleError = title.trim() === "" ? "Your video needs a title" : title.length > TITLE_MAX ? `Keep the title under ${TITLE_MAX} characters` : null
  const descriptionError = description.length > DESCRIPTION_MAX ? `Keep the description under ${DESCRIPTION_MAX} characters` : null

  // Leaving with unsaved edits asks first — inside the app, and on reload/close.
  // Deleting the video navigates away on purpose, and has nothing left to save.
  const leavingRef = React.useRef(false)
  const blocker = useBlocker(
    ({ currentLocation, nextLocation }) => dirty && !saving && !leavingRef.current && currentLocation.pathname !== nextLocation.pathname,
  )
  React.useEffect(() => {
    if (!dirty) return
    const onBeforeUnload = (e: BeforeUnloadEvent) => e.preventDefault()
    window.addEventListener("beforeunload", onBeforeUnload)
    return () => window.removeEventListener("beforeunload", onBeforeUnload)
  }, [dirty])

  const undo = () => {
    setTitle(asset.title)
    setDescription(asset.description ?? "")
    setPendingThumb(null)
  }

  const save = async () => {
    if (titleError || descriptionError) return
    setSaving(true)
    try {
      const update: MediaDetailsUpdate = {}
      if (titleChanged) update.title = title.trim()
      if (descriptionChanged) update.description = description.trim()
      if (pendingThumb?.kind === "link") update.thumbnail_url = pendingThumb.url

      let saved: MediaAsset = asset
      if (Object.keys(update).length > 0) saved = await updateDetails(asset.id, update)
      if (pendingThumb?.kind === "image") saved = await uploadThumbnail(asset.id, pendingThumb.blob)
      if (pendingThumb?.kind === "auto") saved = await resetThumbnail(asset.id)

      // The server trims; adopt what it stored so the form reads as saved.
      setTitle(saved.title)
      setDescription(saved.description ?? "")
      setPendingThumb(null)
      toast(pendingThumb?.kind === "auto" ? "Saved. The new thumbnail appears once a frame has been captured." : "Changes saved", { tone: "success" })
    } catch (err) {
      toast(`Couldn't save: ${errorMessage(err)}`, { tone: "error" })
    } finally {
      setSaving(false)
    }
  }

  const copy = async (text: string, what: string) => {
    toast((await copyText(text)) ? `${what} copied` : "Could not copy to the clipboard")
  }

  const url = playbackUrl(asset)
  const origin = mediaOrigin(asset)
  const captions = asset.captions ?? []

  return (
    <PageBody wide>
      <div className="flex flex-wrap items-center gap-3">
        <h1 className="mr-auto text-2xl font-semibold tracking-tight text-white">Video details</h1>
        <Button variant="ghost" onClick={undo} disabled={!dirty || saving}>
          Undo changes
        </Button>
        <Button variant="default" onClick={save} disabled={!dirty || saving || !!titleError || !!descriptionError} className="min-w-20">
          {saving ? <Loader2 className="h-4 w-4 animate-spin" aria-label="Saving" /> : "Save"}
        </Button>
        <Menu
          label="More actions"
          items={[
            { label: "Copy video link", icon: Copy, onSelect: () => copy(url, "Video link") },
            { label: "Open stream in new tab", icon: ExternalLink, onSelect: () => window.open(url, "_blank", "noopener") },
            { label: "Delete forever", icon: Trash2, destructive: true, separated: true, onSelect: () => setConfirmDelete(true) },
          ]}
          trigger={(props) => (
            <IconButton {...props} label="More actions">
              <EllipsisVertical className="h-5 w-5" />
            </IconButton>
          )}
        />
      </div>

      {/* grid-cols-1, not the implicit auto column: that grows to fit the video
          link, which does not wrap, and pushed the page past a phone's width. */}
      <div className="mt-6 grid grid-cols-1 gap-8 lg:grid-cols-[minmax(0,1fr)_minmax(320px,400px)]">
        {/* Form */}
        <div className="min-w-0 space-y-8">
          <div className="space-y-5">
            <h2 className="text-base font-medium text-white">Details</h2>
            <OutlinedField
              id="title"
              label="Title (required)"
              value={title}
              onChange={setTitle}
              max={TITLE_MAX}
              rows={2}
              error={titleError}
              placeholder="Add a title that describes your video"
            />
            <OutlinedField
              id="description"
              label="Description"
              value={description}
              onChange={setDescription}
              max={DESCRIPTION_MAX}
              rows={7}
              error={descriptionError}
              placeholder="Tell viewers what the video is about"
            />
          </div>

          <ThumbnailSection asset={asset} pending={pendingThumb} onChange={setPendingThumb} />

          <section aria-labelledby="captions-heading">
            <div className="flex items-center justify-between gap-3">
              <h2 id="captions-heading" className="text-base font-medium text-white">
                Captions
              </h2>
              {asset.kind !== "live" && (
                <Link to={`/content/${asset.id}/captions`} className="text-sm font-medium text-emerald-400 hover:text-emerald-300">
                  Manage captions
                </Link>
              )}
            </div>
            {asset.kind === "live" ? (
              <p className="mt-1 text-sm text-zinc-500">Live channels can't have caption tracks.</p>
            ) : captions.length === 0 ? (
              <div className="mt-3 flex flex-wrap items-center gap-4 rounded-lg border border-dashed border-zinc-700 px-4 py-4">
                <Captions className="h-5 w-5 text-zinc-500" aria-hidden="true" />
                <p className="flex-1 text-sm text-zinc-400">No captions yet. Add them so everyone in the room can follow along.</p>
                <Button variant="outline" size="sm" onClick={() => navigate(`/content/${asset.id}/captions?add=1`)}>
                  <Plus className="mr-1 h-3.5 w-3.5" aria-hidden="true" /> Add captions
                </Button>
              </div>
            ) : (
              <ul className="mt-3 divide-y divide-zinc-800 rounded-lg border border-zinc-800">
                {captions.map((c) => (
                  <li key={c.id} className="flex items-center gap-3 px-4 py-3 text-sm">
                    <span className="rounded border border-zinc-700 px-1.5 py-px font-mono text-[11px] uppercase text-zinc-300">{c.language}</span>
                    <span className="flex-1 truncate text-zinc-200">{c.label}</span>
                    {c.is_default && (
                      <span className="inline-flex items-center gap-1 text-xs text-emerald-300">
                        <Star className="h-3 w-3 fill-current" aria-hidden="true" /> Default
                      </span>
                    )}
                    <span className="text-xs text-zinc-500 tabular-nums">{c.cue_count} lines</span>
                  </li>
                ))}
              </ul>
            )}
          </section>
        </div>

        {/* Preview and facts */}
        <aside className="lg:sticky lg:top-20 lg:self-start">
          <div className="overflow-hidden rounded-xl border border-zinc-800 bg-surface">
            {url ? (
              <VideoPlayer src={url} poster={asset.thumbnail_url || undefined} controls preload="metadata" className="aspect-video w-full" />
            ) : (
              <div className="flex aspect-video items-center justify-center bg-black text-sm text-zinc-500">No playable source yet</div>
            )}
            <dl className="space-y-4 p-4 text-sm">
              <Fact label="Video link">
                <div className="flex items-center gap-1">
                  <a href={url} target="_blank" rel="noreferrer" className="min-w-0 flex-1 truncate text-sky-400 hover:underline">
                    {url || "—"}
                  </a>
                  {url && (
                    <IconButton label="Copy video link" size="sm" onClick={() => copy(url, "Video link")}>
                      <Copy className="h-4 w-4" />
                    </IconButton>
                  )}
                </div>
              </Fact>
              <Fact label={origin.origin === "upload" ? "Source file" : "Source"}>
                <span className="block truncate text-zinc-200" title={asset.source_url}>
                  {origin.origin === "upload" ? fileNameOf(asset.source_url) : origin.origin === "live" ? "Live channel" : hostOf(asset.source_url) || asset.source_url}
                </span>
              </Fact>
              <div className="grid grid-cols-2 gap-4">
                <Fact label="Status">
                  <StatusBadge asset={asset} />
                </Fact>
                <Fact label="Duration">
                  <span className="text-zinc-200">{asset.kind === "live" ? "Live" : formatDuration(asset.duration_seconds) || "Unknown"}</span>
                </Fact>
              </div>
              <Fact label="Quality">
                {renditions === null ? (
                  <span className="text-zinc-500">Loading…</span>
                ) : renditions.length === 0 ? (
                  <span className="text-zinc-400">{origin.origin === "upload" ? "Not transcoded yet" : "Played as linked"}</span>
                ) : (
                  <span className="flex flex-wrap gap-1.5">
                    {renditions.map((r) => (
                      <span key={r.id} className="rounded bg-zinc-800 px-1.5 py-0.5 font-mono text-[11px] text-zinc-200" title={`${r.bitrate_kbps} kbps`}>
                        {r.resolution} · {Math.round(r.bitrate_kbps / 100) / 10} Mbps
                      </span>
                    ))}
                  </span>
                )}
              </Fact>
              <Fact label="Asset ID">
                <div className="flex items-center gap-1">
                  <span className="min-w-0 flex-1 truncate font-mono text-xs text-zinc-300">{asset.id}</span>
                  <IconButton label="Copy asset ID" size="sm" onClick={() => copy(asset.id, "Asset ID")}>
                    <Copy className="h-4 w-4" />
                  </IconButton>
                </div>
              </Fact>
              <div className="grid grid-cols-2 gap-4">
                <Fact label={origin.origin === "upload" ? "Uploaded" : "Added"}>
                  <span className="text-zinc-200">{formatDateTime(asset.created_at)}</span>
                </Fact>
                <Fact label="Last updated">
                  <span className="text-zinc-200" title={formatDateTime(asset.updated_at)}>
                    {formatRelative(asset.updated_at)}
                  </span>
                </Fact>
              </div>
            </dl>
          </div>
        </aside>
      </div>

      <ConfirmDialog
        open={blocker.state === "blocked"}
        title="Discard unsaved changes?"
        description="You have edits on this page that haven't been saved."
        confirmLabel="Discard"
        destructive
        onConfirm={() => blocker.proceed?.()}
        onClose={() => blocker.state === "blocked" && blocker.reset?.()}
      />

      <ConfirmDialog
        open={confirmDelete}
        destructive
        title="Permanently delete this video?"
        description={`"${asset.title}" will be removed from the library along with its captions. Rooms playing it will stop being able to load it.`}
        acknowledgement="I understand that deleting is permanent and can't be undone"
        confirmLabel="Delete forever"
        onConfirm={async () => {
          try {
            await deleteMedia(asset.id)
            toast("Video deleted", { tone: "success" })
            leavingRef.current = true
            navigate("/content", { replace: true })
          } catch (err) {
            toast(`Couldn't delete: ${errorMessage(err)}`, { tone: "error" })
          }
        }}
        onClose={() => setConfirmDelete(false)}
      />
    </PageBody>
  )
}

function Fact({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="min-w-0">
      <dt className="text-xs text-zinc-500">{label}</dt>
      <dd className="mt-0.5">{children}</dd>
    </div>
  )
}

interface OutlinedFieldProps {
  id: string
  label: string
  value: string
  onChange: (value: string) => void
  max: number
  rows: number
  error: string | null
  placeholder?: string
}

/** Studio's text field: the label sits inside the outline, the count in the corner. */
function OutlinedField({ id, label, value, onChange, max, rows, error, placeholder }: OutlinedFieldProps) {
  return (
    <div>
      <div
        className={cx(
          "rounded-lg border bg-black/20 px-3 pb-2 pt-2 transition-colors",
          error ? "border-rose-500/70 focus-within:border-rose-400" : "border-zinc-700 hover:border-zinc-600 focus-within:border-zinc-300",
        )}
      >
        <label htmlFor={id} className={cx("block text-xs font-medium", error ? "text-rose-300" : "text-zinc-400")}>
          {label}
        </label>
        <textarea
          id={id}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          rows={rows}
          placeholder={placeholder}
          aria-invalid={!!error}
          aria-describedby={`${id}-count${error ? ` ${id}-error` : ""}`}
          className="mt-1 block w-full resize-y bg-transparent text-sm leading-relaxed text-zinc-100 placeholder:text-zinc-600 focus:outline-none"
        />
        <p id={`${id}-count`} className={cx("text-right text-xs tabular-nums", value.length > max ? "text-rose-300" : "text-zinc-500")}>
          {value.length}/{max}
        </p>
      </div>
      {error && (
        <p id={`${id}-error`} className="mt-1.5 text-xs text-rose-300">
          {error}
        </p>
      )}
    </div>
  )
}
