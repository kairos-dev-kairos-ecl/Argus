/**
 * TimeScrubber — time range selector with preset buttons + custom date/time picker.
 *
 * Presets: 5M / 15M / 1H / 24H
 * Custom:  opens from/to datetime-local inputs
 */

export type TimeRange = '5m' | '15m' | '1h' | '24h' | 'custom'

interface TimeScrubberProps {
  value: TimeRange
  onChange: (v: TimeRange) => void
  customFrom?: string
  customTo?: string
  onCustomChange?: (from: string, to: string) => void
}

const PRESETS: { value: TimeRange; label: string }[] = [
  { value: '5m',  label: '5M' },
  { value: '15m', label: '15M' },
  { value: '1h',  label: '1H' },
  { value: '24h', label: '24H' },
]

const btnBase: React.CSSProperties = {
  background: 'transparent',
  padding: '6px 12px',
  fontFamily: 'var(--font-mono)',
  fontSize: '11px',
  textTransform: 'uppercase',
  cursor: 'pointer',
  letterSpacing: '0.05em',
}

const dtInputStyle: React.CSSProperties = {
  background: 'var(--color-surface)',
  border: 'var(--border-stark)',
  color: 'var(--color-text)',
  fontFamily: 'var(--font-mono)',
  fontSize: '11px',
  padding: '5px 8px',
  colorScheme: 'dark',
}

export function TimeScrubber({ value, onChange, customFrom = '', customTo = '', onCustomChange }: TimeScrubberProps) {
  return (
    <div
      style={{
        padding: '10px 16px',
        borderBottom: 'var(--border-stark)',
        display: 'flex',
        gap: '8px',
        background: 'var(--color-background)',
        alignItems: 'center',
        flexWrap: 'wrap',
      }}
    >
      {PRESETS.map((r) => {
        const isActive = value === r.value
        return (
          <button
            key={r.value}
            onClick={() => onChange(r.value)}
            style={{
              ...btnBase,
              border: isActive ? '1px solid var(--color-primary)' : 'var(--border-stark)',
              color: isActive ? 'var(--color-primary)' : 'var(--color-text)',
            }}
          >
            {r.label}
          </button>
        )
      })}

      {/* Custom button */}
      <button
        onClick={() => onChange('custom')}
        style={{
          ...btnBase,
          border: value === 'custom' ? '1px solid var(--color-warning)' : 'var(--border-stark)',
          color: value === 'custom' ? 'var(--color-warning)' : 'var(--color-text)',
        }}
      >
        CUSTOM
      </button>

      {/* Custom from/to inputs — shown when 'custom' is active */}
      {value === 'custom' && (
        <>
          <span style={{ fontSize: '11px', color: 'var(--color-muted)', fontFamily: 'var(--font-mono)' }}>FROM</span>
          <input
            type="datetime-local"
            value={customFrom}
            onChange={(e) => onCustomChange?.(e.target.value, customTo)}
            style={dtInputStyle}
          />
          <span style={{ fontSize: '11px', color: 'var(--color-muted)', fontFamily: 'var(--font-mono)' }}>TO</span>
          <input
            type="datetime-local"
            value={customTo}
            onChange={(e) => onCustomChange?.(customFrom, e.target.value)}
            style={dtInputStyle}
          />
        </>
      )}
    </div>
  )
}
