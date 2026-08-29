/**
 * Centralized API access for the admin portal.
 *
 * Every backend call must go through here. Previously each hook called
 * `fetch("/api/v1/...")` with a relative path and no credentials, which only
 * worked behind the Vite dev proxy: in any deployed setup (Vercel + tunnelled
 * backend) the requests hit the static host instead of the API and 404'd, and
 * the protected admin routes 401'd because no session was attached.
 */

const SESSION_STORAGE_KEY = "inox_session_id"

/**
 * Resolves the REST base URL, mirroring frontend/src/api/client.ts so both apps
 * behave identically when served from a LAN IP or a tunnel hostname.
 */
function resolveBaseUrl(): string {
  const envUrl = import.meta.env.VITE_API_BASE_URL || "http://localhost:8080/api/v1"

  if (import.meta.env.PROD) {
    return envUrl
  }

  // In dev, a bare "localhost" default breaks when the portal is opened from
  // another device on the network; point it back at whatever host served us.
  if (window.location.hostname !== "localhost" && window.location.hostname !== "127.0.0.1") {
    return envUrl.replace("localhost", window.location.hostname).replace("127.0.0.1", window.location.hostname)
  }
  return envUrl
}

export const API_BASE_URL = resolveBaseUrl()

/** Reads the session ID shared with the backend (see backend middleware.RequireAuth). */
export function getSessionId(): string | null {
  try {
    return localStorage.getItem(SESSION_STORAGE_KEY)
  } catch {
    return null
  }
}

export function setSessionId(id: string): void {
  try {
    localStorage.setItem(SESSION_STORAGE_KEY, id)
  } catch {
    /* storage unavailable (private mode) — requests fall back to cookie auth */
  }
}

export function clearSessionId(): void {
  try {
    localStorage.removeItem(SESSION_STORAGE_KEY)
  } catch {
    /* no-op */
  }
}

/**
 * Adopts a session handed over via `?session_id=...` and strips it back out of
 * the address bar. The admin portal has no login screen of its own and
 * localStorage is origin-scoped, so a session created on the main client app
 * (a different port/host) is otherwise unreachable here — which is why every
 * authenticated panel silently fell back to demo mode.
 */
export function bootstrapSessionFromUrl(): void {
  const params = new URLSearchParams(window.location.search)
  const sessionId = params.get("session_id")
  if (!sessionId) return

  setSessionId(sessionId)
  params.delete("session_id")
  const query = params.toString()
  const cleanUrl = `${window.location.pathname}${query ? `?${query}` : ""}${window.location.hash}`
  window.history.replaceState({}, "", cleanUrl)
}

/** Builds an absolute API URL from a `/api/v1`-relative path. */
export function apiUrl(path: string): string {
  const normalized = path.startsWith("/api/v1") ? path.slice("/api/v1".length) : path
  return `${API_BASE_URL}${normalized.startsWith("/") ? normalized : `/${normalized}`}`
}

/** Builds the telemetry WebSocket URL, carrying the session as a query param. */
export function telemetryWsUrl(explicitUrl?: string): string {
  let base = explicitUrl || import.meta.env.VITE_TELEMETRY_WS_URL

  if (!base) {
    const httpUrl = apiUrl("/admin/telemetry/ws")
    base = httpUrl.replace(/^http:/, "ws:").replace(/^https:/, "wss:")
  } else if (!import.meta.env.PROD && window.location.hostname !== "localhost" && window.location.hostname !== "127.0.0.1") {
    base = base.replace("localhost", window.location.hostname).replace("127.0.0.1", window.location.hostname)
  }

  const sessionId = getSessionId()
  if (!sessionId) return base

  // WebSocket handshakes cannot carry custom headers, so the backend also
  // accepts the session as a query parameter.
  return `${base}${base.includes("?") ? "&" : "?"}session_id=${encodeURIComponent(sessionId)}`
}

/**
 * The telemetry endpoint with no session attached, safe to render in the UI.
 * Never display telemetryWsUrl() itself — it embeds the session ID.
 */
export function telemetryDisplayUrl(): string {
  const base = import.meta.env.VITE_TELEMETRY_WS_URL
  if (base) return base
  return apiUrl("/admin/telemetry/ws").replace(/^http:/, "ws:").replace(/^https:/, "wss:")
}

export class ApiError extends Error {
  status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = "ApiError"
    this.status = status
  }
}

/**
 * fetch() wrapper that attaches the API base URL and the session credentials
 * the backend's RequireAuth middleware expects.
 */
export async function apiFetch(path: string, init: RequestInit = {}): Promise<Response> {
  const sessionId = getSessionId()

  const headers = new Headers(init.headers)
  if (sessionId) {
    headers.set("Authorization", `Bearer ${sessionId}`)
    headers.set("X-Session-ID", sessionId)
  }

  return fetch(apiUrl(path), {
    ...init,
    // The backend sets an HttpOnly `inox_session` cookie with SameSite=None in
    // production, so cross-origin requests must opt in to sending it.
    credentials: "include",
    headers,
  })
}

/**
 * POSTs a body to the API via XHR so upload progress can be reported.
 * fetch() exposes no upload progress events, which is why file uploads need this.
 */
export function uploadWithProgress<T>(
  path: string,
  body: FormData,
  onProgress?: (percent: number) => void,
): Promise<T> {
  return new Promise<T>((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    xhr.open("POST", apiUrl(path), true)
    xhr.withCredentials = true

    const sessionId = getSessionId()
    if (sessionId) {
      xhr.setRequestHeader("Authorization", `Bearer ${sessionId}`)
      xhr.setRequestHeader("X-Session-ID", sessionId)
    }
    // Content-Type is intentionally left unset: the browser must add the
    // multipart boundary itself.

    if (onProgress) {
      xhr.upload.onprogress = (e) => {
        if (e.lengthComputable) {
          onProgress(Math.round((e.loaded / e.total) * 100))
        }
      }
    }

    xhr.onload = () => {
      if (xhr.status < 200 || xhr.status >= 300) {
        reject(new ApiError(xhr.status, xhr.responseText || xhr.statusText || `Upload failed with status ${xhr.status}`))
        return
      }
      try {
        resolve(JSON.parse(xhr.responseText) as T)
      } catch {
        reject(new ApiError(xhr.status, "Upload succeeded but the server returned a malformed response"))
      }
    }

    xhr.onerror = () => reject(new ApiError(0, "Network error during upload"))
    xhr.onabort = () => reject(new ApiError(0, "Upload aborted"))

    xhr.send(body)
  })
}

/** apiFetch + JSON parsing, throwing ApiError on a non-2xx response. */
export async function apiJson<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers)
  if (init.body !== undefined && !(init.body instanceof FormData) && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json")
  }

  const res = await apiFetch(path, { ...init, headers })

  if (!res.ok) {
    const detail = await res.text().catch(() => "")
    throw new ApiError(res.status, detail || res.statusText || `Request failed with status ${res.status}`)
  }

  if (res.status === 204) {
    return undefined as T
  }

  return (await res.json()) as T
}
