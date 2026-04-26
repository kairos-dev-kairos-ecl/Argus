/**
 * Core API Service Layer
 *
 * Provides typed async functions for all Argus backend endpoints.
 * Automatically handles:
 * - JWT token injection from auth store
 * - Error transformation to ApiError
 * - Network error handling with helpful messages
 * - 401 token refresh (delegated to apiRequest wrapper)
 */

import { useAuthStore } from '../stores/auth'
import { ApiError } from './types'
import { getCsrfToken, fetchCsrfToken } from './csrf'
import type {
  SignalsRequest,
  SignalsResponse,
  LayerStatusResponse,
  TraceResponse,
  QueryRequest,
  QueryResponse,
  AlertsResponse,
  AggregationsResponse,
} from './types'

// ============================================================================
// CONFIGURATION
// ============================================================================

// Relative paths — Vite proxies to :8082 in dev, Go serves same origin in prod
const API_BASE_URL = ''

// ============================================================================
// HTTP REQUEST HELPER
// ============================================================================

/**
 * Internal options extended with re-entry guards.
 */
interface HttpRequestOptions extends RequestInit {
  /** Set to true to prevent infinite loop on 401 refresh retry */
  _refreshRetried?: boolean
  /** Set to true to prevent infinite loop on 403 CSRF retry */
  _csrfRetried?: boolean
}

/**
 * Core HTTP request wrapper with auth token injection and error handling.
 *
 * Handles:
 * - JWT Authorization header injection
 * - CSRF token injection for mutating requests (POST/PUT/DELETE/PATCH)
 * - 401 token refresh with single retry
 * - 429 rate-limit surfacing (Retry-After header → ApiError.details.retry_after_seconds)
 * - 403 CSRF token expiry with single refetch + retry
 *
 * @param url - Relative URL (e.g., '/v1/signals')
 * @param options - Fetch options (extended with _csrfRetried guard)
 * @returns Parsed response or throws ApiError
 */
async function httpRequest<T>(
  url: string,
  options: HttpRequestOptions = {}
): Promise<T> {
  const authStore = useAuthStore.getState()
  const fullUrl = `${API_BASE_URL}${url}`

  const headers = new Headers(options.headers || {})

  // Inject Authorization header
  if (authStore.token) {
    headers.set('Authorization', `Bearer ${authStore.token}`)
  }

  // Determine HTTP method
  const method = (options.method || 'GET').toUpperCase()

  // Inject CSRF token for state-mutating methods
  if (method !== 'GET' && method !== 'HEAD') {
    let token = getCsrfToken()
    if (!token) {
      token = await fetchCsrfToken()
    }
    if (token) {
      headers.set('X-CSRF-Token', token)
    }
  }

  // Set Content-Type for JSON if body is present
  if (!headers.has('Content-Type') && options.body) {
    headers.set('Content-Type', 'application/json')
  }

  try {
    const response = await fetch(fullUrl, {
      ...options,
      headers,
      credentials: 'include', // Include httpOnly cookies for refresh token
    })

    // Handle 429 - rate limited
    if (response.status === 429) {
      const retryAfter = response.headers.get('Retry-After')
      const retryAfterSeconds = parseInt(retryAfter || '60', 10)
      throw new ApiError(
        429,
        { retry_after_seconds: retryAfterSeconds },
        `Rate limited — retry in ${retryAfterSeconds}s`
      )
    }

    // Handle 401 - trigger token refresh (single retry)
    if (response.status === 401 && !options._refreshRetried) {
      try {
        const refreshResponse = await fetch(`${API_BASE_URL}/api/v1/auth/refresh`, {
          method: 'POST',
          credentials: 'include',
        })

        if (refreshResponse.ok) {
          const data = await refreshResponse.json()
          authStore.setAccessToken(data.access_token)

          // Retry request with new token (mark as retried to avoid loop)
          return httpRequest<T>(url, {
            ...options,
            _refreshRetried: true,
          })
        } else {
          // Refresh failed - logout user
          await authStore.logout()
          throw new ApiError(401, null, 'Session expired - please login again')
        }
      } catch (error) {
        if (error instanceof ApiError) throw error
        await authStore.logout()
        throw new ApiError(401, null, 'Token refresh failed')
      }
    }

    // Parse response body
    let data: unknown
    try {
      data = await response.json()
    } catch {
      data = null
    }

    // Handle 403 - may be a CSRF token expiry (single retry)
    if (
      response.status === 403 &&
      !options._csrfRetried &&
      data !== null &&
      typeof data === 'object' &&
      'error' in data &&
      typeof (data as any).error === 'string' &&
      (data as any).error.toLowerCase().includes('csrf')
    ) {
      // Refetch CSRF token and retry once
      await fetchCsrfToken()
      return httpRequest<T>(url, {
        ...options,
        _csrfRetried: true,
      })
    }

    // Check for other error statuses
    if (!response.ok) {
      console.error(`HTTP ${response.status}:`, data)
      throw new ApiError(
        response.status,
        data,
        getErrorMessage(response.status, data)
      )
    }

    return data as T
  } catch (error) {
    // Handle network errors
    if (error instanceof ApiError) {
      throw error
    }

    if (error instanceof TypeError) {
      console.error('Network error:', error)
      throw new ApiError(
        0,
        error,
        'Network error - backend may be offline'
      )
    }

    throw new ApiError(500, error, 'Unexpected error')
  }
}

/**
 * Extract helpful error message from API response.
 */
