/**
 * Tests for fail-open behavior
 */

import { ArgusClient } from '../src/argus/client';
import { SignalBuffer } from '../src/argus/buffer';
import { Layer, Severity, ArgusSignal } from '../src/argus/types';

describe('Fail-Open Behavior', () => {
  test('should emit signal with null client gracefully', async () => {
    const client = new ArgusClient('http://unreachable:9999');
    await client.initialize();

    // Should not throw
    const result = await client.emitSignal(
      Layer.L5_OUTPUT_DECODING,
      'test.signal',
      Severity.INFO,
    );

    expect(typeof result).toBe('boolean');
  });

  test('buffer should drop signal when full', async () => {
    const buffer = new SignalBuffer(1);

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

    const add1 = await buffer.add(signal1);
    const add2 = await buffer.add(signal2);

    expect(add1).toBe(true);
    expect(add2).toBe(false);
    expect(buffer.getDropCounter()).toBe(1);
  });

  test('buffer should re-add signals on emit failure', async () => {
    const buffer = new SignalBuffer(10);

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

    const failingEmit = async () => {
      throw new Error('Emit failed');
    };

    const flushed = await buffer.flush(failingEmit);

    expect(flushed).toBe(0);
    // Signal should still be in buffer
    expect(buffer.getStats().bufferSize).toBe(1);
  });

  test('should handle connection timeout gracefully', async () => {
    const client = new ArgusClient('http://192.0.2.0', 'test-app'); // Non-routable address
    await client.initialize();

    // Should timeout gracefully and return false, not throw
    const result = await client.emitSignal(
      Layer.L5_OUTPUT_DECODING,
      'test.signal',
      Severity.INFO,
      {},
      100,
    );

    expect(result).toBe(false);
  });

  test('middleware should handle missing client', () => {
    const { argusMiddleware } = require('../src/argus/middleware');

    const middleware = argusMiddleware({ client: null });
    expect(middleware).toBeDefined();
    expect(typeof middleware).toBe('function');
  });

  test('drop counter signal should be emitted when buffer has drops', async () => {
    const buffer = new SignalBuffer(1);

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

    // Cause a drop
    await buffer.add(signal);
    await buffer.add(signal); // Second add will drop

    expect(buffer.getDropCounter()).toBe(1);

    let emittedDropSignal = false;
    const mockEmit = async (signals: ArgusSignal[]) => {
      if (signals.some((s) => s.category === 'sdk.drop_counter')) {
        emittedDropSignal = true;
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

    expect(emittedDropSignal).toBe(true);
    expect(buffer.getDropCounter()).toBe(0); // Should be reset
  });
});
