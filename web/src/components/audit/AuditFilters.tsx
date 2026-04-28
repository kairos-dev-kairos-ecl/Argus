import { useRef } from 'react';
import type { AuditFilterState } from '../../services/audit-service';

interface AuditFiltersProps {
  value: AuditFilterState;
  onChange: (v: AuditFilterState) => void;
}

export function AuditFilters({ value, onChange }: AuditFiltersProps) {
  const iamTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const ipTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const handleIamChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const v = e.target.value;
    if (iamTimerRef.current) clearTimeout(iamTimerRef.current);
    iamTimerRef.current = setTimeout(() => {
      onChange({ ...value, actor: v || undefined });
    }, 250);
  };

  const handleIpChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const v = e.target.value;
    if (ipTimerRef.current) clearTimeout(ipTimerRef.current);
    ipTimerRef.current = setTimeout(() => {
      onChange({ ...value, ip: v || undefined });
    }, 250);
  };

  const handleLayerChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    const v = e.target.value;
    onChange({ ...value, layer: v ? parseInt(v, 10) : undefined });
  };

  const handleClear = () => {
    onChange({});
  };

  const labelStyle: React.CSSProperties = {
    display: 'block',
    fontSize: '11px',
    color: 'var(--color-muted)',
    textTransform: 'uppercase',
    letterSpacing: '0.05em',
    marginBottom: '4px',
    fontFamily: 'var(--font-mono)',
  };

  const inputStyle: React.CSSProperties = {
    background: 'var(--color-background)',
    border: 'var(--border-stark)',
    color: 'var(--color-text)',
    padding: '4px 8px',
    fontFamily: 'var(--font-mono)',
    fontSize: '12px',
    outline: 'none',
  };

  return (
    <div
      style={{
        display: 'flex',
        gap: '8px',
        padding: '12px 16px',
        borderBottom: 'var(--border-stark)',
        background: 'var(--color-surface)',
        alignItems: 'flex-end',
      }}
    >
      {/* IAM filter */}
      <div>
        <label style={labelStyle}>IAM</label>
        <input
          type="text"
          placeholder="actor email / id"
          defaultValue={value.actor ?? ''}
          onChange={handleIamChange}
          style={inputStyle}
        />
      </div>

      {/* IP filter */}
      <div>
        <label style={labelStyle}>IP</label>
        <input
          type="text"
          placeholder="ip address"
          defaultValue={value.ip ?? ''}
          onChange={handleIpChange}
          style={inputStyle}
        />
      </div>

      {/* LAYER filter */}
      <div>
        <label style={labelStyle}>LAYER</label>
        <select
          value={value.layer != null ? String(value.layer) : ''}
          onChange={handleLayerChange}
          style={inputStyle}
        >
          <option value="">All</option>
          <option value="1">L1</option>
          <option value="2">L2</option>
          <option value="3">L3</option>
          <option value="4">L4</option>
          <option value="5">L5</option>
          <option value="6">L6</option>
          <option value="7">L7</option>
          <option value="8">L8</option>
          <option value="9">L9</option>
          <option value="10">L10</option>
        </select>
      </div>

      {/* CLEAR button */}
      <button
        onClick={handleClear}
        style={{
          border: '1px solid var(--color-muted)',
          color: 'var(--color-muted)',
          padding: '4px 8px',
          textTransform: 'uppercase',
          background: 'transparent',
          fontFamily: 'var(--font-mono)',
          fontSize: '11px',
          cursor: 'pointer',
        }}
      >
        Clear
      </button>
    </div>
  );
}
