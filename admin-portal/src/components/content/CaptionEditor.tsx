import * as React from "react"
import { createPortal } from "react-dom"
import { ArrowLeftToLine, ArrowRightToLine, CircleAlert, FileText, ListVideo, Loader2, Plus, RotateCcw, Trash2, X } from "lucide-react"
import type { MediaAsset, MediaCaption } from "@/types/media"
import { VideoPlayer } from "@/components/content/VideoPlayer"
import { CaptionTimeline } from "@/components/content/CaptionTimeline"
import { Button } from "@/components/ui/button"
import { IconButton } from "@/components/ui/icon-button"
import { ConfirmDialog } from "@/components/ui/confirm-dialog"
import { useCatalog } from "@/lib/contexts"
import { useToast } from "@/hooks/useToast"
import { errorMessage } from "@/lib/api"
import { playbackUrl } from "@/lib/media"
import {
  CaptionParseError,
  emptyDocument,
  formatCueTime,
  newCueKey,
  parseCaptions,
  parseCueTime,
  serializeCaptions,
  validateCues,
  type CaptionDocument,
  type Cue,
} from "@/lib/vtt"
import { cx } from "@/lib/utils"

/** What the editor is editing: an existing track, or a new language typed by hand. */
export interface CaptionEditorTarget {
  caption?: MediaCaption
  language: string
  label: string
  isDefault?: boolean
}

interface CaptionEditorProps {
  asset: MediaAsset
  target: CaptionEditorTarget
  onClose: () => void
}

const NEW_CUE_SECONDS = 2.5

/**
 * Full-screen caption editor, after Studio's subtitle editor: captions on the
 * left, the video on the right with the track previewed on it, and a timeline
 * underneath for timing by eye. "Edit as text" switches to the raw WebVTT.
 */
