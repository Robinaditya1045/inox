import * as React from "react"
import type { MediaAsset, MediaStatus } from "@/types/media"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Play, Trash2, Search, Film, CheckCircle2, Loader2, AlertCircle, Clock, Layers } from "lucide-react"

interface MediaCatalogTableProps {
  assets: MediaAsset[]
  isLoading: boolean
  onSelectAsset: (asset: MediaAsset) => void
  onDeleteAsset: (id: string) => Promise<void>
}

export function MediaCatalogTable({
  assets,
  isLoading,
  onSelectAsset,
  onDeleteAsset,
}: MediaCatalogTableProps) {
  const [searchQuery, setSearchQuery] = React.useState("")
  const [statusFilter, setStatusFilter] = React.useState<string>("all")
  const [deletingId, setDeletingId] = React.useState<string | null>(null)

  const filteredAssets = React.useMemo(() => {
    return assets.filter(asset => {
      const matchesSearch =
        asset.title.toLowerCase().includes(searchQuery.toLowerCase()) ||
        asset.description.toLowerCase().includes(searchQuery.toLowerCase()) ||
        asset.id.toLowerCase().includes(searchQuery.toLowerCase())

      const matchesStatus = statusFilter === "all" || asset.status === statusFilter
      return matchesSearch && matchesStatus
    })
  }, [assets, searchQuery, statusFilter])

  const handleDelete = async (e: React.MouseEvent, id: string, title: string) => {
    e.stopPropagation()
    if (!window.confirm(`Are you sure you want to delete "${title}" and purge all HLS segments?`)) return
    setDeletingId(id)
    try {
      await onDeleteAsset(id)
    } catch (err) {
      alert(`Failed to delete asset: ${err instanceof Error ? err.message : String(err)}`)
    } finally {
      setDeletingId(null)
    }
  }

  const getStatusBadge = (asset: MediaAsset) => {
    switch (asset.status) {
      case "ready":
        return (
          <Badge variant="success" className="flex items-center gap-1.5 px-2 py-0.5 w-fit">
            <CheckCircle2 className="h-3 w-3" /> READY
          </Badge>
        )
      case "processing":
        return (
          <div className="flex flex-col gap-1.5 min-w-[130px]">
            <Badge variant="warning" className="flex items-center gap-1.5 px-2 py-0.5 animate-pulse w-fit">
              <Loader2 className="h-3 w-3 animate-spin" /> TRANSCODING {asset.progress !== undefined && asset.progress > 0 ? `(${asset.progress}%)` : ""}
            </Badge>
            {asset.progress !== undefined && asset.progress > 0 && (
              <div className="w-full bg-zinc-800 rounded-full h-1.5 overflow-hidden border border-zinc-700/50">
                <div
                  className="bg-amber-400 h-1.5 rounded-full transition-all duration-500 shadow-[0_0_8px_rgba(251,191,36,0.6)]"
                  style={{ width: `${asset.progress}%` }}
                />
              </div>
            )}
          </div>
        )
      case "failed":
        return (
          <Badge variant="destructive" className="flex items-center gap-1.5 px-2 py-0.5 w-fit">
            <AlertCircle className="h-3 w-3" /> FAILED
          </Badge>
        )
      default:
        return (
          <div className="flex flex-col gap-1.5 min-w-[130px]">
            <Badge variant="outline" className="w-fit">{asset.status.toUpperCase()} {asset.progress !== undefined && asset.progress > 0 ? `(${asset.progress}%)` : ""}</Badge>
            {asset.progress !== undefined && asset.progress > 0 && (
              <div className="w-full bg-zinc-800 rounded-full h-1.5 overflow-hidden border border-zinc-700/50">
                <div
                  className="bg-cyan-400 h-1.5 rounded-full transition-all duration-500"
                  style={{ width: `${asset.progress}%` }}
                />
              </div>
            )}
          </div>
        )
    }
  }

  const formatDuration = (seconds: number) => {
    if (!seconds || seconds <= 0) return "00:00"
    const mins = Math.floor(seconds / 60)
    const secs = Math.floor(seconds % 60)
    return `${mins.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`
  }

  return (
    <div className="space-y-4">
      {/* Search & Filter Controls */}
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 bg-[#111111] p-4 rounded-xl border border-zinc-800">
        <div className="relative flex-1 max-w-md">
          <Search className="absolute left-3 top-2.5 h-4 w-4 text-zinc-500" />
          <Input
            placeholder="Search by title, description, or asset ID..."
            value={searchQuery}
            onChange={e => setSearchQuery(e.target.value)}
            className="pl-9 bg-black/60 border-zinc-800 text-xs font-mono text-white focus:border-zinc-700"
          />
        </div>

        <div className="flex items-center space-x-1.5 bg-black/40 p-1 rounded-lg border border-zinc-800/80">
          {(["all", "ready", "processing", "failed"] as const).map(tab => (
            <button
              key={tab}
              onClick={() => setStatusFilter(tab)}
              className={`px-3 py-1.5 rounded-md text-xs font-mono font-medium transition-all duration-150 ${
                statusFilter === tab
                  ? "bg-zinc-800 text-white shadow-sm"
                  : "text-zinc-400 hover:text-white hover:bg-zinc-900"
              }`}
            >
              {tab.toUpperCase()}
              <span className="ml-1.5 text-[10px] text-zinc-500 font-normal">
                ({tab === "all" ? assets.length : assets.filter(a => a.status === tab).length})
              </span>
            </button>
          ))}
        </div>
      </div>

      {/* Asset Catalog Table */}
      <div className="rounded-xl border border-zinc-800 bg-[#111111] overflow-hidden shadow-xl">
        <Table>
          <TableHeader className="bg-zinc-900/60 border-b border-zinc-800">
            <TableRow className="hover:bg-transparent border-zinc-800">
              <TableHead className="w-[320px] text-xs font-mono text-zinc-400">ASSET / SOURCE</TableHead>
              <TableHead className="text-xs font-mono text-zinc-400">STATUS</TableHead>
              <TableHead className="text-xs font-mono text-zinc-400">DURATION / ABR</TableHead>
              <TableHead className="text-xs font-mono text-zinc-400">CREATED</TableHead>
              <TableHead className="text-right text-xs font-mono text-zinc-400">ACTIONS</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <TableRow>
                <TableCell colSpan={5} className="h-48 text-center text-zinc-500 font-mono text-xs">
                  <div className="flex flex-col items-center justify-center space-y-2">
                    <Loader2 className="h-6 w-6 animate-spin text-emerald-400" />
                    <span>Loading media library catalog from database...</span>
                  </div>
                </TableCell>
              </TableRow>
            ) : filteredAssets.length === 0 ? (
              <TableRow>
                <TableCell colSpan={5} className="h-48 text-center text-zinc-500 font-mono text-xs">
                  No media assets found matching the selected filter criteria.
                </TableCell>
              </TableRow>
            ) : (
              filteredAssets.map(asset => (
                <TableRow
                  key={asset.id}
                  onClick={() => onSelectAsset(asset)}
                  className="cursor-pointer hover:bg-zinc-900/40 border-zinc-800 transition-colors"
                >
                  <TableCell className="py-3">
                    <div className="flex items-center space-x-3">
                      <div className="h-12 w-20 rounded-md bg-zinc-900 border border-zinc-800 overflow-hidden flex-shrink-0 relative group flex items-center justify-center">
                        {asset.thumbnail_url ? (
                          <img src={asset.thumbnail_url} alt={asset.title} className="w-full h-full object-cover" />
                        ) : (
                          <Film className="h-5 w-5 text-zinc-600" />
                        )}
                        <div className="absolute inset-0 bg-black/60 opacity-0 group-hover:opacity-100 transition-opacity flex items-center justify-center">
                          <Play className="h-5 w-5 text-emerald-400 fill-emerald-400" />
                        </div>
                      </div>
                      <div className="min-w-0 flex-1">
                        <p className="font-semibold text-sm text-white truncate hover:text-emerald-400 transition-colors">
                          {asset.title}
                        </p>
                        <p className="text-xs font-mono text-zinc-500 truncate mt-0.5">
                          ID: {asset.id}
                        </p>
                      </div>
                    </div>
                  </TableCell>

                  <TableCell>{getStatusBadge(asset)}</TableCell>

                  <TableCell>
                    <div className="space-y-1">
                      <div className="flex items-center text-xs font-mono text-zinc-300">
                        <Clock className="h-3 w-3 mr-1 text-zinc-500" />
                        {formatDuration(asset.duration_seconds)}
                      </div>
                      <div className="flex items-center text-[11px] font-mono text-zinc-500">
                        <Layers className="h-3 w-3 mr-1 text-cyan-400" />
                        {asset.renditions ? `${asset.renditions.length} Bitrate Tiers` : "Master Only"}
                      </div>
                    </div>
                  </TableCell>

                  <TableCell className="text-xs font-mono text-zinc-400">
                    {new Date(asset.created_at).toLocaleDateString(undefined, {
                      month: "short",
                      day: "numeric",
                      hour: "2-digit",
                      minute: "2-digit",
                    })}
                  </TableCell>

                  <TableCell className="text-right">
                    <div className="flex items-center justify-end space-x-2">
                      <Button
                        variant="outline-emerald"
                        size="sm"
                        onClick={(e) => {
                          e.stopPropagation()
                          onSelectAsset(asset)
                        }}
                        className="h-8 px-2.5 text-xs font-mono"
                      >
                        <Play className="h-3 w-3 mr-1.5 fill-current" /> Preview
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        disabled={deletingId === asset.id}
                        onClick={(e) => handleDelete(e, asset.id, asset.title)}
                        className="h-8 w-8 p-0 text-zinc-500 hover:text-rose-400 hover:bg-rose-500/10"
                      >
                        {deletingId === asset.id ? (
                          <Loader2 className="h-3.5 w-3.5 animate-spin text-rose-400" />
                        ) : (
                          <Trash2 className="h-3.5 w-3.5" />
                        )}
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>
    </div>
  )
}
