import * as React from "react"
import { AppLayout } from "@/components/layout/AppLayout"
import { SystemPulseDashboard } from "@/components/dashboard/SystemPulseDashboard"
import { MediaCommandCenter } from "@/components/media/MediaCommandCenter"
import { RoomCommandCenter } from "@/components/rooms/RoomCommandCenter"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { useTelemetryStream } from "@/hooks/useTelemetryStream"

export function App() {
  const [activeTab, setActiveTab] = React.useState("pulse")

  // The telemetry stream lives here rather than inside SystemPulseDashboard.
  // Tabs are conditionally rendered, so owning it in the dashboard tore the
  // WebSocket down and discarded all chart history on every tab switch, and
  // left the header's connection badge hardcoded to "LIVE".
  const telemetry = useTelemetryStream()

  return (
    <AppLayout
      activeTab={activeTab}
      setActiveTab={setActiveTab}
      isConnected={telemetry.isConnected}
      isDemoMode={telemetry.isDemoMode}
    >
      {activeTab === "pulse" && <SystemPulseDashboard telemetry={telemetry} />}

      {activeTab === "media" && <MediaCommandCenter />}

      {activeTab === "rooms" && <RoomCommandCenter liveRooms={telemetry.current?.rooms} />}

      {activeTab === "debug" && (
        <Card className="border-zinc-800">
          <CardHeader>
            <CardTitle>Live Tracing & Debug Log Streamer</CardTitle>
            <CardDescription>Real-time terminal log viewer filtered by Room ID, User ID, and Trace-ID across HTTP, WebSocket, and WebRTC signaling layers.</CardDescription>
          </CardHeader>
          <CardContent className="py-12 text-center text-zinc-500 font-mono text-xs">
            <p className="text-emerald-400/80">root@inox-admin:~# tail -f /var/log/inox/telemetry.log</p>
            <p className="mt-2 text-zinc-600">Waiting for log stream connection...</p>
          </CardContent>
        </Card>
      )}
    </AppLayout>
  )
}
