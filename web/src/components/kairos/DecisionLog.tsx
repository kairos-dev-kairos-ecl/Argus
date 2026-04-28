export interface Decision {
  id: string;
  timestamp: string;
  confidence: number;
  action: string;
  override: boolean;
  latency_ms: number;
}

interface DecisionLogProps {
  decisions: Decision[];
}

function confidenceColor(c: number): string {
  if (c >= 0.9) return 'var(--color-primary)';
  if (c >= 0.7) return 'var(--color-warning)';
  return 'var(--color-alert)';
}

export function DecisionLog({ decisions }: DecisionLogProps) {
  return (
    <div style={{
      borderRight: 'var(--border-stark)',
      overflow: 'auto',
      height: '100%',
      fontFamily: 'var(--font-mono)',
      fontSize: '12px',
    }}>
      {/* Header */}
      <div style={{
        display: 'flex',
        alignItems: 'center',
        height: '28px',
        borderBottom: 'var(--border-stark)',
        background: 'var(--color-surface)',
        paddingLeft: '8px',
        position: 'sticky',
        top: 0,
        zIndex: 1,
      }}>
        <span style={{ width: '180px', color: 'var(--color-muted)', textTransform: 'uppercase', letterSpacing: '0.06em' }}>TIMESTAMP</span>
        <span style={{ width: '100px', color: 'var(--color-muted)', textTransform: 'uppercase', letterSpacing: '0.06em' }}>CONFIDENCE</span>
        <span style={{ width: '180px', color: 'var(--color-muted)', textTransform: 'uppercase', letterSpacing: '0.06em' }}>ACTION</span>
        <span style={{ width: '80px',  color: 'var(--color-muted)', textTransform: 'uppercase', letterSpacing: '0.06em' }}>OVERRIDE</span>
        <span style={{ width: '80px',  color: 'var(--color-muted)', textTransform: 'uppercase', letterSpacing: '0.06em' }}>LATENCY</span>
      </div>

      {/* Rows */}
      {decisions.map(d => (
        <div key={d.id} style={{
          display: 'flex',
          alignItems: 'center',
          height: '20px',
          paddingLeft: '8px',
          borderBottom: '1px solid rgba(52,58,64,0.4)',
        }}>
          <span style={{ width: '180px', color: 'var(--color-muted)', overflow: 'hidden', whiteSpace: 'nowrap' }}>
            {d.timestamp.replace('T', ' ').slice(0, 19)}
          </span>
          <span style={{ width: '100px', color: confidenceColor(d.confidence) }}>
            {(d.confidence * 100).toFixed(1)}%
          </span>
          <span style={{ width: '180px', color: 'var(--color-text)', textTransform: 'uppercase' }}>
            {d.action}
          </span>
          <span style={{ width: '80px', color: d.override ? 'var(--color-alert)' : 'var(--color-muted)' }}>
            {d.override ? 'YES' : 'NO'}
          </span>
          <span style={{ width: '80px', color: 'var(--color-text)' }}>
            {d.latency_ms}MS
          </span>
        </div>
      ))}
    </div>
  );
}