function getErrorMessage(status: number, data: unknown): string {
  if (typeof data === 'object' && data !== null && 'message' in data) {
    return (data as any).message
  }

  const messages: Record<number, string> = {
    400: 'Bad request',
    401: 'Unauthorized',
    403: 'Access denied',
    404: 'Not found',
    408: 'Request timeout',
    429: 'Too many requests',
    500: 'Server error',
    503: 'Service unavailable',
  }

  return messages[status] || `HTTP ${status}`
}

// ============================================================================
// SIGNALS API
// ============================================================================

/**
 * Fetch signals with optional filters and pagination.
 *
 * @param request - Filter and pagination parameters
 * @returns Paginated signals list with next cursor
 * @throws ApiError on failure
 *
 * Endpoint: GET /v1/signals?{params}
 */
export async function getSignals(request: SignalsRequest = {}): Promise<SignalsResponse> {
  const params = new URLSearchParams()

  if (request.app_id) params.set('app_id', request.app_id)
  if (request.layer) params.set('layer', String(request.layer))
  if (request.category) params.set('category', request.category)
  if (request.severity) params.set('severity', String(request.severity))
  if (request.start) params.set('start', request.start)
  if (request.end) params.set('end', request.end)
  if (request.cursor) params.set('cursor', request.cursor)
  if (request.limit) params.set('limit', String(request.limit))

  const queryString = params.toString()
  const url = queryString ? `/v1/signals?${queryString}` : '/v1/signals'

  return httpRequest<SignalsResponse>(url)
}

// ============================================================================
// LAYER STATUS API
// ============================================================================

/**
 * Fetch layer status summary across 10-layer system.
 *
 * @returns Layer status with signal counts and health indicators
 * @throws ApiError on failure
 *
 * Endpoint: GET /api/v1/layers/status
 */
export async function getLayerStatus(): Promise<LayerStatusResponse> {
  return httpRequest<LayerStatusResponse>('/api/v1/layers/status')
}

// ============================================================================
// TRACE API
// ============================================================================

/**
 * Fetch complete trace with spans and detections.
 *
 * @param traceId - Trace identifier (UUID)
 * @returns Trace with all spans and correlated detections
 * @throws ApiError on failure (404 if trace not found)
 *
 * Endpoint: GET /api/v1/traces/{traceId}
 */
export async function getTrace(traceId: string): Promise<TraceResponse> {
  if (!traceId) {
    throw new ApiError(400, null, 'Trace ID is required')
  }
  return httpRequest<TraceResponse>(`/api/v1/traces/${traceId}`)
}

// ============================================================================
// QUERY API
// ============================================================================

/**
 * Execute ClickHouse SQL-like query.
 *
 * @param request - Query string
 * @returns Query results with execution metadata
 * @throws ApiError on failure (syntax error, timeout, permission denied)
 *
 * Endpoint: POST /api/v1/query
 */
export async function executeQuery(request: QueryRequest): Promise<QueryResponse> {
  if (!request.query) {
    throw new ApiError(400, null, 'Query string is required')
  }

  return httpRequest<QueryResponse>('/api/v1/query', {
    method: 'POST',
    body: JSON.stringify({ sql: request.sql ?? request.query }),
  })
}

// ============================================================================
// ALERTS API
// ============================================================================

/**
 * Fetch alerts (detections) with pagination.
 *
 * @param limit - Page size (default 50)
 * @param offset - Pagination offset (default 0)
 * @returns Paginated alerts list
 * @throws ApiError on failure
 *
 * Endpoint: GET /api/v1/alerts?limit={limit}&offset={offset}
 */
export async function getAlerts(
  limit: number = 50,
  offset: number = 0
): Promise<AlertsResponse> {
  const params = new URLSearchParams()
  params.set('limit', String(limit))
  params.set('offset', String(offset))

  return httpRequest<AlertsResponse>(`/api/v1/alerts?${params.toString()}`)
}

// ============================================================================
// AGGREGATIONS API
// ============================================================================

/**
 * Fetch aggregated signal counts by layer.
 *
 * @returns Map of layer names to signal counts
 * @throws ApiError on failure
 *
 * Endpoint: GET /api/v1/aggregations/by-layer
 */
export async function getAggregationsByLayer(): Promise<AggregationsResponse> {
  return httpRequest<AggregationsResponse>('/api/v1/aggregations/by-layer')
}

/**
 * Fetch aggregated signal counts by model.
 *
 * @returns Map of model identifiers to signal counts
 * @throws ApiError on failure
 *
 * Endpoint: GET /api/v1/aggregations/by-model
 */
export async function getAggregationsByModel(): Promise<AggregationsResponse> {
  return httpRequest<AggregationsResponse>('/api/v1/aggregations/by-model')
}

/**
 * Generic aggregation fetcher for flexibility.
 * For MVP, returns mock data if endpoint not yet implemented.
 *
 * @param type - Aggregation type ('by-layer' | 'by-model')
 * @returns Aggregation data
 * @throws ApiError on failure
 */
export async function getAggregations(
  type: 'by-layer' | 'by-model'
): Promise<Record<string, number>> {
  if (type === 'by-layer') {
    return getAggregationsByLayer()
  } else if (type === 'by-model') {
    return getAggregationsByModel()
  }

  throw new ApiError(400, null, `Unknown aggregation type: ${type}`)
}

// ============================================================================
// HEALTH CHECK
// ============================================================================

/**
 * Check backend connectivity.
 *
 * @returns True if backend is healthy
 * @throws ApiError if backend is unreachable
 *
 * Endpoint: GET /health
 */
export async function healthCheck(): Promise<boolean> {
  try {
    const response = await fetch(`${API_BASE_URL}/health`, {
      method: 'GET',
      credentials: 'include',
    })
    return response.ok
  } catch {
    console.warn('Health check failed - backend may be offline')
    return false
  }
}
