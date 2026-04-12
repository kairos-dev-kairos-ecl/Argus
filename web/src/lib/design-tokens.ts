/**
 * Argus Design System Tokens
 * Dark-first theme with Geist typography
 */

export const colors = {
  // Background & Surface
  background: '#0A0A0B',
  foreground: '#FFFFFF',
  muted_background: '#1F1F23',
  muted_foreground: '#A0A0A0',
  border: '#2A2A2F',

  // Layer Colors (L1-L10)
  layer: {
    l1: '#EF4444',  // Red
    l2: '#F97316',  // Orange
    l3: '#EAB308',  // Yellow
    l4: '#22C55E',  // Green
    l5: '#10B981',  // Emerald
    l6: '#06B6D4',  // Cyan
    l7: '#3B82F6',  // Blue
    l8: '#8B5CF6',  // Violet
    l9: '#EC4899',  // Pink
    l10: '#F43F5E', // Rose
  },

  // Status Colors
  status: {
    success: '#22C55E',
    warning: '#EAB308',
    error: '#EF4444',
    info: '#3B82F6',
  },

  // Semantic
  primary: '#3B82F6',
  secondary: '#8B5CF6',
  destructive: '#EF4444',
  accent: '#3B82F6',
};

export const typography = {
  fontFamily: {
    sans: '"Geist", -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif',
    mono: '"Geist Mono", "SF Mono", Monaco, "Cascadia Code", monospace',
  },
  fontSize: {
    xs: '12px',
    sm: '14px',
    base: '16px',
    lg: '18px',
    xl: '20px',
    '2xl': '24px',
  },
  fontWeight: {
    normal: 400,
    medium: 500,
    semibold: 600,
    bold: 700,
  },
  lineHeight: {
    tight: 1.2,
    normal: 1.5,
    relaxed: 1.75,
  },
};

export const spacing = {
  0: '0px',
  1: '4px',
  2: '8px',
  3: '12px',
  4: '16px',
  5: '20px',
  6: '24px',
  8: '32px',
  10: '40px',
  12: '48px',
  16: '64px',
};

export const radius = {
  none: '0px',
  sm: '4px',
  base: '6px',
  md: '8px',
  lg: '12px',
  full: '9999px',
};

export const shadows = {
  none: 'none',
  sm: '0 1px 2px 0 rgba(0, 0, 0, 0.05)',
  base: '0 1px 3px 0 rgba(0, 0, 0, 0.1), 0 1px 2px 0 rgba(0, 0, 0, 0.06)',
  md: '0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -1px rgba(0, 0, 0, 0.06)',
  lg: '0 10px 15px -3px rgba(0, 0, 0, 0.1), 0 4px 6px -2px rgba(0, 0, 0, 0.05)',
};

export const transitions = {
  fast: '150ms ease-out',
  base: '200ms ease-out',
  slow: '300ms ease-out',
};

export const zIndex = {
  hide: -1,
  auto: 'auto',
  base: 0,
  dropdown: 1000,
  modal: 1050,
  toast: 1100,
  tooltip: 1200,
};

// Accessibility
export const focus = {
  outline: `2px solid ${colors.primary}`,
  outlineOffset: '2px',
};

export const contrast = {
  minRatio: 4.5, // WCAG AA standard
  touchTarget: 44, // px
};
