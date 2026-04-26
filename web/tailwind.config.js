import { colors, typography, spacing, radius, shadows, transitions } from './src/lib/design-tokens.ts';

/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        background: 'var(--color-background)',
        surface: 'var(--color-surface)',
        foreground: 'var(--color-text)',
        'muted-background': 'var(--color-surface)',
        'muted-foreground': 'var(--color-muted)',
        border: 'var(--color-muted)',
        primary: 'var(--color-primary)',
        secondary: 'var(--color-surface)',
        destructive: 'var(--color-alert)',
        accent: 'var(--color-primary)',
        alert: 'var(--color-alert)',
        warning: 'var(--color-warning)',
        'layer-l1': 'var(--color-layer-l1)',
        'layer-l2': 'var(--color-layer-l2)',
        'layer-l3': 'var(--color-layer-l3)',
        'layer-l4': 'var(--color-layer-l4)',
        'layer-l5': 'var(--color-layer-l5)',
        'layer-l6': 'var(--color-layer-l6)',
        'layer-l7': 'var(--color-layer-l7)',
        'layer-l8': 'var(--color-layer-l8)',
        'layer-l9': 'var(--color-layer-l9)',
        'layer-l10': 'var(--color-layer-l10)',
        'status-success': 'var(--color-success)',
        'status-warning': 'var(--color-status-warning)',
        'status-error': 'var(--color-error)',
        'status-info': 'var(--color-info)',
      },
      fontFamily: {
        sans: typography.fontFamily.sans,
        mono: typography.fontFamily.mono,
        display: typography.fontFamily.display,
      },
      spacing,
      borderRadius: {
        none: '0',
        sm: '0',
        DEFAULT: '0',
        md: '0',
        lg: '0',
        xl: '0',
        full: '0',
      },
      boxShadow: shadows,
    },
  },
  plugins: [],
};
