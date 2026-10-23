import type { MediaCaption } from "../types/media";

export interface CaptionCue {
  start: number;
  end: number;
  /** Plain text, one entry per line */
  lines: string[];
}

const TIMESTAMP = /^(?:(\d+):)?(\d{1,2}):(\d{1,2})[.,](\d{1,3})$/;

function seconds(raw: string): number | null {
  const m = TIMESTAMP.exec(raw.trim());
  if (!m) return null;
  return (
    (m[1] ? Number(m[1]) : 0) * 3600 +
    Number(m[2]) * 60 +
    Number(m[3]) +
    Number(m[4].padEnd(3, "0")) / 1000
  );
}

/** Cue text without markup: <i>, <v Name>, <c.x> tags dropped, entities resolved. */
function plain(text: string): string {
  return text
    .replace(/<[^>]*>/g, "")
    .replace(/&lt;/g, "<")
    .replace(/&gt;/g, ">")
    .replace(/&nbsp;/g, " ")
    .replace(/&lrm;|&rlm;/g, "")
    .replace(/&amp;/g, "&");
}

/**
 * Reads the cues of a WebVTT track. The backend has already checked and
 * normalized it (see backend/internal/media/captions.go), so anything that is
 * not a cue is skipped rather than reported.
 */
export function parseCaptionCues(vtt: string): CaptionCue[] {
  const cues: CaptionCue[] = [];
  const blocks = vtt.replace(/\r\n?/g, "\n").split(/\n{2,}/);
  for (const block of blocks) {
    const lines = block.split("\n").filter((l) => l.trim() !== "");
    const at = lines.findIndex((l) => l.includes("-->"));
    if (at < 0 || at > 1) continue;
    const [left, right] = lines[at].split("-->");
    const start = seconds(left);
    const end = seconds(right.trim().split(/\s+/)[0] ?? "");
    if (start === null || end === null || end <= start) continue;
    cues.push({ start, end, lines: lines.slice(at + 1).map(plain) });
  }
  return cues.sort((a, b) => a.start - b.start);
}

function base(tag: string): string {
  return tag.split("-")[0].toLowerCase();
}

/**
 * The track to show once captions are on: the viewer's last choice (or the
 * same language in another region), then the video's default, then whatever
 * matches the browser's language, then the first.
 */
export function pickCaptionTrack(
  tracks: MediaCaption[],
  preferred: string | null,
): MediaCaption | null {
  if (tracks.length === 0) return null;
  if (preferred) {
    const exact = tracks.find(
      (t) => t.language.toLowerCase() === preferred.toLowerCase(),
    );
    if (exact) return exact;
    const sameLanguage = tracks.find(
      (t) => base(t.language) === base(preferred),
    );
    if (sameLanguage) return sameLanguage;
  }
  const byDefault = tracks.find((t) => t.is_default);
  if (byDefault) return byDefault;
  const browser = typeof navigator !== "undefined" ? navigator.language : "";
  return (
    tracks.find((t) => browser && base(t.language) === base(browser)) ??
    tracks[0]
  );
}
