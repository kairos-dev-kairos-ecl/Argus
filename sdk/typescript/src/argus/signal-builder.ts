/**
 * SignalBuilder - Construct ArgusSignal messages from context
 */

import { v4 as uuidv4 } from 'uuid';
import { ArgusSignal, SignalContext, Layer } from './types';

export class SignalBuilder {
  static build(
    layer: number,
    category: string,
    appId: string,
    appVersion: string,
    sdkVersion: string,
    environment: string,
    instanceId: string,
    severity: number = 1,
    durationMs?: number,
    traceId?: string,
    spanId?: string,
    parentSpanId?: string,
    context?: SignalContext,
  ): ArgusSignal {
    const signalId = SignalBuilder.generateULID();
    const trace = traceId || uuidv4();
    const span = spanId || uuidv4().substring(0, 8);

    const signal: ArgusSignal = {
      signal_id: signalId,
      trace_id: trace,
      span_id: span,
      parent_span_id: parentSpanId || '',
      layer,
      category,
      severity,
      timestamp: new Date().toISOString(),
      duration_ms: durationMs || 0,
      source: {
        app_id: appId,
        app_version: appVersion,
        sdk_version: sdkVersion,
        environment,
        instance_id: instanceId,
      },
    };

    if (context) {
      signal.context = context;
    }

    return signal;
  }

  private static generateULID(): string {
    const timestampMs = Date.now();
    const randomPart = uuidv4().replace(/-/g, '').substring(0, 12).toUpperCase();
    return `${timestampMs.toString().padStart(10, '0')}${randomPart}`;
  }
}
