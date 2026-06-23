import { Component, type ErrorInfo, type ReactNode } from 'react'
import { AlertTriangle, RotateCw } from 'lucide-react'

interface Props {
  children: ReactNode
  /** Optional label for the area that failed (shown in the fallback). */
  label?: string
}

interface State {
  error: Error | null
}

// ErrorBoundary catches render/runtime errors in its subtree and shows a
// recoverable fallback instead of a blank white screen. React error boundaries
// must be class components — there is no hook equivalent.
export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null }

  static getDerivedStateFromError(error: Error): State {
    return { error }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    // Surface it for debugging; a real app would send this to an error tracker.
    console.error('ErrorBoundary caught an error:', error, info.componentStack)
  }

  handleReset = () => this.setState({ error: null })

  render() {
    if (this.state.error) {
      return (
        <div className="flex min-h-[60vh] flex-col items-center justify-center gap-4 p-8 text-center">
          <div className="flex h-14 w-14 items-center justify-center rounded-full bg-[var(--color-error)]/10">
            <AlertTriangle className="h-7 w-7 text-[var(--color-error)]" />
          </div>
          <div>
            <h2 className="text-lg font-bold text-[var(--color-text)]">
              {this.props.label ? `${this.props.label} failed to load` : 'Something went wrong'}
            </h2>
            <p className="mt-1 max-w-md text-sm text-[var(--color-muted-raw)]">
              An unexpected error occurred while rendering this page. You can try again, or reload.
            </p>
          </div>
          <div className="flex gap-2">
            <button
              onClick={this.handleReset}
              className="inline-flex items-center gap-2 rounded-lg bg-[var(--brand-red)] px-4 py-2 text-sm font-semibold text-white transition-colors hover:bg-[var(--brand-red-hover)]"
            >
              <RotateCw className="h-4 w-4" />
              Try again
            </button>
            <button
              onClick={() => window.location.reload()}
              className="rounded-lg border border-[var(--color-border-raw)] px-4 py-2 text-sm font-medium text-[var(--color-text2)] transition-colors hover:bg-[var(--color-surface2)]"
            >
              Reload page
            </button>
          </div>
        </div>
      )
    }

    return this.props.children
  }
}
