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
        // Background & Surface
        background: colors.background,
        foreground: colors.foreground,
        'muted-background': colors.muted_background,
        'muted-foreground': colors.muted_foreground,
        border: colors.border,

        // Layer Colors (L1-L10)
        'layer-l1': colors.layer.l1,
        'layer-l2': colors.layer.l2,
        'layer-l3': colors.layer.l3,
        'layer-l4': colors.layer.l4,
        'layer-l5': colors.layer.l5,
        'layer-l6': colors.layer.l6,
        'layer-l7': colors.layer.l7,
        'layer-l8': colors.layer.l8,
        'layer-l9': colors.layer.l9,
        'layer-l10': colors.layer.l10,

        // Status Colors
        'status-success': colors.status.success,
        'status-warning': colors.status.warning,
        'status-error': colors.status.error,
        'status-info': colors.status.info,

        // Semantic
        primary: colors.primary,
        secondary: colors.secondary,
        destructive: colors.destructive,
        accent: colors.accent,
      },
      fontFamily: {
        sans: typography.fontFamily.sans,
        mono: typography.fontFamily.mono,
      },
      spacing,
      borderRadius: radius,
      boxShadow: shadows,
      transitionDuration: {
        'fast': transitions.fast.split(' ')[0],
        'base': transitions.base.split(' ')[0],
        'slow': transitions.slow.split(' ')[0],
      },
    },
  },
  plugins: [],
}
