/**
 * FilterSidebar — Layer / severity / category filter chips for the dashboard.
 *
 * Token-only styling. No hex color literals.
 */

import React from 'react'

interface FilterState {
  layer?: number
  severity?: number
  category?: string
}

interface FilterSidebarProps {
  filters: FilterState
  onChange: (f: FilterState) => void
}

// Layer chips L1..L10
const LAYER_CHIPS = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10] as const

// Severity chips 1..4 (LOW, MED, HIGH, CRIT)
const SEVERITY_CHIPS: { value: number; label: string }[] = [
  { value: 1, label: 'LOW' },
  { value: 2, label: 'MED' },
  { value: 3, label: 'HIGH' },
  { value: 4, label: 'CRIT' },
]

// Category chips
const CATEGORY_CHIPS = ['METRIC', 'EVENT', 'DETECTION', 'AUDIT'] as const

const sectionHeaderStyle: React.CSSProperties = {
  fontSize: '11px',
  textTransform: 'uppercase',
  color: 'var(--color-muted)',
  marginBottom: '8px',
  fontFamily: 'var(--font-mono)',
}

function chipStyle(active: boolean): React.CSSProperties {
  return {
    padding: '4px 8px',
    border: active ? '1px solid var(--color-primary)' : 'var(--border-stark)',
    fontSize: '11px',
    marginRight: '6px',
    marginBottom: '6px',
    cursor: 'pointer',
    fontFamily: 'var(--font-mono)',
    color: active ? 'var(--color-primary)' : 'var(--color-text)',
    background: 'transparent',
    display: 'inline-flex',
    alignItems: 'center',
    gap: '4px',
  }
}

export function FilterSidebar({ filters, onChange }: FilterSidebarProps) {
  const toggleLayer = (n: number) => {
    onChange({ ...filters, layer: filters.layer === n ? undefined : n })
  }

  const toggleSeverity = (n: number) => {
    onChange({ ...filters, severity: filters.severity === n ? undefined : n })
  }

  const toggleCategory = (c: string) => {
    onChange({ ...filters, category: filters.category === c ? undefined : c })
  }

  return (
    <div
      style={{
        width: '200px',
        borderRight: 'var(--border-stark)',
        padding: '16px',
        fontFamily: 'var(--font-mono)',
        background: 'var(--color-background)',
        overflowY: 'auto',
        flexShrink: 0,
      }}
    >
      {/* LAYER */}
      <div style={{ marginBottom: '16px' }}>
        <div style={sectionHeaderStyle}>LAYER</div>
        <div style={{ display: 'flex', flexWrap: 'wrap' }}>
          {LAYER_CHIPS.map((n) => (
            <button
              key={n}
              onClick={() => toggleLayer(n)}
              style={chipStyle(filters.layer === n)}
            >
              {/* Color stripe for layer indicator */}
              <span
                style={{
                  width: '6px',
                  height: '6px',
                  borderRadius: '50%',
                  background: `var(--color-layer-l${n})`,
                  display: 'inline-block',
                  flexShrink: 0,
                }}
              />
              L{n}
            </button>
          ))}
        </div>
      </div>

      {/* SEVERITY */}
      <div style={{ marginBottom: '16px' }}>
        <div style={sectionHeaderStyle}>SEVERITY</div>
        <div style={{ display: 'flex', flexWrap: 'wrap' }}>
          {SEVERITY_CHIPS.map((s) => (
            <button
              key={s.value}
              onClick={() => toggleSeverity(s.value)}
              style={chipStyle(filters.severity === s.value)}
            >
              {s.label}
            </button>
          ))}
        </div>
      </div>

      {/* CATEGORY */}
      <div style={{ marginBottom: '16px' }}>
        <div style={sectionHeaderStyle}>CATEGORY</div>
        <div style={{ display: 'flex', flexWrap: 'wrap' }}>
          {CATEGORY_CHIPS.map((c) => (
            <button
              key={c}
              onClick={() => toggleCategory(c)}
              style={chipStyle(filters.category === c)}
            >
              {c}
            </button>
          ))}
        </div>
      </div>
    </div>
  )
}
