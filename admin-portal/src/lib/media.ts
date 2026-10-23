import type { MediaAsset } from "@/types/media"
import { hostOf } from "@/lib/format"

/** What a room plays for this asset: exactly what the viewer's library picker sends. */
export function playbackUrl(asset: MediaAsset): string {
  return asset.hls_master_url || asset.source_url
}

export type MediaOrigin = "upload" | "link" | "live"

/**
 * Where a library entry came from. Uploads live in our storage (raw/ before the
 * transcode, hls/ after); a registered link points anywhere else.
 */
export function mediaOrigin(asset: MediaAsset): { origin: MediaOrigin; label: string } {
  if (asset.kind === "live") return { origin: "live", label: "Live channel" }
  const url = playbackUrl(asset)
  if (/\/(hls|raw)\//.test(url) && (url.includes("/media/stream/") || url.includes("/inox-media/"))) {
    return { origin: "upload", label: "Upload" }
  }
  return { origin: "link", label: hostOf(url) || "External link" }
}
