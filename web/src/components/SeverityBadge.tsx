interface SeverityBadgeProps {
  severity: number
}

/**
 * SeverityBadge Component
 *
 * Displays a color-coded severity badge (1-5).
 * Severity 5 = red, 4 = orange, 3 = yellow, 2 = blue, 1 = gray
 *
 * @param severity - Severity level (1-5)
 */
export const SeverityBadge: React.FC<SeverityBadgeProps> = ({ severity }) => {
  const getSeverityColor = (sev: number): string => {
    switch (sev) {
      case 5:
        return 'bg-status-error text-foreground'
      case 4:
        return 'bg-status-warning text-foreground'
      case 3:
        return 'bg-status-warning/70 text-foreground'
      case 2:
        return 'bg-status-info text-foreground'
      case 1:
      default:
        return 'bg-muted-foreground/30 text-foreground'
    }
  }

  return (
    <div
      className={`${getSeverityColor(severity)} flex-shrink-0 px-2 py-1 rounded text-xs font-semibold w-16 text-center h-7 flex items-center justify-center`}
      aria-label={`Severity ${severity}`}
    >
      {severity}
    </div>
  )
}
