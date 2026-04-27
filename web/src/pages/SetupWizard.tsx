import { useState } from 'react'
import { TokenVault } from '../components/setup/TokenVault'
import { ValidationConsole } from '../components/setup/ValidationConsole'
import { createApiKey } from '../services/iam-service'

type Step = 'ORG' | 'INGESTION' | 'TOKEN' | 'VALIDATION'
const STEPS: Step[] = ['ORG', 'INGESTION', 'TOKEN', 'VALIDATION']

const inputStyle = {
  width: '100%',
  marginTop: '6px',
  padding: '8px',
  background: 'var(--color-background)',
  border: 'var(--border-stark)',
  color: 'var(--color-text)',
  fontFamily: 'var(--font-mono)',
  fontSize: '12px',
  boxSizing: 'border-box' as const,
}

const btnPrimary = {
  marginTop: '16px',
  padding: '8px 16px',
  border: '1px solid var(--color-primary)',
  color: 'var(--color-primary)',
  background: 'transparent',
  fontFamily: 'var(--font-mono)',
  fontSize: '12px',
  textTransform: 'uppercase' as const,
  letterSpacing: '0.06em',
  cursor: 'pointer',
}

const labelStyle = {
  fontSize: '11px',
  color: 'var(--color-muted)',
  textTransform: 'uppercase' as const,
  letterSpacing: '0.06em',
  display: 'block',
}

export function SetupWizard() {
  const [step, setStep] = useState<Step>('ORG')
  const [orgName, setOrgName] = useState('')
  const [appName, setAppName] = useState('default-app')
  const [issuedToken, setIssuedToken] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  const issueToken = async () => {
    setLoading(true)
    setError(null)
    try {
      const res = await createApiKey(appName || 'default-app')
      setIssuedToken(res.key)
      setStep('TOKEN')
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Token issuance failed')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div style={{
      minHeight: '100vh',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      background: 'var(--color-background)',
      color: 'var(--color-text)',
      fontFamily: 'var(--font-mono)',
      padding: '24px',
    }}>
      <div style={{
        width: '100%',
        maxWidth: '800px',
        display: 'grid',
        gridTemplateColumns: '30% 70%',
        border: 'var(--border-stark)',
        background: 'var(--color-surface)',
        minHeight: '500px',
      }}>
        {/* ── Left: progress tracker ── */}
        <aside style={{
          borderRight: 'var(--border-stark)',
          padding: '24px 16px',
          background: 'var(--color-background)',
        }}>
          <div style={{
            fontFamily: 'var(--font-display)',
            fontSize: '16px',
            color: 'var(--color-primary)',
            textTransform: 'uppercase',
            marginBottom: '20px',
            letterSpacing: '0.04em',
          }}>
            ARGUS_XDR<span style={{ animation: 'blink 1s steps(1) infinite' }}>_</span>
          </div>

          {STEPS.map((s, i) => {
            const isDone = STEPS.indexOf(step) > i
            const isActive = s === step
            return (
              <div key={s} style={{
                padding: '8px 0',
                paddingLeft: '8px',
                fontSize: '12px',
                textTransform: 'uppercase',
                letterSpacing: '0.06em',
                color: isActive ? 'var(--color-primary)' : isDone ? 'var(--color-text)' : 'var(--color-muted)',
                borderLeft: isActive ? '2px solid var(--color-primary)' : '2px solid transparent',
              }}>
                {String(i + 1).padStart(2, '0')} · {s}
              </div>
            )
          })}
        </aside>

        {/* ── Right: active step ── */}
        <main style={{ padding: '24px' }}>
          {step === 'ORG' && (
            <div>
              <h2 style={{ fontFamily: 'var(--font-display)', fontSize: '20px', color: 'var(--color-primary)', textTransform: 'uppercase', marginBottom: '16px' }}>
                STEP 01 · ORG
              </h2>
              <label style={labelStyle}>ORGANIZATION NAME</label>
              <input
                value={orgName}
                onChange={(e) => setOrgName(e.target.value)}
                placeholder="Acme Corp"
                style={inputStyle}
              />
              <button
                onClick={() => setStep('INGESTION')}
                disabled={!orgName.trim()}
                style={{ ...btnPrimary, opacity: orgName.trim() ? 1 : 0.4 }}
              >
                NEXT
              </button>
            </div>
          )}

          {step === 'INGESTION' && (
            <div>
              <h2 style={{ fontFamily: 'var(--font-display)', fontSize: '20px', color: 'var(--color-primary)', textTransform: 'uppercase', marginBottom: '16px' }}>
                STEP 02 · INGESTION
              </h2>
              <label style={labelStyle}>APP NAME</label>
              <input
                value={appName}
                onChange={(e) => setAppName(e.target.value)}
                placeholder="my-llm-app"
                style={inputStyle}
              />
              <div style={{ marginTop: '8px', fontSize: '11px', color: 'var(--color-muted)' }}>
                A dedicated API key will be issued for this application.
              </div>
              {error && (
                <div style={{ marginTop: '12px', color: 'var(--color-alert)', fontSize: '11px', textTransform: 'uppercase' }}>
                  ERROR: {error}
                </div>
              )}
              <button
                onClick={issueToken}
                disabled={loading || !appName.trim()}
                style={{ ...btnPrimary, opacity: loading || !appName.trim() ? 0.4 : 1 }}
              >
                {loading ? 'ISSUING…' : 'ISSUE TOKEN'}
              </button>
            </div>
          )}

          {step === 'TOKEN' && issuedToken && (
            <div>
              <h2 style={{ fontFamily: 'var(--font-display)', fontSize: '20px', color: 'var(--color-primary)', textTransform: 'uppercase', marginBottom: '16px' }}>
                STEP 03 · TOKEN
              </h2>
              <TokenVault token={issuedToken} onAcknowledge={() => setStep('VALIDATION')} />
            </div>
          )}

          {step === 'VALIDATION' && (
            <div>
              <h2 style={{ fontFamily: 'var(--font-display)', fontSize: '20px', color: 'var(--color-primary)', textTransform: 'uppercase', marginBottom: '16px' }}>
                STEP 04 · VALIDATION
              </h2>
              <ValidationConsole onComplete={() => { window.location.href = '/dashboard' }} />
            </div>
          )}
        </main>
      </div>

      <style>{`@keyframes blink { 0%, 50% { opacity: 1; } 51%, 100% { opacity: 0; } }`}</style>
    </div>
  )
}
