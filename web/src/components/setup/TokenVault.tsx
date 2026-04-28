import { useState } from 'react'

interface TokenVaultProps {
  token: string
  onAcknowledge: () => void
}

export function TokenVault({ token, onAcknowledge }: TokenVaultProps) {
  const [copied, setCopied] = useState(false)

  const handleCopy = () => {
    navigator.clipboard.writeText(token).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    })
  }

  return (
    <div style={{
      border: '1px solid var(--color-primary)',
      background: 'var(--color-surface)',
      padding: '16px',
      marginTop: '16px',
    }}>
      <div style={{
        fontFamily: 'var(--font-display)',
        fontSize: '14px',
        color: 'var(--color-primary)',
        textTransform: 'uppercase',
        letterSpacing: '0.06em',
        marginBottom: '8px',
      }}>
        API TOKEN — ONE-TIME VISIBILITY
      </div>

      <div style={{
        background: 'var(--color-warning)',
        color: 'var(--color-background)',
        padding: '6px 10px',
        fontSize: '11px',
        textTransform: 'uppercase',
        letterSpacing: '0.06em',
        marginBottom: '12px',
        fontFamily: 'var(--font-mono)',
        fontWeight: 700,
      }}>
        STORE THIS NOW. WE WILL NOT SHOW IT AGAIN.
      </div>

      <code style={{
        display: 'block',
        padding: '12px',
        background: 'var(--color-background)',
        border: 'var(--border-stark)',
        fontFamily: 'var(--font-mono)',
        fontSize: '13px',
        wordBreak: 'break-all',
        color: 'var(--color-text)',
        userSelect: 'all',
      }}>
        {token}
      </code>

      <div style={{ marginTop: '12px', display: 'flex', gap: '8px' }}>
        <button
          onClick={handleCopy}
          style={{
            padding: '8px 16px',
            border: '1px solid var(--color-primary)',
            color: copied ? 'var(--color-success)' : 'var(--color-primary)',
            background: 'transparent',
            fontFamily: 'var(--font-mono)',
            fontSize: '12px',
            textTransform: 'uppercase',
            cursor: 'pointer',
            letterSpacing: '0.06em',
          }}
        >
          {copied ? 'COPIED ✓' : 'COPY'}
        </button>

        <button
          onClick={onAcknowledge}
          style={{
            padding: '8px 16px',
            border: '1px solid var(--color-primary)',
            color: 'var(--color-primary)',
            background: 'transparent',
            fontFamily: 'var(--font-mono)',
            fontSize: '12px',
            textTransform: 'uppercase',
            cursor: 'pointer',
            letterSpacing: '0.06em',
          }}
        >
          I HAVE STORED THE TOKEN
        </button>
      </div>
    </div>
  )
}
