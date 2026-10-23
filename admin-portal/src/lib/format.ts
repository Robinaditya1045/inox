/** Display formatting shared by the content pages. */

/** 3:07, 1:02:45: how a running time reads on a thumbnail. */
export function formatDuration(seconds: number | undefined): string {
  if (!seconds || seconds <= 0 || !Number.isFinite(seconds)) return ""
  const total = Math.round(seconds)
  const h = Math.floor(total / 3600)
  const m = Math.floor(total / 60) % 60
  const s = String(total % 60).padStart(2, "0")
  return h > 0 ? `${h}:${String(m).padStart(2, "0")}:${s}` : `${m}:${s}`
}

export function formatDate(iso: string | undefined): string {
  if (!iso) return ""
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ""
  return d.toLocaleDateString(undefined, { month: "short", day: "numeric", year: "numeric" })
}

export function formatDateTime(iso: string | undefined): string {
  if (!iso) return ""
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ""
  return d.toLocaleString(undefined, { month: "short", day: "numeric", year: "numeric", hour: "numeric", minute: "2-digit" })
}

/** "just now", "5 min ago", "3 days ago" — falls back to the date past a month. */
export function formatRelative(iso: string | undefined): string {
  if (!iso) return ""
  const then = new Date(iso).getTime()
  if (Number.isNaN(then)) return ""
  const diff = Math.max(0, Date.now() - then) / 1000
  if (diff < 60) return "just now"
  if (diff < 3600) return `${Math.floor(diff / 60)} min ago`
  if (diff < 86400) return `${Math.floor(diff / 3600)} hr ago`
  if (diff < 86400 * 30) {
    const days = Math.floor(diff / 86400)
    return `${days} day${days === 1 ? "" : "s"} ago`
  }
  return formatDate(iso)
}

/** The last path segment of a URL, for "Filename"-style display. */
export function fileNameOf(url: string | undefined): string {
  if (!url) return ""
  try {
    const parts = new URL(url).pathname.split("/").filter(Boolean)
    return decodeURIComponent(parts[parts.length - 1] ?? url)
  } catch {
    return url
  }
}

export function hostOf(url: string | undefined): string {
  if (!url) return ""
  try {
    return new URL(url).host
  } catch {
    return ""
  }
}
