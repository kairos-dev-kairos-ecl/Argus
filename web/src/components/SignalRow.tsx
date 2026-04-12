import { useNavigate } from 'react-router-dom'
import type { ArgusSignal } from '../types'
import { SeverityBadge } from './SeverityBadge'

interface SignalRowProps {
  signal: ArgusSignal
  style: React.CSSProperties
}

/**
 * SignalRow Component
 *
 * Renders a single signal row with severity badge, timestamp, layer, category, message, and trace_id.
 * Clickable to navigate to trace view.
 * Positioned absolutely for virtual scrolling integration.
 *
 * @param signal - The ArgusSignal to display
 * @param style - CSS transform style for virtual scroller positioning
 */
export const SignalRow: React.FC<SignalRowProps> = ({ signal, style }) => {
  const navigate = useNavigate()

  const handleClick = () => {
    navigate(`/trace/${signal.trace_id}`)
  }

  // Format timestamp to HH:mm:ss.SSS
  const formatTime = (timestamp: string): string => {
    try {
      const date = new Date(timestamp)
      return date.toLocaleTimeString('en-US', {
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
        hour12: false,
        fractionalSecondDigits: 3
      })
    } catch {
      return 'N/A'
    }
  }

  return (
    <div
      style={style}
      className="absolute top-0 left-0 right-0 h-[50px] flex items-center border-b border-border px-4 hover:bg-background cursor-pointer group transition-colors"
      onClick={handleClick}
    >
      {/* Severity badge */}
      <SeverityBadge severity={signal.severity} />

      {/* Timestamp */}
      <span className="text-xs text-slate-400 w-40 font-mono">
        {formatTime(signal.timestamp)}
      </span>

      {/* Layer */}
      <span className="text-sm text-slate-300 w-24">{signal.layer}</span>

      {/* Category */}
      <span className="text-sm text-slate-400 flex-shrink-0 w-32">{signal.category}</span>

      {/* Message preview */}
      <span className="text-sm text-slate-300 flex-1 truncate px-4">{signal.message}</span>

      {/* Trace ID (clickable) */}
      <code className="text-xs bg-slate-700 text-slate-100 px-2 py-1 rounded font-mono group-hover:bg-primary flex-shrink-0">
        {signal.trace_id.slice(0, 8)}...
      </code>
    </div>
  )
}
