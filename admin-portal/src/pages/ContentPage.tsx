import * as React from "react"
import { Link, useNavigate, useSearchParams } from "react-router-dom"
import {
  ArrowDown,
  ArrowUp,
  Captions,
  ChevronLeft,
  ChevronRight,
  Copy,
  EllipsisVertical,
  ExternalLink,
  Filter,
  Link2,
  Play,
  RefreshCw,
  SquarePen,
  Trash2,
  TriangleAlert,
  UploadCloud,
  X,
} from "lucide-react"
import type { MediaAsset } from "@/types/media"
import { PageBody, PageHeader } from "@/components/layout/PageHeader"
import { Thumb } from "@/components/content/Thumb"
import { StatusBadge } from "@/components/content/StatusBadge"
import { FilterChip } from "@/components/content/FilterChip"
import { PreviewDialog } from "@/components/content/PreviewDialog"
import { IconButton } from "@/components/ui/icon-button"
import { Menu } from "@/components/ui/menu"
import { ConfirmDialog } from "@/components/ui/confirm-dialog"
import { Button } from "@/components/ui/button"
import { useCatalog, useStudioActions } from "@/lib/contexts"
import { useToast } from "@/hooks/useToast"
import { errorMessage } from "@/lib/api"
import { formatDate } from "@/lib/format"
import { mediaOrigin, playbackUrl } from "@/lib/media"
import { copyText, cx } from "@/lib/utils"

type Tab = "videos" | "live"
type StatusFilter = "all" | "ready" | "processing" | "failed"
type CaptionFilter = "all" | "with" | "without"
type Sort = "date-desc" | "date-asc" | "title-asc" | "title-desc"

const ROWS_KEY = "inox_admin_rows_per_page"
const ROW_OPTIONS = [10, 30, 50] as const

function readRowsPerPage(): number {
  try {
    const n = Number(localStorage.getItem(ROWS_KEY))
    return ROW_OPTIONS.includes(n as (typeof ROW_OPTIONS)[number]) ? n : 10
  } catch {
    return 10
  }
}

function matchesStatus(asset: MediaAsset, status: StatusFilter): boolean {
  if (status === "all") return true
  if (status === "processing") return asset.status === "processing" || asset.status === "pending"
  return asset.status === status
}

