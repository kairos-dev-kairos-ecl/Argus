/**
 * Tests for SignalBuffer
 */

import { SignalBuffer } from '../src/argus/buffer';
import { ArgusSignal } from '../src/argus/types';

describe('SignalBuffer', () => {
  let buffer: SignalBuffer;

  beforeEach(() => {
    buffer = new SignalBuffer(2, 0.1);
  });

  afterEach(async () => {
    await buffer.stopAutoFlush();
  });

  test('should initialize with default max size', () => {
    const defaultBuffer = new SignalBuffer();
    expect(defaultBuffer).toBeDefined();
  });

  test('should add signals to buffer', async () => {
    const signal: ArgusSignal = {
      signal_id: 'signal-1',
      trace_id: 'trace-1',
      span_id: 'span-1',
      layer: 10,
      category: 'test',
      severity: 1,
      timestamp: new Date().toISOString(),
      source: {
        app_id: 'test-app',
        app_version: '1.0',
        sdk_version: '0.1.0',
        environment: 'test',
        instance_id: 'instance-1',
      },
    };

    const result = await buffer.add(signal);
    expect(result).toBe(true);
    expect(buffer.getStats().bufferSize).toBe(1);
  });

  test('should increment drop counter when buffer is full', async () => {
    const signal1: ArgusSignal = {
      signal_id: 'signal-1',
      trace_id: 'trace-1',
      span_id: 'span-1',
      layer: 10,
      category: 'test',
      severity: 1,
      timestamp: new Date().toISOString(),
      source: {
        app_id: 'test-app',
        app_version: '1.0',
        sdk_version: '0.1.0',
        environment: 'test',
        instance_id: 'instance-1',
      },
    };

    const signal2: ArgusSignal = { ...signal1, signal_id: 'signal-2' };
    const signal3: ArgusSignal = { ...signal1, signal_id: 'signal-3' };

    await buffer.add(signal1);
    await buffer.add(signal2);

    const result = await buffer.add(signal3);

    expect(result).toBe(false);
    expect(buffer.getDropCounter()).toBe(1);
    expect(buffer.getStats().totalDropped).toBe(1);
  });

  test('should flush signals', async () => {
    const signal: ArgusSignal = {
      signal_id: 'signal-1',
      trace_id: 'trace-1',
      span_id: 'span-1',
      layer: 10,
      category: 'test',
      severity: 1,
      timestamp: new Date().toISOString(),
      source: {
        app_id: 'test-app',
        app_version: '1.0',
        sdk_version: '0.1.0',
        environment: 'test',
        instance_id: 'instance-1',
      },
    };

    await buffer.add(signal);
    expect(buffer.getStats().bufferSize).toBe(1);

    let flushedCount = 0;
    const mockEmit = async (signals: ArgusSignal[]) => {
      flushedCount = signals.length;
    };

    await buffer.flush(mockEmit);

    expect(flushedCount).toBe(1);
    expect(buffer.getStats().bufferSize).toBe(0);
  });

  test('should handle flush failure gracefully', async () => {
    const signal: ArgusSignal = {
      signal_id: 'signal-1',
      trace_id: 'trace-1',
      span_id: 'span-1',
      layer: 10,
      category: 'test',
      severity: 1,
      timestamp: new Date().toISOString(),
      source: {
        app_id: 'test-app',
        app_version: '1.0',
        sdk_version: '0.1.0',
        environment: 'test',
        instance_id: 'instance-1',
      },
    };

    await buffer.add(signal);

    const mockEmitFailing = async () => {
      throw new Error('Emit failed');
    };

    const result = await buffer.flush(mockEmitFailing);

    expect(result).toBe(0);
    // Signal should be re-added on failure
    expect(buffer.getStats().bufferSize).toBe(1);
  });

  test('should support auto flush', async () => {
    const signal: ArgusSignal = {
      signal_id: 'signal-1',
      trace_id: 'trace-1',
      span_id: 'span-1',
      layer: 10,
      category: 'test',
      severity: 1,
      timestamp: new Date().toISOString(),
      source: {
        app_id: 'test-app',
        app_version: '1.0',
        sdk_version: '0.1.0',
        environment: 'test',
        instance_id: 'instance-1',
      },
    };

    let flushCount = 0;
    const mockEmit = async (signals: ArgusSignal[]) => {
      flushCount++;
    };

    await buffer.add(signal);
    await buffer.startAutoFlush(mockEmit);

    // Wait for at least one flush
    await new Promise((resolve) => setTimeout(resolve, 200));

    await buffer.stopAutoFlush();

    expect(flushCount).toBeGreaterThan(0);
  });

  test('should get stats', () => {
    const stats = buffer.getStats();

    expect(stats).toBeDefined();
    expect(stats.totalEmitted).toBe(0);
    expect(stats.totalDropped).toBe(0);
    expect(stats.bufferSize).toBe(0);
  });

  test('should emit drop counter signal', async () => {
    buffer.getDropCounter();

    // Simulate some drops
    const signal: ArgusSignal = {
      signal_id: 'signal-1',
      trace_id: 'trace-1',
      span_id: 'span-1',
      layer: 10,
      category: 'test',
      severity: 1,
      timestamp: new Date().toISOString(),
      source: {
        app_id: 'test-app',
        app_version: '1.0',
        sdk_version: '0.1.0',
        environment: 'test',
        instance_id: 'instance-1',
      },
    };

    // Fill buffer
    await buffer.add(signal);
    await buffer.add(signal);
    await buffer.add(signal); // Should be dropped

    let emittedSignal: ArgusSignal | null = null;
    const mockEmit = async (signals: ArgusSignal[]) => {
      if (signals.length > 0) {
        emittedSignal = signals[0];
      }
    };

    await buffer.emitDropCounterSignal(
      mockEmit,
      'test-app',
      '1.0',
      '0.1.0',
      'test',
      'instance-1',
    );

    expect(emittedSignal).toBeDefined();
    expect(emittedSignal?.category).toBe('sdk.drop_counter');
  });
});
