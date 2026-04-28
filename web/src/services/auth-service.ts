/**
 * Authentication Service Layer
 *
 * Provides async functions for authentication operations:
 * - login: Submit credentials, receive JWT and user profile
 * - logout: Revoke refresh token and clear session
 * - refreshToken: Silent token refresh using httpOnly cookie
 * - getProfile: Fetch current user profile to verify session
 */

// No base URL — all requests use relative paths so Vite proxy (dev) and
// Go's static file server (prod) both work without CORS or cookie issues.
const API_BASE_URL = ''

import { fetchCsrfToken, getCsrfToken } from './csrf'

export interface LoginRequest {
  email: string
  password: string
}

export interface LoginResponse {
  access_token: string
  user: {
    id: string
    email: string
    display_name: string
    role: 'admin' | 'analyst' | 'viewer'
    permissions: string[]
    status: 'active' | 'suspended' | 'pending_invite'
    created_at: string
  }
  /** Set to true when the account has MFA enabled — access_token will be absent */
  mfa_required?: boolean
  /** Opaque token passed to POST /api/v1/auth/mfa/challenge to complete login */
  challenge_token?: string
}

export interface MfaChallengeRequest {
  challenge_token: string
  code: string
}

export interface MfaEnrollResponse {
  secret: string
  qr_code_data_url: string
  backup_codes: string[]
}

export interface RefreshResponse {
  access_token: string
}

export interface LogoutResponse {
  message: string
}

export class ApiError extends Error {
  status: number
  details?: unknown

  constructor(status: number, details?: unknown, message?: string) {
    super(message || `API Error: ${status}`)
    this.name = 'ApiError'
    this.status = status
    this.details = details
  }
}

/**
 * Build CSRF-aware headers for mutating requests.
 * Fetches the token if not yet in memory.
 */
async function csrfHeaders(extra: Record<string, string> = {}): Promise<Record<string, string>> {
  let token = getCsrfToken()
  if (!token) {
    token = await fetchCsrfToken()
  }
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...extra,
  }
  if (token) {
    headers['X-CSRF-Token'] = token
  }
  return headers
}

/**
 * Generic helper for auth POST endpoints with CSRF + optional 403-retry.
 */
async function authPost<T>(
  path: string,
  body: unknown,
  retried = false
): Promise<T> {
  const headers = await csrfHeaders()
  const response = await fetch(`${API_BASE_URL}${path}`, {
    method: 'POST',
    headers,
    body: JSON.stringify(body),
    credentials: 'include',
  })

  if (!response.ok) {
    let errorDetails: unknown
    try {
      errorDetails = await response.json()
    } catch {
      errorDetails = null
    }

    // 403 with CSRF error: refetch token and retry once
    if (
      response.status === 403 &&
      !retried &&
      errorDetails !== null &&
      typeof errorDetails === 'object' &&
      'error' in errorDetails &&
      typeof (errorDetails as any).error === 'string' &&
      (errorDetails as any).error.toLowerCase().includes('csrf')
    ) {
      await fetchCsrfToken()
      return authPost<T>(path, body, true)
    }

    throw new ApiError(
      response.status,
      errorDetails,
      errorDetails && typeof errorDetails === 'object' && 'message' in errorDetails
        ? (errorDetails as any).message
        : `Request to ${path} failed`
    )
  }

  return response.json()
}

/**
 * Login with email and password
 *
 * @param email User email address
 * @param password User password
 * @returns Promise with access_token and user profile, OR mfa_required + challenge_token
 * @throws ApiError on non-2xx response
 */
export async function login(
  email: string,
  password: string
): Promise<LoginResponse> {
  return authPost<LoginResponse>('/api/v1/auth/login', { email, password })
}

/**
 * Complete MFA challenge after receiving mfa_required=true from login.
 *
 * @param challengeToken Opaque token from LoginResponse.challenge_token
 * @param code 6-digit TOTP code entered by the user
 * @returns Full LoginResponse (access_token + user)
 * @throws ApiError on non-2xx response
 */
export async function mfaChallenge(
  challengeToken: string,
  code: string
): Promise<LoginResponse> {
  return authPost<LoginResponse>('/api/v1/auth/mfa/challenge', {
    challenge_token: challengeToken,
    code,
  })
}

/**
 * Enroll MFA — returns secret, QR code data URL, and backup codes.
 * Requires a valid JWT (authenticated request).
 *
 * @returns Enrollment payload with TOTP secret and backup codes
 * @throws ApiError on non-2xx response
 */
export async function mfaEnroll(): Promise<MfaEnrollResponse> {
  return authPost<MfaEnrollResponse>('/api/v1/auth/mfa/enroll', {})
}

/**
 * Verify a TOTP code to activate MFA after enrollment.
 *
 * @param code 6-digit TOTP code from authenticator app
 * @returns { enabled: true } on success
 * @throws ApiError on non-2xx response
 */
export async function mfaVerify(code: string): Promise<{ enabled: true }> {
  return authPost<{ enabled: true }>('/api/v1/auth/mfa/verify', { code })
}

/**
 * Logout - revoke refresh token on server
 *
 * Always clears local state even if server call fails.
 * @throws ApiError on network error (logged but not thrown to caller)
 */
export async function logout(): Promise<void> {
  try {
    const response = await fetch(`${API_BASE_URL}/api/v1/auth/logout`, {
      method: 'POST',
      credentials: 'include', // Include refresh token cookie
    })

    if (!response.ok) {
      console.warn('Logout endpoint returned non-2xx status:', response.status)
    }
  } catch (error) {
    console.error('Logout request failed:', error)
    // Ignore errors - local state will be cleared by caller
  }
}

/**
 * Refresh access token using httpOnly refresh token cookie
 *
 * @returns Promise with new access_token
 * @throws ApiError if refresh fails (token expired or invalid)
 */
export async function refreshToken(): Promise<RefreshResponse> {
  const response = await fetch(`${API_BASE_URL}/api/v1/auth/refresh`, {
    method: 'POST',
    credentials: 'include', // Send httpOnly refresh token cookie
  })

  if (!response.ok) {
    let errorDetails: unknown
    try {
      errorDetails = await response.json()
    } catch {
      errorDetails = null
    }
    throw new ApiError(
      response.status,
      errorDetails,
      'Token refresh failed'
    )
  }

  return response.json()
}

/**
 * Fetch current user profile
 *
 * Used to validate session on app load.
 * @param token - Optional access token (if not provided, will look for it in auth header)
 * @returns Promise with current user profile
 * @throws ApiError on non-2xx response (401 means session expired)
 */
export async function getProfile(token?: string): Promise<LoginResponse['user']> {
  const headers: Record<string, string> = {}

  // Only add Authorization header if token is provided
  if (token) {
    headers['Authorization'] = `Bearer ${token}`
  }

  const response = await fetch(`${API_BASE_URL}/api/v1/auth/me`, {
    method: 'GET',
    headers,
    credentials: 'include',
  })

  if (!response.ok) {
    let errorDetails: unknown
    try {
      errorDetails = await response.json()
    } catch {
      errorDetails = null
    }
    throw new ApiError(
      response.status,
      errorDetails,
      response.status === 401 ? 'Session expired' : 'Failed to fetch profile'
    )
  }

  return response.json()
}
