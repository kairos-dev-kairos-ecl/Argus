/**
 * Authentication Service Layer
 *
 * Provides async functions for authentication operations:
 * - login: Submit credentials, receive JWT and user profile
 * - logout: Revoke refresh token and clear session
 * - refreshToken: Silent token refresh using httpOnly cookie
 * - getProfile: Fetch current user profile to verify session
 */

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
}

export interface RefreshResponse {
  access_token: string
}

export interface LogoutResponse {
  message: string
}

export class ApiError extends Error {
  constructor(
    public status: number,
    public details?: unknown,
    message?: string
  ) {
    super(message || `API Error: ${status}`)
    this.name = 'ApiError'
  }
}

/**
 * Login with email and password
 *
 * @param email User email address
 * @param password User password
 * @returns Promise with access_token and user profile
 * @throws ApiError on non-2xx response
 */
export async function login(
  email: string,
  password: string
): Promise<LoginResponse> {
  const response = await fetch('/api/v1/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password }),
    credentials: 'include', // Include cookies for any existing refresh token
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
      errorDetails && typeof errorDetails === 'object' && 'message' in errorDetails
        ? (errorDetails as any).message
        : 'Login failed'
    )
  }

  return response.json()
}

/**
 * Logout - revoke refresh token on server
 *
 * Always clears local state even if server call fails.
 * @throws ApiError on network error (logged but not thrown to caller)
 */
export async function logout(): Promise<void> {
  try {
    const response = await fetch('/api/v1/auth/logout', {
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
  const response = await fetch('/api/v1/auth/refresh', {
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
 * @returns Promise with current user profile
 * @throws ApiError on non-2xx response (401 means session expired)
 */
export async function getProfile(): Promise<LoginResponse['user']> {
  const authStore = await import('../stores/auth').then((m) => m.useAuthStore.getState())

  const response = await fetch('/api/v1/auth/me', {
    method: 'GET',
    headers: {
      'Authorization': `Bearer ${authStore.token || ''}`,
    },
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
