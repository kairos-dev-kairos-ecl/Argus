/**
 * Skeleton Component
 * Placeholder loading shimmer for async content
 * Uses animate-pulse for smooth loading effect
 */
export const Skeleton: React.FC<{
  width?: string
  height?: string
  className?: string
  circle?: boolean
}> = ({ width = '100%', height = '20px', className = '', circle = false }) => {
  const baseClass = 'bg-muted-foreground/20 animate-pulse'
  const circleClass = circle ? 'rounded-full' : 'rounded'

  return (
    <div
      className={`${baseClass} ${circleClass} ${className}`}
      style={{ width, height }}
      aria-busy={true}
    />
  )
}

/**
 * SkeletonTable Component
 * Skeleton loader for table data
 */
export const SkeletonTable: React.FC<{ rows?: number; cols?: number }> = ({ rows = 5, cols = 4 }) => {
  return (
    <div className="space-y-2">
      {/* Header row */}
      <div className="flex gap-3 p-3 bg-muted-background/50 rounded">
        {Array.from({ length: cols }).map((_, i) => (
          <Skeleton key={`header-${i}`} width="100%" height="20px" />
        ))}
      </div>

      {/* Data rows */}
      {Array.from({ length: rows }).map((_, rowIdx) => (
        <div key={`row-${rowIdx}`} className="flex gap-3 p-3 border border-border rounded">
          {Array.from({ length: cols }).map((_, colIdx) => (
            <Skeleton key={`cell-${rowIdx}-${colIdx}`} width="100%" height="20px" />
          ))}
        </div>
      ))}
    </div>
  )
}

/**
 * SkeletonGrid Component
 * Skeleton loader for grid layouts
 */
export const SkeletonGrid: React.FC<{ count?: number }> = ({ count = 4 }) => {
  return (
    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
      {Array.from({ length: count }).map((_, i) => (
        <div key={`skeleton-${i}`} className="space-y-2">
          <Skeleton height="120px" />
          <Skeleton height="16px" width="80%" />
          <Skeleton height="16px" width="60%" />
        </div>
      ))}
    </div>
  )
}

/**
 * SkeletonText Component
 * Skeleton loader for text content (multiple lines)
 */
export const SkeletonText: React.FC<{ lines?: number }> = ({ lines = 3 }) => {
  return (
    <div className="space-y-2">
      {Array.from({ length: lines }).map((_, i) => (
        <Skeleton
          key={`line-${i}`}
          height="16px"
          width={i === lines - 1 ? '70%' : '100%'}
        />
      ))}
    </div>
  )
}
