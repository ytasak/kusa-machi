// Thin fetch wrapper for the same-Origin /api surface.
//
// The CSRF token is issued per game day by GET /api/home and kept in memory
// only (never in storage), then sent on every mutating request.

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
  // Raw image bytes; the body is already a JPEG blob.
  upload: (path, blob) => send('POST', path, { headers: { 'Content-Type': blob.type }, body: blob }),
};
