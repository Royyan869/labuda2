import { Component, type ErrorInfo, type ReactNode } from 'react'

interface Props {
  children: ReactNode
}

interface State {
  error: Error | null
}

/**
 * Catches render-time exceptions in the active page (e.g. a page reading a
 * field off a response shape that didn't come back as expected) so the
 * sidebar/topbar stay usable and the admin sees an error message instead of
 * a blank white screen. Scoped around the routed page content only — a
 * crash in one page must not take down the whole shell.
 *
 * Reset on navigation: the caller must pass a `key` derived from the route
 * (e.g. location.pathname) so React remounts this boundary — and clears a
 * tripped error state — whenever the admin navigates to a different page.
 */
export class RouteErrorBoundary extends Component<Props, State> {
  state: State = { error: null }

  static getDerivedStateFromError(error: Error): State {
    return { error }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('Admin page crashed:', error, info.componentStack)
  }

  render() {
    if (this.state.error) {
      return (
        <div className="rounded-lg border border-red-200 bg-red-50 p-8 text-center">
          <h2 className="text-lg font-semibold text-gray-900 mb-2">This page failed to load</h2>
          <p className="text-gray-600 mb-4">{this.state.error.message}</p>
          <button
            onClick={() => window.location.reload()}
            className="px-4 py-2 bg-primary text-white rounded-lg hover:bg-primary-hover"
          >
            Reload
          </button>
        </div>
      )
    }

    return this.props.children
  }
}
