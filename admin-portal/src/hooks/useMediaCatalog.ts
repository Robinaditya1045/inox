import * as React from "react"
import { apiFetch, apiJson, uploadWithProgress } from "@/lib/api"
import type { MediaAsset, PresignedUploadResponse, RegisterMediaRequest } from "@/types/media"

export interface UseMediaCatalogResult {
  assets: MediaAsset[]
  isLoading: boolean
  isDemoMode: boolean
  error: string | null
  uploadMedia: (file: File, title: string, description: string, onProgress?: (percent: number) => void) => Promise<MediaAsset>
  uploadDirectS3: (file: File, title: string, description: string, onProgress?: (percent: number) => void) => Promise<MediaAsset>
  registerExternal: (req: RegisterMediaRequest) => Promise<MediaAsset>
  deleteMedia: (id: string) => Promise<void>
  refresh: () => Promise<void>
  toggleDemoMode: () => void
}

const MOCK_ASSETS: MediaAsset[] = [
  {
    id: "asset-cybp-01",
    title: "Cyberpunk 2077: Phantom Liberty Trailer",
    description: "Official 4K HLS streaming trailer with 3-tier adaptive bitrate ladder.",
    source_url: "https://cdn.inox.stream/videos/cyberpunk_trailer.mp4",
    status: "ready",
    duration_seconds: 184,
    thumbnail_url: "https://images.unsplash.com/photo-1542751371-adc38448a05e?auto=format&fit=crop&w=600&q=80",
    hls_master_url: "https://cdn.inox.stream/hls/cyberpunk_trailer/master.m3u8",
    created_by: "admin-robin",
    created_at: new Date(Date.now() - 3600000 * 24).toISOString(),
    updated_at: new Date(Date.now() - 3600000 * 23).toISOString(),
    renditions: [
      { id: "rend-1080p", media_asset_id: "asset-cybp-01", resolution: "1080p", bitrate_kbps: 4500, playlist_url: "https://cdn.inox.stream/hls/cyberpunk_trailer/stream_1080p.m3u8", created_at: new Date().toISOString() },
      { id: "rend-720p", media_asset_id: "asset-cybp-01", resolution: "720p", bitrate_kbps: 2500, playlist_url: "https://cdn.inox.stream/hls/cyberpunk_trailer/stream_720p.m3u8", created_at: new Date().toISOString() },
      { id: "rend-480p", media_asset_id: "asset-cybp-01", resolution: "480p", bitrate_kbps: 1000, playlist_url: "https://cdn.inox.stream/hls/cyberpunk_trailer/stream_480p.m3u8", created_at: new Date().toISOString() },
    ]
  },
  {
    id: "asset-mtrx-02",
    title: "The Matrix Remastered (1999) - Lobby Shootout",
    description: "High bitrate Watch Party test clip with synchronized 5.1 AAC audio track.",
    source_url: "https://cdn.inox.stream/videos/matrix_lobby.mp4",
    status: "ready",
    duration_seconds: 312,
    thumbnail_url: "https://images.unsplash.com/photo-1526374965328-7f61d4dc18c5?auto=format&fit=crop&w=600&q=80",
    hls_master_url: "https://cdn.inox.stream/hls/matrix_lobby/master.m3u8",
    created_by: "admin-robin",
    created_at: new Date(Date.now() - 3600000 * 48).toISOString(),
    updated_at: new Date(Date.now() - 3600000 * 47).toISOString(),
    renditions: [
      { id: "rend-m-1080p", media_asset_id: "asset-mtrx-02", resolution: "1080p", bitrate_kbps: 4500, playlist_url: "https://cdn.inox.stream/hls/matrix_lobby/stream_1080p.m3u8", created_at: new Date().toISOString() },
      { id: "rend-m-720p", media_asset_id: "asset-mtrx-02", resolution: "720p", bitrate_kbps: 2500, playlist_url: "https://cdn.inox.stream/hls/matrix_lobby/stream_720p.m3u8", created_at: new Date().toISOString() },
    ]
  },
  {
    id: "asset-intr-03",
    title: "Interstellar IMAX Flight Sequence",
    description: "Deep space telemetry and SFU sync stress test stream @ 60 FPS.",
    source_url: "https://cdn.inox.stream/videos/interstellar_imax.mp4",
    status: "ready",
    duration_seconds: 420,
    thumbnail_url: "https://images.unsplash.com/photo-1451187580459-43490279c0fa?auto=format&fit=crop&w=600&q=80",
    hls_master_url: "https://cdn.inox.stream/hls/interstellar/master.m3u8",
    created_by: "admin-robin",
    created_at: new Date(Date.now() - 3600000 * 72).toISOString(),
    updated_at: new Date(Date.now() - 3600000 * 71).toISOString(),
    renditions: [
      { id: "rend-i-1080p", media_asset_id: "asset-intr-03", resolution: "1080p", bitrate_kbps: 5000, playlist_url: "https://cdn.inox.stream/hls/interstellar/stream_1080p.m3u8", created_at: new Date().toISOString() },
    ]
  },
  {
    id: "asset-proc-04",
    title: "Inox SFU Platform Demo Showcase",
    description: "Currently undergoing FFmpeg 3-tier ABR ladder transcoding in background worker...",
    source_url: "https://cdn.inox.stream/uploads/raw_demo_v2.mp4",
    status: "processing",
    duration_seconds: 120,
    thumbnail_url: "https://images.unsplash.com/photo-1550751827-4bd374c3f58b?auto=format&fit=crop&w=600&q=80",
    hls_master_url: "",
    created_by: "admin-robin",
    created_at: new Date(Date.now() - 60000 * 15).toISOString(),
    updated_at: new Date(Date.now() - 60000 * 5).toISOString(),
    progress: 45
  },
  {
    id: "asset-fail-05",
    title: "Corrupted MP4 Stream Test",
    description: "FFmpeg exited with code 1: moov atom not found in MP4 header.",
    source_url: "https://cdn.inox.stream/uploads/corrupted_header.mp4",
    status: "failed",
    duration_seconds: 0,
    thumbnail_url: "",
    hls_master_url: "",
    created_by: "system-test",
    created_at: new Date(Date.now() - 3600000 * 96).toISOString(),
    updated_at: new Date(Date.now() - 3600000 * 96).toISOString(),
  }
]

