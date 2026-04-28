/**
 * API Service Type Definitions
 *
 * Request and response types for all Argus backend endpoints.
 * Ensures type safety across the frontend service layer.
 */

import type {
  ArgusSignal,
  LayerStatus,
  Detection,
  Layer,
} from '../types'

// ============================================================================
// REQUEST TYPES (Query Parameters)
// ============================================================================

export interface SignalsRequest {
  app_id?: string
  layer?: number | Layer
  category?: string
  severity?: number
  start?: string // ISO8601
  end?: string // ISO8601
  cursor?: string
  limit?: number
}

export interface LayerStatusRequest {
  // No parameters
}

export interface TraceRequest {
  traceId: string
}

export interface QueryRequest {
  query: string
  sql?: string
}

export interface AlertsRequest {
  limit?: number
  offset?: number
  status?: 'resolved' | 'open' | 'acknowledged'
}

// ============================================================================
// RESPONSE TYPES (API Responses)
// ============================================================================

/**
 * Response from GET /v1/signals
 * Returns paginated signal list with cursor for pagination.
 */
export interface SignalsResponse {
  signals: ArgusSignal[]
  next_cursor?: string
  total_hint: number
}

/**
 * Response from GET /api/v1/layers/status
 * Layer coverage and status across 10-layer system.
 */
export interface LayerStatusResponse {
  layers: LayerStatus[]
}

/**
 * Span shape returned by GET /api/v1/traces/{traceId}.
 * Uses numeric layer (1-10) and millisecond-epoch start_ms for waterfall math.
 */
export interface TraceSpan {
  span_id: string
  parent_span_id?: string
  name: string
  start_ms: number
  duration_ms: number
  /** Numeric layer 1-10 matching --color-layer-l{n} CSS variables */
  layer: number
  category: string
  prompt?: string
  response?: string
  tokens?: number
  ttft_ms?: number
  gpu_util?: number
}

/**
 * Response from GET /api/v1/traces/{traceId}
 * Complete trace with spans and detections.
 */
export interface TraceResponse {
  trace_id: string
  spans: TraceSpan[]
  detections: Detection[]
  duration_ms: number
}

/**
 * Response from POST /api/v1/query
 * Query execution results with execution metadata.
 */
export interface QueryResponse {
  rows: Record<string, any>[]
  cursor?: string
  total: number
  execution_time_ms: number
}

/**
 * Response from GET /api/v1/alerts
 * Paginated alerts list.
 */
export interface AlertsResponse {
  alerts: Detection[]
  total: number
}

/**
 * Response from GET /api/v1/aggregations
 * Pre-aggregated signal counts.
 */
export interface AggregationsResponse {
  [key: string]: number
}

// ============================================================================
// ERROR HANDLING
// ============================================================================

/**
 * Standard API error with status code and details.
 * Extends Error for proper error handling in catch blocks.
 */
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

// ============================================================================
// TYPE GUARDS
// ============================================================================

export function isSignalsResponse(data: unknown): data is SignalsResponse {
  return (
    typeof data === 'object' &&
    data !== null &&
    'signals' in data &&
    Array.isArray((data as any).signals)
  )
}

export function isLayerStatusResponse(data: unknown): data is LayerStatusResponse {
  return (
    typeof data === 'object' &&
    data !== null &&
    'layers' in data &&
    Array.isArray((data as any).layers)
  )
}

export function isTraceResponse(data: unknown): data is TraceResponse {
  return (
    typeof data === 'object' &&
    data !== null &&
    'trace_id' in data &&
    'spans' in data &&
    Array.isArray((data as any).spans)
  )
}

export function isQueryResponse(data: unknown): data is QueryResponse {
  return (
    typeof data === 'object' &&
    data !== null &&
    'rows' in data &&
    Array.isArray((data as any).rows) &&
    'execution_time_ms' in data
  )
}

export function isAlertsResponse(data: unknown): data is AlertsResponse {
  return (
    typeof data === 'object' &&
    data !== null &&
    'alerts' in data &&
    Array.isArray((data as any).alerts) &&
    'total' in data
  )
}
