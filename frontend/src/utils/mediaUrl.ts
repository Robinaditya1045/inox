/**
 * Normalizes media streaming URLs (HLS .m3u8, .ts segments, or .mp4) so that whether
 * accessing via Localhost or a Cloudflare Tunnel, browsers never encounter CORS or
 * "Connection Refused" errors caused by hardcoded localhost:9000 or localhost:8080 URLs.
 */
export function normalizeMediaUrl(url: string | undefined | null): string {
  if (!url) return '';

  const streamBase =
    (import.meta.env.VITE_MEDIA_STREAM_BASE_URL as string) ||
    (import.meta.env.VITE_API_BASE_URL?.replace('/api/v1', '/media/stream') as string) ||
    'http://localhost:8080/media/stream';

  // If URL points to local MinIO (localhost:9000 / 127.0.0.1:9000 / inox-media)
  if (url.includes('localhost:9000/inox-media') || url.includes('127.0.0.1:9000/inox-media')) {
    const cleanPath = url.replace(/^http:\/\/(localhost|127\.0\.0\.1):9000\/inox-media\/?/, '');
    return `${streamBase.replace(/\/$/, '')}/${cleanPath}`;
  }
  if (url.includes('localhost:9000') || url.includes('127.0.0.1:9000')) {
    const cleanPath = url.replace(/^http:\/\/(localhost|127\.0\.0\.1):9000\/?/, '');
    return `${streamBase.replace(/\/$/, '')}/${cleanPath}`;
  }

  // If accessed from a Cloudflare Tunnel (or HTTPS) but URL is hardcoded to localhost:8080
  if (streamBase.includes('trycloudflare.com') || streamBase.startsWith('https://')) {
    if (url.includes('localhost:8080/media/stream')) {
      const cleanPath = url.replace(/^http:\/\/localhost:8080\/media\/stream\/?/, '');
      return `${streamBase.replace(/\/$/, '')}/${cleanPath}`;
    }
    if (url.includes('localhost:8080/inox-media')) {
      const cleanPath = url.replace(/^http:\/\/localhost:8080\/inox-media\/?/, '');
      return `${streamBase.replace(/\/$/, '')}/${cleanPath}`;
    }
  }

  return url;
}
