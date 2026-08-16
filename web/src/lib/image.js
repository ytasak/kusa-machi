// プロフィール写真のクライアント側の下準備。
//
// 目的はアップロードを小さくすることであって、安全にすることではない。
// サーバは届いたものを必ずデコードして再エンコードするため、
// ここにあるものはセキュリティ上の防御ではない。

const MAX_EDGE = 1024;
const QUALITY = 0.85;

async function decode(file) {
  if (typeof createImageBitmap === 'function') {
    try {
      return await createImageBitmap(file);
    } catch {
      // ビットマップデコーダが受け付けない形式は <img> 経由にフォールバックする。
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
 * 写真を中央で正方形に切り抜き、長辺が MAX_EDGE 以下になるよう縮小して
 * JPEG の Blob を返す。5MB のスマホ写真がおよそ200KBになる。
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
