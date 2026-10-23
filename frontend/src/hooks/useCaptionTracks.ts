import { useEffect, useState } from "react";
import { apiClient } from "../api/client";
import type { MediaCaption } from "../types/media";
import { normalizeMediaUrl } from "../utils/mediaUrl";
import { parseCaptionCues, type CaptionCue } from "../utils/captions";

/**
 * The caption tracks of the library video a room is playing. Rooms only know
 * what they play by URL, so the backend looks the video up from it
 * (GET /media/captions?url=). Anything that is not a library video has none.
 */
export function useCaptionTracks(mediaUrl: string | null): MediaCaption[] {
  const [result, setResult] = useState<{
    url: string;
    tracks: MediaCaption[];
  } | null>(null);

  useEffect(() => {
    if (!mediaUrl) return;
    let cancelled = false;
    apiClient
      .get<MediaCaption[] | null>("/media/captions", {
        params: { url: mediaUrl },
      })
      .then((tracks) => {
        if (!cancelled) setResult({ url: mediaUrl, tracks: tracks ?? [] });
      })
      .catch(() => {
        // Captions are an extra: a failed lookup just means none are offered.
        if (!cancelled) setResult({ url: mediaUrl, tracks: [] });
      });
    return () => {
      cancelled = true;
    };
  }, [mediaUrl]);

  // A result for the previous video is never shown against the next one.
  return result && result.url === mediaUrl ? result.tracks : [];
}

/** The cues of one track, fetched when it is chosen. */
export function useCaptionCues(trackUrl: string | null): CaptionCue[] {
  const [result, setResult] = useState<{
    url: string;
    cues: CaptionCue[];
  } | null>(null);

  useEffect(() => {
    if (!trackUrl) return;
    let cancelled = false;
    // Tracks are public, cacheable files on the stream proxy; no session needed.
    fetch(normalizeMediaUrl(trackUrl))
      .then((res) => (res.ok ? res.text() : ""))
      .then((text) => {
        if (!cancelled)
          setResult({ url: trackUrl, cues: parseCaptionCues(text) });
      })
      .catch(() => {
        if (!cancelled) setResult({ url: trackUrl, cues: [] });
      });
    return () => {
      cancelled = true;
    };
  }, [trackUrl]);

  return result && result.url === trackUrl ? result.cues : [];
}
