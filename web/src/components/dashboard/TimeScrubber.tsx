/**
 * TimeScrubber — 4-button time range selector for the dashboard top bar.
 *
 * Renders 5M / 15M / 1H / 24H range buttons with active state styling
 * using design tokens only (no hex color literals).
 */


export type TimeRange = '5m' | '15m' | '1h' | '24h'

interface TimeScrubberProps {
  value: TimeRange
  onChange: (v: TimeRange) => void
}

const RANGES: { value: TimeRange; label: string }[] = [
  { value: '5m', label: '5M' },
  { value: '15m', label: '15M' },
  { value: '1h', label: '1H' },
  { value: '24h', label: '24H' },
]

export function TimeScrubber({ value, onChange }: TimeScrubberProps) {
  return (
    <div
      style={{
        padding: '12px 16px',
        borderBottom: 'var(--border-stark)',
        display: 'flex',
        gap: '8px',
        background: 'var(--color-background)',
      }}
    >
      {RANGES.map((r) => {
        const isActive = value === r.value
        return (
          <button
            key={r.value}
            onClick={() => onChange(r.value)}
            style={{
              background: 'transparent',
              border: isActive
                ? '1px solid var(--color-primary)'
                : 'var(--border-stark)',
              padding: '6px 12px',
              fontFamily: 'var(--font-mono)',
              fontSize: '11px',
              textTransform: 'uppercase',
              color: isActive ? 'var(--color-primary)' : 'var(--color-text)',
              cursor: 'pointer',
            }}
          >
            {r.label}
          </button>
        )
      })}
    </div>
  )
}
