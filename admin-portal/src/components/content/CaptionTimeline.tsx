import * as React from "react"
import type { Cue } from "@/lib/vtt"
import { cuePlainText } from "@/lib/vtt"
import { cx } from "@/lib/utils"

interface CaptionTimelineProps {
  cues: Cue[]
  duration: number
  currentTime: number
  selectedKey: string | null
  onSeek: (time: number) => void
  onSelect: (key: string) => void
  onRetime: (key: string, start: number, end: number) => void
}

// How much of the video the strip shows at once, and how far behind the
// playhead its left edge sits, so what is coming up is always in view.
const WINDOW_SECONDS = 30
const LEAD_SECONDS = 8
const MIN_CUE_SECONDS = 0.2

type DragMode = "move" | "start" | "end"
interface Drag {
  key: string
  mode: DragMode
  x: number
  start: number
  end: number
  windowStart: number
}

const round = (t: number) => Math.round(t * 100) / 100

/**
 * The editor's timeline: a window onto the video that follows the playhead,
 * with each caption as a block. Drag a block to move it, or its edges to
 * retime it; click the strip to seek.
 */
export function CaptionTimeline({ cues, duration, currentTime, selectedKey, onSeek, onSelect, onRetime }: CaptionTimelineProps) {
  const trackRef = React.useRef<HTMLDivElement>(null)
  const [drag, setDrag] = React.useState<Drag | null>(null)

  const span = Math.max(1, Math.min(WINDOW_SECONDS, duration || WINDOW_SECONDS))
  const following = Math.min(Math.max(0, currentTime - LEAD_SECONDS), Math.max(0, (duration || 0) - span))
  // Frozen while dragging, so the strip does not slide out from under the pointer.
  const windowStart = drag ? drag.windowStart : following
  const windowEnd = windowStart + span

  const toPct = (t: number) => ((t - windowStart) / span) * 100
  const secondsPerPx = () => span / (trackRef.current?.clientWidth || 1)

  const beginDrag = (e: React.PointerEvent, cue: Cue, mode: DragMode) => {
    e.stopPropagation()
    e.currentTarget.setPointerCapture(e.pointerId)
    onSelect(cue.key)
    setDrag({ key: cue.key, mode, x: e.clientX, start: cue.start, end: cue.end, windowStart })
  }

  const onPointerMove = (e: React.PointerEvent) => {
    if (!drag) return
    const dt = (e.clientX - drag.x) * secondsPerPx()
    const max = duration || Infinity
    let { start, end } = drag
    if (drag.mode === "move") {
      const len = end - start
      start = Math.min(Math.max(0, start + dt), max - len)
      end = start + len
    } else if (drag.mode === "start") {
      start = Math.min(Math.max(0, start + dt), end - MIN_CUE_SECONDS)
    } else {
      end = Math.max(Math.min(max, end + dt), start + MIN_CUE_SECONDS)
    }
    onRetime(drag.key, round(start), round(end))
  }

  const endDrag = () => setDrag(null)

  const ticks: number[] = []
  for (let t = Math.ceil(windowStart); t <= windowEnd; t++) ticks.push(t)

  const visible = cues.filter((c) => c.end > windowStart && c.start < windowEnd)

  return (
    <div className="select-none">
      <div className="mb-1.5 flex items-center justify-between text-[11px] text-zinc-500">
        <span>Timeline</span>
        <span>Drag a caption to move it, or its edges to change when it starts and ends</span>
      </div>
      <div
        ref={trackRef}
        onPointerMove={onPointerMove}
        onPointerUp={endDrag}
        onPointerCancel={endDrag}
        onClick={(e) => {
          if (drag) return
          const rect = e.currentTarget.getBoundingClientRect()
          onSeek(windowStart + ((e.clientX - rect.left) / rect.width) * span)
        }}
        className="relative h-24 touch-none overflow-hidden rounded-lg border border-zinc-800 bg-zinc-950 cursor-text"
      >
        {/* Ruler */}
        <div className="absolute inset-x-0 top-0 h-6 border-b border-zinc-800/80">
          {ticks.map((t) => (
            <div key={t} className="absolute top-0 h-full" style={{ left: `${toPct(t)}%` }}>
              <div className={cx("w-px bg-zinc-700", t % 5 === 0 ? "h-3" : "h-1.5")} />
              {t % 5 === 0 && <span className="absolute left-1 top-1 font-mono text-[10px] text-zinc-500">{formatTick(t)}</span>}
            </div>
          ))}
        </div>

        {/* Caption blocks */}
        {visible.map((cue) => {
          const left = Math.max(0, toPct(cue.start))
          const right = Math.min(100, toPct(cue.end))
          const selected = cue.key === selectedKey
          const active = currentTime >= cue.start && currentTime < cue.end
          return (
            <div
              key={cue.key}
              onPointerDown={(e) => beginDrag(e, cue, "move")}
              onClick={(e) => e.stopPropagation()}
              title={cuePlainText(cue.text)}
              className={cx(
                "absolute bottom-2 top-8 flex items-center overflow-hidden rounded-md border px-2 text-[11px] leading-tight cursor-grab active:cursor-grabbing",
                selected
                  ? "border-emerald-300 bg-emerald-500/30 text-white z-10"
                  : active
                    ? "border-emerald-500/60 bg-emerald-500/20 text-emerald-50"
                    : "border-zinc-600 bg-zinc-800/90 text-zinc-200 hover:border-zinc-400",
              )}
              style={{ left: `${left}%`, width: `${Math.max(0.5, right - left)}%` }}
            >
              <span className="line-clamp-2 pointer-events-none">{cuePlainText(cue.text) || "…"}</span>
              <span
                aria-hidden="true"
                onPointerDown={(e) => beginDrag(e, cue, "start")}
                className="absolute inset-y-0 left-0 w-2 cursor-ew-resize hover:bg-white/20"
              />
              <span
                aria-hidden="true"
                onPointerDown={(e) => beginDrag(e, cue, "end")}
                className="absolute inset-y-0 right-0 w-2 cursor-ew-resize hover:bg-white/20"
              />
            </div>
          )
        })}

        {/* Playhead */}
        {currentTime >= windowStart && currentTime <= windowEnd && (
          <div aria-hidden="true" className="pointer-events-none absolute inset-y-0 z-20 w-px bg-rose-500" style={{ left: `${toPct(currentTime)}%` }}>
            <div className="absolute -left-[5px] top-0 h-0 w-0 border-x-[5px] border-t-[6px] border-x-transparent border-t-rose-500" />
          </div>
        )}
      </div>
    </div>
  )
}

function formatTick(t: number): string {
  const m = Math.floor(t / 60)
  const s = String(Math.floor(t % 60)).padStart(2, "0")
  return `${m}:${s}`
}
