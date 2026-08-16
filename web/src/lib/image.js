// Client-side preparation of a profile picture.
//
// This exists to keep uploads small, not to keep them safe: the server decodes
// and re-encodes whatever arrives, so nothing here is a security control.

const MAX_EDGE = 1024;
const QUALITY = 0.85;

async function decode(file) {
  if (typeof createImageBitmap === 'function') {
    try {
      return await createImageBitmap(file);
    } catch {
      // Fall through to the <img> path for formats the bitmap decoder refuses.
    }
  }

  const url = URL.createObjectURL(file);
  try {
    const img = new Image();
    img.src = url;
    await img.decode();
    return img;
  } finally {
    URL.revokeObjectURL(url);
  }
}

/**
 * Center-crops the picture to a square and scales it to at most MAX_EDGE,
 * returning a JPEG blob. A 5MB phone photo comes out around 200KB.
 */
export async function prepareUpload(file) {
  const source = await decode(file);
  const width = source.width ?? source.naturalWidth;
  const height = source.height ?? source.naturalHeight;

  const side = Math.min(width, height);
  const size = Math.min(side, MAX_EDGE);

  const canvas = document.createElement('canvas');
  canvas.width = size;
  canvas.height = size;

  const ctx = canvas.getContext('2d');
  ctx.imageSmoothingQuality = 'high';
  ctx.drawImage(source, (width - side) / 2, (height - side) / 2, side, side, 0, 0, size, size);
  source.close?.();

  const blob = await new Promise((resolve) => canvas.toBlob(resolve, 'image/jpeg', QUALITY));
  if (!blob) throw new Error('画像を変換できませんでした');
  return blob;
}
