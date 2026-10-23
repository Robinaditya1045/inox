import * as React from "react"
import type Hls from "hls.js"
import { CircleAlert } from "lucide-react"
import { cx } from "@/lib/utils"

export interface VideoPlayerProps extends Omit<React.VideoHTMLAttributes<HTMLVideoElement>, "src" | "ref"> {
  src: string
  ref?: React.Ref<HTMLVideoElement>
  /** Shown instead of the picture when the stream cannot be played */
  errorHint?: string
  /** Called with a short reason when playback fails for good */
  onPlaybackError?: (reason: string) => void
}

function isHlsUrl(url: string): boolean {
  return /\.m3u8(?:$|[?#])/i.test(url)
}

/**
 * The portal's video element: HLS through hls.js where the browser has no
 * native support (anything but Safari), plain src otherwise.
 *
 * hls.js matters beyond compatibility: it feeds the element through Media
 * Source Extensions from CORS requests, which leaves the picture readable by a
 * canvas. That is what lets "Choose a frame" turn the current frame into a
 * thumbnail for a transcoded upload.
 */
export function VideoPlayer({ src, ref, className, errorHint, onPlaybackError, crossOrigin, ...props }: VideoPlayerProps) {
  const innerRef = React.useRef<HTMLVideoElement | null>(null)
  // Keyed by source, so a new src starts clean without resetting state in the effect.
  const [failure, setFailure] = React.useState<{ src: string; reason: string } | null>(null)
  const error = failure?.src === src ? failure.reason : null
  const onErrorRef = React.useRef(onPlaybackError)
  React.useEffect(() => {
    onErrorRef.current = onPlaybackError
  }, [onPlaybackError])

  const setRefs = React.useCallback(
    (node: HTMLVideoElement | null) => {
      innerRef.current = node
      if (typeof ref === "function") ref(node)
      else if (ref) (ref as React.RefObject<HTMLVideoElement | null>).current = node
    },
    [ref],
  )

  React.useEffect(() => {
    const video = innerRef.current
    if (!video || !src) return

    const fail = (reason: string) => {
      setFailure({ src, reason })
      onErrorRef.current?.(reason)
    }

    let hls: Hls | null = null
    let cancelled = false
    // hls.js reports its own failures; this covers files the element loads itself.
    const onNativeError = () => fail("This video could not be loaded.")

    if (isHlsUrl(src)) {
      // Loaded on first use: it is most of the portal's weight, and only the
      // pages with a player need it.
      import("hls.js")
        .then(({ default: HlsJs }) => {
          if (cancelled) return
          if (!HlsJs.isSupported()) {
            video.addEventListener("error", onNativeError)
            video.src = src
            return
          }
          const instance = new HlsJs({ enableWorker: true })
          hls = instance
          instance.on(HlsJs.Events.ERROR, (_e, data) => {
            if (!data.fatal) return
            if (data.type === HlsJs.ErrorTypes.MEDIA_ERROR) {
              instance.recoverMediaError()
              return
            }
            fail(data.type === HlsJs.ErrorTypes.NETWORK_ERROR ? "The stream could not be loaded." : "The stream could not be decoded.")
            instance.destroy()
            hls = null
          })
          instance.loadSource(src)
          instance.attachMedia(video)
        })
        .catch(() => !cancelled && fail("The video player could not be loaded."))
    } else {
      video.addEventListener("error", onNativeError)
      video.src = src
    }

    return () => {
      cancelled = true
      video.removeEventListener("error", onNativeError)
      hls?.destroy()
      // Release the source so a closed player stops downloading (and playing).
      video.pause()
      video.removeAttribute("src")
      video.load()
    }
  }, [src, crossOrigin])

  return (
    <div className={cx("relative overflow-hidden bg-black", className)}>
      <video ref={setRefs} crossOrigin={crossOrigin} playsInline className="h-full w-full object-contain" {...props} />
      {error && (
        <div role="status" className="absolute inset-0 flex flex-col items-center justify-center gap-2 bg-black/85 p-6 text-center">
          <CircleAlert className="h-7 w-7 text-rose-400" aria-hidden="true" />
          <p className="text-sm font-medium text-zinc-100">{error}</p>
          {errorHint && <p className="max-w-xs text-xs text-zinc-400">{errorHint}</p>}
        </div>
      )}
    </div>
  )
}
