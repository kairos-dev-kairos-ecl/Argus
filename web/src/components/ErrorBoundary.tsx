import React, { type ReactNode, useState } from 'react'

interface ErrorBoundaryProps {
  children: ReactNode
  fallback?: (error: Error, reset: () => void) => ReactNode
}

interface ErrorBoundaryState {
  hasError: boolean
  error: Error | null
}

/**
 * ErrorBoundary Component
 * Catches React component errors and displays graceful error UI
 * Allows recovery with "Try Again" button
 */
export class ErrorBoundary extends React.Component<ErrorBoundaryProps, ErrorBoundaryState> {
  constructor(props: ErrorBoundaryProps) {
    super(props)
    this.state = { hasError: false, error: null }
  }

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { hasError: true, error }
  }

  componentDidCatch(error: Error, errorInfo: React.ErrorInfo) {
    console.error('ErrorBoundary caught:', error, errorInfo)
  }

  reset = () => {
    this.setState({ hasError: false, error: null })
  }

  render() {
    if (this.state.hasError && this.state.error) {
      if (this.props.fallback) {
        return this.props.fallback(this.state.error, this.reset)
      }

      return (
        <div className="p-4 bg-status-error/10 border border-status-error/30 rounded-lg">
          <h2 className="text-status-error font-semibold mb-2">Something went wrong</h2>
          <p className="text-status-error/80 text-sm mb-3">{this.state.error.message}</p>
          <button
            onClick={this.reset}
            className="h-9 px-3 bg-status-error hover:bg-status-error/90 text-foreground text-sm rounded transition-colors"
            aria-label="Try again"
          >
            Try Again
          </button>
        </div>
      )
    }

    return this.props.children
  }
}

/**
 * useErrorBoundary Hook
 * For functional components that need error boundary behavior
 */
export function useErrorBoundary() {
  const [error, setError] = useState<Error | null>(null)

  if (error) throw error

  return {
    showError: (err: Error) => setError(err),
    clearError: () => setError(null),
  }
}
