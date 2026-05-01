import { useState } from 'react'
import { useNavigate, Link } from 'react-router-dom'
import { useAuthStore } from '../stores/auth'
import { performSetup } from '../services/iam-service'
import backdropUrl from '../assets/backdrop.png'

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

      if (res.access_token) {
        setAccessToken(res.access_token)
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
      position: 'relative',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      color: 'var(--color-text)',
      fontFamily: 'var(--font-mono)',
      padding: '24px',
    }}>
      {/* Backdrop image */}
      <div style={{
        position: 'fixed',
        inset: 0,
        backgroundImage: `url(${backdropUrl})`,
        backgroundSize: 'cover',
        backgroundPosition: 'center right',
        zIndex: 0,
      }} />
      <div style={{
        position: 'fixed',
        inset: 0,
        background: 'rgba(10, 10, 11, 0.82)',
        zIndex: 1,
      }} />

      {/* Wizard panel */}
      <div style={{
        position: 'relative',
        zIndex: 2,
        width: '100%',
        maxWidth: '820px',
        display: 'grid',
        gridTemplateColumns: '220px 1fr',
        border: 'var(--border-stark)',
        background: 'rgba(10, 10, 11, 0.92)',
        backdropFilter: 'blur(12px)',
        minHeight: '480px',
      }}>
        {/* Left: progress tracker */}
        <aside style={{
          borderRight: 'var(--border-stark)',
          padding: '28px 20px',
          display: 'flex',
          flexDirection: 'column',
        }}>
          <div style={{
            fontFamily: 'var(--font-display)',
            fontSize: '15px',
            color: 'var(--color-primary)',
            textTransform: 'uppercase',
            marginBottom: '28px',
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
                padding: '9px 0',
                paddingLeft: '10px',
                fontSize: '11px',
                textTransform: 'uppercase',
                letterSpacing: '0.06em',
                color: isActive ? 'var(--color-primary)' : isDone ? 'var(--color-text)' : 'var(--color-muted)',
                borderLeft: isActive ? '2px solid var(--color-primary)' : '2px solid transparent',
                marginBottom: '2px',
              }}>
                {isDone ? '✓' : String(i + 1).padStart(2, '0')} · {s}
              </div>
            )
          })}

          {/* Back to login link */}
          <div style={{ marginTop: 'auto', paddingTop: '24px', borderTop: 'var(--border-stark)' }}>
            <Link
              to="/login"
              style={{ fontSize: '11px', color: 'var(--color-muted)', textDecoration: 'none', textTransform: 'uppercase', letterSpacing: '0.06em' }}
            >
              ← BACK TO LOGIN
            </Link>
          </div>
        </aside>

        {/* Right: active step */}
        <main style={{ padding: '28px 28px' }}>
          {step === 'ACCOUNT' && (
            <form onSubmit={handleAccountNext}>
              <h2 style={{ fontFamily: 'var(--font-display)', fontSize: '17px', color: 'var(--color-primary)', textTransform: 'uppercase', marginBottom: '20px', marginTop: 0, letterSpacing: '0.04em' }}>
                01 · ADMIN ACCOUNT
              </h2>
              <p style={{ fontSize: '12px', color: 'var(--color-muted)', marginBottom: '16px', lineHeight: 1.5 }}>
                Create the first administrator account. This will be the owner of this Argus instance.
              </p>
              <div style={fieldGroupStyle}>
                <label style={labelStyle}>EMAIL</label>
                <input type="email" value={email} onChange={e => setEmail(e.target.value)} placeholder="admin@example.com" required autoComplete="email" style={inputStyle} />
              </div>
              <div style={fieldGroupStyle}>
                <label style={labelStyle}>DISPLAY NAME</label>
                <input type="text" value={displayName} onChange={e => setDisplayName(e.target.value)} placeholder="Your name" required style={inputStyle} />
              </div>
              <div style={fieldGroupStyle}>
                <label style={labelStyle}>PASSWORD <span style={{ color: 'var(--color-muted)', fontWeight: 'normal' }}>(min 12 chars)</span></label>
                <input type="password" value={password} onChange={e => setPassword(e.target.value)} required autoComplete="new-password" style={inputStyle} />
              </div>
              <div style={fieldGroupStyle}>
                <label style={labelStyle}>CONFIRM PASSWORD</label>
                <input type="password" value={confirmPassword} onChange={e => setConfirmPassword(e.target.value)} required autoComplete="new-password" style={inputStyle} />
              </div>
              {error && <div style={{ marginTop: '12px', color: 'var(--color-alert)', fontSize: '11px', textTransform: 'uppercase' }}>ERROR: {error}</div>}
              <button type="submit" style={btnPrimary(false)}>NEXT →</button>
            </form>
          )}

          {step === 'ORG' && (
            <form onSubmit={handleOrgSubmit}>
              <h2 style={{ fontFamily: 'var(--font-display)', fontSize: '17px', color: 'var(--color-primary)', textTransform: 'uppercase', marginBottom: '20px', marginTop: 0, letterSpacing: '0.04em' }}>
                02 · ORGANIZATION
              </h2>
              <p style={{ fontSize: '12px', color: 'var(--color-muted)', marginBottom: '16px', lineHeight: 1.5 }}>
                Name your Argus instance and the first application you want to monitor.
              </p>
              <div style={fieldGroupStyle}>
                <label style={labelStyle}>INSTANCE NAME</label>
                <input type="text" value={instanceName} onChange={e => setInstanceName(e.target.value)} placeholder="acme-argus" required style={inputStyle} />
              </div>
              <div style={fieldGroupStyle}>
                <label style={labelStyle}>FIRST APP NAME</label>
                <input type="text" value={appName} onChange={e => setAppName(e.target.value)} placeholder="my-llm-app" required style={inputStyle} />
                <span style={{ fontSize: '11px', color: 'var(--color-muted)', marginTop: '4px' }}>
                  An API key will be generated for this app to send signals.
                </span>
              </div>
              {error && <div style={{ marginTop: '12px', color: 'var(--color-alert)', fontSize: '11px', textTransform: 'uppercase' }}>ERROR: {error}</div>}
              <div style={{ display: 'flex', gap: '12px', marginTop: '16px' }}>
                <button type="button" onClick={() => setStep('ACCOUNT')} style={{ ...btnPrimary(false), marginTop: 0, color: 'var(--color-muted)', borderColor: 'var(--color-muted)' }}>← BACK</button>
                <button type="submit" disabled={loading} style={{ ...btnPrimary(loading), marginTop: 0 }}>
                  {loading ? 'CREATING...' : 'CREATE INSTANCE'}
                </button>
              </div>
            </form>
          )}

          {step === 'TOKEN' && apiKey && (
            <div>
              <h2 style={{ fontFamily: 'var(--font-display)', fontSize: '17px', color: 'var(--color-primary)', textTransform: 'uppercase', marginBottom: '16px', marginTop: 0, letterSpacing: '0.04em' }}>
                03 · SAVE YOUR API KEY
              </h2>
              <div style={{ marginBottom: '16px', padding: '10px 12px', border: '1px solid #EAB308', fontSize: '11px', color: '#EAB308', textTransform: 'uppercase', letterSpacing: '0.04em', lineHeight: 1.5 }}>
                ⚠ THIS KEY IS SHOWN ONCE. COPY IT NOW — IT CANNOT BE RETRIEVED LATER.
              </div>
              <pre style={{
                background: 'var(--color-background)',
                border: 'var(--border-stark)',
                padding: '12px 14px',
                fontFamily: 'var(--font-mono)',
                fontSize: '12px',
                color: 'var(--color-primary)',
                wordBreak: 'break-all',
                whiteSpace: 'pre-wrap',
                margin: '0 0 4px',
                lineHeight: 1.6,
              }}>
                {apiKey}
              </pre>
              <div style={{ display: 'flex', gap: '12px', marginTop: '16px' }}>
                <button onClick={handleCopy} style={btnPrimary(false)}>
                  {copied ? '✓ COPIED' : 'COPY KEY'}
                </button>
                <button onClick={() => setStep('DONE')} style={{ ...btnPrimary(false), color: 'var(--color-muted)', borderColor: 'var(--color-muted)' }}>
                  I'VE SAVED IT →
                </button>
              </div>
            </div>
          )}

          {step === 'DONE' && (
            <div>
              <h2 style={{ fontFamily: 'var(--font-display)', fontSize: '17px', color: 'var(--color-primary)', textTransform: 'uppercase', marginBottom: '20px', marginTop: 0, letterSpacing: '0.04em' }}>
                04 · SETUP COMPLETE
              </h2>
              <p style={{ fontSize: '13px', color: 'var(--color-text)', lineHeight: 1.7, marginBottom: '8px' }}>
                Your Argus instance is ready. You're logged in as admin.
              </p>
              <p style={{ fontSize: '12px', color: 'var(--color-muted)', lineHeight: 1.6, marginBottom: '24px' }}>
                Use the API key to configure your SDK. Invite your team from the <strong style={{ color: 'var(--color-text)' }}>Users</strong> page in the dashboard.
              </p>
              <button onClick={() => navigate('/')} style={btnPrimary(false)}>
                GO TO DASHBOARD →
              </button>
            </div>
          )}
        </main>
      </div>

      <style>{`@keyframes blink { 0%, 50% { opacity: 1; } 51%, 100% { opacity: 0; } }`}</style>
    </div>
  )
}