export function ContentPage() {
  const { assets, isLoading, isDemoMode, error, deleteMedia, refresh } = useCatalog()
  const { openUpload } = useStudioActions()
  const { toast } = useToast()
  const navigate = useNavigate()
  const [params, setParams] = useSearchParams()

  const tab: Tab = params.get("tab") === "live" ? "live" : "videos"
  const query = params.get("q") ?? ""
  const status = (params.get("status") as StatusFilter) || "all"
  const captionFilter = (params.get("captions") as CaptionFilter) || "all"
  const sort = (params.get("sort") as Sort) || "date-desc"

  const setParam = (key: string, value: string, neutral: string) => {
    const next = new URLSearchParams(params)
    if (!value || value === neutral) next.delete(key)
    else next.set(key, value)
    setParams(next, { replace: true })
    setPage(0)
  }

  const [page, setPage] = React.useState(0)
  const [rowsPerPage, setRowsPerPage] = React.useState(readRowsPerPage)
  const [selected, setSelected] = React.useState<Set<string>>(() => new Set())
  const [preview, setPreview] = React.useState<MediaAsset | null>(null)
  const [pendingDelete, setPendingDelete] = React.useState<MediaAsset[] | null>(null)

  const counts = React.useMemo(
    () => ({
      videos: assets.filter((a) => a.kind !== "live").length,
      live: assets.filter((a) => a.kind === "live").length,
    }),
    [assets],
  )

  const rows = React.useMemo(() => {
    const q = query.trim().toLowerCase()
    const filtered = assets.filter((a) => {
      if ((tab === "live") !== (a.kind === "live")) return false
      if (!matchesStatus(a, status)) return false
      const tracks = a.captions?.length ?? 0
      if (captionFilter === "with" && tracks === 0) return false
      if (captionFilter === "without" && tracks > 0) return false
      if (!q) return true
      // The API omits or nulls description on some rows; coerce before matching.
      return (
        (a.title ?? "").toLowerCase().includes(q) ||
        (a.description ?? "").toLowerCase().includes(q) ||
        a.id.toLowerCase().includes(q)
      )
    })
    const byDate = (a: MediaAsset) => new Date(a.created_at).getTime() || 0
    return filtered.sort((a, b) => {
      switch (sort) {
        case "date-asc":
          return byDate(a) - byDate(b)
        case "title-asc":
          return (a.title ?? "").localeCompare(b.title ?? "")
        case "title-desc":
          return (b.title ?? "").localeCompare(a.title ?? "")
        default:
          return byDate(b) - byDate(a)
      }
    })
  }, [assets, tab, status, captionFilter, query, sort])

  const pageCount = Math.max(1, Math.ceil(rows.length / rowsPerPage))
  const currentPage = Math.min(page, pageCount - 1)
  const pageRows = rows.slice(currentPage * rowsPerPage, (currentPage + 1) * rowsPerPage)

  // Selection only ever names rows that still exist and are on screen.
  const visibleSelected = pageRows.filter((a) => selected.has(a.id))
  const allOnPage = pageRows.length > 0 && visibleSelected.length === pageRows.length
  const headerCheckbox = React.useRef<HTMLInputElement>(null)
  React.useEffect(() => {
    if (headerCheckbox.current) {
      headerCheckbox.current.indeterminate = visibleSelected.length > 0 && !allOnPage
    }
  }, [visibleSelected.length, allOnPage])

  const toggleRow = (id: string) =>
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })

  const togglePage = () =>
    setSelected((prev) => {
      const next = new Set(prev)
      if (allOnPage) pageRows.forEach((a) => next.delete(a.id))
      else pageRows.forEach((a) => next.add(a.id))
      return next
    })

  const filtersActive = query !== "" || status !== "all" || captionFilter !== "all"
  const clearFilters = () => {
    const next = new URLSearchParams()
    if (tab === "live") next.set("tab", "live")
    setParams(next, { replace: true })
    setPage(0)
  }

  const copy = async (text: string, what: string) => {
    toast((await copyText(text)) ? `${what} copied` : "Could not copy to the clipboard", {
      tone: "default",
    })
  }

  const runDelete = async (targets: MediaAsset[]) => {
    let failed = 0
    for (const asset of targets) {
      try {
        await deleteMedia(asset.id)
      } catch (err) {
        failed++
        toast(`Could not delete "${asset.title}": ${errorMessage(err)}`, { tone: "error" })
      }
    }
    setSelected(new Set())
    const done = targets.length - failed
    if (done > 0) toast(done === 1 ? "Video deleted" : `${done} videos deleted`, { tone: "success" })
  }

  const sortHeader = (label: string, asc: Sort, desc: Sort, className?: string) => {
    const active = sort === asc || sort === desc
    return (
      <th scope="col" aria-sort={active ? (sort === asc ? "ascending" : "descending") : "none"} className={className}>
        <button
          type="button"
          onClick={() => setParam("sort", sort === desc ? asc : desc, "date-desc")}
          className={cx("inline-flex items-center gap-1 rounded hover:text-white cursor-pointer", active && "text-zinc-200")}
        >
          {label}
          {active && (sort === asc ? <ArrowUp className="h-3.5 w-3.5" aria-hidden="true" /> : <ArrowDown className="h-3.5 w-3.5" aria-hidden="true" />)}
        </button>
      </th>
    )
  }

  return (
    <PageBody wide>
      <PageHeader
        title="Library content"
        description="Everything rooms can play. Edit details, thumbnails and captions for each video."
        actions={
          <>
            <Button variant="outline" size="sm" onClick={() => refresh()}>
              <RefreshCw className="mr-1.5 h-3.5 w-3.5" aria-hidden="true" /> Refresh
            </Button>
            <Button variant="outline" size="sm" onClick={() => openUpload("link")}>
              <Link2 className="mr-1.5 h-3.5 w-3.5" aria-hidden="true" /> Import link
            </Button>
            <Button variant="emerald" size="sm" onClick={() => openUpload("file")}>
              <UploadCloud className="mr-1.5 h-3.5 w-3.5" aria-hidden="true" /> Upload video
            </Button>
          </>
        }
      />

      {isDemoMode && (
        <div role="status" className="mt-5 flex flex-wrap items-center gap-3 rounded-lg border border-amber-500/25 bg-amber-500/5 px-4 py-3 text-sm text-amber-200">
          <TriangleAlert className="h-4 w-4 shrink-0 text-amber-400" aria-hidden="true" />
          <span className="flex-1">{error ?? "Showing the demo catalog."} Changes made here are not saved.</span>
          <Button variant="outline" size="sm" onClick={() => refresh()}>
            Retry
          </Button>
        </div>
      )}

      {/* Tabs */}
      <div role="tablist" aria-label="Content type" className="mt-6 flex gap-6 border-b border-zinc-800">
        {(
          [
            ["videos", "Videos", counts.videos],
            ["live", "Live", counts.live],
          ] as const
        ).map(([value, label, count]) => (
          <button
            key={value}
            role="tab"
            type="button"
            aria-selected={tab === value}
            onClick={() => {
              const next = new URLSearchParams()
              if (value === "live") next.set("tab", "live")
              setParams(next, { replace: true })
              setPage(0)
              setSelected(new Set())
            }}
            className={cx(
              "relative -mb-px flex items-center gap-2 border-b-2 px-1 pb-3 pt-1 text-sm font-medium transition-colors cursor-pointer",
              tab === value ? "border-white text-white" : "border-transparent text-zinc-400 hover:text-zinc-200",
            )}
          >
            {label}
            <span className="rounded-full bg-zinc-800 px-1.5 py-px text-[11px] text-zinc-400">{count}</span>
          </button>
        ))}
      </div>

      {/* Filter bar, or the bulk-action bar while rows are selected */}
      <div className="flex min-h-14 flex-wrap items-center gap-2 border-b border-zinc-800 py-2.5">
        {visibleSelected.length > 0 ? (
          <div className="flex w-full items-center gap-3 rounded-lg bg-zinc-800/70 px-3 py-1.5 animate-enter">
            <span className="text-sm font-medium text-white">{visibleSelected.length} selected</span>
            <span className="h-5 w-px bg-zinc-700" aria-hidden="true" />
            <Button variant="ghost" size="sm" onClick={() => setPendingDelete(visibleSelected)} className="text-rose-300 hover:text-rose-200">
              <Trash2 className="mr-1.5 h-3.5 w-3.5" aria-hidden="true" /> Delete forever
            </Button>
            <IconButton label="Clear selection" size="sm" wrapperClassName="ml-auto" onClick={() => setSelected(new Set())}>
              <X className="h-4 w-4" />
            </IconButton>
          </div>
        ) : (
          <>
            <label className="flex min-w-[220px] flex-1 items-center gap-2 text-sm">
              <Filter className="h-4 w-4 shrink-0 text-zinc-500" aria-hidden="true" />
              <span className="sr-only">Filter by title, description or ID</span>
              <input
                value={query}
                onChange={(e) => setParam("q", e.target.value, "")}
                placeholder="Filter by title, description or ID"
                className="h-9 w-full bg-transparent text-zinc-100 placeholder:text-zinc-500 focus:outline-none"
              />
            </label>
            <FilterChip
              label="Status"
              value={status}
              neutral="all"
              onChange={(v) => setParam("status", v, "all")}
              options={[
                { value: "all", label: "Any" },
                { value: "ready", label: "Ready" },
                { value: "processing", label: "Processing" },
                { value: "failed", label: "Failed" },
              ]}
            />
            {tab === "videos" && (
              <FilterChip
                label="Captions"
                value={captionFilter}
                neutral="all"
                onChange={(v) => setParam("captions", v, "all")}
                options={[
                  { value: "all", label: "Any" },
                  { value: "with", label: "Has captions" },
                  { value: "without", label: "No captions" },
                ]}
              />
            )}
            {filtersActive && (
              <Button variant="ghost" size="sm" onClick={clearFilters}>
                Clear
              </Button>
            )}
          </>
        )}
      </div>

      {/* Table. relative, so anything absolutely positioned inside is clipped
          by this scroller rather than widening the page on phones. */}
      <div className="relative overflow-x-auto">
        <table className="w-full min-w-[920px] border-collapse text-left">
          <thead>
            <tr className="border-b border-zinc-800 text-xs font-medium text-zinc-400 [&>th]:py-3 [&>th]:font-medium">
              <th scope="col" className="w-12 pl-1">
                <input
                  ref={headerCheckbox}
                  type="checkbox"
                  aria-label="Select all on this page"
                  checked={allOnPage}
                  onChange={togglePage}
                  disabled={pageRows.length === 0}
                  className="h-4 w-4 accent-emerald-500 cursor-pointer"
                />
              </th>
              {sortHeader(tab === "live" ? "Channel" : "Video", "title-asc", "title-desc", "min-w-[380px] pr-4")}
              <th scope="col" className="w-40 pr-4">Status</th>
              {tab === "videos" && <th scope="col" className="w-40 pr-4">Captions</th>}
              <th scope="col" className="w-44 pr-4">Source</th>
              {sortHeader("Date", "date-asc", "date-desc", "w-36 pr-4")}
            </tr>
          </thead>
          <tbody>
            {isLoading ? (
              Array.from({ length: 5 }, (_, i) => (
                <tr key={i} className="border-b border-zinc-800/70" aria-hidden="true">
                  <td className="py-3 pl-1"><div className="h-4 w-4 rounded bg-zinc-800" /></td>
                  <td className="py-3 pr-4">
                    <div className="flex items-center gap-4">
                      <div className="aspect-video w-[120px] animate-pulse rounded-md bg-zinc-800/80" />
                      <div className="flex-1 space-y-2">
                        <div className="h-3.5 w-2/3 animate-pulse rounded bg-zinc-800/80" />
                        <div className="h-3 w-1/2 animate-pulse rounded bg-zinc-800/60" />
                      </div>
                    </div>
                  </td>
                  <td colSpan={4}><div className="h-3 w-3/4 animate-pulse rounded bg-zinc-800/60" /></td>
                </tr>
              ))
            ) : pageRows.length === 0 ? (
              <tr>
                <td colSpan={6} className="py-16">
                  <EmptyContent
                    filtered={filtersActive}
                    live={tab === "live"}
                    onClear={clearFilters}
                    onUpload={() => openUpload("file")}
                    onLive={() => navigate("/live")}
                  />
                </td>
              </tr>
            ) : (
              pageRows.map((asset) => (
                <ContentRow
                  key={asset.id}
                  asset={asset}
                  showCaptions={tab === "videos"}
                  selected={selected.has(asset.id)}
                  onToggle={() => toggleRow(asset.id)}
                  onPreview={() => setPreview(asset)}
                  onCopy={copy}
                  onDelete={() => setPendingDelete([asset])}
                />
              ))
            )}
          </tbody>
        </table>
      </div>

      {/* Pagination */}
      {!isLoading && rows.length > 0 && (
        <div className="flex flex-wrap items-center justify-end gap-x-6 gap-y-2 py-3 text-sm text-zinc-400">
          <label className="flex items-center gap-2">
            Rows per page:
            <select
              value={rowsPerPage}
              onChange={(e) => {
                const n = Number(e.target.value)
                setRowsPerPage(n)
                setPage(0)
                try {
                  localStorage.setItem(ROWS_KEY, String(n))
                } catch {
                  /* not remembered */
                }
              }}
              className="h-8 rounded-md border border-zinc-700 bg-zinc-900 px-2 text-zinc-100 cursor-pointer"
            >
              {ROW_OPTIONS.map((n) => (
                <option key={n} value={n}>
                  {n}
                </option>
              ))}
            </select>
          </label>
          <span className="tabular-nums">
            {currentPage * rowsPerPage + 1}–{Math.min(rows.length, (currentPage + 1) * rowsPerPage)} of {rows.length}
          </span>
          <div className="flex items-center gap-1">
            <IconButton label="Previous page" size="sm" disabled={currentPage === 0} onClick={() => setPage(currentPage - 1)}>
              <ChevronLeft className="h-4 w-4" />
            </IconButton>
            <IconButton label="Next page" size="sm" disabled={currentPage >= pageCount - 1} onClick={() => setPage(currentPage + 1)}>
              <ChevronRight className="h-4 w-4" />
            </IconButton>
          </div>
        </div>
      )}

      <PreviewDialog asset={preview} onClose={() => setPreview(null)} />

      <ConfirmDialog
        open={!!pendingDelete}
        destructive
        title={pendingDelete && pendingDelete.length > 1 ? `Permanently delete ${pendingDelete.length} videos?` : "Permanently delete this video?"}
        description={
          pendingDelete && pendingDelete.length === 1
            ? `"${pendingDelete[0].title}" will be removed from the library along with its captions. Rooms playing it will stop being able to load it.`
            : "These videos will be removed from the library along with their captions. Rooms playing them will stop being able to load them."
        }
        acknowledgement="I understand that deleting is permanent and can't be undone"
        confirmLabel="Delete forever"
        onConfirm={() => runDelete(pendingDelete ?? [])}
        onClose={() => setPendingDelete(null)}
      />
    </PageBody>
  )
}

