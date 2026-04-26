/**
 * CSRF Token Service
 *
 * Manages the CSRF token lifecycle:
 * - Fetches the token from the backend on demand (deduplicates inflight requests)
 * - Stores the token in memory (not localStorage — single-page session only)
 * - Exposes getCsrfToken() for use in HTTP clients
 * - Exposes clearCsrfToken() for use on logout
 *
 * Backend endpoint: GET /api/v1/auth/csrf-token
 * Response shape:  { csrf_token: string }
 * Cookie:          csrf_token (HttpOnly: false so JS can also read it, but we track in memory)
 */

let csrfToken: string | null = null;
let inflight: Promise<string> | null = null;

/**
 * Fetch the CSRF token from the backend.
 * Deduplicates concurrent calls — only one request is ever in flight.
 * Result is stored in memory so subsequent calls to getCsrfToken() are synchronous.
 */
export async function fetchCsrfToken(): Promise<string> {
  if (inflight) return inflight;
  inflight = (async () => {
    const res = await fetch('/api/v1/auth/csrf-token', { method: 'GET', credentials: 'include' });
    if (!res.ok) throw new Error(`CSRF fetch failed: ${res.status}`);
    const data = await res.json();
    csrfToken = data.csrf_token;
    return csrfToken!;
  })();
  try {
    return await inflight;
  } finally {
    inflight = null;
  }
}

/**
 * Return the in-memory CSRF token (null if not yet fetched).
 */
export function getCsrfToken(): string | null {
  return csrfToken;
}

/**
 * Clear the in-memory CSRF token (call on logout).
 */
export function clearCsrfToken(): void {
  csrfToken = null;
}
