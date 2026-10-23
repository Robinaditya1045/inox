import { CircleAlert, CircleCheck, Clock, Loader2 } from "lucide-react"
import type { MediaAsset } from "@/types/media"

/** Ready / Processing n% / Failed, in words with an icon rather than colour alone. */
export function StatusBadge({ asset }: { asset: MediaAsset }) {
  switch (asset.status) {
    case "ready":
      return (
        <span className="inline-flex items-center gap-1.5 text-sm text-zinc-200">
          <CircleCheck className="h-4 w-4 text-emerald-400" aria-hidden="true" />
          Ready
        </span>
      )
    case "processing": {
      const pct = asset.progress && asset.progress > 0 ? asset.progress : null
      return (
        <span className="inline-flex flex-col gap-1">
          <span className="inline-flex items-center gap-1.5 text-sm text-amber-200">
            <Loader2 className="h-4 w-4 animate-spin text-amber-400" aria-hidden="true" />
            Processing{pct !== null && ` ${pct}%`}
          </span>
          {pct !== null && (
            <span className="h-1 w-24 overflow-hidden rounded-full bg-zinc-800" aria-hidden="true">
              <span className="block h-full rounded-full bg-amber-400 transition-[width] duration-500 ease-snappy" style={{ width: `${pct}%` }} />
            </span>
          )}
        </span>
      )
    }
    case "failed":
      return (
        <span className="inline-flex items-center gap-1.5 text-sm text-rose-300">
          <CircleAlert className="h-4 w-4 text-rose-400" aria-hidden="true" />
          Failed
        </span>
      )
    default:
      return (
        <span className="inline-flex items-center gap-1.5 text-sm text-zinc-400">
          <Clock className="h-4 w-4" aria-hidden="true" />
          Queued
        </span>
      )
  }
}