interface ContentRowProps {
  asset: MediaAsset
  showCaptions: boolean
  selected: boolean
  onToggle: () => void
  onPreview: () => void
  onCopy: (text: string, what: string) => void
  onDelete: () => void
}

function ContentRow({ asset, showCaptions, selected, onToggle, onPreview, onCopy, onDelete }: ContentRowProps) {
  const navigate = useNavigate()
  const details = `/content/${asset.id}/details`
  const origin = mediaOrigin(asset)
  const captions = asset.captions ?? []

  return (
    <tr className={cx("group border-b border-zinc-800/70 transition-colors", selected ? "bg-emerald-500/[0.04]" : "hover:bg-zinc-900/70")}>
      <td className="py-3 pl-1 align-top">
        <input
          type="checkbox"
          aria-label={`Select ${asset.title}`}
          checked={selected}
          onChange={onToggle}
          className="mt-1 h-4 w-4 accent-emerald-500 cursor-pointer"
        />
      </td>
      <td className="py-3 pr-4 align-top">
        <div className="flex items-start gap-4">
          <Link to={details} tabIndex={-1} aria-hidden="true" className="w-[120px] shrink-0">
            <Thumb asset={asset} />
          </Link>
          <div className="min-w-0 flex-1">
            <Link to={details} className="line-clamp-1 text-sm font-medium text-zinc-100 hover:underline focus-visible:underline focus-visible:outline-none">
              {asset.title}
            </Link>
            <p className="mt-1 line-clamp-2 text-xs leading-relaxed text-zinc-500 group-hover:hidden group-focus-within:hidden [@media(hover:none)]:hidden">
              {asset.description || "No description"}
            </p>
            <div className="-ml-1.5 mt-0.5 hidden items-center group-hover:flex group-focus-within:flex [@media(hover:none)]:flex">
              <IconButton label="Details" size="sm" onClick={() => navigate(details)}>
                <SquarePen className="h-4 w-4" />
              </IconButton>
              {asset.kind !== "live" && (
                <IconButton label="Captions" size="sm" onClick={() => navigate(`/content/${asset.id}/captions`)}>
                  <Captions className="h-4 w-4" />
                </IconButton>
              )}
              <IconButton label="Preview" size="sm" onClick={onPreview} disabled={!playbackUrl(asset)}>
                <Play className="h-4 w-4" />
              </IconButton>
              <Menu
                label={`Options for ${asset.title}`}
                items={[
                  { label: "Copy video link", icon: Link2, onSelect: () => onCopy(playbackUrl(asset), "Video link") },
                  { label: "Copy asset ID", icon: Copy, onSelect: () => onCopy(asset.id, "Asset ID") },
                  { label: "Open stream in new tab", icon: ExternalLink, onSelect: () => window.open(playbackUrl(asset), "_blank", "noopener") },
                  { label: "Delete forever", icon: Trash2, destructive: true, separated: true, onSelect: onDelete },
                ]}
                trigger={(props) => (
                  <IconButton {...props} label="Options" size="sm">
                    <EllipsisVertical className="h-4 w-4" />
                  </IconButton>
                )}
              />
            </div>
          </div>
        </div>
      </td>
      <td className="py-3 pr-4 align-top">
        <StatusBadge asset={asset} />
      </td>
      {showCaptions && (
        <td className="py-3 pr-4 align-top">
          {captions.length === 0 ? (
            <Link to={`/content/${asset.id}/captions`} className="text-sm text-zinc-500 hover:text-emerald-300">
              None
            </Link>
          ) : (
            <span className="flex flex-wrap gap-1" title={captions.map((c) => c.label).join(", ")}>
              {captions.slice(0, 3).map((c) => (
                <span
                  key={c.id}
                  className={cx(
                    "rounded border px-1.5 py-px font-mono text-[11px] uppercase",
                    c.is_default ? "border-emerald-500/40 text-emerald-300" : "border-zinc-700 text-zinc-300",
                  )}
                >
                  {c.language}
                </span>
              ))}
              {captions.length > 3 && <span className="text-xs text-zinc-500">+{captions.length - 3}</span>}
            </span>
          )}
        </td>
      )}
      <td className="py-3 pr-4 align-top">
        <span className="block truncate text-sm text-zinc-300">{origin.label}</span>
        {origin.origin === "upload" && <span className="text-xs text-zinc-500">Transcoded to HLS</span>}
        {origin.origin === "link" && <span className="text-xs text-zinc-500">External link</span>}
      </td>
      <td className="py-3 pr-4 align-top">
        <span className="block text-sm text-zinc-300">{formatDate(asset.created_at)}</span>
        <span className="text-xs text-zinc-500">{origin.origin === "upload" ? "Uploaded" : "Added"}</span>
      </td>
    </tr>
  )
}