export function useMediaCatalog(): UseMediaCatalogResult {
  const [assets, setAssets] = React.useState<MediaAsset[]>([])
  const [isLoading, setIsLoading] = React.useState<boolean>(true)
  const [isDemoMode, setIsDemoMode] = React.useState<boolean>(false)
  const [error, setError] = React.useState<string | null>(null)

  const assetsRef = React.useRef<MediaAsset[]>([])
  const isDemoModeRef = React.useRef<boolean>(false)

  // Mirror state into refs from an effect rather than during render, so the
  // refs stay consistent if React discards a render pass.
  React.useEffect(() => {
    assetsRef.current = assets
  }, [assets])

  React.useEffect(() => {
    isDemoModeRef.current = isDemoMode
  }, [isDemoMode])

  const fetchAssets = React.useCallback(async (silent = false) => {
    if (isDemoModeRef.current) {
      if (!silent) {
        setAssets(MOCK_ASSETS)
        setIsLoading(false)
      }
      return
    }

    if (!silent) setIsLoading(true)
    try {
      const data = await apiJson<MediaAsset[] | null>("/media?limit=50&offset=0")
      // An empty catalog is a legitimate state; substituting mock rows here made
      // the table show assets that do not exist, whose delete/preview actions
      // then failed against the real API.
      setAssets(Array.isArray(data) ? data : [])
      setError(null)
    } catch (err) {
      if (!silent) {
        console.warn("Failed to fetch from backend API, switching to Demo Mode:", err)
        setAssets(MOCK_ASSETS)
        setIsDemoMode(true)
        isDemoModeRef.current = true
        setError("Backend unreachable. Displaying interactive simulated catalog.")
      }
    } finally {
      if (!silent) setIsLoading(false)
    }
  }, [])

  React.useEffect(() => {
    fetchAssets(false)
  }, [fetchAssets])

  // Background auto-polling with ref check to avoid re-render storms and dependency loops
  React.useEffect(() => {
    const interval = setInterval(() => {
      const hasProcessing = assetsRef.current.some(a => a.status === "processing" || a.status === "pending")
      if (hasProcessing && !isDemoModeRef.current) {
        fetchAssets(true)
      }
    }, 3000)

    return () => clearInterval(interval)
  }, [fetchAssets])

  const uploadMedia = React.useCallback(async (
    file: File,
    title: string,
    description: string,
    onProgress?: (percent: number) => void
  ): Promise<MediaAsset> => {
    if (isDemoModeRef.current) {
      for (let p = 10; p <= 100; p += 20) {
        if (onProgress) onProgress(p)
        await new Promise(r => setTimeout(r, 150))
      }

      const newAsset: MediaAsset = {
        id: `asset-sim-${Date.now().toString().slice(-4)}`,
        title: title || file.name,
        description: description || "Uploaded via Inox Admin Portal multipart uploader.",
        source_url: `https://cdn.inox.stream/uploads/${file.name}`,
        status: "processing",
        duration_seconds: 180,
        thumbnail_url: "https://images.unsplash.com/photo-1618005182384-a83a8bd57fbe?auto=format&fit=crop&w=600&q=80",
        hls_master_url: "",
        created_by: "admin-local",
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString(),
        progress: 15
      }

      setAssets(prev => [newAsset, ...prev])
      return newAsset
    }

    const formData = new FormData()
    formData.append("file", file)
    formData.append("title", title)
    formData.append("description", description)

    // Uploaded via XHR rather than fetch(): fetch cannot report upload progress,
    // so the modal's progress bar used to sit at 5% for the whole transfer and
    // then jump straight to "complete".
    const asset = await uploadWithProgress<MediaAsset>("/admin/media/upload", formData, onProgress)
    await fetchAssets(true)
    return asset
  }, [fetchAssets])

  const uploadDirectS3 = React.useCallback(async (
    file: File,
    title: string,
    description: string,
    onProgress?: (percent: number) => void
  ): Promise<MediaAsset> => {
    const maxSize = 3 * 1024 * 1024 * 1024 // 3GB
    if (file.size > maxSize) {
      throw new Error(`File size (${(file.size / (1024 * 1024)).toFixed(2)} MB) exceeds the 3GB direct upload limit.`)
    }

    if (isDemoModeRef.current) {
      for (let p = 5; p <= 100; p += 15) {
        if (onProgress) onProgress(Math.min(100, p))
        await new Promise(r => setTimeout(r, 200))
      }

      const newAsset: MediaAsset = {
        id: `asset-s3-${Date.now().toString().slice(-4)}`,
        title: title || file.name,
        description: description || "Direct pre-signed object storage upload (Up to 3GB).",
        source_url: `https://minio.inox.stream/videos-raw/${file.name}`,
        status: "processing",
        duration_seconds: 360,
        thumbnail_url: "https://images.unsplash.com/photo-1536440136628-849c177e76a1?auto=format&fit=crop&w=600&q=80",
        hls_master_url: "",
        created_by: "admin-s3-direct",
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString(),
        progress: 20
      }

      setAssets(prev => [newAsset, ...prev])
      return newAsset
    }

    const { asset, upload_url, key } = await apiJson<PresignedUploadResponse>("/admin/media/presigned-url", {
      method: "POST",
      body: JSON.stringify({
        title: title || file.name,
        description: description || "Direct S3/MinIO upload",
        filename: file.name,
        content_type: file.type || "video/mp4",
        size: file.size,
      }),
    })

    await new Promise<void>((resolve, reject) => {
      const xhr = new XMLHttpRequest()
      xhr.open("PUT", upload_url, true)
      xhr.setRequestHeader("Content-Type", file.type || "video/mp4")

      if (xhr.upload && onProgress) {
        xhr.upload.onprogress = (e) => {
          if (e.lengthComputable) {
            const percent = Math.round((e.loaded / e.total) * 100)
            onProgress(percent)
          }
        }
      }

      xhr.onload = () => {
        if (xhr.status >= 200 && xhr.status < 300) {
          resolve()
        } else {
          reject(new Error(`Storage PUT failed with status ${xhr.status}`))
        }
      }

      xhr.onerror = () => reject(new Error("Network error during direct storage upload"))
      xhr.send(file)
    })

    const completedAsset = await apiJson<MediaAsset>("/admin/media/complete-upload", {
      method: "POST",
      body: JSON.stringify({
        asset_id: asset.id,
        key: key,
      }),
    })

    await fetchAssets(true)
    return completedAsset
  }, [fetchAssets])

  const registerExternal = React.useCallback(async (req: RegisterMediaRequest): Promise<MediaAsset> => {
    if (isDemoModeRef.current) {
      const newAsset: MediaAsset = {
        id: `asset-ext-${Date.now().toString().slice(-4)}`,
        title: req.title,
        description: req.description || "Registered external HLS manifest.",
        source_url: req.source_url,
        status: "ready",
        duration_seconds: 240,
        thumbnail_url: req.thumbnail_url || "https://images.unsplash.com/photo-1579783902614-a3fb3927b675?auto=format&fit=crop&w=600&q=80",
        hls_master_url: req.source_url,
        created_by: "admin-local",
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString(),
        renditions: [
          { id: `rend-${Date.now()}`, media_asset_id: `asset-ext`, resolution: "1080p (External)", bitrate_kbps: 4500, playlist_url: req.source_url, created_at: new Date().toISOString() }
        ]
      }
      setAssets(prev => [newAsset, ...prev])
      return newAsset
    }

    const asset = await apiJson<MediaAsset>("/admin/media/register", {
      method: "POST",
      body: JSON.stringify(req),
    })

    await fetchAssets(true)
    return asset
  }, [fetchAssets])

  const deleteMedia = React.useCallback(async (id: string): Promise<void> => {
    if (isDemoModeRef.current) {
      setAssets(prev => prev.filter(a => a.id !== id))
      return
    }

    const res = await apiFetch(`/admin/media/${id}`, { method: "DELETE" })

    if (!res.ok && res.status !== 204) {
      const detail = await res.text().catch(() => "")
      throw new Error(detail || `Delete failed: ${res.statusText}`)
    }

    setAssets(prev => prev.filter(a => a.id !== id))
  }, [])

  const toggleDemoMode = React.useCallback(() => {
    // Keep the updater pure and drive the reload from the resolved value, so
    // flipping the switch actually swaps the catalog instead of leaving the
    // previous mode's rows on screen.
    const next = !isDemoModeRef.current
    isDemoModeRef.current = next
    setIsDemoMode(next)
    setError(null)

    if (next) {
      setAssets(MOCK_ASSETS)
      setIsLoading(false)
    } else {
      void fetchAssets(false)
    }
  }, [fetchAssets])

  return {
    assets,
    isLoading,
    isDemoMode,
    error,
    uploadMedia,
    uploadDirectS3,
    registerExternal,
    deleteMedia,
    refresh: () => fetchAssets(false),
    toggleDemoMode,
  }
}