export function CaptionEditor({ asset, target, onClose }: CaptionEditorProps) {
  const { createCaption, updateCaption } = useCatalog()
  const { toast } = useToast()
  const videoRef = React.useRef<HTMLVideoElement | null>(null)
  const trackRef = React.useRef<TextTrack | null>(null)
  const listRef = React.useRef<HTMLOListElement>(null)

  const [loadState, setLoadState] = React.useState<"loading" | "ready" | "error">(target.caption ? "loading" : "ready")
  const [doc, setDoc] = React.useState<CaptionDocument>(emptyDocument)
  const [initial, setInitial] = React.useState(() => serializeCaptions(emptyDocument()))
  const [mode, setMode] = React.useState<"cues" | "text">("cues")
  const [text, setText] = React.useState("")
  const [textError, setTextError] = React.useState<string | null>(null)
  const [selectedKey, setSelectedKey] = React.useState<string | null>(null)
  const [time, setTime] = React.useState(0)
  const [duration, setDuration] = React.useState(asset.duration_seconds || 0)
  const [saving, setSaving] = React.useState(false)
  const [confirmDiscard, setConfirmDiscard] = React.useState(false)
  const [reloadToken, setReloadToken] = React.useState(0)

  // Load the track being edited.
  React.useEffect(() => {
    const url = target.caption?.url
    if (!url) return
    let cancelled = false
    fetch(url)
      .then((res) => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`)
        return res.text()
      })
      .then((body) => {
        if (cancelled) return
        const parsed = parseCaptions(body)
        setDoc(parsed)
        setInitial(serializeCaptions(parsed))
        setLoadState("ready")
      })
      .catch(() => !cancelled && setLoadState("error"))
    return () => {
      cancelled = true
    }
  }, [target.caption?.url, reloadToken])

  // Body scroll stays put under the editor.
  React.useEffect(() => {
    const previous = document.body.style.overflow
    document.body.style.overflow = "hidden"
    return () => {
      document.body.style.overflow = previous
    }
  }, [])

  // The playhead, polled rather than taken from timeupdate (about 4 Hz, too
  // coarse to time captions by). State only moves in tenths of a second, so a
  // long track's list is not re-rendered every frame; handlers read the refs.
  const timeRef = React.useRef(0)
  const durationRef = React.useRef(duration)
  React.useEffect(() => {
    let raf = 0
    const tick = () => {
      const v = videoRef.current
      if (v) {
        timeRef.current = v.currentTime
        setTime((prev) => (Math.abs(prev - v.currentTime) >= 0.1 || (v.paused && prev !== v.currentTime) ? v.currentTime : prev))
        if (Number.isFinite(v.duration) && v.duration > 0 && v.duration !== durationRef.current) {
          durationRef.current = v.duration
          setDuration(v.duration)
        }
      }
      raf = requestAnimationFrame(tick)
    }
    raf = requestAnimationFrame(tick)
    return () => cancelAnimationFrame(raf)
  }, [])

  const cues = doc.cues
  const docText = React.useMemo(() => serializeCaptions(doc), [doc])
  const dirty = loadState === "ready" && (docText !== initial || (mode === "text" && text !== docText))
  const issues = React.useMemo(() => validateCues(cues, duration), [cues, duration])
  const issueFor = React.useMemo(() => new Map(issues.map((i) => [i.key, i.message])), [issues])

  // Preview the track on the video through a native text track, rebuilt as cues change.
  React.useEffect(() => {
    const video = videoRef.current
    if (!video || loadState !== "ready") return
    if (!trackRef.current) {
      trackRef.current = video.addTextTrack("captions", target.label, target.language)
      trackRef.current.mode = "showing"
    }
    const track = trackRef.current
    for (const cue of Array.from(track.cues ?? [])) track.removeCue(cue)
    for (const cue of cues) {
      if (cue.end > cue.start && cue.text.trim()) track.addCue(new VTTCue(cue.start, cue.end, cue.text))
    }
  }, [cues, loadState, target.label, target.language])

  const activeKey = cues.find((c) => time >= c.start && time < c.end)?.key ?? null

  // Keep the caption being spoken in view while the video plays.
  React.useEffect(() => {
    const v = videoRef.current
    if (!activeKey || !v || v.paused) return
    listRef.current?.querySelector(`[data-cue="${activeKey}"]`)?.scrollIntoView({ block: "nearest", behavior: "smooth" })
  }, [activeKey])

  const seek = React.useCallback((t: number) => {
    const v = videoRef.current
    if (!v) return
    v.currentTime = Math.max(0, Math.min(t, Number.isFinite(v.duration) ? v.duration : t))
    timeRef.current = v.currentTime
    setTime(v.currentTime)
  }, [])

  // Stable for the life of the editor, so memoized rows only re-render when
  // their own caption changes. Everything time-dependent reads the refs.
  const actions = React.useMemo(() => {
    const setCues = (update: (cues: Cue[]) => Cue[]) => setDoc((d) => ({ ...d, cues: update(d.cues) }))
    const patch = (key: string, p: Partial<Cue>) => setCues((cs) => cs.map((c) => (c.key === key ? { ...c, ...p } : c)))
    const now = () => Math.round(timeRef.current * 100) / 100

    const focusCue = (key: string) => {
      requestAnimationFrame(() => {
        const el = listRef.current?.querySelector<HTMLTextAreaElement>(`[data-cue="${key}"] textarea`)
        el?.focus()
        el?.scrollIntoView({ block: "nearest" })
      })
    }

    /** Adds a caption at `at`, running until the next one starts (or a couple of seconds). */
    const addAt = (at: number) => {
      const key = newCueKey()
      const start = Math.max(0, Math.round(at * 100) / 100)
      setCues((cs) => {
        const next = cs.filter((c) => c.start > start).sort((a, b) => a.start - b.start)[0]
        let end = start + NEW_CUE_SECONDS
        if (next && next.start - start >= 0.5) end = Math.min(end, next.start)
        if (durationRef.current > 0) end = Math.min(end, durationRef.current)
        if (end <= start) end = start + 0.5
        return [...cs, { key, start, end: Math.round(end * 100) / 100, text: "", settings: "" }].sort((a, b) => a.start - b.start)
      })
      setSelectedKey(key)
      focusCue(key)
    }

    return {
      patch,
      focusCue,
      addAtPlayhead: () => addAt(timeRef.current),
      select: (key: string, start: number) => {
        setSelectedKey(key)
        seek(start)
      },
      focusText: (key: string) => setSelectedKey(key),
      sort: () => setCues((cs) => [...cs].sort((a, b) => a.start - b.start)),
      setStart: (key: string) => patch(key, { start: now() }),
      setEnd: (key: string) => patch(key, { end: now() }),
      remove: (key: string) => setCues((cs) => cs.filter((c) => c.key !== key)),
      addAfter: (cue: Cue) => addAt(Math.max(cue.end, timeRef.current)),
    }
  }, [seek])

  const switchMode = (next: "cues" | "text") => {
    if (next === mode) return
    if (next === "text") {
      setText(serializeCaptions(doc))
      setTextError(null)
      setMode("text")
      return
    }
    try {
      const parsed = parseCaptions(text)
      setDoc(parsed)
      setTextError(null)
      setMode("cues")
    } catch (err) {
      setTextError(err instanceof CaptionParseError ? err.message : "This text isn't valid WebVTT or SubRip.")
    }
  }

  const save = async () => {
    let toSave = doc
    if (mode === "text") {
      try {
        toSave = parseCaptions(text)
      } catch (err) {
        setTextError(err instanceof CaptionParseError ? err.message : "This text isn't valid WebVTT or SubRip.")
        return
      }
    }
    const problems = validateCues(toSave.cues, duration)
    if (toSave.cues.length === 0) {
      toast("Add at least one caption before saving.", { tone: "error" })
      return
    }
    if (problems.length > 0) {
      toast(`${problems.length} caption${problems.length === 1 ? " needs" : "s need"} fixing before saving.`, { tone: "error" })
      if (mode === "cues") {
        setSelectedKey(problems[0].key)
        actions.focusCue(problems[0].key)
      }
      return
    }

    setSaving(true)
    try {
      const content = serializeCaptions(toSave)
      if (target.caption) {
        await updateCaption(asset.id, target.caption.id, { content })
      } else {
        await createCaption(asset.id, { language: target.language, label: target.label, content, is_default: target.isDefault })
      }
      toast(`${target.label} captions saved`, { tone: "success" })
      onClose()
    } catch (err) {
      toast(`Couldn't save captions: ${errorMessage(err)}`, { tone: "error" })
    } finally {
      setSaving(false)
    }
  }

  const requestClose = () => (dirty ? setConfirmDiscard(true) : onClose())

  const onKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Escape" && !confirmDiscard) {
      e.stopPropagation()
      requestClose()
    } else if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "s") {
      e.preventDefault()
      void save()
    }
  }

  return createPortal(
    <div
      role="dialog"
      aria-modal="true"
      aria-label={`Edit ${target.label} captions`}
      onKeyDown={onKeyDown}
      className="fixed inset-0 z-50 flex flex-col bg-canvas animate-enter"
    >
      {/* Header */}
      <header className="flex h-14 shrink-0 items-center gap-3 border-b border-zinc-800 px-3 sm:px-5">
        <IconButton label="Close editor" tooltip="bottom" onClick={requestClose}>
          <X className="h-5 w-5" />
        </IconButton>
        <div className="min-w-0 flex-1">
          <p className="truncate text-sm font-medium text-white">
            {target.label} <span className="ml-1 rounded border border-zinc-700 px-1.5 py-px font-mono text-[11px] uppercase text-zinc-400">{target.language}</span>
          </p>
          <p className="truncate text-xs text-zinc-500">{asset.title}</p>
        </div>

        <div role="tablist" aria-label="Editing mode" className="hidden rounded-lg border border-zinc-800 bg-zinc-900 p-0.5 sm:flex">
          {(
            [
              ["cues", "Timing", ListVideo],
              ["text", "Edit as text", FileText],
            ] as const
          ).map(([value, label, Icon]) => (
            <button
              key={value}
              role="tab"
              type="button"
              aria-selected={mode === value}
              onClick={() => switchMode(value)}
              className={cx(
                "inline-flex items-center gap-1.5 rounded-md px-3 py-1.5 text-xs font-medium transition-colors cursor-pointer",
                mode === value ? "bg-zinc-700 text-white" : "text-zinc-400 hover:text-white",
              )}
            >
              <Icon className="h-3.5 w-3.5" aria-hidden="true" />
              {label}
            </button>
          ))}
        </div>

        <Button variant="default" onClick={save} disabled={saving || loadState !== "ready" || (!dirty && !!target.caption)} className="min-w-20" title="Save (Ctrl+S)">
          {saving ? <Loader2 className="h-4 w-4 animate-spin" aria-label="Saving" /> : "Save"}
        </Button>
      </header>

      {loadState !== "ready" ? (
        <div className="flex flex-1 flex-col items-center justify-center gap-3 text-sm text-zinc-400">
          {loadState === "loading" ? (
            <Loader2 className="h-6 w-6 animate-spin" aria-label="Loading captions" />
          ) : (
            <>
              <CircleAlert className="h-6 w-6 text-rose-400" aria-hidden="true" />
              This caption track could not be loaded.
              <Button
                variant="outline"
                size="sm"
                onClick={() => {
                  setLoadState("loading")
                  setReloadToken((n) => n + 1)
                }}
              >
                <RotateCcw className="mr-1.5 h-3.5 w-3.5" aria-hidden="true" /> Try again
              </Button>
            </>
          )}
        </div>
      ) : (
        <div className="flex min-h-0 flex-1 flex-col-reverse lg:grid lg:grid-cols-[minmax(360px,1fr)_minmax(0,1.35fr)]">
          {/* Captions */}
          <section aria-label="Captions" className="flex min-h-0 flex-1 flex-col border-zinc-800 lg:border-r">
            {mode === "cues" ? (
              <>
                <div className="flex items-center gap-2 border-b border-zinc-800 px-4 py-2.5">
                  <Button variant="outline" size="sm" onClick={actions.addAtPlayhead}>
                    <Plus className="mr-1 h-3.5 w-3.5" aria-hidden="true" /> Caption
                  </Button>
                  <span className="text-xs text-zinc-500">at {formatCueTime(time)}</span>
                  <span className="ml-auto text-xs tabular-nums text-zinc-500">
                    {cues.length} line{cues.length === 1 ? "" : "s"}
                    {issues.length > 0 && <span className="ml-2 text-rose-300">· {issues.length} to fix</span>}
                  </span>
                </div>
                {cues.length === 0 ? (
                  <div className="flex flex-1 flex-col items-center justify-center gap-2 px-8 text-center">
                    <p className="text-sm font-medium text-zinc-200">No captions yet</p>
                    <p className="text-sm text-zinc-500">Play the video and press "+ Caption" where someone starts speaking, then type what they say.</p>
                  </div>
                ) : (
                  <ol ref={listRef} className="flex-1 space-y-2 overflow-y-auto p-3">
                    {cues.map((cue, i) => (
                      <CueRow
                        key={cue.key}
                        index={i}
                        cue={cue}
                        active={cue.key === activeKey}
                        selected={cue.key === selectedKey}
                        issue={issueFor.get(cue.key)}
                        actions={actions}
                      />
                    ))}
                  </ol>
                )}
                <p className="hidden border-t border-zinc-800 px-4 py-2 text-[11px] text-zinc-500 lg:block">
                  Ctrl+Enter in a caption adds the next one · Ctrl+S saves · Esc closes
                </p>
              </>
            ) : (
              <div className="flex min-h-0 flex-1 flex-col p-3">
                <label htmlFor="vtt-text" className="mb-2 text-xs text-zinc-500">
                  WebVTT. Paste SubRip (.srt) here too — it is converted when you switch back or save.
                </label>
                <textarea
                  id="vtt-text"
                  value={text}
                  onChange={(e) => {
                    setText(e.target.value)
                    setTextError(null)
                  }}
                  spellCheck={false}
                  className="min-h-0 flex-1 resize-none rounded-lg border border-zinc-800 bg-black/40 p-3 font-mono text-xs leading-relaxed text-zinc-100 focus:border-zinc-600 focus:outline-none"
                />
                {textError && (
                  <p role="alert" className="mt-2 flex items-center gap-1.5 text-sm text-rose-300">
                    <CircleAlert className="h-4 w-4" aria-hidden="true" /> {textError}
                  </p>
                )}
              </div>
            )}
          </section>

          {/* Video and timeline */}
          <section aria-label="Preview" className="shrink-0 space-y-4 border-b border-zinc-800 p-3 sm:p-5 lg:overflow-y-auto lg:border-b-0">
            <VideoPlayer
              ref={videoRef}
              src={playbackUrl(asset)}
              poster={asset.thumbnail_url || undefined}
              controls
              preload="auto"
              className="mx-auto aspect-video w-full max-w-4xl rounded-lg"
            />
            <div className="mx-auto hidden max-w-4xl sm:block">
              <CaptionTimeline
                cues={cues}
                duration={duration}
                currentTime={time}
                selectedKey={selectedKey}
                onSeek={seek}
                onSelect={(key) => {
                  setSelectedKey(key)
                  listRef.current?.querySelector(`[data-cue="${key}"]`)?.scrollIntoView({ block: "nearest" })
                }}
                onRetime={(key, start, end) => actions.patch(key, { start, end })}
              />
            </div>
          </section>
        </div>
      )}

      <ConfirmDialog
        open={confirmDiscard}
        title="Discard caption changes?"
        description="Your edits to these captions haven't been saved."
        confirmLabel="Discard"
        destructive
        onConfirm={onClose}
        onClose={() => setConfirmDiscard(false)}
      />
    </div>,
    document.body,
  )
}

