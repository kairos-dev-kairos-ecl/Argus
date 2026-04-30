import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuthStore } from '../stores/auth'
import { performSetup } from '../services/iam-service'

type Step = 'ACCOUNT' | 'ORG' | 'TOKEN' | 'DONE'
const STEPS: Step[] = ['ACCOUNT', 'ORG', 'TOKEN', 'DONE']

const inputStyle: React.CSSProperties = {
  width: '100%',
  marginTop: '6px',
  padding: '8px 10px',
  background: 'var(--color-background)',
  border: 'var(--border-stark)',
  color: 'var(--color-text)',
  fontFamily: 'var(--font-mono)',
  fontSize: '13px',
  boxSizing: 'border-box',
  outline: 'none',
}

const btnPrimary = (disabled: boolean): React.CSSProperties => ({
  marginTop: '16px',
  padding: '10px 20px',
  border: '1px solid var(--color-primary)',
  color: disabled ? 'var(--color-muted)' : 'var(--color-primary)',
  borderColor: disabled ? 'var(--color-muted)' : 'var(--color-primary)',
  background: 'transparent',
  fontFamily: 'var(--font-mono)',
  fontSize: '12px',
  textTransform: 'uppercase' as const,
  letterSpacing: '0.06em',
  cursor: disabled ? 'not-allowed' : 'pointer',
  opacity: disabled ? 0.5 : 1,
})

const labelStyle: React.CSSProperties = {
  fontSize: '11px',
  color: 'var(--color-muted)',
  textTransform: 'uppercase' as const,
  letterSpacing: '0.06em',
  display: 'block',
  marginTop: '12px',
}

const fieldGroupStyle: React.CSSProperties = {
  display: 'flex',
  flexDirection: 'column',
  gap: '2px',
}

