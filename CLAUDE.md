# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Context exclusions

`.claudeignore` at the repo root excludes `INTERVIEW_PREP.md` and `/docs` from context. Do not read, search, summarize, or cite either — treat them as outside the repo, and answer from the source instead. Keep new entries in `.claudeignore` rather than restating them here.

## Project overview

Inox is a synchronized watch-party platform: three independently deployable apps in one monorepo — a Go backend (REST API + WebSocket hub + WebRTC SFU), a React/Vite viewer client (`frontend/`), and a React/Vite admin dashboard (`admin-portal/`).

## Commands

### Local infrastructure
- `make up` / `make down` — start/stop Postgres, Redis, MinIO, pgAdmin via Docker Compose
- `make dev` — starts infra, then runs backend + frontend + admin-portal concurrently (ports 8080 / 5173 / 3000)
- `./dev.sh` — same three services in a tmux session (`inox`) instead of `concurrently`
- `mprocs` — same three services driven by `mprocs.yaml` (requires the `mprocs` binary)

### Backend (`backend/`, Go 1.25)
- `go run ./cmd/server` — run the API/WebSocket server
- `go run ./cmd/worker` — run the asynq background worker (media transcoding)
- `go run ./cmd/migrate --command=up|down|status [--steps=N] [--dir=migrations]` — hand-rolled SQL migration runner (not golang-migrate); run with `backend/` as cwd, or pass `--db`
- `go build ./...`, `go vet ./...`
- `go test ./...` — full suite; CI runs `go test -v -race ./...`
- Single test: `go test ./internal/auth/... -run TestName -v`
- Tests use stdlib `testing` with hand-written in-memory fakes (no testify/mockgen)

### Frontend (`frontend/`) and Admin Portal (`admin-portal/`)
Both are separate Vite + React 19 + TypeScript apps with identical scripts. They style themselves differently and are not interchangeable: `frontend/` uses CSS Modules + CSS custom properties (`src/styles/theme.css`), while `admin-portal/` uses Tailwind v4 via `@tailwindcss/vite` (`@import "tailwindcss"` in `src/index.css`, with only genuinely custom rules — theme vars, scrollbar, badge glows, `animate-pulse-subtle` — kept alongside it). Don't hand-write utility classes into `admin-portal/src/index.css`; let Tailwind generate them.
- `npm run dev` — Vite dev server
- `npm run build` — `tsc -b && vite build`
- `npm run lint` — ESLint
- `npx tsc --noEmit -p tsconfig.app.json` — typecheck only (what CI runs)
- Neither app has a test runner configured yet (no vitest/jest) — don't assume `npm test` works.

Root `package.json` only wires up Husky + lint-staged: `gofmt`/`goimports` on `backend/**/*.go`, `prettier` on `frontend/`/`admin-portal/` `.ts(x)/.js(x)/.css/.md`, run automatically on commit.

## Architecture

### Three deployables, one repo
- `backend/` — Go service exposing a REST API, a WebSocket hub for room events, and a Pion-based WebRTC SFU for voice chat/screen share. Entry points: `cmd/server` (API), `cmd/worker` (asynq media processing), `cmd/migrate` (schema).
- `frontend/` — the watch-party client end users join: synced video playback, chat, and voice/screen-share.
- `admin-portal/` — a separate operator dashboard (room inspector, media catalog, live telemetry), deployed independently (Vercel) from a different origin than the backend — this is why CORS and session-cookie `SameSite` handling matter.

### Backend layering (`backend/internal/`)
`app.App.Run()` is the single composition root: it wires repositories, services, the WS hub, and the SFU manager, then owns graceful shutdown. `api/router.go` registers all routes on the stdlib `net/http.ServeMux` (Go 1.22+ method+pattern routing) and layers `CORS → RequestLogger → metrics` around it; handlers live in `api/handler`, cross-cutting concerns (`RequireAuth`, `RequireRoomMembership`, `RequireAdminRole`, `RateLimit`) in `api/middleware`. Domain packages (`auth`, `room`, `media`, `sfu`, `ws`, `observability`) each expose a repository + service pair; `domain/` holds shared structs. `storage` abstracts MinIO vs. local-filesystem media storage and falls back to local automatically if MinIO is unreachable at boot (see `app.Run`).

