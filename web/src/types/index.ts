// Layer type representing the 10-layer LLM system taxonomy
export type Layer =
  | 'L1_HARDWARE'
  | 'L2_MODEL_WEIGHTS'
  | 'L3_TOKENIZER'
  | 'L4_TRANSFORMER'
  | 'L5_OUTPUT_DECODING'
  | 'L6_SAFETY'
  | 'L7_RAG_RETRIEVAL'
  | 'L8_AGENTS'
  | 'L9_API_GATEWAY'
  | 'L10_APPLICATION'

export const LAYERS: Layer[] = [
  'L1_HARDWARE',
  'L2_MODEL_WEIGHTS',
  'L3_TOKENIZER',
  'L4_TRANSFORMER',
  'L5_OUTPUT_DECODING',
  'L6_SAFETY',
  'L7_RAG_RETRIEVAL',
  'L8_AGENTS',
  'L9_API_GATEWAY',
  'L10_APPLICATION'
]

// Helper to convert numeric layer (1-10) to Layer name
export function getLayerName(layerNum: number): Layer {
  return LAYERS[layerNum - 1] || 'L10_APPLICATION'
}

// ArgusSignal matches backend wire format - core signal envelope
export interface ArgusSignal {
  signal_id: string // ULID
  trace_id: string // UUID
  span_id: string
  layer: number // 1-10 (maps to L1-L10)
  category: string
  severity: number // 1-5
  timestamp: { seconds: number; nanos: number } | string // ISO8601 or protobuf timestamp
  ingested_at?: { seconds: number; nanos: number }
  source?: {
    app_id: string
    app_version: string
    sdk_version: string
    environment: string
  }
  message?: string
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
