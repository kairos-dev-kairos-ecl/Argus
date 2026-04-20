import { create } from 'zustand'
import type { User, AuthState } from '../types'
import * as authService from '../services/auth-service'

interface AuthStateStore extends AuthState {
  // State
  loading: boolean
  error: string | null
  // Actions
  login(email: string, password: string): Promise<void>
  logout(): Promise<void>
  refreshToken(): Promise<void>
  validateSession(): Promise<void>
  setUser(user: User | null): void
  setAccessToken(token: string): void
  clearError(): void
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

  login: async (email: string, password: string) => {
    set({ loading: true, error: null })
    try {
      const { access_token, user } = await authService.login(email, password)
      set({
        token: access_token,
        user: user as User,
        is_authenticated: true,
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
      set({ token: access_token })
    } catch (error) {
      // Silently fail - refresh token may be expired
      // Caller can redirect to login if needed
      set({ is_authenticated: false, token: null })
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
