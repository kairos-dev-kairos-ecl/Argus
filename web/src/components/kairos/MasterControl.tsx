type KairosState = 'engage' | 'standby' | 'kill';

interface MasterControlProps {
  state: KairosState;
  onChange: (s: KairosState) => void;
}

const BUTTON_CONFIG: { id: KairosState; label: string; color: string }[] = [
  { id: 'engage',  label: 'ENGAGE',  color: 'var(--color-success)' },
  { id: 'standby', label: 'STANDBY', color: 'var(--color-warning)' },
  { id: 'kill',    label: 'KILL',    color: 'var(--color-alert)'   },
];

export function MasterControl({ state, onChange }: MasterControlProps) {
  const activeColor = BUTTON_CONFIG.find(b => b.id === state)?.color ?? 'var(--color-muted)';

  return (
    <div style={{
      padding: '16px',
      borderBottom: 'var(--border-stark)',
      display: 'flex',
      gap: '12px',
      alignItems: 'center',
      background: 'var(--color-surface)',
    }}>
      <div style={{
        fontFamily: 'var(--font-display)',
        fontSize: '14px',
        color: 'var(--color-primary)',
        textTransform: 'uppercase',
        marginRight: '24px',
        letterSpacing: '0.08em',
      }}>
        KAIROS CONTROL
      </div>

      {BUTTON_CONFIG.map(({ id, label, color }) => {
        const isActive = state === id;
        return (
          <button
            key={id}
            onClick={() => onChange(id)}
            style={{
              padding: '12px 24px',
              fontFamily: 'var(--font-mono)',
              fontSize: '12px',
              textTransform: 'uppercase',
              letterSpacing: '0.08em',
              border: isActive ? `1px solid ${color}` : 'var(--border-stark)',
              background: 'transparent',
              cursor: 'pointer',
              color: isActive ? color : 'var(--color-muted)',
              transition: 'color 0.15s, border-color 0.15s',
            }}
          >
            {label}
          </button>
        );
      })}

      <div style={{
        marginLeft: 'auto',
        fontSize: '12px',
        fontFamily: 'var(--font-mono)',
        textTransform: 'uppercase',
        color: activeColor,
        letterSpacing: '0.06em',
      }}>
        STATE: {state.toUpperCase()}
      </div>
    </div>
  );
}
