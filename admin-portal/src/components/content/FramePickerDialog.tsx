import * as React from "react"
import { ChevronLeft, ChevronRight, ChevronsLeft, ChevronsRight, Loader2 } from "lucide-react"
import type { MediaAsset } from "@/types/media"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"
import { IconButton } from "@/components/ui/icon-button"
import { VideoPlayer } from "@/components/content/VideoPlayer"
import { playbackUrl } from "@/lib/media"
import { formatCueTime } from "@/lib/vtt"

interface FramePickerDialogProps {
  asset: MediaAsset | null
  onCancel: () => void
  /** An object URL of the captured frame, for the crop step */
  onPick: (frameUrl: string) => void
}

// Videos carry no frame rate in the catalog; 30 fps is the transcoder's output
// and close enough for stepping by hand.
const FRAME = 1 / 30

/** Scrub to a moment in the video and take that frame as the thumbnail. */
export function FramePickerDialog({ asset, onCancel, onPick }: FramePickerDialogProps) {
  const videoRef = React.useRef<HTMLVideoElement>(null)
  const [time, setTime] = React.useState(0)
  const [error, setError] = React.useState<string | null>(null)
  const [busy, setBusy] = React.useState(false)

  const src = asset ? playbackUrl(asset) : ""
  // hls.js reads segments over CORS, which keeps the picture capturable. A
  // plain file needs the element itself to load it with CORS for the same.
  const crossOrigin = /\.m3u8(?:$|[?#])/i.test(src) ? undefined : "anonymous"

  const step = (delta: number) => {
    const v = videoRef.current
    if (!v) return
    v.pause()
    const end = Number.isFinite(v.duration) ? v.duration : Infinity
    v.currentTime = Math.min(end, Math.max(0, v.currentTime + delta))
  }

  const capture = async () => {
    const v = videoRef.current
    if (!v) return
    if (v.readyState < 2 || !v.videoWidth) {
      setError("The video has not loaded a picture at this point yet.")
      return
    }
    setBusy(true)
    setError(null)
    try {
      v.pause()
      const canvas = document.createElement("canvas")
      canvas.width = v.videoWidth
      canvas.height = v.videoHeight
      canvas.getContext("2d")?.drawImage(v, 0, 0)
      const blob = await new Promise<Blob | null>((resolve, reject) => {
        try {
          canvas.toBlob(resolve, "image/jpeg", 0.92)
        } catch (err) {
          reject(err)
        }
      })
      if (!blob) throw new Error("no image")
      onPick(URL.createObjectURL(blob))
    } catch (err) {
      setError(
        err instanceof DOMException && err.name === "SecurityError"
          ? "This video's host doesn't allow frames to be captured. Upload an image or paste a link instead."
          : "This frame could not be captured.",
      )
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog
      open={!!asset}
      onOpenChange={(open) => {
        if (!open && !busy) {
          setError(null)
          onCancel()
        }
      }}
    >
      <DialogContent className="sm:max-w-3xl">
        <DialogHeader>
          <DialogTitle>Choose a frame</DialogTitle>
          <DialogDescription>Play or scrub to the moment you want, then use it as the thumbnail.</DialogDescription>
        </DialogHeader>

        {asset && (
          <VideoPlayer
            ref={videoRef}
            src={src}
            crossOrigin={crossOrigin}
            controls
            muted
            preload="auto"
            onTimeUpdate={(e) => setTime(e.currentTarget.currentTime)}
            onSeeked={(e) => setTime(e.currentTarget.currentTime)}
            className="aspect-video w-full rounded-lg"
            errorHint="Its host may not allow frames to be captured. Upload an image or paste a link instead."
          />
        )}

        <div className="flex flex-wrap items-center justify-center gap-1">
          <IconButton label="Back 1 second" size="sm" onClick={() => step(-1)}>
            <ChevronsLeft className="h-4 w-4" />
          </IconButton>
          <IconButton label="Previous frame" size="sm" onClick={() => step(-FRAME)}>
            <ChevronLeft className="h-4 w-4" />
          </IconButton>
          <span className="w-24 text-center font-mono text-sm tabular-nums text-zinc-300" aria-live="off">
            {formatCueTime(time)}
          </span>
          <IconButton label="Next frame" size="sm" onClick={() => step(FRAME)}>
            <ChevronRight className="h-4 w-4" />
          </IconButton>
          <IconButton label="Forward 1 second" size="sm" onClick={() => step(1)}>
            <ChevronsRight className="h-4 w-4" />
          </IconButton>
        </div>

        {error && (
          <p role="alert" className="text-sm text-rose-400">
            {error}
          </p>
        )}

        <DialogFooter className="gap-2">
          <Button variant="ghost" onClick={onCancel} disabled={busy}>
            Cancel
          </Button>
          <Button variant="emerald" onClick={capture} disabled={busy}>
            {busy && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
            Use this frame
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
