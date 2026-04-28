/**
 * RolePermissionMatrix
 *
 * All roles see ALL 10 layers. Access control is at the action level:
 * - admin:   ● full CRUD (create, read, update, delete)
 * - analyst: ◉ read + limited write (read, flag, acknowledge)
 * - viewer:  ○ read-only (read only)
 */

const LAYERS = [
  { id: 'l1',  label: 'L1',  name: 'Hardware',    color: 'var(--color-layer-l1)' },
  { id: 'l2',  label: 'L2',  name: 'Model',       color: 'var(--color-layer-l2)' },
  { id: 'l3',  label: 'L3',  name: 'Inference',   color: 'var(--color-layer-l3)' },
  { id: 'l4',  label: 'L4',  name: 'Attention',   color: 'var(--color-layer-l4)' },
  { id: 'l5',  label: 'L5',  name: 'Logprob',     color: 'var(--color-layer-l5)' },
  { id: 'l6',  label: 'L6',  name: 'Safety',      color: 'var(--color-layer-l6)' },
  { id: 'l7',  label: 'L7',  name: 'Retrieval',   color: 'var(--color-layer-l7)' },
  { id: 'l8',  label: 'L8',  name: 'Tool',        color: 'var(--color-layer-l8)' },
  { id: 'l9',  label: 'L9',  name: 'API',         color: 'var(--color-layer-l9)' },
  { id: 'l10', label: 'L10', name: 'Application', color: 'var(--color-layer-l10)' },
] as const

type ActionSet = 'crud' | 'read-write' | 'read-only'

const ROLES: { id: string; label: string; actions: ActionSet; description: string }[] = [
  { id: 'admin',   label: 'ADMIN',   actions: 'crud',       description: 'Full CRUD across all layers' },
  { id: 'analyst', label: 'ANALYST', actions: 'read-write', description: 'Read + flag + acknowledge' },
  { id: 'viewer',  label: 'VIEWER',  actions: 'read-only',  description: 'Read-only across all layers' },
]

function ActionDot({ actions }: { actions: ActionSet }) {
  if (actions === 'crud') {
    return (
      <span title="Full CRUD" style={{ color: 'var(--color-primary)', fontSize: '14px' }}>●</span>
    )
  }
  if (actions === 'read-write') {
    return (
      <span title="Read + acknowledge + flag" style={{ color: 'var(--color-warning)', fontSize: '14px' }}>◉</span>
    )
  }
  return (
    <span title="Read-only" style={{ color: 'var(--color-muted)', fontSize: '14px' }}>○</span>
  )
}

const cellStyle: React.CSSProperties = {
  padding: '6px 8px',
  fontFamily: 'var(--font-mono)',
  fontSize: '11px',
  border: 'var(--border-stark)',
  background: 'var(--color-surface)',
  textAlign: 'center' as const,
}

const headerCellStyle: React.CSSProperties = {
  ...cellStyle,
  color: 'var(--color-muted)',
  textTransform: 'uppercase' as const,
  fontSize: '10px',
}

const roleHeaderCellStyle: React.CSSProperties = {
  ...cellStyle,
  color: 'var(--color-muted)',
  textTransform: 'uppercase' as const,
  textAlign: 'left' as const,
  minWidth: '80px',
}

export function RolePermissionMatrix() {
  return (
    <div style={{ padding: '16px', borderTop: 'var(--border-stark)' }}>
      <div style={{
        fontFamily: 'var(--font-display)',
        fontSize: '14px',
        color: 'var(--color-primary)',
        textTransform: 'uppercase',
        marginBottom: '4px',
        letterSpacing: '0.08em',
      }}>
        ROLE × LAYER PERMISSIONS
      </div>
      <div style={{ fontSize: '11px', color: 'var(--color-muted)', marginBottom: '12px', fontFamily: 'var(--font-mono)' }}>
        All roles see all 10 layers. Access control is action-level.
      </div>

      {/* Legend */}
      <div style={{ display: 'flex', gap: '16px', marginBottom: '12px', fontSize: '11px', fontFamily: 'var(--font-mono)' }}>
        <span><span style={{ color: 'var(--color-primary)' }}>●</span> FULL CRUD</span>
        <span><span style={{ color: 'var(--color-warning)' }}>◉</span> READ + WRITE</span>
        <span><span style={{ color: 'var(--color-muted)' }}>○</span> READ ONLY</span>
      </div>

      <div style={{
        display: 'grid',
        gridTemplateColumns: '80px repeat(10, 1fr)',
        overflowX: 'auto',
      }}>
        {/* Header row */}
        <div style={roleHeaderCellStyle} />
        {LAYERS.map((layer, i) => (
          <div
            key={layer.id}
            style={{
              ...headerCellStyle,
              color: `var(--color-layer-l${i + 1})`,
            }}
            title={layer.name}
          >
            {layer.label}
          </div>
        ))}

        {/* Role rows */}
        {ROLES.map((role) => (
          <>
            <div
              key={`${role.id}-label`}
              style={roleHeaderCellStyle}
              title={role.description}
            >
              {role.label}
            </div>
            {LAYERS.map((_, i) => (
              <div
                key={`${role.id}-l${i + 1}`}
                style={cellStyle}
              >
                <ActionDot actions={role.actions} />
              </div>
            ))}
          </>
        ))}
      </div>
    </div>
  )
}
