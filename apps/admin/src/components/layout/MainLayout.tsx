import { useEffect, useRef, useState } from 'react'
import { Outlet, Navigate, useLocation } from 'react-router-dom'
import { Sidebar } from './Sidebar'
import { Topbar } from './Topbar'
import { RouteErrorBoundary } from './RouteErrorBoundary'
import { useAuth } from '@/hooks/useAuth'
import { useIsDesktop } from '@/hooks/useMediaQuery'

/**
 * The one Admin application shell.
 *
 * Layout contract:
 * - The document is the single scroll owner. The sidebar is fixed; the top
 *   bar is sticky. There is no inner scroll container competing with `body`
 *   (which would break the shared Modal's body scroll lock).
 * - The sidebar is docked on desktop (`lg`) and becomes the same element
 *   sliding in as an overlay drawer below `lg` — no second navigation surface.
 * - Page titles live in the pages; the top bar carries only shell chrome.
 */
export function MainLayout() {
  const { isLoading, isAuthenticated, isAdmin } = useAuth()
  const location = useLocation()
  const isDesktop = useIsDesktop()

  // The ONE owner of mobile navigation visibility. Desktop docking is CSS.
  const [isNavOpen, setIsNavOpen] = useState(false)
  const menuButtonRef = useRef<HTMLButtonElement>(null)
  const sidebarRef = useRef<HTMLElement>(null)
  const wasOpenRef = useRef(false)

  // Reset the drawer when the route changes (including programmatic
  // navigation and browser back/forward). This is React's documented
  // "adjust state during render" pattern, not an effect.
  const [prevPathname, setPrevPathname] = useState(location.pathname)
  if (prevPathname !== location.pathname) {
    setPrevPathname(location.pathname)
    setIsNavOpen(false)
  }

  // Growing to desktop docks the sidebar; dismiss the drawer so a later
  // shrink cannot resurface it unexpectedly.
  const [prevIsDesktop, setPrevIsDesktop] = useState(isDesktop)
  if (prevIsDesktop !== isDesktop) {
    setPrevIsDesktop(isDesktop)
    setIsNavOpen(false)
  }

  // Escape dismisses the mobile drawer.
  useEffect(() => {
    if (!isNavOpen) return
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setIsNavOpen(false)
    }
    document.addEventListener('keydown', onKeyDown)
    return () => document.removeEventListener('keydown', onKeyDown)
  }, [isNavOpen])

  // Move focus into the drawer when it opens and back to the trigger when it
  // closes, so keyboard users are never stranded behind the scrim.
  useEffect(() => {
    if (isDesktop) return
    if (isNavOpen) {
      sidebarRef.current?.querySelector<HTMLElement>('a, button')?.focus()
    } else if (wasOpenRef.current) {
      menuButtonRef.current?.focus()
    }
    wasOpenRef.current = isNavOpen
  }, [isNavOpen, isDesktop])

  // Lock document scroll while the mobile drawer is open, so the page behind
  // the scrim cannot move. The document is the shell's scroll owner.
  useEffect(() => {
    if (isDesktop || !isNavOpen) return
    const previous = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => {
      document.body.style.overflow = previous
    }
  }, [isNavOpen, isDesktop])

  // Show loading state
  if (isLoading) {
    return (
      <div className="flex h-screen items-center justify-center bg-background">
        <div className="text-center">
          <div className="inline-block h-8 w-8 animate-spin rounded-full border-4 border-solid border-primary border-r-transparent"></div>
          <p className="mt-4 text-muted-foreground">Loading...</p>
        </div>
      </div>
    )
  }

  // Redirect to login if not authenticated
  if (!isAuthenticated) {
    return <Navigate to="/login" replace />
  }

  // Show access denied if not admin
  if (!isAdmin) {
    return (
      <div className="flex h-screen items-center justify-center bg-background">
        <div className="text-center max-w-md">
          <div className="mx-auto mb-4 h-16 w-16 rounded-full bg-destructive-bg flex items-center justify-center">
            <svg
              className="h-8 w-8 text-destructive"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
              />
            </svg>
          </div>
          <h2 className="text-2xl font-bold text-foreground mb-2">Access Denied</h2>
          <p className="text-muted-foreground mb-6">
            You don't have admin privileges to access this dashboard.
          </p>
          <button
            onClick={() => window.location.href = '/login'}
            className="px-4 py-2 bg-primary text-white rounded-lg hover:bg-primary-hover"
          >
            Back to Login
          </button>
        </div>
      </div>
    )
  }

  return (
    <div className="flex min-h-screen bg-background">
      {/* Navigation — docked on desktop, overlay drawer below lg. */}
      <Sidebar
        ref={sidebarRef}
        isOpen={isNavOpen}
        isDesktop={isDesktop}
        onNavigate={() => setIsNavOpen(false)}
      />

      {/* Content column — offset by the docked sidebar. While the mobile
          drawer is open the column is inert, which contains focus inside the
          drawer without a hand-rolled focus trap. */}
      <div
        inert={!isDesktop && isNavOpen}
        className="flex min-w-0 flex-1 flex-col lg:pl-64"
      >
        <Topbar
          onMenuClick={() => setIsNavOpen((open) => !open)}
          isNavOpen={isNavOpen}
          menuButtonRef={menuButtonRef}
        />

        <main id="admin-main" className="flex-1 p-4 sm:p-6">
          <RouteErrorBoundary key={location.pathname}>
            <Outlet />
          </RouteErrorBoundary>
        </main>
      </div>
    </div>
  )
}
