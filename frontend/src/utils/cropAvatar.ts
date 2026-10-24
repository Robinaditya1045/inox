/** Edge length of an uploaded avatar: sharp at the largest size it is drawn
 *  (the 80px settings preview) on a 3x display. */
const AVATAR_SIZE = 256;

/** Images larger than this are refused before decoding them. */
export const MAX_AVATAR_SOURCE_BYTES = 20 * 1024 * 1024;

/**
 * Centre-crops an image file to a square and scales it down to a small JPEG,
 * so uploads stay tiny whatever the user picks. Avatars are drawn round, so
 * the centre square is what shows anyway. Throws if the file isn't an image
 * the browser can decode.
 */
export async function cropAvatar(file: File): Promise<Blob> {
  const bitmap = await createImageBitmap(file);
  try {
    const side = Math.min(bitmap.width, bitmap.height);
    const size = Math.min(AVATAR_SIZE, side);
    const canvas = document.createElement("canvas");
    canvas.width = size;
    canvas.height = size;
    const ctx = canvas.getContext("2d");
    if (!ctx) throw new Error("Canvas is unavailable");

    // JPEG has no alpha: give transparent PNGs a neutral backdrop, not black.
    ctx.fillStyle = "#ffffff";
    ctx.fillRect(0, 0, size, size);
    ctx.imageSmoothingQuality = "high";
    ctx.drawImage(
      bitmap,
      (bitmap.width - side) / 2,
      (bitmap.height - side) / 2,
      side,
      side,
      0,
      0,
      size,
      size,
    );

    return await new Promise<Blob>((resolve, reject) =>
      canvas.toBlob(
        (blob) =>
          blob ? resolve(blob) : reject(new Error("Couldn't encode image")),
        "image/jpeg",
        0.9,
      ),
    );
  } finally {
    bitmap.close();
  }
}
