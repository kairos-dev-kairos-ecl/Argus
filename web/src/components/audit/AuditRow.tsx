import { useState } from 'react';
import type { AuditEntry } from '../../services/audit-service';

interface AuditRowProps {
  entry: AuditEntry;
  expanded: boolean;
  onToggle: () => void;
}

export function AuditRow({ entry, expanded, onToggle }: AuditRowProps) {
  const [hovered, setHovered] = useState(false);

  const rowStyle: React.CSSProperties = {
    height: '20px',
    lineHeight: '20px',
    display: 'flex',
    gap: '12px',
    fontFamily: 'var(--font-mono)',
    fontSize: '12px',
    padding: '0 12px',
    cursor: 'pointer',
    borderBottom: 'var(--border-stark)',
    background: hovered ? 'var(--color-surface)' : 'transparent',
    overflow: 'hidden',
    whiteSpace: 'nowrap',
  };

  const expansionStyle: React.CSSProperties = {
    background: 'var(--color-surface)',
    borderBottom: 'var(--border-stark)',
    padding: '12px',
    fontFamily: 'var(--font-mono)',
    fontSize: '11px',
    maxHeight: expanded ? '600px' : '0px',
    overflow: 'hidden',
    transition: 'max-height 200ms ease',
  };

  return (
    <>
      <div
        style={rowStyle}
        onMouseEnter={() => setHovered(true)}
        onMouseLeave={() => setHovered(false)}
        onClick={onToggle}
      >
        {/* TIMESTAMP */}
        <div style={{ width: '180px', color: 'var(--color-muted)', overflow: 'hidden', flexShrink: 0 }}>
          {entry.timestamp.slice(0, 19).replace('T', ' ')}
        </div>
        {/* ACTION */}
        <div style={{ width: '140px', color: 'var(--color-text)', overflow: 'hidden', flexShrink: 0 }}>
          {entry.action}
        </div>
        {/* ACTOR */}
        <div style={{ width: '180px', color: 'var(--color-text)', overflow: 'hidden', flexShrink: 0 }}>
          {entry.actor_email ?? entry.actor_id ?? '—'}
        </div>
        {/* RESOURCE */}
        <div style={{ width: '180px', color: 'var(--color-text)', overflow: 'hidden', flexShrink: 0 }}>
          {entry.resource_type}:{entry.resource_id}
        </div>
        {/* HASH */}
        <div style={{ width: '120px', color: 'var(--color-primary)', overflow: 'hidden', flexShrink: 0 }}>
          <span title={entry.hash}>{entry.hash.slice(0, 12)}&hellip;</span>
        </div>
        {/* IP */}
        <div style={{ flex: 1, color: 'var(--color-muted)', overflow: 'hidden' }}>
          {entry.ip_address}
        </div>
      </div>

      {/* Expansion panel */}
      <div style={expansionStyle}>
        {expanded && (
          <pre style={{ margin: 0, whiteSpace: 'pre-wrap', color: 'var(--color-text)' }}>
            {JSON.stringify(entry.metadata, null, 2)}
          </pre>
        )}
      </div>
    </>
  );
}