export function SetupWizard() {
  const navigate = useNavigate()
  const { setAccessToken, setUser } = useAuthStore()

  const [step, setStep] = useState<Step>('ACCOUNT')
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  // ACCOUNT step fields
  const [email, setEmail] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')

  // ORG step fields
  const [instanceName, setInstanceName] = useState('')
  const [appName, setAppName] = useState('Argus XDR')

  // TOKEN step data
  const [apiKey, setApiKey] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)

  const validateAccount = (): string | null => {
    if (!email || !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) return 'Valid email required'
    if (!displayName.trim()) return 'Display name required'
    if (password.length < 12) return 'Password must be at least 12 characters'
    if (password !== confirmPassword) return 'Passwords do not match'
    return null
  }

  const handleAccountNext = (e: React.FormEvent) => {
    e.preventDefault()
    const err = validateAccount()
    if (err) { setError(err); return }
    setError(null)
    setStep('ORG')
  }

  const handleOrgSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!instanceName.trim()) { setError('Instance name required'); return }
    if (!appName.trim()) { setError('App name required'); return }

    setLoading(true)
    setError(null)
    try {
      const res = await performSetup({
        email,
        password,
        display_name: displayName,
        instance_name: instanceName,
        app_name: appName || 'Argus XDR',
      })

      // Store access token so the user is logged in
      if (res.access_token) {
        setAccessToken(res.access_token)
        // Decode user from JWT to populate store
        try {
          const payload = JSON.parse(atob(res.access_token.split('.')[1]))
          setUser({
            id: payload.sub,
            email: payload.email,
            display_name: payload.name,
            role: payload.role,
            permissions: payload.permissions ?? [],
            status: 'active',
            created_at: new Date(payload.iat * 1000).toISOString(),
          } as any)
        } catch {
          // JWT decode failed — user not set but token is stored
        }
      }

      setApiKey(res.api_key)
      setStep('TOKEN')
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Setup failed')
    } finally {
      setLoading(false)
    }
  }

  const handleCopy = () => {
    if (apiKey) {
      navigator.clipboard.writeText(apiKey).then(() => {
        setCopied(true)
        setTimeout(() => setCopied(false), 2000)
      })
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
        minHeight: '480px',
      }}>
        {/* Left: progress tracker */}
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
            marginBottom: '24px',
            letterSpacing: '0.04em',
          }}>
            ARGUS_XDR<span style={{ animation: 'blink 1s steps(1) infinite' }}>_</span>
          </div>

          {STEPS.map((s, i) => {
            const stepIdx = STEPS.indexOf(step)
            const isDone = stepIdx > i
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
                marginBottom: '4px',
              }}>
                {isDone ? '✓' : String(i + 1).padStart(2, '0')} · {s}
              </div>
            )
          })}
        </aside>

        {/* Right: active step */}
        <main style={{ padding: '28px 24px' }}>
          {step === 'ACCOUNT' && (
            <form onSubmit={handleAccountNext}>
              <h2 style={{ fontFamily: 'var(--font-display)', fontSize: '18px', color: 'var(--color-primary)', textTransform: 'uppercase', marginBottom: '20px', marginTop: 0 }}>
                01 · ACCOUNT SETUP
              </h2>
              <div style={fieldGroupStyle}>
                <label style={labelStyle}>EMAIL</label>
                <input
                  type="email"
                  value={email}
                  onChange={e => setEmail(e.target.value)}
                  placeholder="admin@example.com"
                  required
                  autoComplete="email"
                  style={inputStyle}
                />
              </div>
              <div style={fieldGroupStyle}>
                <label style={labelStyle}>DISPLAY NAME</label>
                <input
                  type="text"
                  value={displayName}
                  onChange={e => setDisplayName(e.target.value)}
                  placeholder="Your name"
                  required
                  style={inputStyle}
                />
              </div>
              <div style={fieldGroupStyle}>
                <label style={labelStyle}>PASSWORD (min 12 chars)</label>
                <input
                  type="password"
                  value={password}
                  onChange={e => setPassword(e.target.value)}
                  required
                  autoComplete="new-password"
                  style={inputStyle}
                />
              </div>
              <div style={fieldGroupStyle}>
                <label style={labelStyle}>CONFIRM PASSWORD</label>
                <input
                  type="password"
                  value={confirmPassword}
                  onChange={e => setConfirmPassword(e.target.value)}
                  required
                  autoComplete="new-password"
                  style={inputStyle}
                />
              </div>
              {error && (
                <div style={{ marginTop: '12px', color: 'var(--color-alert)', fontSize: '11px', textTransform: 'uppercase' }}>
                  ERROR: {error}
                </div>
              )}
              <button type="submit" style={btnPrimary(false)}>NEXT</button>
            </form>
          )}

          {step === 'ORG' && (
            <form onSubmit={handleOrgSubmit}>
              <h2 style={{ fontFamily: 'var(--font-display)', fontSize: '18px', color: 'var(--color-primary)', textTransform: 'uppercase', marginBottom: '20px', marginTop: 0 }}>
                02 · ORGANIZATION
              </h2>
              <div style={fieldGroupStyle}>
                <label style={labelStyle}>INSTANCE NAME</label>
                <input
                  type="text"
                  value={instanceName}
                  onChange={e => setInstanceName(e.target.value)}
                  placeholder="acme-argus"
                  required
                  style={inputStyle}
                />
              </div>
              <div style={fieldGroupStyle}>
                <label style={labelStyle}>APP NAME</label>
                <input
                  type="text"
                  value={appName}
                  onChange={e => setAppName(e.target.value)}
                  placeholder="Argus XDR"
                  required
                  style={inputStyle}
                />
              </div>
              {error && (
                <div style={{ marginTop: '12px', color: 'var(--color-alert)', fontSize: '11px', textTransform: 'uppercase' }}>
                  ERROR: {error}
                </div>
              )}
              <button type="submit" disabled={loading} style={btnPrimary(loading)}>
                {loading ? 'SETTING UP...' : 'CREATE ADMIN ACCOUNT'}
              </button>
            </form>
          )}

          {step === 'TOKEN' && apiKey && (
            <div>
              <h2 style={{ fontFamily: 'var(--font-display)', fontSize: '18px', color: 'var(--color-primary)', textTransform: 'uppercase', marginBottom: '16px', marginTop: 0 }}>
                03 · API KEY
              </h2>
              <div style={{ marginBottom: '16px', padding: '10px 12px', border: '1px solid var(--color-alert, #EAB308)', fontSize: '11px', color: 'var(--color-alert, #EAB308)', textTransform: 'uppercase', letterSpacing: '0.04em' }}>
                WARNING: This key is shown once. Copy it now.
              </div>
              <div style={{ position: 'relative' }}>
                <pre style={{
                  background: 'var(--color-background)',
                  border: 'var(--border-stark)',
                  padding: '12px',
                  fontFamily: 'var(--font-mono)',
                  fontSize: '12px',
                  color: 'var(--color-primary)',
                  wordBreak: 'break-all',
                  whiteSpace: 'pre-wrap',
                  margin: 0,
                }}>
                  {apiKey}
                </pre>
              </div>
              <button onClick={handleCopy} style={{ ...btnPrimary(false), marginTop: '12px' }}>
                {copied ? 'COPIED!' : 'COPY KEY'}
              </button>
              <button onClick={() => setStep('DONE')} style={{ ...btnPrimary(false), marginLeft: '12px' }}>
                CONTINUE
              </button>
            </div>
          )}

          {step === 'DONE' && (
            <div>
              <h2 style={{ fontFamily: 'var(--font-display)', fontSize: '18px', color: 'var(--color-primary)', textTransform: 'uppercase', marginBottom: '16px', marginTop: 0 }}>
                04 · COMPLETE
              </h2>
              <p style={{ fontSize: '13px', color: 'var(--color-text)', lineHeight: 1.6, marginBottom: '24px' }}>
                Setup complete. Welcome to Argus XDR.
              </p>
              <button onClick={() => navigate('/')} style={btnPrimary(false)}>
                GO TO DASHBOARD
              </button>
            </div>
          )}
        </main>
      </div>

      <style>{`@keyframes blink { 0%, 50% { opacity: 1; } 51%, 100% { opacity: 0; } }`}</style>
    </div>
  )
}
