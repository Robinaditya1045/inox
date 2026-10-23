/**
 * WebVTT reading and writing for the caption editor.
 *
 * The backend is the authority on what a valid track is (it re-checks and
 * normalizes everything it is sent, see backend/internal/media/captions.go);
 * this is only enough to turn a track into editable cues and back. SubRip is
 * read too, so a pasted .srt works in the text view.
 */

export interface Cue {
  /** Stable React key; never written to the file */
  key: string
  /** Seconds */
  start: number
  end: number
  text: string
  /** WebVTT cue settings from the original file, e.g. "line:90% align:start" */
  settings: string
}

export interface CaptionDocument {
  /** The WEBVTT line and any header lines under it */
  header: string
  /** STYLE and REGION blocks, kept verbatim so editing a track keeps its styling */
  blocks: string[]
  cues: Cue[]
}

export class CaptionParseError extends Error {
  line: number

  constructor(line: number, message: string) {
    super(`Line ${line}: ${message}`)
    this.name = "CaptionParseError"
    this.line = line
  }
}

let keySeq = 0
export function newCueKey(): string {
  keySeq += 1
  return `cue-${Date.now().toString(36)}-${keySeq}`
}

export function emptyDocument(): CaptionDocument {
  return { header: "WEBVTT", blocks: [], cues: [] }
}

const TIMESTAMP = /^(?:(\d+):)?(\d{1,2}):(\d{1,2})[.,](\d{1,3})$/

function parseFileTimestamp(raw: string): number | null {
  const m = TIMESTAMP.exec(raw.trim())
  if (!m) return null
  const h = m[1] ? Number(m[1]) : 0
  const min = Number(m[2])
  const sec = Number(m[3])
  if (min > 59 || sec > 59) return null
  const ms = Number(m[4].padEnd(3, "0"))
  return h * 3600 + min * 60 + sec + ms / 1000
}

/** Reads WebVTT or SubRip into cues. Throws CaptionParseError naming the line. */
export function parseCaptions(input: string): CaptionDocument {
  const text = input.replace(/^\uFEFF/, "").replace(/\r\n?/g, "\n")
  const lines = text.split("\n")

  // Group non-blank lines into blocks, remembering where each starts.
  const groups: { line: number; lines: string[] }[] = []
  let current: { line: number; lines: string[] } | null = null
  lines.forEach((l, i) => {
    if (l.trim() === "") {
      current = null
      return
    }
    if (!current) {
      current = { line: i + 1, lines: [] }
      groups.push(current)
    }
    current.lines.push(l)
  })

  const doc = emptyDocument()
  let rest = groups
  if (groups.length > 0 && /^WEBVTT(?:[ \t]|$)/.test(groups[0].lines[0])) {
    doc.header = groups[0].lines.join("\n")
    rest = groups.slice(1)
  }

  for (const g of rest) {
    const first = g.lines[0]
    if (/^(STYLE|REGION)(?:[ \t]|$)/.test(first)) {
      doc.blocks.push(g.lines.join("\n"))
      continue
    }
    if (/^NOTE(?:[ \t]|$)/.test(first)) continue

    let timingAt = 0
    if (!first.includes("-->")) {
      if (g.lines.length < 2 || !g.lines[1].includes("-->")) {
        throw new CaptionParseError(g.line, "expected a timing line like 00:00:01.000 --> 00:00:04.000")
      }
      timingAt = 1
    }
    const [left, right] = g.lines[timingAt].split("-->")
    const fields = right.trim().split(/\s+/)
    const start = parseFileTimestamp(left)
    const end = parseFileTimestamp(fields[0] ?? "")
    if (start === null || end === null) {
      throw new CaptionParseError(g.line + timingAt, `"${g.lines[timingAt].trim()}" is not a valid timing line`)
    }
    // SubRip's X1/Y1 coordinates are not WebVTT settings.
    const settings = fields.slice(1).filter((f) => f.includes(":") && !/^[XY]\d:/i.test(f)).join(" ")
    doc.cues.push({
      key: newCueKey(),
      start,
      end,
      text: g.lines.slice(timingAt + 1).join("\n"),
      settings,
    })
  }
  doc.cues.sort((a, b) => a.start - b.start)
  return doc
}

