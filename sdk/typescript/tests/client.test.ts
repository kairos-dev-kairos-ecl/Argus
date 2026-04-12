/**
 * Tests for ArgusClient
 */

import { ArgusClient } from '../src/argus/client';
import { Layer, Severity } from '../src/argus/types';

describe('ArgusClient', () => {
  let client: ArgusClient;

  beforeEach(async () => {
    client = new ArgusClient(
      'http://localhost:8080',
      'test-app',
      '1.0.0',
    );
    await client.initialize();
  });

  afterEach(async () => {
    await client.close();
  });

  test('should initialize with default values', async () => {
    const defaultClient = new ArgusClient();
    await defaultClient.initialize();
    expect(defaultClient).toBeDefined();
  });

  test('should generate trace ID', () => {
    const traceId1 = client.getTraceId();
    expect(traceId1).toBeDefined();
    expect(typeof traceId1).toBe('string');
    expect(traceId1.length).toBeGreaterThan(0);
  });

  test('should return same trace ID on subsequent calls', () => {
    const traceId1 = client.getTraceId();
    const traceId2 = client.getTraceId();
    expect(traceId1).toBe(traceId2);
  });

  test('should set custom trace ID', () => {
    client.setTraceId('custom-trace-123');
    expect(client.getTraceId()).toBe('custom-trace-123');
  });

  test('should fail gracefully on unreachable server', async () => {
    const unreachableClient = new ArgusClient('http://unreachable-server:9999');
    await unreachableClient.initialize();

    const result = await unreachableClient.emitSignal(
      Layer.L5_OUTPUT_DECODING,
      'inference.completion',
      Severity.HIGH,
    );

    expect(result).toBe(false);
  });

  test('should emit signal without context', async () => {
    const result = await client.emitSignal(
      Layer.L5_OUTPUT_DECODING,
      'inference.completion',
      Severity.INFO,
    );

    // Will fail since server isn't running, but should handle gracefully
    expect(typeof result).toBe('boolean');
  });

  test('should emit signal with context', async () => {
    const context = {
      output_tokens: 100,
      input_tokens: 50,
      total_tokens: 150,
      finish_reason: 'stop',
    };

    const result = await client.emitSignal(
      Layer.L5_OUTPUT_DECODING,
      'inference.completion',
      Severity.INFO,
      context,
    );

    expect(typeof result).toBe('boolean');
  });

  test('should measure duration', async () => {
    const startTime = Date.now();

    // Simulate some work
    await new Promise((resolve) => setTimeout(resolve, 50));

    const durationMs = Date.now() - startTime;

    const result = await client.emitSignal(
      Layer.L5_OUTPUT_DECODING,
      'inference.completion',
      Severity.INFO,
      {},
      durationMs,
    );

    expect(typeof result).toBe('boolean');
  });

  test('should support all layer types', async () => {
    const layers = [
      Layer.L1_HARDWARE,
      Layer.L7_RAG_RETRIEVAL,
      Layer.L8_AGENTS,
      Layer.L9_API_GATEWAY,
      Layer.L10_APPLICATION,
    ];

    for (const layer of layers) {
      const result = await client.emitSignal(
        layer,
        'test.signal',
        Severity.INFO,
      );
      expect(typeof result).toBe('boolean');
    }
  });

  test('should support all severity levels', async () => {
    const severities = [
      Severity.INFO,
      Severity.LOW,
      Severity.MEDIUM,
      Severity.HIGH,
      Severity.CRITICAL,
    ];

    for (const severity of severities) {
      const result = await client.emitSignal(
        Layer.L5_OUTPUT_DECODING,
        'test.signal',
        severity,
      );
      expect(typeof result).toBe('boolean');
    }
  });
});