function EmptyContent({
  filtered,
  live,
  onClear,
  onUpload,
  onLive,
}: {
  filtered: boolean
  live: boolean
  onClear: () => void
  onUpload: () => void
  onLive: () => void
}) {
  if (filtered) {
    return (
      <div className="flex flex-col items-center text-center">
        <p className="text-sm font-medium text-zinc-200">No content matches these filters</p>
        <Button variant="outline" size="sm" onClick={onClear} className="mt-3">
          Clear filters
        </Button>
      </div>
    )
  }
  return (
    <div className="flex flex-col items-center text-center">
      <div className="flex h-24 w-24 items-center justify-center rounded-full bg-zinc-900 ring-1 ring-zinc-800">
        <UploadCloud className="h-10 w-10 text-zinc-600" aria-hidden="true" />
      </div>
      <p className="mt-4 text-sm font-medium text-zinc-200">{live ? "No live channels yet" : "No content available"}</p>
      <p className="mt-1 max-w-sm text-sm text-zinc-500">
        {live ? "Add a live channel and it will show up here for editing." : "Upload a video or import one by link, and rooms will be able to play it."}
      </p>
      <Button variant="emerald" size="sm" onClick={live ? onLive : onUpload} className="mt-4">
        {live ? "Add live channel" : "Upload video"}
      </Button>
    </div>
  )
}
