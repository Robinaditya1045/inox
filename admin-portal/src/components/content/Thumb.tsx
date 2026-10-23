import * as React from "react"
import { Film, Loader2, Radio } from "lucide-react"
import type { MediaAsset } from "@/types/media"
import { formatDuration } from "@/lib/format"
import { cx } from "@/lib/utils"

interface ThumbProps {
  asset: MediaAsset
  /** Overrides the asset's own poster, e.g. a thumbnail that is not saved yet */
  src?: string
  className?: string
  showDuration?: boolean
}

/** A 16:9 poster with the running time in the corner, as Studio draws them. */
export function Thumb({ asset, src, className, showDuration = true }: ThumbProps) {
  const url = src ?? asset.thumbnail_url
  // A link that 404s, or a host that blocks hotlinking, falls back to the
  // default tile rather than a broken-image icon.
  const [failedUrl, setFailedUrl] = React.useState<string | null>(null)
  const showImage = !!url && failedUrl !== url
  const duration = formatDuration(asset.duration_seconds)
  const busy = asset.status === "processing" || asset.status === "pending"

  return (
    <div className={cx("relative aspect-video shrink-0 overflow-hidden rounded-md bg-zinc-900 ring-1 ring-inset ring-white/5", className)}>
      {showImage ? (
        <img src={url} alt="" loading="lazy" onError={() => setFailedUrl(url)} className="h-full w-full object-cover" />
      ) : (
        <div className="flex h-full w-full items-center justify-center bg-gradient-to-br from-zinc-800 to-zinc-900 text-zinc-600">
          {asset.kind === "live" ? <Radio className="h-1/3 w-1/3 max-h-7 max-w-7" aria-hidden="true" /> : <Film className="h-1/3 w-1/3 max-h-7 max-w-7" aria-hidden="true" />}
        </div>
      )}
      {busy && (
        <div className="absolute inset-0 flex items-center justify-center bg-black/55">
          <Loader2 className="h-5 w-5 animate-spin text-amber-300" aria-hidden="true" />
        </div>
      )}
      {asset.kind === "live" ? (
        <span className="absolute bottom-1 right-1 rounded bg-rose-600 px-1 py-px text-[10px] font-bold tracking-wide text-white">LIVE</span>
      ) : (
        showDuration && duration && (
          <span className="absolute bottom-1 right-1 rounded bg-black/80 px-1 py-px font-mono text-[11px] font-medium text-white">{duration}</span>
        )
      )}
    </div>
  )
}