type EditorActions = {
  patch: (key: string, patch: Partial<Cue>) => void
  select: (key: string, start: number) => void
  focusText: (key: string) => void
  sort: () => void
  setStart: (key: string) => void
  setEnd: (key: string) => void
  remove: (key: string) => void
  addAfter: (cue: Cue) => void
}

interface CueRowProps {
  index: number
  cue: Cue
  active: boolean
  selected: boolean
  issue?: string
  actions: EditorActions
}

const CueRow = React.memo(function CueRow({ index, cue, active, selected, issue, actions }: CueRowProps) {
  return (
    <li
      data-cue={cue.key}
      className={cx(
        "group rounded-lg border p-2.5 transition-colors",
        issue ? "border-rose-500/50 bg-rose-500/[0.04]" : selected ? "border-zinc-500 bg-zinc-900" : active ? "border-emerald-500/50 bg-emerald-500/[0.06]" : "border-zinc-800 bg-surface hover:border-zinc-700",
      )}
    >
      <div className="flex items-center gap-1.5">
        <button
          type="button"
          onClick={() => actions.select(cue.key, cue.start)}
          title="Play from here"
          className="w-7 shrink-0 rounded text-left font-mono text-[11px] text-zinc-500 hover:text-white cursor-pointer"
        >
          {index + 1}
        </button>
        <TimeField
          label={`Caption ${index + 1} start`}
          value={cue.start}
          onCommit={(start) => {
            actions.patch(cue.key, { start })
            actions.sort()
          }}
        />
        <span className="text-zinc-600" aria-hidden="true">→</span>
        <TimeField label={`Caption ${index + 1} end`} value={cue.end} onCommit={(end) => actions.patch(cue.key, { end })} />
        <div className="ml-auto flex items-center opacity-100 transition-opacity sm:opacity-0 sm:group-hover:opacity-100 sm:group-focus-within:opacity-100">
          <IconButton label="Start at playhead" size="sm" onClick={() => actions.setStart(cue.key)}>
            <ArrowLeftToLine className="h-3.5 w-3.5" />
          </IconButton>
          <IconButton label="End at playhead" size="sm" onClick={() => actions.setEnd(cue.key)}>
            <ArrowRightToLine className="h-3.5 w-3.5" />
          </IconButton>
          <IconButton label="Delete caption" size="sm" onClick={() => actions.remove(cue.key)} className="hover:text-rose-300">
            <Trash2 className="h-3.5 w-3.5" />
          </IconButton>
        </div>
      </div>
      <textarea
        value={cue.text}
        onChange={(e) => actions.patch(cue.key, { text: e.target.value })}
        onFocus={() => actions.focusText(cue.key)}
        onKeyDown={(e) => {
          if ((e.metaKey || e.ctrlKey) && e.key === "Enter") {
            e.preventDefault()
            actions.addAfter(cue)
          }
        }}
        rows={Math.min(4, Math.max(1, cue.text.split("\n").length))}
        placeholder="Type the caption"
        aria-label={`Caption ${index + 1} text`}
        className="mt-1.5 block w-full resize-none rounded-md bg-black/30 px-2.5 py-1.5 text-sm leading-snug text-zinc-100 placeholder:text-zinc-600 focus:bg-black/50 focus:outline-none focus:ring-1 focus:ring-zinc-600"
      />
      {issue && <p className="mt-1.5 text-xs text-rose-300">{issue}</p>}
    </li>
  )
})

