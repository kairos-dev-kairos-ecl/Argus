import type { CSSProperties } from 'react'

interface MetricsGridProps {
  latencySamples: number[];
  interventionCount: number;
  decisionsPerMin: number;
}

type KairosState = 'engage' | 'standby' | 'kill';

function stateColor(s: KairosState): string {
  switch (s) {
    case 'engage':  return 'var(--color-success)';
    case 'standby': return 'var(--color-warning)';
    case 'kill':    return 'var(--color-alert)';
  }
}

// Deterministic mock timeline — 60 cells cycling engage/standby/kill
const TIMELINE_STATES: KairosState[] = Array.from({ length: 60 }, (_, i) => {
  if (i % 9 === 7) return 'kill';
  if (i % 4 === 3) return 'standby';
  return 'engage';
});

export function MetricsGrid({ latencySamples, interventionCount, decisionsPerMin }: MetricsGridProps) {
  const maxLatency = latencySamples.length > 0 ? Math.max(...latencySamples) : 1;
  const barWidth = latencySamples.length > 0 ? 100 / latencySamples.length : 0;

  const cellStyle: CSSProperties = {
    background: 'var(--color-background)',
    padding: '16px',
    fontFamily: 'var(--font-mono)',
    display: 'flex',
    flexDirection: 'column',
  };

  const bigNumStyle = (color: string): CSSProperties => ({
    fontSize: '32px',
    color,
    lineHeight: 1,
    marginBottom: '8px',
  });

  const labelStyle: CSSProperties = {
    fontSize: '11px',
    color: 'var(--color-muted)',
    textTransform: 'uppercase',
    letterSpacing: '0.06em',
    marginBottom: '8px',
  };

  return (
    <div style={{
      display: 'grid',
      gridTemplateColumns: '1fr 1fr',
      gridTemplateRows: '1fr 1fr',
      gap: '1px',
      background: 'var(--color-muted)',
      height: '100%',
      padding: 0,
    }}>
      {/* Cell 1: LATENCY P99 + sparkline */}
      <div style={cellStyle}>
        <div style={labelStyle}>LATENCY P99</div>
        <div style={bigNumStyle('var(--color-primary)')}>
          {Math.round(maxLatency)}<span style={{ fontSize: '14px', marginLeft: '4px' }}>MS</span>
        </div>
        {/* CSS sparkline */}
        <div style={{ width: '100%', height: '40px', position: 'relative', marginTop: 'auto' }}>
          {latencySamples.map((s, i) => (
            <div
              key={i}
              style={{
                backgroundColor: 'var(--color-primary)',
                width: `${barWidth}%`,
                height: `${(s / maxLatency) * 100}%`,
                position: 'absolute',
                bottom: 0,
                left: `${i * barWidth}%`,
                opacity: 0.8,
              }}
            />
          ))}
        </div>
      </div>

      {/* Cell 2: INTERVENTIONS */}
      <div style={cellStyle}>
        <div style={labelStyle}>OPERATOR OVERRIDES (24H)</div>
        <div style={bigNumStyle('var(--color-warning)')}>
          {interventionCount}
        </div>
        <div style={{ fontSize: '11px', color: 'var(--color-muted)', textTransform: 'uppercase', letterSpacing: '0.06em', marginTop: 'auto' }}>
          INTERVENTIONS
        </div>
      </div>

      {/* Cell 3: DECISIONS/MIN */}
      <div style={cellStyle}>
        <div style={labelStyle}>AUTONOMOUS DECISION RATE</div>
        <div style={bigNumStyle('var(--color-success)')}>
          {decisionsPerMin}
        </div>
        <div style={{ fontSize: '11px', color: 'var(--color-muted)', textTransform: 'uppercase', letterSpacing: '0.06em', marginTop: 'auto' }}>
          DECISIONS/MIN
        </div>
      </div>

      {/* Cell 4: STATE TIMELINE */}
      <div style={cellStyle}>
        <div style={labelStyle}>STATE TIMELINE</div>
        <div style={{ display: 'flex', flexWrap: 'wrap', gap: '2px', marginTop: 'auto' }}>
          {TIMELINE_STATES.map((s, i) => (
            <div
              key={i}
              title={s.toUpperCase()}
              style={{
                width: '10px',
                height: '10px',
                background: stateColor(s),
                opacity: 0.85,
              }}
            />
          ))}
        </div>
      </div>
    </div>
  );
}
