import { useState, useMemo } from 'react';
import { MasterControl } from '../components/kairos/MasterControl';
import { DecisionLog } from '../components/kairos/DecisionLog';
import { MetricsGrid } from '../components/kairos/MetricsGrid';

type State = 'engage' | 'standby' | 'kill';

export function KairosPage() {
  const [state, setState] = useState<State>('standby');

  // Display-only mock data (Kairos backend integration deferred per STATE.md)
  const decisions = useMemo(() => Array.from({ length: 50 }).map((_, i) => ({
    id: `d${i}`,
    timestamp: new Date(Date.now() - i * 30000).toISOString(),
    confidence: 0.6 + Math.random() * 0.4,
    action: ['allow', 'block', 'rate_limit', 'flag'][i % 4],
    override: Math.random() < 0.1,
    latency_ms: Math.round(20 + Math.random() * 80),
  })), []);

  const latencySamples = useMemo(() => Array.from({ length: 60 }).map(() => 30 + Math.random() * 70), []);

  return (
    <div style={{
      display: 'grid',
      gridTemplateRows: 'auto 1fr',
      height: '100vh',
      background: 'var(--color-background)',
      color: 'var(--color-text)',
      fontFamily: 'var(--font-mono)',
    }}>
      <MasterControl state={state} onChange={setState} />
      <div style={{ display: 'grid', gridTemplateColumns: '50% 50%', minHeight: 0 }}>
        <DecisionLog decisions={decisions} />
        <MetricsGrid
          latencySamples={latencySamples}
          interventionCount={decisions.filter(d => d.override).length}
          decisionsPerMin={120}
        />
      </div>
      <div style={{ position: 'fixed', bottom: 8, right: 12, fontSize: '10px', color: 'var(--color-muted)', textTransform: 'uppercase', pointerEvents: 'none' }}>
        DISPLAY-ONLY — KAIROS BACKEND DEFERRED
      </div>
    </div>
  );
}
