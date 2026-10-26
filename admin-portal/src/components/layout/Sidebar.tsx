import { NavLink, useMatch } from "react-router-dom"
import {
  Activity,
  ArrowLeft,
  Captions,
  Clapperboard,
  Film,
  LayoutDashboard,
  Megaphone,
  Radio,
  RadioTower,
  Shield,
  SquarePen,
  Terminal,
  type LucideIcon,
} from "lucide-react"
import { useCatalog } from "@/lib/contexts"
import { formatDuration } from "@/lib/format"
import { cx } from "@/lib/utils"

interface NavItem {
  to: string
  label: string
  icon: LucideIcon
  end?: boolean
}

const MAIN_NAV: NavItem[] = [
  { to: "/dashboard", label: "Dashboard", icon: LayoutDashboard },
  { to: "/content", label: "Content", icon: Clapperboard },
  { to: "/live", label: "Live channels", icon: RadioTower },
  { to: "/rooms", label: "Rooms", icon: Radio },
  { to: "/community", label: "Community", icon: Megaphone },
  { to: "/tracing", label: "Tracing", icon: Terminal },
]

interface SidebarProps {
  /** Desktop rail: icons with small labels instead of the full list */
  collapsed: boolean
  /** Called after following a link, so the mobile drawer can close */
  onNavigate?: () => void
  connection: "live" | "demo" | "offline"
}

export function Sidebar({ collapsed, onNavigate, connection }: SidebarProps) {
  // Inside a video the sidebar becomes that video's own navigation.
  const videoMatch = useMatch("/content/:assetId/*")
  const assetId = videoMatch?.params.assetId

  return (
    <nav aria-label={assetId ? "Video" : "Studio"} className="flex h-full flex-col overflow-y-auto overflow-x-hidden [scrollbar-width:thin]">
      {assetId ? (
        <VideoNav assetId={assetId} collapsed={collapsed} onNavigate={onNavigate} />
      ) : (
        <>
          {!collapsed && <WorkspaceCard connection={connection} />}
          <ul className={cx("flex flex-col gap-0.5", collapsed ? "px-1 pt-2" : "px-3 pt-1")}>
            {MAIN_NAV.map((item) => (
              <li key={item.to}>
                <SidebarLink item={item} collapsed={collapsed} onNavigate={onNavigate} />
              </li>
            ))}
          </ul>
        </>
      )}

      {!collapsed && (
        <div className="mt-auto border-t border-zinc-800/80 px-5 py-4 text-xs text-zinc-500">
          <div className="flex items-center gap-1.5 text-zinc-400">
            <Shield className="h-3.5 w-3.5 text-emerald-400" aria-hidden="true" />
            Admin access
          </div>
          <p className="mt-1 font-mono text-[11px] text-zinc-600">Inox Studio · v1.1</p>
        </div>
      )}
    </nav>
  )
}

function SidebarLink({ item, collapsed, onNavigate }: { item: NavItem; collapsed: boolean; onNavigate?: () => void }) {
  const Icon = item.icon
  return (
    <NavLink
      to={item.to}
      end={item.end}
      onClick={onNavigate}
      title={collapsed ? item.label : undefined}
      className={({ isActive }) =>
        cx(
          "group relative flex items-center rounded-lg transition-[color,background-color] duration-150 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-emerald-400",
          collapsed ? "flex-col gap-1 px-1 py-3 text-[10px] font-medium" : "gap-4 px-3 py-2.5 text-sm font-medium",
          isActive ? "bg-zinc-800/80 text-white" : "text-zinc-400 hover:bg-zinc-800/40 hover:text-zinc-100",
        )
      }
    >
      {({ isActive }) => (
        <>
          {isActive && !collapsed && (
            <span aria-hidden="true" className="absolute left-0 top-1/2 h-5 w-[3px] -translate-y-1/2 rounded-r-full bg-emerald-400" />
          )}
          <Icon
            aria-hidden="true"
            className={cx("h-5 w-5 shrink-0 transition-colors", isActive ? "text-emerald-400" : "text-zinc-500 group-hover:text-zinc-300")}
          />
          <span className={cx("truncate", collapsed && "max-w-full")}>{item.label}</span>
        </>
      )}
    </NavLink>
  )
}

