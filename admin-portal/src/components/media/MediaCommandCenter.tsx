import * as React from "react"
import { useMediaCatalog } from "@/hooks/useMediaCatalog"
import { MediaCatalogTable } from "@/components/media/MediaCatalogTable"
import { VideoUploadModal } from "@/components/media/VideoUploadModal"
import { HLSScrubbingPlayer } from "@/components/media/HLSScrubbingPlayer"
import { MetricCard } from "@/components/dashboard/MetricCard"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import type { MediaAsset } from "@/types/media"
import { Film, UploadCloud, CheckCircle2, Loader2, Layers, RefreshCw, ShieldAlert, Video } from "lucide-react"

export function MediaCommandCenter() {
  const { assets, isLoading, isDemoMode, error, uploadMedia, uploadDirectS3, registerExternal, deleteMedia, refresh } = useMediaCatalog()
  const [isUploadOpen, setIsUploadOpen] = React.useState(false)
  const [previewAsset, setPreviewAsset] = React.useState<MediaAsset | null>(null)

  const readyCount = React.useMemo(() => assets.filter(a => a.status === "ready").length, [assets])
  const processingCount = React.useMemo(() => assets.filter(a => a.status === "processing").length, [assets])
  const totalRenditions = React.useMemo(() => {
    return assets.reduce((acc, curr) => acc + (curr.renditions ? curr.renditions.length : 1), 0)
  }, [assets])

  return (
    <div className="space-y-6 animate-pulse-subtle" style={{ animationDuration: "0.3s", animationIterationCount: 1 }}>
      {/* Top Banner & Control Bar */}
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 p-4 rounded-xl border border-zinc-800 bg-gradient-to-r from-zinc-900/80 to-zinc-950/80 backdrop-blur-md">
        <div className="flex items-center space-x-3">
          <div className="p-2.5 rounded-lg bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
            <Video className="h-6 w-6" />
          </div>
          <div>
            <div className="flex items-center space-x-2">
              <h2 className="font-bold text-white text-base tracking-tight">
                Media Library & Content Command Center
              </h2>
              <Badge variant={isDemoMode ? "warning" : "success"} className="text-[10px]">
                {isDemoMode ? "DEMO CATALOG" : "POSTGRES + MINIO"}
              </Badge>
            </div>
            <p className="text-xs font-mono text-zinc-400 mt-0.5">
              Manage Watch Party video assets, monitor FFmpeg 3-tier ABR transcoding ladders, and inspect HLS manifests.
            </p>
          </div>
        </div>

        <div className="flex items-center space-x-3">
          {error && (
            <span className="text-xs text-amber-400 font-mono flex items-center bg-amber-500/10 px-2.5 py-1 rounded-md border border-amber-500/20">
              <ShieldAlert className="h-3.5 w-3.5 mr-1" /> {error}
            </span>
          )}

          <Button
            variant="outline"
            size="sm"
            onClick={() => refresh()}
            className="text-xs font-mono border-zinc-800 hover:border-zinc-700 text-zinc-300"
          >
            <RefreshCw className="h-3.5 w-3.5 mr-1.5" /> Refresh
          </Button>

          <Button
            variant="emerald"
            size="sm"
            onClick={() => setIsUploadOpen(true)}
            className="text-xs font-mono"
          >
            <UploadCloud className="h-3.5 w-3.5 mr-1.5" /> Add Media Asset
          </Button>
        </div>
      </div>

      {/* Catalog Metric Cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <MetricCard
          title="Total Catalog Assets"
          value={assets.length}
          icon={Film}
          iconColor="text-emerald-400"
          category="USE"
          categoryLabel="INVENTORY"
          trend={`${readyCount} active streams`}
          trendPositive={true}
          progressValue={Math.min(100, (assets.length / 50) * 100)}
          footerText="MinIO / S3 CDN"
        />

        <MetricCard
          title="Ready For Streaming"
          value={readyCount}
          icon={CheckCircle2}
          iconColor="text-cyan-400"
          category="USE"
          categoryLabel="AVAILABILITY"
          trend={`${assets.length > 0 ? ((readyCount / assets.length) * 100).toFixed(0) : 100}% ready rate`}
          trendPositive={readyCount === assets.length}
          progressValue={assets.length > 0 ? (readyCount / assets.length) * 100 : 100}
          footerText="HLS Master Playlists"
        />

        <MetricCard
          title="Transcoding Jobs"
          value={processingCount}
          icon={Loader2}
          iconColor="text-amber-400"
          category="USE"
          categoryLabel="WORKERS"
          trend={processingCount > 0 ? "FFmpeg worker active" : "Queue idle"}
          trendPositive={processingCount === 0}
          progressValue={processingCount > 0 ? 65 : 0}
          footerText="1080p • 720p • 480p"
        />

        <MetricCard
          title="ABR Rendition Tiers"
          value={totalRenditions}
          icon={Layers}
          iconColor="text-purple-400"
          category="USE"
          categoryLabel="LADDER"
          trend="Avg 2.8 tiers / asset"
          trendPositive={true}
          progressValue={Math.min(100, (totalRenditions / 15) * 100)}
          footerText="Adaptive Bitrate HLS"
        />
      </div>

      {/* Media Catalog Data Table */}
      <MediaCatalogTable
        assets={assets}
        isLoading={isLoading}
        onSelectAsset={(asset) => setPreviewAsset(asset)}
        onDeleteAsset={deleteMedia}
      />

      {/* Video Upload Modal */}
      <VideoUploadModal
        isOpen={isUploadOpen}
        onClose={() => setIsUploadOpen(false)}
        onUploadFile={uploadMedia}
        onUploadDirect={uploadDirectS3}
        onRegisterExternal={registerExternal}
      />

      {/* HLS Scrubbing Player Dialog */}
      <HLSScrubbingPlayer
        asset={previewAsset}
        isOpen={!!previewAsset}
        onClose={() => setPreviewAsset(null)}
      />
    </div>
  )
}
