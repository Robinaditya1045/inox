export type MediaStatus = 'pending' | 'processing' | 'ready' | 'failed';

export interface MediaRendition {
  id: string;
  media_asset_id: string;
  resolution: string;
  bitrate_kbps: number;
  playlist_url: string;
  created_at: string;
}

export interface MediaAsset {
  id: string;
  title: string;
  description: string;
  source_url: string;
  status: MediaStatus;
  duration_seconds: number;
  progress: number;
  thumbnail_url: string;
  hls_master_url: string;
  created_by?: string | null;
  created_at: string;
  updated_at: string;
  renditions?: MediaRendition[];
}