function WorkspaceCard({ connection }: { connection: SidebarProps["connection"] }) {
  return (
    <div className="flex flex-col items-center px-5 pb-4 pt-6 text-center">
      <div
        aria-hidden="true"
        className="flex h-20 w-20 items-center justify-center rounded-full bg-gradient-to-br from-emerald-400 via-emerald-600 to-cyan-700 text-2xl font-bold tracking-tight text-black shadow-lg shadow-emerald-950/50"
      >
        IX
      </div>
      <p className="mt-3 text-sm font-semibold text-white">Inox</p>
      <p className="text-xs text-zinc-500">Admin workspace</p>
      <span
        className={cx(
          "mt-2 inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-[11px] font-medium",
          connection === "live" && "bg-emerald-500/10 text-emerald-400",
          connection === "demo" && "bg-amber-500/10 text-amber-400",
          connection === "offline" && "bg-rose-500/10 text-rose-400",
        )}
      >
        <Activity className="h-3 w-3" aria-hidden="true" />
        {connection === "live" ? "Telemetry live" : connection === "demo" ? "Demo data" : "Disconnected"}
      </span>
    </div>
  )
}

function VideoNav({ assetId, collapsed, onNavigate }: { assetId: string; collapsed: boolean; onNavigate?: () => void }) {
  const { assets } = useCatalog()
  const asset = assets.find((a) => a.id === assetId)
  const base = `/content/${assetId}`
  const items: NavItem[] = [
    { to: `${base}/details`, label: "Details", icon: SquarePen },
    { to: `${base}/captions`, label: "Captions", icon: Captions },
  ]

  return (
    <>
      <div className={collapsed ? "px-1 pt-2" : "px-3 pt-3"}>
        <NavLink
          to="/content"
          onClick={onNavigate}
          title={collapsed ? "Back to content" : undefined}
          className={cx(
            "flex items-center rounded-lg text-zinc-400 transition-colors hover:bg-zinc-800/40 hover:text-white focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-emerald-400",
            collapsed ? "flex-col gap-1 px-1 py-3 text-[10px] font-medium" : "gap-4 px-3 py-2.5 text-sm font-medium",
          )}
        >
          <ArrowLeft className="h-5 w-5" aria-hidden="true" />
          <span>{collapsed ? "Back" : "Library content"}</span>
        </NavLink>
      </div>

      {!collapsed && (
        <div className="px-6 pb-3 pt-3">
          <div className="relative aspect-video overflow-hidden rounded-lg border border-zinc-800 bg-zinc-900">
            {asset?.thumbnail_url ? (
              <img src={asset.thumbnail_url} alt="" className="h-full w-full object-cover" />
            ) : (
              <div className="flex h-full w-full items-center justify-center text-zinc-600">
                <Film className="h-7 w-7" aria-hidden="true" />
              </div>
            )}
            {formatDuration(asset?.duration_seconds) && (
              <span className="absolute bottom-1.5 right-1.5 rounded bg-black/80 px-1.5 py-0.5 font-mono text-[11px] font-medium text-white">
                {formatDuration(asset?.duration_seconds)}
              </span>
            )}
          </div>
          <p className="mt-3 text-xs font-medium text-zinc-500">Your video</p>
          <p className="mt-0.5 line-clamp-2 text-sm font-medium text-zinc-100">{asset?.title ?? "Loading…"}</p>
        </div>
      )}

      <ul className={cx("flex flex-col gap-0.5", collapsed ? "px-1" : "px-3")}>
        {items.map((item) => (
          <li key={item.to}>
            <SidebarLink item={item} collapsed={collapsed} onNavigate={onNavigate} />
          </li>
        ))}
      </ul>
    </>
  )
}
