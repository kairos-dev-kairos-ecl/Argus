import { create } from 'zustand'
import type { User, AuthState } from '../types'
import * as authService from '../services/auth-service'

/**
 * Decode user claims from a JWT access token without verifying the signature.
 * Safe here because the token was issued by our own backend — we're just
 * reading the payload that the backend already validated and signed.
 */
function decodeUserFromJWT(token: string): User | null {
  try {
    const payload = JSON.parse(atob(token.split('.')[1]))
    return {
      id: payload.sub,
      email: payload.email,
      display_name: payload.name,
      role: payload.role,
      permissions: payload.permissions ?? [],
      status: 'active',
      created_at: new Date(payload.iat * 1000).toISOString(),
    } as User
  } catch {
    return null
  }
}

interface AuthStateStore extends AuthState {
  // State
  loading: boolean
  error: string | null
  csrfToken: string | null
  mfaPending: { challengeToken: string; email: string } | null
  // Actions
  login(email: string, password: string): Promise<void>
  logout(): Promise<void>
  refreshToken(): Promise<void>
  validateSession(): Promise<void>
  setUser(user: User | null): void
  setAccessToken(token: string): void
  clearError(): void
  setMfaPending(state: { challengeToken: string; email: string } | null): void
  completeMfa(code: string): Promise<void>
  // Checks
  hasPermission(action: string): boolean
  isAdmin(): boolean
  isAnalyst(): boolean
  isViewer(): boolean
  canAccess(requiredRole: 'admin' | 'analyst' | 'viewer'): boolean
}

/**
 * Auth Store (Wave 4)
 * Manages authentication state with JWT access tokens (memory) and HttpOnly refresh tokens (server).
 *
 * Architecture:
 * - Access Token: Short-lived (15m), stored in memory, sent in Authorization header
 * - Refresh Token: Long-lived (7d), HttpOnly Secure cookie, never exposed to JavaScript
 * - Token Refresh: Automatic on 401, transparent to caller
 * - Revocation: Immediate via /api/v1/auth/logout, cached in Redis
 */
export const useAuthStore = create<AuthStateStore>((set, get) => ({
  user: null,
  is_authenticated: false,
  token: null, // In-memory access token only (never stored in localStorage)
  loading: false,
  error: null,
  csrfToken: null,
  mfaPending: null,

  login: async (email: string, password: string) => {
    set({ loading: true, error: null })
    try {
      const response = await authService.login(email, password)

      if (response.mfa_required) {
        // MFA is required — store pending state and wait for completeMfa()
        set({
          mfaPending: {
            challengeToken: response.challenge_token ?? '',
            email,
          },
          is_authenticated: false,
          token: null,
          error: null,
        })
        return
      }

      // Normal login — decode user claims from the JWT payload
      const user = decodeUserFromJWT(response.access_token)
      set({
        token: response.access_token,
        user,
        is_authenticated: true,
        mfaPending: null,
        error: null,
      })
      // Refresh token is set as HttpOnly cookie by server
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Login failed'
      set({ error: message, is_authenticated: false })
      throw error
    } finally {
      set({ loading: false })
    }
  },

  setMfaPending: (state) => {
    set({ mfaPending: state })
  },

  completeMfa: async (code: string) => {
    const { mfaPending } = get()
    if (!mfaPending) throw new Error('No MFA challenge pending')
    set({ loading: true, error: null })
    try {
      const response = await authService.mfaChallenge(mfaPending.challengeToken, code)
      const user = decodeUserFromJWT(response.access_token)
      set({
        token: response.access_token,
        user,
        is_authenticated: true,
        mfaPending: null,
        error: null,
      })
    } catch (error) {
      const message = error instanceof Error ? error.message : 'MFA verification failed'
      set({ error: message })
      throw error
    } finally {
      set({ loading: false })
    }
  },

  logout: async () => {
    set({ loading: true })
    try {
      // Call logout endpoint to revoke refresh token on server
      await authService.logout()
    } catch (error) {
      console.error('Logout error:', error)
    } finally {
      set({
        user: null,
        token: null,
        is_authenticated: false,
        loading: false,
      })
    }
  },

  refreshToken: async () => {
    try {
      const { access_token } = await authService.refreshToken()
      // Decode user from token so ProtectedRoute has role/permissions immediately
      const user = decodeUserFromJWT(access_token)
      set({ token: access_token, user, is_authenticated: true })
    } catch (error) {
      // Refresh token expired or not present — user must log in again
      set({ is_authenticated: false, token: null, user: null })
    }
  },

  validateSession: async () => {
    try {
      // Try to refresh token first (using httpOnly cookie)
      const { access_token } = await authService.refreshToken()
      // If refresh succeeded, verify user is still valid
      const user = await authService.getProfile(access_token)
      set({ token: access_token, user: user as User, is_authenticated: true })
    } catch (error) {
      // Session invalid or expired
      set({
        user: null,
        token: null,
        is_authenticated: false,
      })
    }
  },

  setAccessToken: (token: string) => {
    set({ token })
  },

  setUser: (user: User | null) => {
    set({ user, is_authenticated: !!user })
  },

  clearError: () => {
    set({ error: null })
  },

  hasPermission: (action: string): boolean => {
    const { user } = get()
    if (!user || !('permissions' in user)) return false
    return (user as any).permissions?.includes(action) ?? false
  },

  isAdmin: (): boolean => {
    const { user } = get()
    return user?.role === 'admin'
  },

  isAnalyst: (): boolean => {
    const { user } = get()
    return user?.role === 'analyst'
  },

  isViewer: (): boolean => {
    const { user } = get()
    return user?.role === 'viewer'
  },

  canAccess: (requiredRole: 'admin' | 'analyst' | 'viewer'): boolean => {
    const { user } = get()
    if (!user) return false

    // Hierarchical check (admin > analyst > viewer for read operations)
    const hierarchy: Record<string, number> = { viewer: 1, analyst: 2, admin: 3 }
    return (hierarchy[user.role] ?? 0) >= (hierarchy[requiredRole] ?? 0)
  },
}))
