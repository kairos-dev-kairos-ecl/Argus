import { useState, useEffect, useRef } from 'react'
import { scrambleText } from '../../lib/text-scramble'

interface ValidationConsoleProps {
  onComplete: () => void
}

const SEQUENCE = [
  'INITIATING TELEMETRY HANDSHAKE…',
  'PINGING /health…',
  'OK · 200',
  'POST /v1/signals (test)…',
  'OK · ACCEPTED',
  'VALIDATION COMPLETE',
]

export function ValidationConsole({ onComplete }: ValidationConsoleProps) {
  const [lines, setLines] = useState<string[]>([])
  const [activeLine, setActiveLine] = useState<string>('')
  const [_activeIdx, setActiveIdx] = useState(0)
  const [done, setDone] = useState(false)
  const cancelRef = useRef<(() => void) | null>(null)

  useEffect(() => {
    let idx = 0
    const runNext = () => {
      if (idx >= SEQUENCE.length) {
        setDone(true)
        setTimeout(onComplete, 600)
        return
      }
      const target = SEQUENCE[idx]
      cancelRef.current = scrambleText(
        target,
        (s) => setActiveLine(s),
        800,
        50
      )
      // After scramble completes, push to lines and advance
      setTimeout(() => {
        setLines((prev) => [...prev, target])
        setActiveLine('')
        setActiveIdx((i) => i + 1)
        idx++
        setTimeout(runNext, 200)
      }, 900) // slightly after scramble duration
    }
    const startTimer = setTimeout(runNext, 300)
    return () => {
      clearTimeout(startTimer)
      cancelRef.current?.()
    }
  }, [onComplete])

  return (
    <div style={{
      background: 'var(--color-background)',
      border: 'var(--border-stark)',
      padding: '16px',
      fontFamily: 'var(--font-mono)',
      fontSize: '12px',
      color: 'var(--color-text)',
      height: '240px',
      overflow: 'auto',
    }}>
      {lines.map((line, i) => (
        <div key={i} style={{ lineHeight: '24px', color: 'var(--color-text)' }}>
          {line}
        </div>
      ))}
      {activeLine && (
        <div style={{ lineHeight: '24px', color: 'var(--color-primary)' }}>
          {activeLine}
        </div>
      )}
      {!done && (
        <span style={{
          display: 'inline-block',
          color: 'var(--color-primary)',
          animation: 'blink 1s steps(1) infinite',
        }}>_</span>
      )}
      {done && (
        <div style={{ marginTop: '8px', color: 'var(--color-success)', textTransform: 'uppercase', letterSpacing: '0.06em' }}>
          ● SYSTEM READY
        </div>
      )}
    </div>
  )
}
