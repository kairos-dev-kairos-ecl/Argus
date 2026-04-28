// Design tokens — single source of truth in TypeScript.
// Values MUST match web/src/styles/tokens.css exactly.

export const colors = {
  // Brutalist palette (CONTEXT.md locked values)
  primary: '#00F0FF',
  background: '#050506',
  surface: '#111216',
  foreground: '#E9ECEF',
  muted_background: '#111216',
  muted_foreground: '#343A40',
  border: '#343A40',
  alert: '#FF2A00',
  warning: '#FFB300',
  secondary: '#111216',
  destructive: '#FF2A00',
  accent: '#00F0FF',

  layer: {
    l1: '#EF4444',
    l2: '#F97316',
    l3: '#F59E0B',
    l4: '#EAB308',
    l5: '#84CC16',
    l6: '#22C55E',
    l7: '#14B8A6',
    l8: '#06B6D4',
    l9: '#3B82F6',
    l10: '#F43F5E',
  },

  status: {
    success: '#22C55E',
    warning: '#EAB308',
    error: '#EF4444',
    info: '#3B82F6',
  },
} as const;

export const typography = {
  fontFamily: {
    sans: ['Space Grotesk', 'system-ui', 'sans-serif'],
    mono: ['JetBrains Mono', 'ui-monospace', 'Consolas', 'monospace'],
    display: ['Space Grotesk', 'system-ui', 'sans-serif'],
  },
} as const;

export const spacing = {
  px: '1px',
  0: '0',
  1: '4px',
  2: '8px',
  3: '12px',
  4: '16px',
  5: '20px',
  6: '24px',
  8: '32px',
  10: '40px',
  12: '48px',
  14: '56px',
  16: '64px',
} as const;

// Brutalist: zero radius everywhere
export const radius = {
  none: '0',
  sm: '0',
  DEFAULT: '0',
  md: '0',
  lg: '0',
  xl: '0',
  full: '0',
} as const;

export const borders = {
  stark: '1px solid #343A40',
  width: '1px',
  color: '#343A40',
} as const;

export const shadows = {
  none: 'none',
  sm: 'none',
  DEFAULT: 'none',
  md: 'none',
  lg: 'none',
} as const;

export const transitions = {
  fast: '100ms ease-out',
  base: '150ms ease-out',
  slow: '200ms ease-out',
} as const;

export const rowHeight = '20px';
