import * as React from "react"
import { Outlet, useLocation, useNavigate, useSearchParams } from "react-router-dom"
import { Link2, Menu as MenuIcon, Plus, RadioTower, RefreshCw, Search, UploadCloud, X } from "lucide-react"
import { Sidebar } from "@/components/layout/Sidebar"
import { IconButton } from "@/components/ui/icon-button"
import { Menu } from "@/components/ui/menu"
import { useStudioActions, useTelemetry } from "@/lib/contexts"
import { cx } from "@/lib/utils"

const SIDEBAR_KEY = "inox_admin_sidebar"

function readCollapsed(): boolean {
  try {
    return localStorage.getItem(SIDEBAR_KEY) === "collapsed"
  } catch {
    return false
  }
}

/**
 * The studio shell: a top bar with search and "Create", and a left sidebar that
 * collapses to a rail on desktop and becomes a drawer below the lg breakpoint.
 */
export function AppLayout() {
  const telemetry = useTelemetry()
  const { openUpload } = useStudioActions()
  const navigate = useNavigate()
  const location = useLocation()
  const [params] = useSearchParams()

  const [collapsed, setCollapsed] = React.useState(readCollapsed)
  const [drawerOpen, setDrawerOpen] = React.useState(false)

  const connection = telemetry.isConnected ? "live" : telemetry.isDemoMode ? "demo" : "offline"

  const toggleSidebar = () => {
    if (window.matchMedia("(min-width: 1024px)").matches) {
      setCollapsed((c) => {
        try {
          localStorage.setItem(SIDEBAR_KEY, c ? "expanded" : "collapsed")
        } catch {
          /* storage unavailable: the choice just is not remembered */
        }
        return !c
      })
    } else {
      setDrawerOpen((o) => !o)
    }
  }

  React.useEffect(() => {
    if (!drawerOpen) return
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && setDrawerOpen(false)
    document.addEventListener("keydown", onKey)
    return () => document.removeEventListener("keydown", onKey)
  }, [drawerOpen])

  // The top bar's search is the library search: it lands on Content, filtered.
  const onContent = location.pathname === "/content"
  const [query, setQuery] = React.useState(() => (onContent ? params.get("q") ?? "" : ""))
  const submitSearch = (e: React.FormEvent) => {
    e.preventDefault()
    const q = query.trim()
    navigate(q ? `/content?q=${encodeURIComponent(q)}` : "/content")
  }

  return (
    <div className="min-h-screen bg-canvas text-zinc-100">
      <header className="sticky top-0 z-40 flex h-14 items-center gap-2 border-b border-zinc-800/80 bg-canvas/95 px-2 backdrop-blur-md sm:gap-4 sm:px-4">
        <IconButton label="Main menu" tooltip="bottom" onClick={toggleSidebar}>
          <MenuIcon className="h-5 w-5" />
        </IconButton>

        <a href={import.meta.env.BASE_URL} onClick={(e) => { e.preventDefault(); navigate("/dashboard") }} className="flex shrink-0 items-center gap-2 rounded-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-emerald-400">
          <svg width="28" height="28" viewBox="0 0 32 32" aria-hidden="true" className="shrink-0">
            <rect width="32" height="32" rx="9" fill="#fafafa" />
            <path d="M12.4 10.3c0-1.02 1.1-1.66 1.99-1.15l8.46 4.89a1.33 1.33 0 0 1 0 2.3l-8.46 4.89c-.89.51-1.99-.13-1.99-1.15V10.3Z" fill="#0a0a0a" />
            <path d="M8.5 23.5h15" stroke="#0a0a0a" strokeOpacity=".55" strokeWidth="2" strokeLinecap="round" />
          </svg>
          <span className="hidden text-[17px] font-semibold tracking-tight text-white sm:inline">
            Inox <span className="font-normal text-zinc-400">Studio</span>
          </span>
        </a>

        <form role="search" onSubmit={submitSearch} className="mx-auto hidden w-full max-w-xl md:block">
          <label className="group flex h-10 items-center gap-3 rounded-lg border border-zinc-800 bg-zinc-900/60 px-3 transition-colors focus-within:border-zinc-600 focus-within:bg-zinc-900">
            <Search className="h-4 w-4 shrink-0 text-zinc-500" aria-hidden="true" />
            <span className="sr-only">Search the library</span>
            <input
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Search across your library"
              className="h-full w-full bg-transparent text-sm text-zinc-100 placeholder:text-zinc-500 focus:outline-none"
            />
            {query && (
              <button type="button" onClick={() => setQuery("")} aria-label="Clear search" className="rounded p-0.5 text-zinc-500 hover:text-white cursor-pointer">
                <X className="h-4 w-4" />
              </button>
            )}
          </label>
        </form>

        <div className="ml-auto flex items-center gap-1 sm:gap-2 md:ml-0">
          <IconButton label="Search" tooltip="bottom" wrapperClassName="md:hidden" onClick={() => navigate("/content")}>
            <Search className="h-5 w-5" />
          </IconButton>

          <span
            role="status"
            className={cx(
              "hidden items-center gap-2 rounded-full border px-3 py-1 text-xs font-medium sm:inline-flex",
              connection === "live" && "border-emerald-500/20 bg-emerald-500/5 text-emerald-300",
              connection === "demo" && "border-amber-500/20 bg-amber-500/5 text-amber-300",
              connection === "offline" && "border-rose-500/20 bg-rose-500/5 text-rose-300",
            )}
          >
            <span
              aria-hidden="true"
              className={cx(
                "h-1.5 w-1.5 rounded-full",
                connection === "live" && "bg-emerald-400 animate-pulse",
                connection === "demo" && "bg-amber-400",
                connection === "offline" && "bg-rose-400",
              )}
            />
            {connection === "live" ? "Live" : connection === "demo" ? "Demo" : "Offline"}
          </span>

          <Menu
            label="Create"
            minWidth={220}
            items={[
              { label: "Upload video", icon: UploadCloud, onSelect: () => openUpload("file") },
              { label: "Import from link", icon: Link2, onSelect: () => openUpload("link") },
              { label: "Add live channel", icon: RadioTower, onSelect: () => navigate("/live"), separated: true },
            ]}
            trigger={(props) => (
              <button
                {...props}
                type="button"
                className="inline-flex h-9 items-center gap-2 rounded-lg border border-zinc-700 bg-zinc-900 px-3 text-sm font-medium text-zinc-100 transition-colors hover:border-zinc-600 hover:bg-zinc-800 cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-emerald-400"
              >
                <Plus className="h-4 w-4 text-emerald-400" aria-hidden="true" />
                <span className="hidden sm:inline">Create</span>
              </button>
            )}
          />

          <IconButton label="Reload portal" tooltip="bottom" onClick={() => window.location.reload()} wrapperClassName="hidden sm:inline-flex">
            <RefreshCw className="h-[18px] w-[18px]" />
          </IconButton>
        </div>
      </header>

      {/* Desktop sidebar */}
      <aside
        className={cx(
          "fixed bottom-0 left-0 top-14 z-30 hidden border-r border-zinc-800/80 bg-canvas transition-[width] duration-200 ease-snappy lg:block",
          collapsed ? "w-[76px]" : "w-64",
        )}
      >
        <Sidebar collapsed={collapsed} connection={connection} />
      </aside>

      {/* Mobile drawer */}
      {drawerOpen && (
        <div className="fixed inset-0 z-50 lg:hidden">
          <div className="absolute inset-0 bg-black/70 backdrop-blur-sm animate-overlay-in" onClick={() => setDrawerOpen(false)} />
          <aside className="absolute bottom-0 left-0 top-0 w-72 max-w-[85vw] border-r border-zinc-800 bg-canvas shadow-2xl animate-enter">
            <div className="flex h-14 items-center gap-2 border-b border-zinc-800/80 px-2">
              <IconButton label="Close menu" tooltip="none" onClick={() => setDrawerOpen(false)}>
                <X className="h-5 w-5" />
              </IconButton>
              <span className="text-[17px] font-semibold tracking-tight text-white">
                Inox <span className="font-normal text-zinc-400">Studio</span>
              </span>
            </div>
            <div className="h-[calc(100%-3.5rem)]">
              <Sidebar collapsed={false} connection={connection} onNavigate={() => setDrawerOpen(false)} />
            </div>
          </aside>
        </div>
      )}

      <main className={cx("min-w-0 transition-[padding] duration-200 ease-snappy", collapsed ? "lg:pl-[76px]" : "lg:pl-64")}>
        <Outlet />
      </main>
    </div>
  )
}
