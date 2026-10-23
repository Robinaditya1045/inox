import { ExternalLink } from "lucide-react"
import type { MediaAsset } from "@/types/media"
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "@/components/ui/dialog"
import { VideoPlayer } from "@/components/content/VideoPlayer"
import { playbackUrl } from "@/lib/media"

/** Watches a library video in place, the way viewers will see it. */
export function PreviewDialog({ asset, onClose }: { asset: MediaAsset | null; onClose: () => void }) {
  const url = asset ? playbackUrl(asset) : ""
  return (
    <Dialog open={!!asset} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="sm:max-w-4xl p-0 gap-0 overflow-hidden">
        {asset && (
          <>
            <VideoPlayer src={url} poster={asset.thumbnail_url || undefined} controls autoPlay className="aspect-video w-full" />
            <div className="flex items-start justify-between gap-4 p-5">
              <div className="min-w-0 pr-8">
                <DialogTitle className="truncate text-base">{asset.title}</DialogTitle>
                <DialogDescription className="mt-1 truncate font-mono text-xs">{url}</DialogDescription>
              </div>
              <a
                href={url}
                target="_blank"
                rel="noreferrer"
                className="inline-flex shrink-0 items-center gap-1.5 rounded-md px-2 py-1 text-xs text-zinc-400 hover:bg-zinc-800 hover:text-white"
              >
                <ExternalLink className="h-3.5 w-3.5" aria-hidden="true" /> Open stream
              </a>
            </div>
          </>
        )}
      </DialogContent>
    </Dialog>
  )
}