/** A time typed as m:ss.mmm; applied on Enter or blur. Enter keeps a bad value for fixing, blur reverts it. */
function TimeField({ label, value, onCommit }: { label: string; value: number; onCommit: (seconds: number) => void }) {
  const [draft, setDraft] = React.useState<string | null>(null)
  const [invalid, setInvalid] = React.useState(false)

  const apply = (keepInvalid: boolean) => {
    if (draft === null) return
    const parsed = parseCueTime(draft)
    if (parsed === null) {
      if (keepInvalid) {
        setInvalid(true)
      } else {
        setDraft(null)
        setInvalid(false)
      }
      return
    }
    setDraft(null)
    setInvalid(false)
    if (Math.abs(parsed - value) > 0.0005) onCommit(Math.round(parsed * 1000) / 1000)
  }

  return (
    <input
      value={draft ?? formatCueTime(value)}
      onChange={(e) => {
        setDraft(e.target.value)
        setInvalid(false)
      }}
      onBlur={() => apply(false)}
      onKeyDown={(e) => {
        if (e.key === "Enter") {
          e.preventDefault()
          apply(true)
        } else if (e.key === "Escape" && draft !== null) {
          e.stopPropagation()
          setDraft(null)
          setInvalid(false)
        }
      }}
      aria-label={label}
      aria-invalid={invalid}
      inputMode="decimal"
      className={cx(
        "h-7 w-[5.5rem] rounded-md border bg-black/30 px-1.5 text-center font-mono text-xs tabular-nums text-zinc-100 focus:outline-none",
        invalid ? "border-rose-500" : "border-zinc-800 focus:border-zinc-500",
      )}
    />
  )
}
