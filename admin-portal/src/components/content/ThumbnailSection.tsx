import * as React from "react"
import { Clapperboard, ImageUp, Link2, Undo2, WandSparkles, type LucideIcon } from "lucide-react"
import type { MediaAsset } from "@/types/media"
import { Thumb } from "@/components/content/Thumb"
import { ThumbnailCropDialog } from "@/components/content/ThumbnailCropDialog"
import { FramePickerDialog } from "@/components/content/FramePickerDialog"
import { ThumbnailLinkDialog } from "@/components/content/ThumbnailLinkDialog"
import { useToast } from "@/hooks/useToast"
import { cx } from "@/lib/utils"

/** A thumbnail change staged in the details form, applied on Save. */
export type PendingThumbnail =
  | { kind: "image"; blob: Blob; previewUrl: string }
  | { kind: "link"; url: string }
  | { kind: "auto" }

interface ThumbnailSectionProps {
  asset: MediaAsset
  pending: PendingThumbnail | null
  onChange: (pending: PendingThumbnail | null) => void
}

// Any image the browser can decode is accepted here: what is uploaded is always
// the cropped JPEG, well under the backend's 10 MB poster limit.
const MAX_SOURCE_BYTES = 40 << 20

export function ThumbnailSection({ asset, pending, onChange }: ThumbnailSectionProps) {
  const { toast } = useToast()
  const fileRef = React.useRef<HTMLInputElement>(null)
  const [cropSource, setCropSource] = React.useState<string | null>(null)
  const [framePicking, setFramePicking] = React.useState(false)
  const [linking, setLinking] = React.useState(false)

  const live = asset.kind === "live"
  const canCaptureFrame = !live && asset.status === "ready"

  const closeCrop = () => {
    if (cropSource) URL.revokeObjectURL(cropSource)
    setCropSource(null)
  }

  const onFile = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    // Cleared so that choosing the same file again still fires a change.
    e.target.value = ""
    if (!file) return
    if (!file.type.startsWith("image/")) {
      toast("Choose an image file (JPEG, PNG or WebP).", { tone: "error" })
      return
    }
    if (file.size > MAX_SOURCE_BYTES) {
      toast("That image is larger than 40 MB.", { tone: "error" })
      return
    }
    setCropSource(URL.createObjectURL(file))
  }

  const previewSrc =
    pending?.kind === "image" ? pending.previewUrl : pending?.kind === "link" ? pending.url : pending?.kind === "auto" ? "" : undefined

  const currentCaption =
    pending?.kind === "auto"
      ? "A frame will be captured from the video after you save."
      : pending
        ? "New thumbnail — save to apply."
        : asset.thumbnail_url
          ? ""
          : "No thumbnail yet: rooms and the library show the default poster."

  return (
    <section aria-labelledby="thumb-heading">
      <h2 id="thumb-heading" className="text-base font-medium text-white">
        Thumbnail
      </h2>
      <p className="mt-1 text-sm text-zinc-400">
        Pick an image that shows what's in the video. It appears on library cards, in playlists, and in rooms before playback starts.
      </p>

      <input ref={fileRef} type="file" accept="image/jpeg,image/png,image/webp,image/*" className="hidden" onChange={onFile} />

      <div className="mt-4 grid grid-cols-2 gap-3 sm:grid-cols-3 xl:grid-cols-5">
        <div className={cx("relative rounded-lg p-0.5 ring-2", pending ? "ring-emerald-400" : "ring-zinc-600")}>
          <Thumb asset={asset} src={previewSrc} showDuration={false} className="rounded-[6px]" />
          <span
            className={cx(
              "absolute left-1.5 top-1.5 rounded px-1.5 py-0.5 text-[10px] font-semibold uppercase tracking-wide",
              pending ? "bg-emerald-500 text-black" : "bg-black/75 text-zinc-200",
            )}
          >
            {pending ? "New" : "Current"}
          </span>
        </div>
        <OptionTile icon={ImageUp} label="Upload file" onClick={() => fileRef.current?.click()} />
        <OptionTile
          icon={Clapperboard}
          label="Choose a frame"
          disabled={!canCaptureFrame}
          disabledReason={live ? "Live channels have no fixed frames" : "Available once the video is ready"}
          onClick={() => setFramePicking(true)}
        />
        <OptionTile icon={Link2} label="Paste link" onClick={() => setLinking(true)} />
        <OptionTile
          icon={WandSparkles}
          label="Auto-generate"
          active={pending?.kind === "auto"}
          disabled={live}
          disabledReason="Live channels have no fixed frames"
          onClick={() => onChange({ kind: "auto" })}
        />
      </div>
      <div className="mt-2.5 flex flex-wrap items-center gap-x-3 gap-y-1">
        <p className={cx("text-xs", pending ? "text-emerald-300" : "text-zinc-500")}>{currentCaption}</p>
        {pending && (
          <button
            type="button"
            onClick={() => onChange(null)}
            className="inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-xs text-zinc-400 hover:bg-zinc-800 hover:text-white cursor-pointer"
          >
            <Undo2 className="h-3 w-3" aria-hidden="true" /> Discard
          </button>
        )}
      </div>

      <ThumbnailCropDialog
        source={cropSource}
        onCancel={closeCrop}
        onApply={(blob) => {
          onChange({ kind: "image", blob, previewUrl: URL.createObjectURL(blob) })
          closeCrop()
        }}
      />
      <FramePickerDialog
        asset={framePicking ? asset : null}
        onCancel={() => setFramePicking(false)}
        onPick={(frameUrl) => {
          setFramePicking(false)
          setCropSource(frameUrl)
        }}
      />
      <ThumbnailLinkDialog
        open={linking}
        initialUrl={pending?.kind === "link" ? pending.url : ""}
        onCancel={() => setLinking(false)}
        onApply={(url) => {
          setLinking(false)
          onChange({ kind: "link", url })
        }}
      />
    </section>
  )
}

function OptionTile({
  icon: Icon,
  label,
  onClick,
  active,
  disabled,
  disabledReason,
}: {
  icon: LucideIcon
  label: string
  onClick: () => void
  active?: boolean
  disabled?: boolean
  disabledReason?: string
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      title={disabled ? disabledReason : undefined}
      className={cx(
        "flex aspect-video w-full flex-col items-center justify-center gap-1.5 rounded-lg border border-dashed text-xs font-medium transition-colors cursor-pointer disabled:cursor-not-allowed disabled:opacity-40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-emerald-400",
        active
          ? "border-emerald-400 bg-emerald-500/10 text-emerald-200"
          : "border-zinc-700 text-zinc-300 hover:border-zinc-500 hover:bg-zinc-900 hover:text-white",
      )}
    >
      <Icon className="h-5 w-5" aria-hidden="true" />
      {label}
    </button>
  )
}
