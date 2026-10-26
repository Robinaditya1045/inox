/**
 * Which Community posts this browser has already seen, so the nav can badge new
 * ones. Stored as the newest published_at the viewer has had on screen — a
 * server timestamp, so a skewed local clock can't hide or resurrect posts.
 * It's a per-viewer convenience: losing it only means a badge reappears.
 */
const STORAGE_KEY = "inox_community_last_seen";
const EVENT = "inox:community-seen";

/** With nothing stored yet, only posts from the last fortnight count as new. */
const FIRST_VISIT_WINDOW_MS = 14 * 86_400_000;

export function readLastSeen(): string | null {
  try {
    return localStorage.getItem(STORAGE_KEY);
  } catch {
    return null;
  }
}

/** Records the newest of these publish dates as seen, if it's newer than what's stored. */
export function markSeen(publishedAts: (string | null)[]): void {
  const newest = publishedAts.reduce<string | null>((max, at) => {
    if (!at) return max;
    return !max || Date.parse(at) > Date.parse(max) ? at : max;
  }, null);
  if (!newest) return;

  const current = readLastSeen();
  if (current && Date.parse(current) >= Date.parse(newest)) return;
  try {
    localStorage.setItem(STORAGE_KEY, newest);
  } catch {
    /* storage unavailable: the badge just stays until next time */
  }
  window.dispatchEvent(new Event(EVENT));
}

export function isUnread(
  publishedAt: string | null,
  lastSeen: string | null,
): boolean {
  if (!publishedAt) return false;
  const at = Date.parse(publishedAt);
  if (lastSeen) return at > Date.parse(lastSeen);
  return Date.now() - at < FIRST_VISIT_WINDOW_MS;
}

/** Calls back when the seen marker moves, here or in another tab. */
export function onSeenChange(callback: () => void): () => void {
  const onStorage = (e: StorageEvent) => {
    if (e.key === STORAGE_KEY) callback();
  };
  window.addEventListener(EVENT, callback);
  window.addEventListener("storage", onStorage);
  return () => {
    window.removeEventListener(EVENT, callback);
    window.removeEventListener("storage", onStorage);
  };
}
