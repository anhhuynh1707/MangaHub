import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Eye, EyeOff, Loader2, KeyRound, CheckCircle2 } from 'lucide-react'
import { authApi } from '@/api/auth'

export default function ChangePasswordPage() {
  const [oldPassword, setOldPassword]   = useState('')
  const [newPassword, setNewPassword]   = useState('')
  const [confirm, setConfirm]           = useState('')
  const [showOld, setShowOld]           = useState(false)
  const [showNew, setShowNew]           = useState(false)
  const [error, setError]               = useState('')
  const [success, setSuccess]           = useState(false)
  const [loading, setLoading]           = useState(false)
  const navigate = useNavigate()

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError('')
    setSuccess(false)

    if (!oldPassword || !newPassword || !confirm) {
      setError('Please fill in all fields.')
      return
    }
    if (newPassword.length < 6) {
      setError('New password must be at least 6 characters.')
      return
    }
    if (newPassword === oldPassword) {
      setError('New password must be different from current password.')
      return
    }
    if (newPassword !== confirm) {
      setError('New passwords do not match.')
      return
    }

    setLoading(true)
    try {
      await authApi.changePassword({ old_password: oldPassword, new_password: newPassword })
      setSuccess(true)
      setOldPassword('')
      setNewPassword('')
      setConfirm('')
    } catch (err: unknown) {
      const msg = extractError(err)
      setError(msg || 'Failed to change password. Check your current password and try again.')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="mx-auto max-w-md pt-4">
      {/* Page header */}
      <div className="mb-6 flex items-center gap-3">
        <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-[var(--brand-red)]/10">
          <KeyRound className="h-5 w-5 text-[var(--brand-red)]" />
        </div>
        <div>
          <h1 className="text-xl font-bold text-[var(--color-text)]">Change Password</h1>
          <p className="text-sm text-[var(--color-muted-raw)]">Update your account password</p>
        </div>
      </div>

      {/* Card */}
      <div className="rounded-2xl border border-[var(--color-border-raw)] bg-[var(--color-surface)] p-6 shadow-sm">

        {/* Success state */}
        {success ? (
          <div className="flex flex-col items-center gap-4 py-4 text-center">
            <CheckCircle2 className="h-12 w-12 text-[var(--color-success)]" />
            <div>
              <p className="font-semibold text-[var(--color-text)]">Password updated!</p>
              <p className="mt-1 text-sm text-[var(--color-muted-raw)]">Your password has been changed successfully.</p>
            </div>
            <button onClick={() => navigate(-1)} className={secondaryCls}>
              Go back
            </button>
          </div>
        ) : (
          <form onSubmit={handleSubmit} className="flex flex-col gap-4">
            {error && <ErrorBanner message={error} />}

            {/* Security note */}
            <div className="rounded-lg bg-[var(--color-surface2)] px-3 py-2.5 text-xs text-[var(--color-muted-raw)]">
              You must enter your <strong className="text-[var(--color-text2)]">current password</strong> to confirm this is your account.
            </div>

            <Field label="Current Password">
              <PasswordInput
                value={oldPassword}
                onChange={setOldPassword}
                show={showOld}
                onToggle={() => setShowOld((v) => !v)}
                placeholder="your current password"
                autoComplete="current-password"
              />
            </Field>

            <div className="my-1 border-t border-[var(--color-border-raw)]" />

            <Field label="New Password">
              <PasswordInput
                value={newPassword}
                onChange={setNewPassword}
                show={showNew}
                onToggle={() => setShowNew((v) => !v)}
                placeholder="at least 6 characters"
                autoComplete="new-password"
              />
            </Field>

            <Field label="Confirm New Password">
              <PasswordInput
                value={confirm}
                onChange={setConfirm}
                show={showNew}
                onToggle={() => setShowNew((v) => !v)}
                placeholder="repeat new password"
                autoComplete="new-password"
              />
              {/* Live match indicator */}
              {confirm.length > 0 && (
                <p className={`text-xs mt-1 ${newPassword === confirm ? 'text-[var(--color-success)]' : 'text-[var(--color-error)]'}`}>
                  {newPassword === confirm ? '✓ Passwords match' : '✗ Passwords do not match'}
                </p>
              )}
            </Field>

            <div className="flex gap-3 pt-1">
              <button
                type="button"
                onClick={() => navigate(-1)}
                className={secondaryCls}
              >
                Cancel
              </button>
              <button type="submit" disabled={loading} className={submitCls}>
                {loading ? <Loader2 className="h-4 w-4 animate-spin" /> : 'Update Password'}
              </button>
            </div>
          </form>
        )}
      </div>
    </div>
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
  placeholder,
  autoComplete,
}: {
  value: string
  onChange: (v: string) => void
  show: boolean
  onToggle: () => void
  placeholder?: string
  autoComplete?: string
}) {
  return (
    <div className="relative">
      <input
        type={show ? 'text' : 'password'}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        autoComplete={autoComplete}
        className={inputCls + ' pr-10'}
      />
      <button
        type="button"
        onClick={onToggle}
        tabIndex={-1}
        className="absolute right-3 top-1/2 -translate-y-1/2 text-[var(--color-muted-raw)] hover:text-[var(--color-text2)]"
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
  if (err && typeof err === 'object' && 'response' in err) {
    const r = (err as { response?: { data?: { error?: string; message?: string } } }).response
    return r?.data?.error ?? r?.data?.message ?? ''
  }
  return ''
}

const inputCls =
  'w-full rounded-lg border border-[var(--color-border-raw)] bg-[var(--color-surface2)] px-3 py-2.5 text-sm text-[var(--color-text)] placeholder:text-[var(--color-muted-raw)] outline-none transition focus:border-[var(--brand-red)] focus:ring-2 focus:ring-[var(--brand-red)]/20'

const submitCls =
  'flex h-10 flex-1 items-center justify-center gap-2 rounded-lg bg-[var(--brand-red)] text-sm font-semibold text-white transition hover:bg-[var(--brand-red-hover)] disabled:opacity-60 disabled:cursor-not-allowed'

const secondaryCls =
  'flex h-10 flex-1 items-center justify-center gap-2 rounded-lg border border-[var(--color-border-raw)] bg-[var(--color-surface2)] text-sm font-medium text-[var(--color-text2)] transition hover:bg-[var(--color-card-hover)]'
