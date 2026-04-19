import { useAuthStore } from '../stores/auth'

/**
 * Wrapper around fetch that automatically includes JWT token in Authorization header
 */
export async function apiRequest(
  url: string,
  options: RequestInit = {}
): Promise<Response> {
  const authStore = useAuthStore.getState()

  const headers = new Headers(options.headers || {})

  // Add Authorization header if token exists
  if (authStore.token) {
    headers.set('Authorization', `Bearer ${authStore.token}`)
  }

  // Add Content-Type if not already set
  if (!headers.has('Content-Type') && options.body) {
    headers.set('Content-Type', 'application/json')
  }

  const response = await fetch(url, {
    ...options,
    headers,
    credentials: 'include' // Include cookies for refresh token
  })

  // Handle 401 - token may have expired
  if (response.status === 401) {
    console.warn('Received 401 - attempting token refresh')
    try {
      // Try to refresh the token silently
      const refreshResponse = await fetch('/api/v1/auth/refresh', {
        method: 'POST',
        credentials: 'include'
      })

      if (refreshResponse.ok) {
        const data = await refreshResponse.json()
        authStore.setAccessToken(data.access_token)

        // Retry the original request with new token
        headers.set('Authorization', `Bearer ${data.access_token}`)
        return fetch(url, {
          ...options,
          headers,
          credentials: 'include'
        })
      } else {
        // Refresh failed - user needs to login again
        authStore.logout()
      }
    } catch (error) {
      console.error('Token refresh failed:', error)
      authStore.logout()
    }
  }

  return response
}
