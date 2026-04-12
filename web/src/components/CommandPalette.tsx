import React, { useState, useEffect, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import Fuse from 'fuse.js'

interface CommandItem {
  id: string
  title: string
  category: 'navigation' | 'action' | 'recent'
  description?: string
  icon?: string
  shortcut?: string
  action?: () => void
  href?: string
}

/**
 * Command Palette Component
 * Global search and navigation (⌘K / Ctrl+K)
 * - Real-time fuzzy search across navigation items
 * - Recent items tracking
 * - Keyboard navigation (arrow keys, enter, escape)
 */
export const CommandPalette: React.FC = () => {
  const [open, setOpen] = useState(false)
  const [search, setSearch] = useState('')
  const [selectedIndex, setSelectedIndex] = useState(0)
  const [recent, setRecent] = useState<CommandItem[]>([])
  const inputRef = useRef<HTMLInputElement>(null)
  const navigate = useNavigate()

  const navigationItems: CommandItem[] = [
    // Dashboard
    { id: 'dash-overview', title: 'Dashboard Overview', category: 'navigation', icon: '📊', href: '/dashboard', shortcut: '⌘⇧D' },
    { id: 'signals', title: 'Signal Stream', category: 'navigation', icon: '📡', href: '/signals', shortcut: '⌘⇧S' },
    { id: 'incidents', title: 'Incidents', category: 'navigation', icon: '⚠️', href: '/incidents', shortcut: '⌘⇧I' },
    { id: 'alerts', title: 'Alerts', category: 'navigation', icon: '🔔', href: '/alerts', shortcut: '⌘⇧A' },
    { id: 'rules', title: 'Detection Rules', category: 'navigation', icon: '📋', href: '/rules', shortcut: '⌘⇧R' },
    { id: 'apps', title: 'Apps', category: 'navigation', icon: '📱', href: '/apps', shortcut: '⌘⇧O' },
    { id: 'connectors', title: 'LLM Connectors', category: 'navigation', icon: '🔌', href: '/connectors', shortcut: '⌘⇧L' },
    { id: 'settings', title: 'Settings', category: 'navigation', icon: '⚙️', href: '/settings', shortcut: '⌘⇧,' },
    { id: 'query', title: 'Query Interface', category: 'navigation', icon: '💾', href: '/query', shortcut: '⌘⇧Q' },
    { id: 'trace', title: 'Trace View', category: 'navigation', icon: '🔍', href: '/trace', shortcut: '⌘⇧T' },

    // Actions
    { id: 'action-new-app', title: 'Create New App', category: 'action', icon: '✨', description: 'Register a new monitored application', action: () => navigate('/apps?new=true') },
    { id: 'action-new-rule', title: 'Create Detection Rule', category: 'action', icon: '📝', description: 'Add a new detection rule', action: () => navigate('/rules?new=true') },
    { id: 'action-theme-toggle', title: 'Toggle Theme', category: 'action', icon: '🌙', description: 'Switch between light and dark theme', action: () => {} },
  ]

  // Load recent items from localStorage
  useEffect(() => {
    const stored = localStorage.getItem('argus:recent-items')
    if (stored) {
      try {
        setRecent(JSON.parse(stored))
      } catch (e) {
        // Ignore parse errors
      }
    }
  }, [])

  // Keyboard shortcut listener
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      // ⌘K or Ctrl+K
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault()
        setOpen(!open)
        setSearch('')
        setSelectedIndex(0)
      }
      // Escape to close
      if (e.key === 'Escape' && open) {
        setOpen(false)
        setSearch('')
      }
    }

    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [open])

  // Focus input on open
  useEffect(() => {
    if (open && inputRef.current) {
      inputRef.current.focus()
    }
  }, [open])

  // Fuzzy search
  const fuse = new Fuse(
    [...navigationItems, ...recent.filter(r => !navigationItems.find(n => n.id === r.id))],
    {
      keys: ['title', 'description'],
      threshold: 0.3,
      minMatchCharLength: 1,
    }
  )

  const results = search
    ? fuse.search(search).map(r => r.item)
    : [
        ...recent.filter(r => !navigationItems.find(n => n.id === r.id)).slice(0, 5),
        ...navigationItems.slice(0, 10),
      ]

  const handleSelect = (item: CommandItem) => {
    // Track recent item
    const newRecent = [item, ...recent.filter(r => r.id !== item.id)].slice(0, 10)
    setRecent(newRecent)
    localStorage.setItem('argus:recent-items', JSON.stringify(newRecent))

    if (item.href) {
      navigate(item.href)
    } else if (item.action) {
      item.action()
    }
    setOpen(false)
    setSearch('')
  }

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    switch (e.key) {
      case 'ArrowDown':
        e.preventDefault()
        setSelectedIndex(prev => (prev + 1) % results.length)
        break
      case 'ArrowUp':
        e.preventDefault()
        setSelectedIndex(prev => (prev - 1 + results.length) % results.length)
        break
      case 'Enter':
        e.preventDefault()
        if (results[selectedIndex]) {
          handleSelect(results[selectedIndex])
        }
        break
    }
  }

  if (!open) return null

  return (
    <>
      {/* Backdrop */}
      <div
        className="fixed inset-0 bg-black/50 z-40"
        onClick={() => setOpen(false)}
      />

      {/* Modal */}
      <div className="fixed inset-0 z-50 flex items-start justify-center pt-20">
        <div className="w-full max-w-2xl mx-4">
          <div className="bg-muted-background border border-border rounded-lg shadow-lg overflow-hidden">
            {/* Search Input */}
            <div className="p-4 border-b border-border">
              <input
                ref={inputRef}
                type="text"
                placeholder="Search pages, actions..."
                value={search}
                onChange={e => {
                  setSearch(e.target.value)
                  setSelectedIndex(0)
                }}
                onKeyDown={handleKeyDown}
                className="w-full bg-background text-foreground placeholder-muted-foreground px-3 py-2 rounded border border-border focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent"
              />
            </div>

            {/* Results */}
            <div className="max-h-96 overflow-y-auto">
              {results.length === 0 ? (
                <div className="px-4 py-8 text-center text-muted-foreground">
                  No results found
                </div>
              ) : (
                <>
                  {/* Recent section */}
                  {!search && recent.length > 0 && (
                    <>
                      <div className="px-4 py-2 text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                        Recent
                      </div>
                      {recent.slice(0, 3).map((item) => {
                        const actualIdx = results.indexOf(item)
                        return (
                          <CommandPaletteItem
                            key={item.id}
                            item={item}
                            isSelected={selectedIndex === actualIdx}
                            onSelect={() => handleSelect(item)}
                            onHover={() => setSelectedIndex(actualIdx)}
                          />
                        )
                      })}
                    </>
                  )}

                  {/* Navigation section */}
                  {results.some(r => r.category === 'navigation') && (
                    <>
                      <div className="px-4 py-2 text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                        Navigation
                      </div>
                      {results
                        .filter(r => r.category === 'navigation')
                        .map((item) => {
                          const actualIdx = results.indexOf(item)
                          return (
                            <CommandPaletteItem
                              key={item.id}
                              item={item}
                              isSelected={selectedIndex === actualIdx}
                              onSelect={() => handleSelect(item)}
                              onHover={() => setSelectedIndex(actualIdx)}
                            />
                          )
                        })}
                    </>
                  )}

                  {/* Actions section */}
                  {results.some(r => r.category === 'action') && (
                    <>
                      <div className="px-4 py-2 text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                        Actions
                      </div>
                      {results
                        .filter(r => r.category === 'action')
                        .map((item) => {
                          const actualIdx = results.indexOf(item)
                          return (
                            <CommandPaletteItem
                              key={item.id}
                              item={item}
                              isSelected={selectedIndex === actualIdx}
                              onSelect={() => handleSelect(item)}
                              onHover={() => setSelectedIndex(actualIdx)}
                            />
                          )
                        })}
                    </>
                  )}
                </>
              )}
            </div>

            {/* Footer hint */}
            <div className="px-4 py-3 border-t border-border bg-background/50 text-xs text-muted-foreground flex justify-between">
              <span>Use arrow keys to navigate, Enter to select, Esc to close</span>
            </div>
          </div>
        </div>
      </div>
    </>
  )
}

interface CommandPaletteItemProps {
  item: CommandItem
  isSelected: boolean
  onSelect: () => void
  onHover: () => void
}

const CommandPaletteItem: React.FC<CommandPaletteItemProps> = ({
  item,
  isSelected,
  onSelect,
  onHover,
}) => (
  <button
    onClick={onSelect}
    onMouseEnter={onHover}
    className={`w-full px-4 py-3 flex items-center justify-between text-left transition-colors ${
      isSelected
        ? 'bg-primary/20 border-l-2 border-primary'
        : 'hover:bg-muted-background'
    }`}
  >
    <div className="flex items-center gap-3 flex-1">
      {item.icon && <span className="text-base">{item.icon}</span>}
      <div>
        <div className="text-sm font-medium text-foreground">{item.title}</div>
        {item.description && (
          <div className="text-xs text-muted-foreground">{item.description}</div>
        )}
      </div>
    </div>
    {item.shortcut && (
      <div className="text-xs text-muted-foreground ml-4">{item.shortcut}</div>
    )}
  </button>
)
