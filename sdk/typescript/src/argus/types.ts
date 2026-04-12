/**
 * Types for Argus SDK
 */

export enum Layer {
  L1_HARDWARE = 1,
  L2_MODEL_WEIGHTS = 2,
  L3_TOKENIZER = 3,
  L4_TRANSFORMER = 4,
  L5_OUTPUT_DECODING = 5,
  L6_SAFETY = 6,
  L7_RAG_RETRIEVAL = 7,
  L8_AGENTS = 8,
  L9_API_GATEWAY = 9,
  L10_APPLICATION = 10,
}

export enum Severity {
  INFO = 1,
  LOW = 2,
  MEDIUM = 3,
  HIGH = 4,
  CRITICAL = 5,
}

export interface SignalContext {
  [key: string]: any;
}

export interface ArgusSignal {
  signal_id: string;
  trace_id: string;
  span_id: string;
  parent_span_id?: string;
  layer: number;
  category: string;
  severity: number;
  timestamp: string; // ISO 8601
  duration_ms?: number;
  source: {
    app_id: string;
    app_version: string;
    sdk_version: string;
    environment: string;
    instance_id: string;
  };
  context?: SignalContext;
}

export interface ClientConfig {
  baseUrl?: string;
  appId?: string;
  appVersion?: string;
  sdkVersion?: string;
  environment?: string;
  timeout?: number;
  bufferSize?: number;
  flushInterval?: number;
}

export interface BufferStats {
  totalEmitted: number;
  totalDropped: number;
  bufferSize: number;
  lastFlush?: number;
}
