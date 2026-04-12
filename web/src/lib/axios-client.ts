import axios, { type AxiosInstance, type AxiosError, type InternalAxiosRequestConfig } from 'axios'
import { useAuthStore } from '../stores/auth'

/**
 * Create and configure Axios client for API requests
 * Handles:
 * - Automatic Authorization header injection
 * - Transparent token refresh on 401
 * - Request/response interceptors
 */
export function createApiClient(): AxiosInstance {
  const client = axios.create({
    baseURL: '/api/v1',
    withCredentials: true, // Include cookies (HttpOnly refresh token)
  })

  // Track if we're currently refreshing to prevent infinite loops
  let isRefreshing = false
  let failedQueue: Array<{ resolve: Function; reject: Function }> = []

  const processQueue = (error: Error | null = null) => {
    failedQueue.forEach((prom) => {
      if (error) {
        prom.reject(error)
      } else {
        prom.resolve(null)
      }
    })
    isRefreshing = false
    failedQueue = []
  }

  // Request interceptor: Add Authorization header
  client.interceptors.request.use(
    (config: InternalAxiosRequestConfig) => {
      const authStore = useAuthStore.getState()
      if (authStore.token) {
        config.headers.Authorization = `Bearer ${authStore.token}`
      }
      return config
    },
    (error) => {
      return Promise.reject(error)
    }
  )

  // Response interceptor: Handle 401 and refresh token
  client.interceptors.response.use(
    (response) => response,
    async (error: AxiosError) => {
      const originalRequest = error.config as InternalAxiosRequestConfig & {
        _retry?: boolean
      }

      // If error is 401 and we haven't retried yet
      if (error.response?.status === 401 && !originalRequest._retry) {
        if (isRefreshing) {
          // Queue the request until refresh completes
          return new Promise((resolve, reject) => {
            failedQueue.push({
              resolve: () => resolve(client(originalRequest)),
              reject,
            })
          })
        }

        originalRequest._retry = true
        isRefreshing = true

        try {
          // Try to refresh token
          const authStore = useAuthStore.getState()
          await authStore.refreshToken()

          // Retry original request with new token
          processQueue()
          return client(originalRequest)
        } catch (refreshError) {
          // Refresh failed, redirect to login
          const authStore = useAuthStore.getState()
          authStore.logout()
          processQueue(refreshError as Error)
          window.location.href = '/login'
          return Promise.reject(refreshError)
        }
      }

      return Promise.reject(error)
    }
  )

  return client
}

export const apiClient = createApiClient()
