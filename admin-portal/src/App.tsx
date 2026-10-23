import * as React from "react"
import { createBrowserRouter, Navigate, RouterProvider } from "react-router-dom"
import { AppLayout } from "@/components/layout/AppLayout"
import { PageBody, PageHeader } from "@/components/layout/PageHeader"
import { UploadDialog } from "@/components/content/UploadDialog"
import { LiveChannelCenter } from "@/components/live/LiveChannelCenter"
import { RoomCommandCenter } from "@/components/rooms/RoomCommandCenter"
import { ToastProvider } from "@/components/ui/toast"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { DashboardPage } from "@/pages/DashboardPage"
import { ContentPage } from "@/pages/ContentPage"
import { VideoLayout } from "@/pages/VideoLayout"
import { VideoDetailsPage } from "@/pages/VideoDetailsPage"
import { VideoCaptionsPage } from "@/pages/VideoCaptionsPage"
import { useTelemetryStream } from "@/hooks/useTelemetryStream"
import { useMediaCatalog } from "@/hooks/useMediaCatalog"
import { CatalogContext, StudioActionsContext, TelemetryContext, useTelemetry, type StudioActions } from "@/lib/contexts"

export function App() {
  // The telemetry stream lives here rather than inside a page. Pages mount and
  // unmount with the route, so owning it lower down tore the WebSocket down and
  // discarded all chart history on every navigation, and the header's
  // connection badge needs it everywhere. The library is held here for the same
  // reason: its transcode polling and loaded rows outlive any one page.
  const telemetry = useTelemetryStream()
  const catalog = useMediaCatalog()

  return (
    <ToastProvider>
      <TelemetryContext.Provider value={telemetry}>
        <CatalogContext.Provider value={catalog}>
          <RouterProvider router={router} />
        </CatalogContext.Provider>
      </TelemetryContext.Provider>
    </ToastProvider>
  )
}

/** The root route: the shell plus the upload dialog any page can open. */
function Studio() {
  const [upload, setUpload] = React.useState<{ open: boolean; mode: "file" | "link" }>({ open: false, mode: "file" })
  const actions = React.useMemo<StudioActions>(() => ({ openUpload: (mode = "file") => setUpload({ open: true, mode }) }), [])

  return (
    <StudioActionsContext.Provider value={actions}>
      <AppLayout />
      <UploadDialog
        open={upload.open}
        mode={upload.mode}
        onModeChange={(mode) => setUpload((u) => ({ ...u, mode }))}
        onClose={() => setUpload((u) => ({ ...u, open: false }))}
      />
    </StudioActionsContext.Provider>
  )
}

function LivePage() {
  return (
    <PageBody wide>
      <PageHeader title="Live channels" description="Channels that resolve to a live HLS stream rooms can tune into." className="mb-6" />
      <LiveChannelCenter />
    </PageBody>
  )
}

function RoomsPage() {
  const telemetry = useTelemetry()
  return (
    <PageBody wide>
      <PageHeader title="Rooms" description="Watch parties, who is in them, and what they are playing." className="mb-6" />
      <RoomCommandCenter liveRooms={telemetry.current?.rooms} />
    </PageBody>
  )
}

function TracingPage() {
  return (
    <PageBody wide>
      <PageHeader title="Tracing" className="mb-6" />
      <Card className="border-zinc-800">
        <CardHeader>
          <CardTitle>Live Tracing & Debug Log Streamer</CardTitle>
          <CardDescription>
            Real-time terminal log viewer filtered by Room ID, User ID, and Trace-ID across HTTP, WebSocket, and WebRTC signaling layers.
          </CardDescription>
        </CardHeader>
        <CardContent className="py-12 text-center text-zinc-500 font-mono text-xs">
          <p className="text-emerald-400/80">root@inox-admin:~# tail -f /var/log/inox/telemetry.log</p>
          <p className="mt-2 text-zinc-600">Waiting for log stream connection...</p>
        </CardContent>
      </Card>
    </PageBody>
  )
}

// The portal is mounted under /admin/ on the backend host and at / on Vercel;
// Vite's BASE_URL carries whichever it was built for.
const basename = import.meta.env.BASE_URL.replace(/\/+$/, "") || "/"

const router = createBrowserRouter(
  [
    {
      path: "/",
      element: <Studio />,
      children: [
        { index: true, element: <Navigate to="/dashboard" replace /> },
        { path: "dashboard", element: <DashboardPage /> },
        { path: "content", element: <ContentPage /> },
        {
          path: "content/:assetId",
          element: <VideoLayout />,
          children: [
            { index: true, element: <Navigate to="details" replace /> },
            { path: "details", element: <VideoDetailsPage /> },
            { path: "captions", element: <VideoCaptionsPage /> },
          ],
        },
        { path: "live", element: <LivePage /> },
        { path: "rooms", element: <RoomsPage /> },
        { path: "tracing", element: <TracingPage /> },
        { path: "*", element: <Navigate to="/dashboard" replace /> },
      ],
    },
  ],
  { basename },
)
