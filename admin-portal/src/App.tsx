import * as React from "react"
import { AppLayout } from "@/components/layout/AppLayout"
import { SystemPulseDashboard } from "@/components/dashboard/SystemPulseDashboard"
import { MediaCommandCenter } from "@/components/media/MediaCommandCenter"
import { RoomCommandCenter } from "@/components/rooms/RoomCommandCenter"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"

export function App() {
  const [activeTab, setActiveTab] = React.useState("pulse")

  return (
    <AppLayout activeTab={activeTab} setActiveTab={setActiveTab} isConnected={true}>
      {activeTab === "pulse" && <SystemPulseDashboard />}

      {activeTab === "media" && <MediaCommandCenter />}

      {activeTab === "rooms" && <RoomCommandCenter />}

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
