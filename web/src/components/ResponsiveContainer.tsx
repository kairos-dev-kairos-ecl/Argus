/**
 * ResponsiveContainer Component
 * Provides responsive layout wrapper with breakpoint-aware classes
 * Breakpoints: sm=640px, md=768px, lg=1024px, xl=1280px
 */
export const ResponsiveContainer: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  return (
    <div className="w-full max-w-full">
      {children}
    </div>
  )
}

/**
 * MobileMenu Component
 * Hamburger menu for mobile devices
 */
export const MobileMenu: React.FC<{ isOpen: boolean; onToggle: () => void }> = ({ isOpen, onToggle }) => {
  return (
    <button
      onClick={onToggle}
      className="md:hidden h-10 w-10 p-2 text-foreground hover:bg-muted-background rounded transition-colors"
      aria-label="Toggle mobile menu"
      aria-expanded={isOpen}
    >
      <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d={isOpen ? "M6 18L18 6M6 6l12 12" : "M4 6h16M4 12h16M4 18h16"} />
      </svg>
    </button>
  )
}

/**
 * ResponsiveTable Component
 * Table that stacks to horizontal scroll on mobile
 */
export const ResponsiveTable: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  return (
    <div className="w-full overflow-x-auto">
      <table className="w-full min-w-max">
        {children}
      </table>
    </div>
  )
}

/**
 * ResponsiveGrid Component
 * Grid that adjusts columns based on viewport
 */
export const ResponsiveGrid: React.FC<{ children: React.ReactNode; cols?: string }> = ({ children, cols = "grid-cols-1 md:grid-cols-2 lg:grid-cols-3" }) => {
  return (
    <div className={`grid gap-4 ${cols}`}>
      {children}
    </div>
  )
}
