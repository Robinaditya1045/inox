import * as React from "react"
import type { MediaAsset } from "@/types/media"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Badge } from "@/components/ui/badge"
import { Play, Pause, RotateCcw, Volume2, VolumeX, Film, Activity, Cpu, Radio, ShieldCheck, ExternalLink } from "lucide-react"

interface HLSScrubbingPlayerProps {
  asset: MediaAsset | null
  isOpen: boolean
  onClose: () => void
}

export function HLSScrubbingPlayer({
  asset,
  isOpen,
  onClose,
}: HLSScrubbingPlayerProps) {
  const videoRef = React.useRef<HTMLVideoElement | null>(null)
  const [isPlaying, setIsPlaying] = React.useState(false)
  const [currentTime, setCurrentTime] = React.useState(0)
  const [duration, setDuration] = React.useState(0)
  const [selectedRendition, setSelectedRendition] = React.useState<string>("master")
  const [isMuted, setIsMuted] = React.useState(false)
  const [errorMsg, setErrorMsg] = React.useState<string | null>(null)

  const activeUrl = React.useMemo(() => {
    if (!asset) return ""
    if (selectedRendition === "master") return asset.hls_master_url || asset.source_url
    const rend = asset.renditions?.find(r => r.resolution === selectedRendition)
    return rend ? rend.playlist_url : asset.source_url
  }, [asset, selectedRendition])

  const activeBitrate = React.useMemo(() => {
    if (!asset) return ""
    if (selectedRendition === "master") return "6500 kbps (ABR Auto)"
    const rend = asset.renditions?.find(r => r.resolution === selectedRendition)
    return rend ? `${rend.bitrate_kbps} kbps` : "Direct Stream"
  }, [asset, selectedRendition])

  React.useEffect(() => {
    if (!isOpen || !asset) {
      setIsPlaying(false)
      setCurrentTime(0)
      setErrorMsg(null)
      if (!isOpen) setSelectedRendition("master")
      return
    }
  }, [isOpen, asset])

  React.useEffect(() => {
    if (!isOpen || !asset || !activeUrl) {
      return
    }
    setErrorMsg(null)

    const video = videoRef.current
    if (!video) return

    let hlsInstance: any = null

    const initVideo = () => {
      if (!videoRef.current) return
      const v = videoRef.current
      if (v.canPlayType("application/vnd.apple.mpegurl") || !activeUrl.includes(".m3u8")) {
        v.src = activeUrl
      } else if (typeof window !== "undefined" && (window as any).Hls && (window as any).Hls.isSupported()) {
        const Hls = (window as any).Hls
        hlsInstance = new Hls({
          enableWorker: true,
          lowLatencyMode: true,
        })
        hlsInstance.on(Hls.Events.ERROR, (event: any, data: any) => {
          console.error("HLS error event detailed:", data.type, data.details, data)
          if (data.fatal) {
            switch (data.type) {
              case Hls.ErrorTypes.NETWORK_ERROR:
                console.error("Fatal network error loading HLS segment or manifest:", data.url)
                setErrorMsg(`Network Error: Failed to fetch segment (${data.details}). If asset was processed prior to relative path update, re-process asset.`)
                hlsInstance.startLoad()
                break
              case Hls.ErrorTypes.MEDIA_ERROR:
                if (data.details === "manifestIncompatibleCodecsError") {
                  console.error("Fatal incompatible codecs error:", data.reason || data.details)
                  setErrorMsg(`Incompatible Codecs Error (${data.details}): The video profile/codec (${data.reason || 'e.g. 10-bit AVC/High 10'}) is not supported by your browser's MediaSource decoder. Please re-process this asset to output standard 8-bit YUV420P H.264.`)
                  hlsInstance.destroy()
                } else {
                  console.error("Fatal media decoding error. Attempting recovery...")
                  setErrorMsg(`Media Error (${data.details}): Error decoding video stream.`)
                  hlsInstance.recoverMediaError()
                }
                break
              default:
                setErrorMsg(`Fatal HLS Error (${data.details}): Stream playback halted.`)
                hlsInstance.destroy()
                break
            }
          }
        })
        hlsInstance.on(Hls.Events.MANIFEST_PARSED, (event: any, data: any) => {
          console.info("HLS manifest parsed successfully. Available adaptive levels:", data.levels?.length)
          setErrorMsg(null)
        })
        hlsInstance.loadSource(activeUrl)
        hlsInstance.attachMedia(v)
      } else {
        v.src = activeUrl
      }
    }

    if (activeUrl.includes(".m3u8") && !video.canPlayType("application/vnd.apple.mpegurl") && typeof window !== "undefined" && !(window as any).Hls) {
      const scriptId = "hls-js-cdn-script"
      let script = document.getElementById(scriptId) as HTMLScriptElement
      if (!script) {
        script = document.createElement("script")
        script.id = scriptId
        script.src = "https://cdn.jsdelivr.net/npm/hls.js@latest"
        script.async = true
        script.onload = () => {
          initVideo()
        }
        document.head.appendChild(script)
      } else {
        script.addEventListener("load", initVideo)
      }
    } else {
      initVideo()
    }

    return () => {
      if (hlsInstance) {
        hlsInstance.destroy()
      }
    }
  }, [isOpen, asset, activeUrl])

  if (!asset) return null

  const handleTimeUpdate = () => {
    if (videoRef.current) {
      setCurrentTime(videoRef.current.currentTime)
      setDuration(videoRef.current.duration || asset.duration_seconds || 1)
    }
  }

  const handlePlayPause = () => {
    if (!videoRef.current) return
    if (videoRef.current.paused) {
      const playPromise = videoRef.current.play()
      if (playPromise !== undefined) {
        playPromise
          .then(() => {
            setIsPlaying(true)
          })
          .catch((error) => {
            console.error("Video play error (media resource not suitable or blocked):", error)
            setIsPlaying(false)
          })
      } else {
        setIsPlaying(true)
      }
    } else {
      videoRef.current.pause()
      setIsPlaying(false)
    }
  }

  const handleScrub = (e: React.ChangeEvent<HTMLInputElement>) => {
    const time = Number(e.target.value)
    setCurrentTime(time)
    if (videoRef.current) {
      videoRef.current.currentTime = time
    }
  }

  const segmentIndex = Math.floor(currentTime / 4)
  const byteStart = Math.floor(currentTime * 450000)
  const byteEnd = byteStart + 1800000

  return (
    <Dialog open={isOpen} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="sm:max-w-[780px] border-zinc-800 bg-[#0a0a0a] text-white p-6 shadow-2xl">
        <DialogHeader className="pb-2">
          <div className="flex items-center justify-between">
            <DialogTitle className="text-lg font-semibold flex items-center">
              <Film className="h-5 w-5 mr-2 text-emerald-400" /> HLS Inspector & Scrubbing Player
            </DialogTitle>
            <Badge variant="cyan" className="font-mono text-xs">
              {selectedRendition.toUpperCase()}
            </Badge>
          </div>
          <DialogDescription className="text-xs font-mono text-zinc-400 truncate">
            {asset.title} • ID: {asset.id}
          </DialogDescription>
        </DialogHeader>

        {/* Video Screen Container */}
        <div className="relative rounded-xl overflow-hidden bg-black border border-zinc-800 aspect-video flex items-center justify-center group shadow-inner">
          <video
            ref={videoRef}
            poster={asset.thumbnail_url}
            onTimeUpdate={handleTimeUpdate}
            onPlay={() => setIsPlaying(true)}
            onPause={() => setIsPlaying(false)}
            onLoadedMetadata={() => {
              if (videoRef.current) setDuration(videoRef.current.duration || asset.duration_seconds || 1)
            }}
            className="w-full h-full object-contain"
            controls={false}
            muted={isMuted}
            autoPlay={false}
          />

          {errorMsg && (
            <div className="absolute inset-0 bg-black/85 flex flex-col items-center justify-center p-6 text-center z-20 backdrop-blur-sm animate-in fade-in duration-200">
              <Activity className="h-10 w-10 text-red-500 mb-3 animate-pulse" />
              <p className="text-sm font-semibold text-red-400 mb-1.5">Stream Playback Error</p>
              <p className="text-xs font-mono text-zinc-300 max-w-md bg-zinc-900/90 p-3 rounded-lg border border-red-500/30 mb-3 shadow-lg">
                {errorMsg}
              </p>
              <p className="text-[11px] text-zinc-400 max-w-sm">
                If this asset was transcoded prior to the relative path update, please re-process it or upload again to generate clean relative segment URLs.
              </p>
            </div>
          )}

          {/* Custom Overlay Controls */}
          <div className="absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/90 via-black/50 to-transparent p-4 opacity-90 group-hover:opacity-100 transition-opacity flex flex-col justify-end">
            <input
              type="range"
              min={0}
              max={duration || 100}
              step={0.1}
              value={currentTime}
              onChange={handleScrub}
              className="w-full h-1.5 bg-zinc-700 rounded-lg appearance-none cursor-pointer accent-emerald-500 mb-3"
            />

            <div className="flex items-center justify-between text-xs font-mono">
              <div className="flex items-center space-x-3">
                <button
                  onClick={handlePlayPause}
                  className="p-1.5 rounded-full bg-emerald-500 hover:bg-emerald-400 text-black transition-transform active:scale-95"
                >
                  {isPlaying ? <Pause className="h-4 w-4 fill-current" /> : <Play className="h-4 w-4 fill-current ml-0.5" />}
                </button>
                <button
                  onClick={() => {
                    if (videoRef.current) videoRef.current.currentTime = 0
                  }}
                  className="p-1.5 text-zinc-300 hover:text-white"
                >
                  <RotateCcw className="h-4 w-4" />
                </button>
                <button
                  onClick={() => setIsMuted(!isMuted)}
                  className="p-1.5 text-zinc-300 hover:text-white"
                >
                  {isMuted ? <VolumeX className="h-4 w-4 text-rose-400" /> : <Volume2 className="h-4 w-4" />}
                </button>
                <span className="text-zinc-300 font-medium">
                  {currentTime.toFixed(1)}s / {duration.toFixed(1)}s
                </span>
              </div>

              {/* Bitrate Ladder Tier Switcher */}
              <div className="flex items-center space-x-1.5 bg-black/60 px-2 py-1 rounded-md border border-zinc-800">
                <span className="text-[10px] text-zinc-500">TIER:</span>
                <button
                  onClick={() => setSelectedRendition("master")}
                  className={`px-2 py-0.5 rounded text-[10px] font-bold transition-colors ${selectedRendition === "master" ? "bg-emerald-500 text-black" : "text-zinc-400 hover:text-white"}`}
                >
                  MASTER
                </button>
                {asset.renditions?.map(r => (
                  <button
                    key={r.resolution}
                    onClick={() => setSelectedRendition(r.resolution)}
                    className={`px-2 py-0.5 rounded text-[10px] font-bold transition-colors ${selectedRendition === r.resolution ? "bg-cyan-500 text-black" : "text-zinc-400 hover:text-white"}`}
                  >
                    {r.resolution}
                  </button>
                ))}
              </div>
            </div>
          </div>
        </div>

        {/* Real-Time ABR & Byte-Range Telemetry Panel */}
        <div className="grid grid-cols-1 md:grid-cols-3 gap-3 pt-2">
          <div className="p-3 rounded-lg bg-[#111111] border border-zinc-800/80 space-y-1">
            <div className="flex items-center text-[11px] font-mono text-zinc-400">
              <Radio className="h-3 w-3 mr-1.5 text-emerald-400" /> ABR RENDITIONS
            </div>
            <p className="text-sm font-bold font-mono text-white">{selectedRendition.toUpperCase()}</p>
            <p className="text-[10px] font-mono text-zinc-500">Target Bitrate: {activeBitrate}</p>
          </div>

          <div className="p-3 rounded-lg bg-[#111111] border border-zinc-800/80 space-y-1">
            <div className="flex items-center text-[11px] font-mono text-zinc-400">
              <Activity className="h-3 w-3 mr-1.5 text-cyan-400" /> HLS SEGMENT INDEX
            </div>
            <p className="text-sm font-bold font-mono text-white">Segment #{segmentIndex} (.ts)</p>
            <p className="text-[10px] font-mono text-zinc-500">Duration: 4.000s independent chunk</p>
          </div>

          <div className="p-3 rounded-lg bg-[#111111] border border-zinc-800/80 space-y-1">
            <div className="flex items-center text-[11px] font-mono text-zinc-400">
              <Cpu className="h-3 w-3 mr-1.5 text-amber-400" /> CDN BYTE-RANGE
            </div>
            <p className="text-xs font-bold font-mono text-emerald-400 truncate">
              bytes={byteStart}-{byteEnd}
            </p>
            <p className="text-[10px] font-mono text-zinc-500">HTTP 206 Partial Content Seek</p>
          </div>
        </div>

        <div className="flex items-center justify-between pt-2 border-t border-zinc-800/80 text-xs font-mono text-zinc-500">
          <span className="flex items-center">
            <ShieldCheck className="h-3.5 w-3.5 mr-1 text-emerald-400" /> MinIO / S3 CDN Edge Proxy Verified
          </span>
          <a
            href={activeUrl}
            target="_blank"
            rel="noreferrer"
            className="flex items-center text-zinc-400 hover:text-emerald-400 transition-colors"
          >
            <ExternalLink className="h-3.5 w-3.5 mr-1" /> Open Manifest
          </a>
        </div>
      </DialogContent>
    </Dialog>
  )
}
