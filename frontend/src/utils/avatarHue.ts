const HUE_COUNT = 8;

/** Stable per-name colour token, so the same person or room is the same colour everywhere. */
export function hueFor(name: string): string {
  let hash = 0;
  for (let i = 0; i < name.length; i++) {
    hash = (hash * 31 + name.charCodeAt(i)) | 0;
  }
  return `var(--avatar-hue-${Math.abs(hash) % HUE_COUNT})`;
}
