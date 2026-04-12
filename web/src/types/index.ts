// Layer type representing the 10-layer LLM system taxonomy
export type Layer =
  | 'L1_HARDWARE'
  | 'L2_KERNEL'
  | 'L3_RUNTIME'
  | 'L4_NETWORK'
  | 'L5_CONTAINER'
  | 'L6_MIDDLEWARE'
  | 'L7_APPLICATION'
  | 'L8_LLM_FRAMEWORK'
  | 'L9_LLM_API'
  | 'L10_SEMANTIC'

export const LAYERS: Layer[] = [
  'L1_HARDWARE',
  'L2_KERNEL',
  'L3_RUNTIME',
  'L4_NETWORK',
  'L5_CONTAINER',
  'L6_MIDDLEWARE',
  'L7_APPLICATION',
  'L8_LLM_FRAMEWORK',
  'L9_LLM_API',
  'L10_SEMANTIC'
]

// ArgusSignal matches backend wire format - core signal envelope
export interface ArgusSignal {
  signal_id: string // ULID
  trace_id: string // UUID
  layer: Layer
  category: string
  severity: number // 1-5
  timestamp: string // ISO8601
  source_app_id: string
  message: string
  enrichment?: {
    baseline_deviation: number | null // z-score
    correlation_tags: string[]
    geoip_location?: string
  }
  context?: Record<string, unknown>
}

// Layer status for coverage map
export interface LayerStatus {
  layer: Layer
  status: 'green' | 'yellow' | 'gray' | 'red'
  last_signal_time: string | null
  signal_count_5min: number
  error_message?: string
}

// User and RBAC
export interface User {
  id: string
  email: string
  display_name: string
  role: 'admin' | 'analyst' | 'viewer'
  permissions: string[]
  status: 'active' | 'suspended' | 'pending_invite'
  last_login_at?: string
  created_at: string
}

export interface AuthState {
  user: User | null
  token: string | null
  is_authenticated: boolean
  loading?: boolean
  error?: string | null
}

// Signal stream filter
export interface SignalFilter {
  layers?: Layer[]
  severity_min?: number
  category?: string
  search?: string
  time_range?: {
    from: string // ISO8601
    to: string // ISO8601
  }
}

// Query result
export interface QueryResult {
  rows: Record<string, unknown>[]
  cursor?: string
  total: number
  execution_time_ms: number
}

// Detection result
export interface Detection {
  detection_id: string
  rule_id: string
  rule_name: string
  signal_id: string
  confidence: number
  severity: number
  matched_at: string
}

// Trace view data
export interface Trace {
  trace_id: string
  spans: Span[]
  detections: Detection[]
  duration_ms: number
}

export interface Span {
  signal_id: string
  layer: Layer
  start_time: string // ISO8601
  duration_ms: number
  parent_signal_id?: string
  status: 'ok' | 'error'
  message: string
}

// Config types
export interface DetectionRule {
  id: string
  name: string
  description: string
  yaml_content: string
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface HuntPlaybook {
  id: string
  name: string
  description: string
  sql_query: string
  tags: string[]
  created_at: string
  updated_at: string
}