/** HH:MM:SS.mmm, the form written to files. */
function formatTimestamp(seconds: number): string {
  const totalMs = Math.max(0, Math.round(seconds * 1000))
  const h = Math.floor(totalMs / 3_600_000)
  const m = Math.floor(totalMs / 60_000) % 60
  const s = Math.floor(totalMs / 1000) % 60
  const ms = totalMs % 1000
  return `${String(h).padStart(2, "0")}:${String(m).padStart(2, "0")}:${String(s).padStart(2, "0")}.${String(ms).padStart(3, "0")}`
}

/** Writes cues back out as WebVTT, in time order. Cue identifiers are not kept. */
export function serializeCaptions(doc: CaptionDocument): string {
  const parts = [doc.header.trim() || "WEBVTT", ...doc.blocks]
  for (const cue of [...doc.cues].sort((a, b) => a.start - b.start)) {
    const timing = `${formatTimestamp(cue.start)} --> ${formatTimestamp(cue.end)}${cue.settings ? ` ${cue.settings}` : ""}`
    // A blank line inside a payload would end the cue early, and "-->" would
    // read as a timing line.
    const payload = cue.text
      .split("\n")
      .filter((l) => l.trim() !== "")
      .map((l) => l.replaceAll("-->", "->"))
      .join("\n")
    parts.push(`${timing}\n${payload}`)
  }
  return parts.join("\n\n") + "\n"
}

/** "1:02.500" (or "1:01:02.500"): the compact form shown in the editor's inputs. */
export function formatCueTime(seconds: number): string {
  const totalMs = Math.max(0, Math.round(seconds * 1000))
  const h = Math.floor(totalMs / 3_600_000)
  const m = Math.floor(totalMs / 60_000) % 60
  const s = Math.floor(totalMs / 1000) % 60
  const ms = String(totalMs % 1000).padStart(3, "0")
  const ss = String(s).padStart(2, "0")
  return h > 0 ? `${h}:${String(m).padStart(2, "0")}:${ss}.${ms}` : `${m}:${ss}.${ms}`
}

/**
 * Reads what someone typed into a time field: "1:02.5", "62.5", "0:01:02,500".
 * Returns null for anything that is not a time.
 */
export function parseCueTime(input: string): number | null {
  const raw = input.trim().replace(",", ".")
  if (raw === "") return null
  if (/^\d+(?:\.\d+)?$/.test(raw)) return Number(raw)
  const m = /^(?:(\d+):)?(\d{1,2}):(\d{1,2}(?:\.\d{1,3})?)$/.exec(raw)
  if (!m) return null
  const h = m[1] ? Number(m[1]) : 0
  const min = Number(m[2])
  const sec = Number(m[3])
  if (min > 59 || sec >= 60) return null
  return h * 3600 + min * 60 + sec
}

export interface CueIssue {
  key: string
  message: string
}

/** What would stop a track saving, per cue. Overlaps are allowed, as in WebVTT. */
export function validateCues(cues: Cue[], duration?: number): CueIssue[] {
  const issues: CueIssue[] = []
  for (const cue of cues) {
    if (!(cue.end > cue.start)) {
      issues.push({ key: cue.key, message: "Ends before it starts" })
    } else if (cue.text.trim() === "") {
      issues.push({ key: cue.key, message: "Caption text is empty" })
    } else if (duration && Number.isFinite(duration) && cue.start >= duration) {
      issues.push({ key: cue.key, message: "Starts after the video ends" })
    }
  }
  return issues
}

/** Cue text as plain lines: tags (<i>, <v Name>, <c.yellow>) and entities resolved. */
export function cuePlainText(text: string): string {
  return text
    .replace(/<[^>]*>/g, "")
    .replace(/&lt;/g, "<")
    .replace(/&gt;/g, ">")
    .replace(/&nbsp;/g, "\u00a0")
    .replace(/&lrm;|&rlm;/g, "")
    .replace(/&amp;/g, "&")
}
