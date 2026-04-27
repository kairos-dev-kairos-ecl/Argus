const TACTICS = [
  'Reconnaissance',
  'ResourceDevelopment',
  'InitialAccess',
  'MLModelAccess',
  'Execution',
  'Persistence',
  'PrivilegeEscalation',
  'DefenseEvasion',
  'Discovery',
  'Collection',
  'MLAttackStaging',
  'Exfiltration',
  'Impact',
] as const;

interface KillChainMatrixProps {
  activeTactic?: string;
  counts?: Record<string, number>;
}

export function KillChainMatrix({ activeTactic, counts = {} }: KillChainMatrixProps) {
  return (
    <div
      style={{
        padding: '12px',
        borderTop: 'var(--border-stark)',
      }}
    >
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fit, minmax(120px, 1fr))',
          gap: '1px',
          background: 'var(--border-stark)',
        }}
      >
        {TACTICS.map((tactic) => {
          const count = counts[tactic] ?? 0;
          const isActive = tactic === activeTactic;
          return (
            <div
              key={tactic}
              style={{
                padding: '8px',
                border: isActive
                  ? '1px solid var(--color-primary)'
                  : 'var(--border-stark)',
                fontFamily: 'var(--font-mono)',
                fontSize: '11px',
                color: isActive ? 'var(--color-primary)' : 'var(--color-text)',
                background: 'var(--color-surface)',
                borderTop: count > 0 ? '2px solid var(--color-alert)' : undefined,
                display: 'flex',
                flexDirection: 'column',
                gap: '4px',
              }}
            >
              <span style={{ textTransform: 'uppercase', letterSpacing: '0.02em' }}>
                {tactic}
              </span>
              <span
                style={{
                  color: count > 0 ? 'var(--color-alert)' : 'var(--color-muted)',
                  fontWeight: count > 0 ? 700 : 400,
                }}
              >
                {count}
              </span>
            </div>
          );
        })}
      </div>
    </div>
  );
}
