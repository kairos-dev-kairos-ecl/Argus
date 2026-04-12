/**
 * SignalBuffer - In-memory signal buffer with drop counter
 */

import { ArgusSignal, BufferStats } from './types';

export class SignalBuffer {
  private buffer: ArgusSignal[] = [];
  private maxSize: number;
  private flushIntervalSeconds: number;
  private dropCounter: number = 0;
  private stats: BufferStats = {
    totalEmitted: 0,
    totalDropped: 0,
    bufferSize: 0,
  };
  private flushInterval: NodeJS.Timeout | null = null;

  constructor(maxSize: number = 100, flushIntervalSeconds: number = 1.0) {
    this.maxSize = maxSize;
    this.flushIntervalSeconds = flushIntervalSeconds;
  }

  /**
   * Add a signal to the buffer
   */
  async add(signal: ArgusSignal): Promise<boolean> {
    if (this.buffer.length >= this.maxSize) {
      // Buffer is full, drop signal
      this.dropCounter++;
      this.stats.totalDropped++;
      return false;
    }

    this.buffer.push(signal);
    this.stats.totalEmitted++;
    return true;
  }

  /**
   * Flush buffered signals
   */
  async flush(emitFn: (signals: ArgusSignal[]) => Promise<void>): Promise<number> {
    if (this.buffer.length === 0) {
      return 0;
    }

    const signals = this.buffer.splice(0, this.buffer.length);

    try {
      await emitFn(signals);
      this.stats.lastFlush = Date.now();
      return signals.length;
    } catch (error) {
      // Re-add signals on failure (fail-open)
      this.buffer.unshift(...signals);
      console.error('Failed to flush signals:', error);
      return 0;
    }
  }

  /**
   * Start automatic flush task
   */
  async startAutoFlush(emitFn: (signals: ArgusSignal[]) => Promise<void>): Promise<void> {
    this.flushInterval = setInterval(async () => {
      try {
        await this.flush(emitFn);
      } catch (error) {
        console.error('Error in auto flush:', error);
      }
    }, this.flushIntervalSeconds * 1000);
  }

  /**
   * Stop automatic flush task
   */
  async stopAutoFlush(): Promise<void> {
    if (this.flushInterval) {
      clearInterval(this.flushInterval);
      this.flushInterval = null;
    }
  }

  /**
   * Get the current drop counter
   */
  getDropCounter(): number {
    return this.dropCounter;
  }

  /**
   * Get buffer statistics
   */
  getStats(): BufferStats {
    return {
      totalEmitted: this.stats.totalEmitted,
      totalDropped: this.stats.totalDropped,
      bufferSize: this.buffer.length,
      lastFlush: this.stats.lastFlush,
    };
  }

  /**
   * Emit a drop counter signal
   */
  async emitDropCounterSignal(
    emitFn: (signals: ArgusSignal[]) => Promise<void>,
    appId: string,
    appVersion: string,
    sdkVersion: string,
    environment: string,
    instanceId: string,
  ): Promise<void> {
    if (this.dropCounter === 0) {
      return;
    }

    const signal: ArgusSignal = {
      signal_id: this.generateULID(),
      trace_id: `drop-counter-${Date.now()}`,
      span_id: 'drop-counter',
      layer: 10, // L10_APPLICATION
      category: 'sdk.drop_counter',
      severity: 4, // HIGH
      timestamp: new Date().toISOString(),
      duration_ms: 0,
      source: {
        app_id: appId,
        app_version: appVersion,
        sdk_version: sdkVersion,
        environment,
        instance_id: instanceId,
      },
      context: {
        dropped_signals: this.dropCounter,
      },
    };

    try {
      await emitFn([signal]);
      this.dropCounter = 0; // Reset after successful emit
    } catch (error) {
      console.error('Failed to emit drop counter signal:', error);
    }
  }

  private generateULID(): string {
    const timestampMs = Date.now();
    const randomPart = Math.random().toString(36).substring(2, 14).toUpperCase();
    return `${timestampMs.toString().padStart(10, '0')}${randomPart.padEnd(12, '0')}`;
  }
}
