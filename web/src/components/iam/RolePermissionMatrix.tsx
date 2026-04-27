/**
 * RolePermissionMatrix
 *
 * Static display of role × layer permissions.
 * Rows: admin, analyst, viewer
 * Columns: L1 through L10
 *
 * Permission rules:
 * - admin: full access all 10 layers
 * - analyst: full access L4-L10, no access L1-L3
 * - viewer: full access L9-L10, no access L1-L8
 */

const LAYERS = [
  { id: 'l1', label: 'L1', color: 'var(--color-layer-l1)' },
  { id: 'l2', label: 'L2', color: 'var(--color-layer-l2)' },
  { id: 'l3', label: 'L3', color: 'var(--color-layer-l3)' },
  { id: 'l4', label: 'L4', color: 'var(--color-layer-l4)' },
  { id: 'l5', label: 'L5', color: 'var(--color-layer-l5)' },
  { id: 'l6', label: 'L6', color: 'var(--color-layer-l6)' },
  { id: 'l7', label: 'L7', color: 'var(--color-layer-l7)' },
  { id: 'l8', label: 'L8', color: 'var(--color-layer-l8)' },
  { id: 'l9', label: 'L9', color: 'var(--color-layer-l9)' },
  { id: 'l10', label: 'L10', color: 'var(--color-layer-l10)' },
] as const;

const ROLES: { id: string; label: string; allowed: number[] }[] = [
  { id: 'admin',   label: 'admin',   allowed: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10] },
  { id: 'analyst', label: 'analyst', allowed: [4, 5, 6, 7, 8, 9, 10] },
  { id: 'viewer',  label: 'viewer',  allowed: [9, 10] },
];

const cellStyle: React.CSSProperties = {
  padding: '6px 8px',
  fontFamily: 'var(--font-mono)',
  fontSize: '11px',
  border: 'var(--border-stark)',
  background: 'var(--color-surface)',
  textAlign: 'center' as const,
};

const headerCellStyle: React.CSSProperties = {
  ...cellStyle,
  color: 'var(--color-muted)',
  textTransform: 'uppercase' as const,
  fontSize: '10px',
};

const roleHeaderCellStyle: React.CSSProperties = {
  ...cellStyle,
  color: 'var(--color-muted)',
  textTransform: 'uppercase' as const,
  textAlign: 'left' as const,
  minWidth: '80px',
};

export function RolePermissionMatrix() {
  return (
    <div style={{ padding: '16px', borderTop: 'var(--border-stark)' }}>
      <div
        style={{
          fontFamily: 'var(--font-display)',
          fontSize: '14px',
          color: 'var(--color-primary)',
          textTransform: 'uppercase',
          marginBottom: '12px',
          letterSpacing: '0.08em',
        }}
      >
        ROLE × LAYER PERMISSIONS
      </div>

      <div
        style={{
          display: 'grid',
          gridTemplateColumns: '80px repeat(10, 1fr)',
          overflowX: 'auto',
        }}
      >
        {/* Header row */}
        <div style={roleHeaderCellStyle} />
        {LAYERS.map((layer, i) => (
          <div
            key={layer.id}
            style={{
              ...headerCellStyle,
              color: `var(--color-layer-l${i + 1})`,
            }}
          >
            {layer.label}
          </div>
        ))}

        {/* Role rows */}
        {ROLES.map((role) => (
          <>
            <div key={`${role.id}-label`} style={roleHeaderCellStyle}>
              {role.label}
            </div>
            {LAYERS.map((_, i) => {
              const layerNum = i + 1;
              const hasAccess = role.allowed.includes(layerNum);
              return (
                <div
                  key={`${role.id}-l${layerNum}`}
                  style={{
                    ...cellStyle,
                    color: hasAccess ? 'var(--color-primary)' : 'var(--color-muted)',
                  }}
                >
                  {hasAccess ? '●' : '—'}
                </div>
              );
            })}
          </>
        ))}
      </div>
    </div>
  );
}