### Auth & sessions
Session-based, not JWT: `auth.SessionStore` persists sessions in Redis under cookie name `inox_session`. `RequireAuth` also accepts `Authorization: Bearer`, `X-Session-ID`, or `?session_id=` as fallbacks (useful where a client can't set custom headers). Cookie `SameSite` is `Lax` in development and `None`+`Secure` in production/staging, because `admin-portal`/`frontend` and the backend live on different origins in deployment. `config.Load()` refuses to start in prod/staging if `SESSION_SECRET` is the default value or under 32 characters.

There is no `role` column on `users`: `domain.Session.Role` is always `user`, so admin access (`RequireAdminRole`, gating `/api/v1/admin/*` and `/metrics`) is granted solely by listing an address in the `ADMIN_EMAILS` env var. Without it every admin route returns 403.

Neither client app talks to the API with bare `fetch`. Each has one client that attaches the base URL and credentials — `frontend/src/api/client.ts` and `admin-portal/src/lib/api.ts` — sending `credentials: 'include'` plus `Authorization: Bearer` / `X-Session-ID` from the `inox_session_id` localStorage key. Route new calls through those; a relative `fetch('/api/v1/...')` only works behind the Vite dev proxy and 404s once deployed. The admin portal has no login screen of its own, and localStorage is origin-scoped, so it adopts a session from `?session_id=…` on first load (`bootstrapSessionFromUrl`, called in `main.tsx`) and strips it from the URL. WebSocket handshakes can't carry headers, so the telemetry socket passes the session as a query parameter instead.

### Real-time protocol
Chat, playback sync (`PLAY`/`PAUSE`/`SEEK`/`CHANGE_MEDIA`/`SYNC_PLAYBACK`), and WebRTC signaling (`SFU_OFFER`/`SFU_ANSWER`/`SFU_ICE_CANDIDATE`) all flow over one WebSocket per room as typed `ws.Event` JSON messages (`backend/internal/ws/event.go`). `ws.Hub` fans events out per room; when Redis is configured, `RedisEventBus` mirrors events across processes so the hub scales horizontally. Voice/screen-share RTP media itself doesn't cross the WebSocket — only SDP/ICE signaling does; actual audio/video is routed through `sfu.Manager` (one Pion-backed `sfu.Room` per room acting as a selective forwarding unit).

On the frontend this maps to matching Context+Provider+hook triples — `AuthProvider`/`auth.context`/`useAuth`, `RoomProvider`/`room.context`/`useRoom`, `RTCProvider`/`rtc.context`/`useRTC` — all built on top of one `useRoomSocket` connection. `rtcService` (`frontend/src/services/rtc/rtc.service.ts`) wraps the raw `RTCPeerConnection`; `RTCProvider` owns signaling and exposes connection/mute/deafen/screen-share state.

Browser autoplay policy is relevant here: `AudioRenderer` creates `<audio>` elements for remote SFU tracks asynchronously (often well after the click that started signaling), so it explicitly calls `.play()` and retries on the next `pointerdown`/`keydown` if the browser blocks it — don't revert to relying on the bare `autoplay` attribute, since that fails silently whenever a peer's track arrives without a very recent, directly-linked user gesture.

### Media pipeline
Uploads go through `media.Service` → `storage.Service` (MinIO or local disk) → an asynq job picked up by `cmd/worker`, which transcodes to HLS. `mediaHandler.StreamProxy` serves both raw assets and HLS segments; `ReconcileOrphanedAssets` runs at boot to recover jobs stuck from a previous server crash.

### Observability
`observability.EventAggregator` batches and persists WS/room events to Postgres for historical analytics. `observability.TelemetryHub` streams a live snapshot over `/api/v1/admin/telemetry/ws` to the admin portal's `SystemPulseDashboard`/`LiveTelemetryChart` — one snapshot on connect, then every 2s. `/metrics` exposes Prometheus format, gated by `RequireMetricsAccess`.

`useTelemetryStream` is owned by `admin-portal/src/App.tsx`, not by the dashboard: tabs render conditionally, so hooking it lower down tore down the socket and discarded chart history on every tab switch. It also feeds the header's connection badge and the Room Inspector's live rooms. Its liveness watchdog compares two local `Date.now()` readings — never the server-sent `timestamp` — since server/browser clock skew above the 10s threshold would otherwise close a healthy socket every 5s.

Each panel falls back to built-in mock fixtures when the backend is unreachable ("demo mode"). Keep that state honest: demo mode must not report `isConnected`, and mock samples must not be mixed into live chart history or used to decide whether the backend is alive.
