import { useState } from 'react'
import { useNavigate, Navigate, Link } from 'react-router-dom'
import { motion, AnimatePresence } from 'framer-motion'
import { BookOpen, Eye, EyeOff, Loader2 } from 'lucide-react'
import { authApi } from '@/api/auth'
import { useAuthStore } from '@/store/authStore'
import { ThemeToggle } from '@/components/ui/ThemeToggle'
import { notify } from '@/lib/notify'

type Tab = 'login' | 'register'

export default function AuthPage() {
  const [tab, setTab] = useState<Tab>('login')
  const [prefillUsername, setPrefillUsername] = useState('')
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated)
  const navigate = useNavigate()

  // Already logged in — go to browse
  if (isAuthenticated) return <Navigate to="/" replace />

  return (
    <div className="min-h-screen bg-[var(--color-bg)] flex flex-col">
      {/* Top bar */}
      <div className="flex items-center justify-between px-6 py-4">
        <Link to="/" className="flex items-center gap-2 font-bold text-[var(--color-text)] no-underline transition-opacity hover:opacity-80">
          <div className="flex h-7 w-7 items-center justify-center rounded-lg bg-[var(--brand-red)] text-white">
            <BookOpen className="h-4 w-4" />
          </div>
          <span>Manga<span className="text-[var(--brand-red)]"> Hub</span></span>
        </Link>
        <ThemeToggle />
      </div>

      {/* Main content */}
      <div className="flex flex-1 items-center justify-center px-4 py-8">
        <div className="w-full max-w-md">

          {/* Card */}
          <div className="rounded-2xl border border-[var(--color-border-raw)] bg-[var(--color-surface)] p-8 shadow-sm">

            {/* Tab switcher */}
            <div className="mb-7 flex rounded-xl bg-[var(--color-surface2)] p-1">
              {(['login', 'register'] as Tab[]).map((t) => (
                <button
                  key={t}
                  data-testid={`tab-${t}`}
                  onClick={() => setTab(t)}
                  className={`flex-1 rounded-lg py-2 text-sm font-semibold capitalize transition-all duration-200 ${
                    tab === t
                      ? 'bg-[var(--color-surface)] text-[var(--color-text)] shadow-sm'
                      : 'text-[var(--color-muted-raw)] hover:text-[var(--color-text2)]'
                  }`}
                >
                  {t === 'login' ? 'Sign In' : 'Register'}
                </button>
              ))}
            </div>

            {/* Form */}
            <AnimatePresence mode="wait">
              {tab === 'login' ? (
                <motion.div
                  key="login"
                  initial={{ opacity: 0, x: -16 }}
                  animate={{ opacity: 1, x: 0 }}
                  exit={{ opacity: 0, x: 16 }}
                  transition={{ duration: 0.18 }}
                >
                  <LoginForm onSuccess={() => navigate('/')} initialUsername={prefillUsername} />
                </motion.div>
              ) : (
                <motion.div
                  key="register"
                  initial={{ opacity: 0, x: 16 }}
                  animate={{ opacity: 1, x: 0 }}
                  exit={{ opacity: 0, x: -16 }}
                  transition={{ duration: 0.18 }}
                >
                  <RegisterForm
                    onSuccess={(username) => {
                      setPrefillUsername(username)
                      setTab('login')
                    }}
                  />
                </motion.div>
              )}
            </AnimatePresence>

            {/* Switch tab hint */}
            <p className="mt-5 text-center text-sm text-[var(--color-muted-raw)]">
              {tab === 'login' ? (
                <>
                  No account?{' '}
                  <button onClick={() => setTab('register')} className="font-semibold text-[var(--brand-red)] hover:underline">
                    Register
                  </button>
                </>
              ) : (
                <>
                  Already have an account?{' '}
                  <button onClick={() => setTab('login')} className="font-semibold text-[var(--brand-red)] hover:underline">
                    Sign in
                  </button>
                </>
              )}
            </p>
          </div>
        </div>
      </div>
    </div>
  )
}

/* ── Login Form ──────────────────────────────────────────────────── */
function LoginForm({ onSuccess, initialUsername = '' }: { onSuccess: () => void; initialUsername?: string }) {
  const [username, setUsername] = useState(initialUsername)
  const [password, setPassword] = useState('')
  const [showPw, setShowPw] = useState(false)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const setAuth = useAuthStore((s) => s.setAuth)

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError('')

    if (!username.trim() || !password) {
      setError('Please fill in all fields.')
      return
    }

    setLoading(true)
    try {
      const res = await authApi.login({ username: username.trim(), password })
      const { token, user } = res.data.data
      setAuth(token, user.id, user.username)
      notify.success(`Welcome back, ${user.username}!`)
      onSuccess()
    } catch (err: unknown) {
      const msg = extractError(err)
      setError(msg || 'Invalid username or password.')
    } finally {
      setLoading(false)
    }
  }

  return (
    <form onSubmit={handleSubmit} className="flex flex-col gap-4">
      <div>
        <h2 className="text-xl font-bold text-[var(--color-text)]">Welcome back</h2>
        <p className="mt-0.5 text-sm text-[var(--color-muted-raw)]">Sign in to your Manga Hub account</p>
      </div>

      {error && <ErrorBanner message={error} />}

      <Field label="Username">
        <input
          type="text"
          value={username}
          onChange={(e) => setUsername(e.target.value)}
          placeholder="Your username"
          autoComplete="username"
          className={inputCls}
        />
      </Field>

      <Field label="Password">
        <PasswordInput value={password} onChange={setPassword} show={showPw} onToggle={() => setShowPw((v) => !v)} />
      </Field>

      <button type="submit" disabled={loading} data-testid="submit-login" className={submitCls}>
        {loading ? <Loader2 className="h-4 w-4 animate-spin" /> : 'Sign In'}
      </button>
    </form>
  )
}

