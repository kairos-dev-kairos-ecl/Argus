import { useEffect, useState } from 'react';
import { AuditFilters } from '../components/audit/AuditFilters';
import { AuditRow } from '../components/audit/AuditRow';
import { fetchAuditEntries, type AuditEntry, type AuditFilterState } from '../services/audit-service';

export function AuditLogPage() {
  const [entries, setEntries] = useState<AuditEntry[]>([]);
  const [total, setTotal] = useState(0);
  const [filters, setFilters] = useState<AuditFilterState>({});
  const [expanded, setExpanded] = useState<Set<string>>(new Set());
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    fetchAuditEntries(filters, 100, 0)
      .then(res => { if (!cancelled) { setEntries(res.entries); setTotal(res.total); setError(null); } })
      .catch(e => { if (!cancelled) setError(e?.message ?? 'Failed to load audit'); })
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [filters]);

  const toggle = (id: string) => {
    setExpanded(prev => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id); else next.add(id);
      return next;
    });
  };

  return (
    <div style={{
      display: 'flex',
      flexDirection: 'column',
      height: '100vh',
      background: 'var(--color-background)',
      color: 'var(--color-text)',
      fontFamily: 'var(--font-mono)',
    }}>
      {/* Sticky header */}
      <div style={{ padding: '12px 16px', borderBottom: 'var(--border-stark)', background: 'var(--color-surface)' }}>
        <div style={{ fontFamily: 'var(--font-display)', fontSize: '16px', color: 'var(--color-primary)', textTransform: 'uppercase' }}>
          Audit Ledger
        </div>
        <div style={{ fontSize: '11px', color: 'var(--color-muted)', textTransform: 'uppercase', marginTop: '4px' }}>
          {total} ENTRIES · ZERO-TRUST IMMUTABLE LOG
        </div>
      </div>

      <AuditFilters value={filters} onChange={setFilters} />

      {/* Column header */}
      <div style={{
        display: 'flex',
        gap: '12px',
        padding: '0 12px',
        height: '20px',
        lineHeight: '20px',
        fontSize: '11px',
        color: 'var(--color-muted)',
        textTransform: 'uppercase',
        borderBottom: 'var(--border-stark)',
        background: 'var(--color-surface)',
      }}>
        <div style={{ width: '180px' }}>TIMESTAMP</div>
        <div style={{ width: '140px' }}>ACTION</div>
        <div style={{ width: '180px' }}>ACTOR</div>
        <div style={{ width: '180px' }}>RESOURCE</div>
        <div style={{ width: '120px' }}>HASH</div>
        <div style={{ flex: 1 }}>IP</div>
      </div>

      {/* Body */}
      <div style={{ flex: 1, overflow: 'auto' }}>
        {error && (
          <div style={{ padding: '12px', color: 'var(--color-alert)', fontSize: '12px', textTransform: 'uppercase' }}>
            ERROR: {error}
          </div>
        )}
        {!error && entries.length === 0 && !loading && (
          <div style={{ padding: '12px', color: 'var(--color-muted)', fontSize: '12px', textTransform: 'uppercase' }}>
            NO AUDIT ENTRIES
          </div>
        )}
        {entries.map(e => (
          <AuditRow
            key={e.id}
            entry={e}
            expanded={expanded.has(e.id)}
            onToggle={() => toggle(e.id)}
          />
        ))}
      </div>

      {/* Sticky footer */}
      <div style={{ padding: '6px 16px', borderTop: 'var(--border-stark)', background: 'var(--color-surface)', fontSize: '11px', color: 'var(--color-muted)', textTransform: 'uppercase' }}>
        SHOWING {entries.length} OF {total}
      </div>
    </div>
  );
}
