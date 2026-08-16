// 同一 Origin の /api に対する薄い fetch ラッパ。
//
// CSRF トークンはゲーム日ごとに GET /api/home が発行し、メモリ上にだけ保持して
// （ストレージには置かない）、更新系リクエストのたびに送る。

const CSRF_HEADER = 'X-CSRF-Token';

let csrfToken = null;

export function setCsrfToken(token) {
  csrfToken = token;
}

export class ApiError extends Error {
  constructor(code, message, status) {
    super(message || code);
    this.name = 'ApiError';
    this.code = code;
    this.status = status;
  }
}

async function send(method, path, { headers = {}, body } = {}) {
  if (method !== 'GET') headers[CSRF_HEADER] = csrfToken ?? '';

  const res = await fetch(path, { method, headers, credentials: 'same-origin', body });

  const text = await res.text();
  const payload = text ? JSON.parse(text) : null;

  if (!res.ok) {
    const err = payload?.error;
    throw new ApiError(err?.code ?? 'InternalError', err?.message ?? res.statusText, res.status);
  }
  return payload;
}

function request(method, path, body) {
  return send(method, path, {
    headers: body === undefined ? {} : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
}

export const api = {
  get: (path) => request('GET', path),
  post: (path, body) => request('POST', path, body ?? {}),
  patch: (path, body) => request('PATCH', path, body ?? {}),
  delete: (path) => request('DELETE', path),
  // 生の画像バイト列。ボディはすでに JPEG の Blob になっている。
  upload: (path, blob) => send('POST', path, { headers: { 'Content-Type': blob.type }, body: blob }),
};