/* ── Register Form ───────────────────────────────────────────────── */
function RegisterForm({ onSuccess }: { onSuccess: (username: string) => void }) {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [showPw, setShowPw] = useState(false)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError('')

    if (!username.trim() || !password || !confirm) {
      setError('Please fill in all fields.')
      return
    }
    if (username.trim().length < 3) {
      setError('Username must be at least 3 characters.')
      return
    }
    if (password.length < 6) {
      setError('Password must be at least 6 characters.')
      return
    }
    if (password !== confirm) {
      setError('Passwords do not match.')
      return
    }

    setLoading(true)
    try {
      const name = username.trim()
      await authApi.register({ username: name, password })
      // Do NOT auto-login — send the user to the sign-in tab to log in.
      notify.success('Account created — please sign in.')
      onSuccess(name)
    } catch (err: unknown) {
      setError(extractError(err) || 'Registration failed. Please try again.')
    } finally {
      setLoading(false)
    }
  }

  return (
    <form onSubmit={handleSubmit} className="flex flex-col gap-4">
      <div>
        <h2 className="text-xl font-bold text-[var(--color-text)]">Create account</h2>
        <p className="mt-0.5 text-sm text-[var(--color-muted-raw)]">Join Manga Hub and track your reading</p>
      </div>

      {error && <ErrorBanner message={error} />}

      <Field label="Username">
        <input
          type="text"
          value={username}
          onChange={(e) => setUsername(e.target.value)}
          placeholder="Your username"
          autoComplete="username"
          className={inputCls}
        />
      </Field>

      <Field label="Password">
        <PasswordInput value={password} onChange={setPassword} show={showPw} onToggle={() => setShowPw((v) => !v)} />
      </Field>

      <Field label="Confirm Password">
        <PasswordInput
          value={confirm}
          onChange={setConfirm}
          show={showPw}
          onToggle={() => setShowPw((v) => !v)}
          placeholder="Repeat your password"
        />
      </Field>

      <button type="submit" disabled={loading} data-testid="submit-register" className={submitCls}>
        {loading ? <Loader2 className="h-4 w-4 animate-spin" /> : 'Create Account'}
      </button>
    </form>
  )
}

/* ── Shared primitives ───────────────────────────────────────────── */
function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex flex-col gap-1.5">
      <label className="text-sm font-medium text-[var(--color-text2)]">{label}</label>
      {children}
    </div>
  )
}

function PasswordInput({
  value,
  onChange,
  show,
  onToggle,
  placeholder = 'Your password',
}: {
  value: string
  onChange: (v: string) => void
  show: boolean
  onToggle: () => void
  placeholder?: string
}) {
  return (
    <div className="relative">
      <input
        type={show ? 'text' : 'password'}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        autoComplete="current-password"
        className={`${inputCls} pr-10`}
      />
      <button
        type="button"
        onClick={onToggle}
        className="absolute right-3 top-1/2 -translate-y-1/2 text-[var(--color-muted-raw)] hover:text-[var(--color-text2)]"
        tabIndex={-1}
      >
        {show ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
      </button>
    </div>
  )
}

function ErrorBanner({ message }: { message: string }) {
  return (
    <div className="rounded-lg border border-[var(--color-error)]/30 bg-[var(--color-error)]/10 px-3 py-2 text-sm text-[var(--color-error)]">
      {message}
    </div>
  )
}

function extractError(err: unknown): string {
  if (err && typeof err === 'object') {
    const e = err as {
      response?: { data?: { error?: string; message?: string }; status?: number }
      code?: string
    }
    // The server responded — surface its real error/message
    const fromBody = e.response?.data?.error ?? e.response?.data?.message
    if (fromBody) return fromBody
    if (e.response?.status) return `Server error (${e.response.status})`
    // No response at all → network-level failure (server unreachable / timeout)
    if (e.code === 'ERR_NETWORK' || e.code === 'ECONNABORTED') {
      return 'Cannot reach the server. Make sure the API is running.'
    }
  }
  return ''
}

const inputCls =
  'w-full rounded-lg border border-[var(--color-border-raw)] bg-[var(--color-surface2)] px-3 py-2.5 text-sm text-[var(--color-text)] placeholder:text-[var(--color-muted-raw)] outline-none transition focus:border-[var(--brand-red)] focus:ring-2 focus:ring-[var(--brand-red)]/20'

const submitCls =
  'flex h-10 w-full items-center justify-center gap-2 rounded-lg bg-[var(--brand-red)] text-sm font-semibold text-white transition hover:bg-[var(--brand-red-hover)] disabled:opacity-60 disabled:cursor-not-allowed mt-1'
