export type MediaStatus = "pending" | "processing" | "ready" | "failed"

/** A quality an upload can be transcoded to; each one is a separate encode. */
export type TranscodeProfile = "1080p" | "720p" | "480p"

export interface MediaRendition {
  id: string
  media_asset_id: string
  resolution: string
  bitrate_kbps: number
  playlist_url: string
  created_at: string
}

/** One language's caption track for a video, served as WebVTT from `url`. */
export interface MediaCaption {
  id: string
  media_asset_id: string
  /** BCP 47 tag, e.g. "en" or "pt-BR" */
  language: string
  label: string
  url: string
  cue_count: number
  is_default: boolean
  created_at: string
  updated_at: string
}

export interface MediaAsset {
  id: string
  /** "live" for a live channel's library entry; uploads and links are "vod" */
  kind?: "vod" | "live"
  title: string
  description: string
  source_url: string
  status: MediaStatus
  duration_seconds: number
  progress?: number
  thumbnail_url: string
  hls_master_url: string
  created_by: string | null
  created_at: string
  updated_at: string
  renditions?: MediaRendition[]
  /** The qualities an upload was sent to be transcoded to; omitted means all of them */
  transcode_profiles?: TranscodeProfile[]
  /** Omitted by the API when a video has none */
  captions?: MediaCaption[]
}

/** PATCH /admin/media/{id}: fields left out are left as they are. */
export interface MediaDetailsUpdate {
  title?: string
  description?: string
  /** Points the poster at an image hosted elsewhere */
  thumbnail_url?: string
}

export interface CaptionInput {
  language: string
  label?: string
  /** WebVTT or SubRip text (the editor) — or `file` (an upload) */
  content?: string
  file?: File
  is_default?: boolean
}

export type CaptionUpdate = Partial<Omit<CaptionInput, "file">> & { file?: File }

export interface RegisterMediaRequest {
  title: string
  description: string
  source_url: string
  thumbnail_url?: string
}

export interface PresignedUploadRequest {
  title: string
  description: string
  filename: string
  content_type: string
  size: number
}

export interface PresignedUploadResponse {
  asset: MediaAsset
  upload_url: string
  key: string
}
