/**
 * ArgusClient - Async HTTP client for emitting signals to Argus
 */

import axios, { AxiosInstance } from 'axios';
import { v4 as uuidv4 } from 'uuid';
import { Layer, Severity, ArgusSignal, SignalContext } from './types';

export class ArgusClient {
  private baseUrl: string;
  private appId: string;
  private appVersion: string;
  private sdkVersion: string;
  private environment: string;
  private timeout: number;
  private instanceId: string;
  private client: AxiosInstance | null = null;
  private traceId: string | null = null;

  constructor(
    baseUrl: string = 'http://localhost:8080',
    appId: string = 'test-app',
    appVersion: string = '0.1.0',
    sdkVersion: string = '0.1.0',
    environment: string = 'test',
    timeout: number = 30000,
  ) {
    this.baseUrl = baseUrl.replace(/\/$/, '');
    this.appId = appId;
    this.appVersion = appVersion;
    this.sdkVersion = sdkVersion;
    this.environment = environment;
    this.timeout = timeout;
    this.instanceId = uuidv4().substring(0, 8);
  }

  /**
   * Initialize the client
   */
  async initialize(): Promise<void> {
    this.client = axios.create({
      baseURL: this.baseUrl,
      timeout: this.timeout,
      headers: {
        'Content-Type': 'application/protobuf',
      },
    });
  }

  /**
   * Close the client
   */
  async close(): Promise<void> {
    // Axios doesn't need explicit closing, but we can add cleanup if needed
  }

  /**
   * Set trace ID for subsequent signals
   */
  setTraceId(traceId: string): void {
    this.traceId = traceId;
  }

  /**
   * Get or generate trace ID
   */
  getTraceId(): string {
    if (!this.traceId) {
      this.traceId = uuidv4();
    }
    return this.traceId;
  }

  /**
   * Emit a signal to Argus
   */
  async emitSignal(
    layer: Layer,
    category: string,
    severity: Severity = Severity.INFO,
    context?: SignalContext,
    durationMs?: number,
    traceId?: string,
    parentSpanId?: string,
  ): Promise<boolean> {
    if (!this.client) {
      throw new Error('Client not initialized. Call initialize() first.');
    }

    const signalId = this.generateULID();
    const trace = traceId || this.getTraceId();
    const span = uuidv4().substring(0, 8);
    const now = new Date().toISOString();

    const signal: ArgusSignal = {
      signal_id: signalId,
      trace_id: trace,
      span_id: span,
      parent_span_id: parentSpanId || '',
      layer,
      category,
      severity,
      timestamp: now,
      duration_ms: durationMs || 0,
      source: {
        app_id: this.appId,
        app_version: this.appVersion,
        sdk_version: this.sdkVersion,
        environment: this.environment,
        instance_id: this.instanceId,
      },
    };

    if (context) {
      signal.context = context;
    }

    try {
      // For now, send as JSON (protobuf serialization would require proto-ts)
      const response = await this.client.post('/api/v1/signals', signal, {
        headers: {
          'Content-Type': 'application/json',
        },
      });

      return response.status === 200 || response.status === 202;
    } catch (error) {
      console.error('Error emitting signal:', error);
      return false;
    }
  }

  /**
   * Generate ULID-like unique ID
   */
  private generateULID(): string {
    const timestampMs = Date.now();
    const randomPart = uuidv4().replace(/-/g, '').substring(0, 12).toUpperCase();
    return `${timestampMs.toString().padStart(10, '0')}${randomPart}`;
  }
}
