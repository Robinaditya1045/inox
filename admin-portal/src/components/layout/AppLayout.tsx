import * as React from "react"
import { Activity, Radio, Video, Terminal, Shield, RefreshCw, Layers } from "lucide-react"
import { Badge } from "@/components/ui/badge"

interface AppLayoutProps {
  children: React.ReactNode
  activeTab: string
  setActiveTab: (tab: string) => void
  isConnected?: boolean
}

export function AppLayout({ children, activeTab, setActiveTab, isConnected = true }: AppLayoutProps) {
  const navItems = [
    { id: "pulse", label: "System Pulse", icon: Activity },
    { id: "media", label: "Media Library", icon: Video },
    { id: "rooms", label: "Room Inspector", icon: Radio },
    { id: "debug", label: "Live Tracing", icon: Terminal },
  ]

  return (
    <div className="min-h-screen bg-[#0a0a0a] text-zinc-100 flex flex-col">
      {/* Top Navigation Bar — Sleek Vercel / Linear Aesthetic */}
      <header className="sticky top-0 z-50 border-b border-zinc-800 bg-[#0a0a0a]/90 backdrop-blur-md px-6 py-3 flex items-center justify-between">
        <div className="flex items-center space-x-4">
          <div className="flex items-center space-x-2">
            <div className="h-8 w-8 rounded-lg bg-white flex items-center justify-center shadow-lg shadow-white/10">
              <Layers className="h-5 w-5 text-black" />
            </div>
            <div>
              <span className="font-bold text-base tracking-tight text-white">INOX</span>
              <span className="text-xs font-mono text-zinc-500 ml-2">v1.0.0-PROD</span>
            </div>
          </div>
          
          <div className="hidden sm:block h-4 w-[1px] bg-zinc-800 mx-2" />
          
          <span className="hidden lg:inline text-sm font-medium text-zinc-400">
            System Analytics & Admin Portal
          </span>
        </div>

        {/* Navigation Tabs — Always Visible */}
        <nav className="flex items-center space-x-1 bg-zinc-900/80 p-1 rounded-lg border border-zinc-800/80 overflow-x-auto max-w-full my-1">
          {navItems.map((item) => {
            const Icon = item.icon
            const isActive = activeTab === item.id
            return (
              <button
                key={item.id}
                onClick={() => setActiveTab(item.id)}
                className={`flex items-center space-x-2 px-3 py-1.5 rounded-md text-sm font-medium transition-all cursor-pointer whitespace-nowrap ${
                  isActive
                    ? "bg-zinc-800 text-white shadow-sm border border-zinc-700/50"
                    : "text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800/30"
                }`}
              >
                <Icon className={`h-4 w-4 ${isActive ? "text-emerald-400" : "text-zinc-500"}`} />
                <span>{item.label}</span>
              </button>
            )
          })}
        </nav>

        {/* Status Indicators & Controls */}
        <div className="flex items-center space-x-4">
          <div className="flex items-center space-x-2 px-3 py-1 rounded-full bg-zinc-900/80 border border-zinc-800">
            <span className={`h-2 w-2 rounded-full ${isConnected ? "bg-emerald-500 animate-pulse" : "bg-rose-500"}`} />
            <span className="text-xs font-mono font-medium text-zinc-300">
              {isConnected ? "TELEMETRY: LIVE" : "DISCONNECTED"}
            </span>
          </div>

          <Badge variant="success" className="hidden sm:inline-flex">
            <Shield className="h-3 w-3 mr-1" />
            RBAC ADMIN
          </Badge>

          <button
            onClick={() => window.location.reload()}
            title="Refresh Portal"
            className="p-2 rounded-md hover:bg-zinc-800 text-zinc-400 hover:text-white transition-colors cursor-pointer"
          >
            <RefreshCw className="h-4 w-4" />
          </button>
        </div>
      </header>

      {/* Main Content Area */}
      <main className="flex-1 max-w-7xl w-full mx-auto p-6">
        {children}
      </main>

      {/* Footer */}
      <footer className="border-t border-zinc-800/80 py-4 px-6 text-center text-xs text-zinc-500 font-mono">
        Inox Real-Time SFU & Watch Party Platform • High-Frequency Telemetry Engine • Built with React 19 & Shadcn UI
      </footer>
    </div>
  )
}
