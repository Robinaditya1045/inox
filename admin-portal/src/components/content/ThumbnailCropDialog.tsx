import * as React from "react"
import { Loader2, Move, ZoomIn, ZoomOut } from "lucide-react"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"

interface ThumbnailCropDialogProps {
  /** Object/data URL of the image to crop; null closes the dialog */
  source: string | null
  title?: string
  onCancel: () => void
  onApply: (image: Blob) => void
}

const ASPECT = 16 / 9
const MAX_ZOOM = 4
// Posters are shown at card size; 1280 wide covers a 2x display at Studio's
// largest tile. Small sources are not upscaled past their own detail.
const MAX_OUTPUT_WIDTH = 1280
const MIN_OUTPUT_WIDTH = 640

interface View {
  /** The viewport centre, in image pixels */
  cx: number
  cy: number
  zoom: number
}

/**
 * Crops an image to 16:9 by panning and zooming it under a fixed frame, then
 * renders the frame to a JPEG. The view is kept in image coordinates, so
 * resizing the dialog never shifts what is framed.
 */
export function ThumbnailCropDialog({ source, title = "Adjust thumbnail", onCancel, onApply }: ThumbnailCropDialogProps) {
  const viewportRef = React.useRef<HTMLDivElement>(null)
  const [img, setImg] = React.useState<{ el: HTMLImageElement; src: string } | null>(null)
  // Keyed by source, so opening another image starts without the last one's error.
  const [failure, setFailure] = React.useState<{ src: string | null; message: string } | null>(null)
  const loadError = failure && failure.src === source ? failure.message : null
  const [box, setBox] = React.useState({ w: 0, h: 0 })
  const [view, setView] = React.useState<View>({ cx: 0, cy: 0, zoom: 1 })
  const [busy, setBusy] = React.useState(false)
  const drag = React.useRef<{ x: number; y: number; view: View } | null>(null)

  const image = img && img.src === source ? img.el : null

  React.useEffect(() => {
    if (!source) return
    let cancelled = false
    const el = new Image()
    el.src = source
    el.decode()
      .then(() => {
        if (cancelled) return
        setImg({ el, src: source })
        setView({ cx: el.naturalWidth / 2, cy: el.naturalHeight / 2, zoom: 1 })
      })
      .catch(() => !cancelled && setFailure({ src: source, message: "This image could not be read. Try a JPEG, PNG or WebP file." }))
    return () => {
      cancelled = true
    }
  }, [source])

  // The dialog animates in, so the viewport's size is only known once laid out.
  React.useEffect(() => {
    const el = viewportRef.current
    if (!el) return
    const ro = new ResizeObserver(([entry]) => {
      const w = entry.contentRect.width
      setBox({ w, h: w / ASPECT })
    })
    ro.observe(el)
    return () => ro.disconnect()
  }, [source, image])

  const baseScale = image && box.w ? Math.max(box.w / image.naturalWidth, box.h / image.naturalHeight) : 1
  const scale = baseScale * view.zoom

  // Keeps the frame covered: the image may never pull away from an edge.
  const clamp = React.useCallback(
    (v: View): View => {
      if (!image || !box.w) return v
      const zoom = Math.min(MAX_ZOOM, Math.max(1, v.zoom))
      const s = baseScale * zoom
      const halfW = box.w / s / 2
      const halfH = box.h / s / 2
      return {
        zoom,
        cx: Math.min(image.naturalWidth - halfW, Math.max(halfW, v.cx)),
        cy: Math.min(image.naturalHeight - halfH, Math.max(halfH, v.cy)),
      }
    },
    [image, box.w, box.h, baseScale],
  )

  const setZoom = React.useCallback((zoom: number) => setView((v) => clamp({ ...v, zoom })), [clamp])

  // Wheel zoom needs a non-passive listener to stop the dialog scrolling.
  React.useEffect(() => {
    const el = viewportRef.current
    if (!el) return
    const onWheel = (e: WheelEvent) => {
      e.preventDefault()
      setView((v) => clamp({ ...v, zoom: v.zoom * Math.exp(-e.deltaY / 500) }))
    }
    el.addEventListener("wheel", onWheel, { passive: false })
    return () => el.removeEventListener("wheel", onWheel)
  }, [clamp])

  const onPointerDown = (e: React.PointerEvent) => {
    if (!image) return
    e.currentTarget.setPointerCapture(e.pointerId)
    drag.current = { x: e.clientX, y: e.clientY, view }
  }
  const onPointerMove = (e: React.PointerEvent) => {
    const d = drag.current
    if (!d) return
    setView(clamp({ ...d.view, cx: d.view.cx - (e.clientX - d.x) / scale, cy: d.view.cy - (e.clientY - d.y) / scale }))
  }
  const onPointerUp = () => {
    drag.current = null
  }

  const onKeyDown = (e: React.KeyboardEvent) => {
    const step = 12 / scale
    const moves: Record<string, [number, number]> = {
      ArrowLeft: [-step, 0],
      ArrowRight: [step, 0],
      ArrowUp: [0, -step],
      ArrowDown: [0, step],
    }
    if (moves[e.key]) {
      e.preventDefault()
      const [dx, dy] = moves[e.key]
      setView((v) => clamp({ ...v, cx: v.cx + dx, cy: v.cy + dy }))
    } else if (e.key === "+" || e.key === "=") {
      e.preventDefault()
      setZoom(view.zoom * 1.1)
    } else if (e.key === "-") {
      e.preventDefault()
      setZoom(view.zoom / 1.1)
    }
  }

  const apply = async () => {
    if (!image || !box.w) return
    setBusy(true)
    try {
      const v = clamp(view)
      const s = baseScale * v.zoom
      const sw = box.w / s
      const sh = box.h / s
      const outW = Math.round(Math.min(MAX_OUTPUT_WIDTH, Math.max(MIN_OUTPUT_WIDTH, sw)))
      const outH = Math.round(outW / ASPECT)
      const canvas = document.createElement("canvas")
      canvas.width = outW
      canvas.height = outH
      const ctx = canvas.getContext("2d")
      if (!ctx) throw new Error("canvas unavailable")
      ctx.imageSmoothingQuality = "high"
      ctx.drawImage(image, v.cx - sw / 2, v.cy - sh / 2, sw, sh, 0, 0, outW, outH)
      const blob = await new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, "image/jpeg", 0.9))
      if (!blob) throw new Error("encoding failed")
      onApply(blob)
    } catch {
      setFailure({ src: source, message: "The thumbnail could not be created from this image." })
    } finally {
      setBusy(false)
    }
  }

  const displayed = clamp(view)
  const x = box.w / 2 - displayed.cx * scale
  const y = box.h / 2 - displayed.cy * scale

  return (
    <Dialog open={!!source} onOpenChange={(open) => !open && !busy && onCancel()}>
      <DialogContent className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>Drag to reposition and zoom to crop. Thumbnails are 16:9, like the video player.</DialogDescription>
        </DialogHeader>

        <div
          ref={viewportRef}
          tabIndex={0}
          role="application"
          aria-label="Thumbnail framing. Drag or use the arrow keys to move, plus and minus to zoom."
          onPointerDown={onPointerDown}
          onPointerMove={onPointerMove}
          onPointerUp={onPointerUp}
          onPointerCancel={onPointerUp}
          onKeyDown={onKeyDown}
          className="relative aspect-video w-full touch-none select-none overflow-hidden rounded-lg bg-zinc-950 ring-1 ring-zinc-800 cursor-grab active:cursor-grabbing focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-emerald-400"
        >
          {image ? (
            <img
              src={source ?? undefined}
              alt=""
              draggable={false}
              className="pointer-events-none absolute left-0 top-0 max-w-none origin-top-left"
              style={{ width: image.naturalWidth * scale, height: image.naturalHeight * scale, transform: `translate(${x}px, ${y}px)` }}
            />
          ) : (
            <div className="flex h-full items-center justify-center text-sm text-zinc-500">
              {loadError ?? <Loader2 className="h-5 w-5 animate-spin" aria-label="Loading image" />}
            </div>
          )}
          {/* Rule-of-thirds guides */}
          <div aria-hidden="true" className="pointer-events-none absolute inset-0 grid grid-cols-3 grid-rows-3">
            {Array.from({ length: 9 }, (_, i) => (
              <div key={i} className="border-white/10 [&:nth-child(-n+6)]:border-b [&:not(:nth-child(3n))]:border-r" />
            ))}
          </div>
          {image && (
            <span className="pointer-events-none absolute left-2 top-2 inline-flex items-center gap-1 rounded bg-black/60 px-1.5 py-0.5 text-[11px] text-zinc-300">
              <Move className="h-3 w-3" aria-hidden="true" /> Drag to move
            </span>
          )}
        </div>

        <div className="flex items-center gap-3">
          <button type="button" onClick={() => setZoom(view.zoom / 1.2)} aria-label="Zoom out" className="rounded-md p-1.5 text-zinc-400 hover:bg-zinc-800 hover:text-white cursor-pointer">
            <ZoomOut className="h-4 w-4" />
          </button>
          <input
            type="range"
            min={1}
            max={MAX_ZOOM}
            step={0.01}
            value={displayed.zoom}
            onChange={(e) => setZoom(Number(e.target.value))}
            aria-label="Zoom"
            disabled={!image}
            className="h-1.5 flex-1 cursor-pointer accent-emerald-500"
          />
          <button type="button" onClick={() => setZoom(view.zoom * 1.2)} aria-label="Zoom in" className="rounded-md p-1.5 text-zinc-400 hover:bg-zinc-800 hover:text-white cursor-pointer">
            <ZoomIn className="h-4 w-4" />
          </button>
          <span className="w-12 text-right font-mono text-xs tabular-nums text-zinc-500">{displayed.zoom.toFixed(1)}×</span>
        </div>

        {loadError && image && <p className="text-sm text-rose-400">{loadError}</p>}

        <DialogFooter className="gap-2">
          <Button variant="ghost" onClick={onCancel} disabled={busy}>
            Cancel
          </Button>
          <Button variant="emerald" onClick={apply} disabled={!image || busy}>
            {busy && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
            Use thumbnail
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
