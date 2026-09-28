import * as React from "react";
import {
  Activity,
  Radio,
  RadioTower,
  Video,
  Terminal,
  Shield,
  RefreshCw,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";

interface AppLayoutProps {
  children: React.ReactNode;
  activeTab: string;
  setActiveTab: (tab: string) => void;
  isConnected?: boolean;
  isDemoMode?: boolean;
}

export function AppLayout({
  children,
  activeTab,
  setActiveTab,
  isConnected = false,
  isDemoMode = false,
}: AppLayoutProps) {
  const navItems = [
    { id: "pulse", label: "System Pulse", icon: Activity },
    { id: "media", label: "Media Library", icon: Video },
    { id: "live", label: "Live Channels", icon: RadioTower },
    { id: "rooms", label: "Room Inspector", icon: Radio },
    { id: "debug", label: "Live Tracing", icon: Terminal },
  ];

  return (
    <div className="min-h-screen bg-[#0a0a0a] text-zinc-100 flex flex-col">
      {/* Top Navigation Bar — Sleek Vercel / Linear Aesthetic. On narrow
          screens the tab strip drops to its own scrollable row. */}
      <header className="sticky top-0 z-50 border-b border-zinc-800 bg-[#0a0a0a]/90 backdrop-blur-md px-4 sm:px-6 py-2 flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
        <div className="flex items-center gap-3 min-w-0">
          <div className="flex items-center gap-2">
            <svg
              width="32"
              height="32"
              viewBox="0 0 32 32"
              aria-hidden="true"
              className="shrink-0"
            >
              <rect width="32" height="32" rx="9" fill="#fafafa" />
              <path
                d="M12.4 10.3c0-1.02 1.1-1.66 1.99-1.15l8.46 4.89a1.33 1.33 0 0 1 0 2.3l-8.46 4.89c-.89.51-1.99-.13-1.99-1.15V10.3Z"
                fill="#0a0a0a"
              />
              <path
                d="M8.5 23.5h15"
                stroke="#0a0a0a"
                strokeOpacity=".55"
                strokeWidth="2"
                strokeLinecap="round"
              />
            </svg>
            <div className="whitespace-nowrap">
              <span className="font-bold text-base tracking-tight text-white">
                INOX
              </span>
              <span className="text-xs font-mono text-zinc-500 ml-2">
                v1.0.0-PROD
              </span>
            </div>
          </div>

          <div className="hidden 2xl:block h-4 w-px bg-zinc-800" />

          <span className="hidden 2xl:inline text-sm font-medium text-zinc-400 whitespace-nowrap">
            System Analytics & Admin Portal
          </span>
        </div>

        {/* Navigation Tabs — Always Visible */}
        <nav
          aria-label="Admin sections"
          className="order-last lg:order-none w-full lg:w-auto flex items-center gap-1 bg-zinc-900/80 p-1 rounded-lg border border-zinc-800/80 overflow-x-auto [scrollbar-width:none]"
        >
          {navItems.map((item) => {
            const Icon = item.icon;
            const isActive = activeTab === item.id;
            return (
              <button
                key={item.id}
                type="button"
                onClick={() => setActiveTab(item.id)}
                aria-current={isActive ? "page" : undefined}
                className={`flex items-center gap-2 px-3 py-1.5 rounded-md text-sm font-medium border cursor-pointer whitespace-nowrap transition-[color,background-color,border-color,transform] duration-150 ease-snappy active:scale-[0.97] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-emerald-400 ${
                  isActive
                    ? "bg-zinc-800 text-white shadow-sm border-zinc-700/50"
                    : "border-transparent text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800/40"
                }`}
              >
                <Icon
                  aria-hidden="true"
                  className={`h-4 w-4 transition-colors duration-150 ${isActive ? "text-emerald-400" : "text-zinc-500"}`}
                />
                <span>{item.label}</span>
              </button>
            );
          })}
        </nav>

        {/* Status Indicators & Controls */}
        <div className="flex items-center gap-2 sm:gap-3">
          <div
            className="flex items-center gap-2 px-3 py-1 rounded-full bg-zinc-900/80 border border-zinc-800"
            role="status"
          >
            <span
              className={`h-2 w-2 rounded-full ${
                isConnected
                  ? "bg-emerald-500 animate-pulse"
                  : isDemoMode
                    ? "bg-amber-500"
                    : "bg-rose-500"
              }`}
              aria-hidden="true"
            />
            <span className="text-xs font-mono font-medium text-zinc-300 whitespace-nowrap">
              {isConnected
                ? "TELEMETRY: LIVE"
                : isDemoMode
                  ? "TELEMETRY: DEMO"
                  : "DISCONNECTED"}
            </span>
          </div>

          <Badge variant="success" className="hidden md:inline-flex">
            <Shield className="h-3 w-3 mr-1" aria-hidden="true" />
            RBAC ADMIN
          </Badge>

          <button
            type="button"
            onClick={() => window.location.reload()}
            title="Refresh Portal"
            aria-label="Refresh portal"
            className="group p-2 rounded-md hover:bg-zinc-800 text-zinc-400 hover:text-white transition-colors duration-150 cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-emerald-400"
          >
            <RefreshCw className="h-4 w-4 transition-transform duration-300 ease-snappy group-hover:rotate-180" />
          </button>
        </div>
      </header>

      {/* Main Content Area */}
      <main className="flex-1 max-w-7xl w-full mx-auto p-4 sm:p-6">
        {children}
      </main>

      {/* Footer */}
      <footer className="border-t border-zinc-800/80 py-4 px-6 text-center text-xs text-zinc-500 font-mono">
        Inox Real-Time SFU & Watch Party Platform • High-Frequency Telemetry
        Engine • Built with React 19 & Shadcn UI
      </footer>
    </div>
  );
}
