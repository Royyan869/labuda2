import { useEffect, useRef, useState, type RefObject } from 'react'
import { useNavigate } from 'react-router-dom'
import { LogOut, User, ChevronDown, Sun, Moon, Monitor, Menu } from 'lucide-react'
import { useAuthStore } from '@/store/authStore'
import { useTheme, type ThemeMode } from '@/lib/theme'
import { logoutAdmin } from '@/lib/api'
import { Avatar } from '@/components/ui/Avatar'

interface TopbarProps {
  /** Opens the mobile navigation drawer. */
  onMenuClick?: () => void
  /** True while the mobile navigation drawer is open. */
  isNavOpen?: boolean
  /** Ref to the mobile menu trigger, so focus can return when the drawer closes. */
  menuButtonRef?: RefObject<HTMLButtonElement | null>
}

/**
 * Admin top bar.
 *
 * The page title is owned by each page (`type-page-title`), so the shell does
 * not render a second, competing title. This bar carries only shell-level
 * chrome: the mobile navigation trigger and the account menu.
 */
export function Topbar({ onMenuClick, isNavOpen = false, menuButtonRef }: TopbarProps) {
  const { user } = useAuthStore()
  const navigate = useNavigate()
  const { mode, setMode } = useTheme()
  const [isDropdownOpen, setIsDropdownOpen] = useState(false)
  const menuContainerRef = useRef<HTMLDivElement>(null)

  const handleSignOut = async () => {
    try {
      logoutAdmin()
      window.location.href = '/login'
    } catch (error) {
      console.error('Sign out error:', error)
    }
  }

  // Escape closes the account disclosure. Focus stays on the trigger, which
  // is the native source of the disclosure.
  useEffect(() => {
    if (!isDropdownOpen) return
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        setIsDropdownOpen(false)
        menuContainerRef.current?.querySelector<HTMLButtonElement>('button')?.focus()
      }
    }
    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [isDropdownOpen])

  return (
    <header className="sticky top-0 z-30 flex h-16 shrink-0 items-center justify-between border-b border-border bg-surface px-4 sm:px-6">
      {/* Mobile navigation trigger — the only way to reach navigation below lg. */}
      <button
        ref={menuButtonRef}
        type="button"
        onClick={onMenuClick}
        aria-label={isNavOpen ? 'Close navigation menu' : 'Open navigation menu'}
        aria-expanded={isNavOpen}
        aria-controls="admin-sidebar"
        className="inline-flex h-10 w-10 items-center justify-center rounded-lg text-muted-foreground hover:bg-surface-muted hover:text-foreground lg:hidden"
      >
        <Menu className="h-5 w-5" aria-hidden="true" />
      </button>

      {/* User Menu */}
      <div className="relative ml-auto" ref={menuContainerRef}>
        <button
          type="button"
          onClick={() => setIsDropdownOpen(!isDropdownOpen)}
          aria-expanded={isDropdownOpen}
          aria-controls="admin-account-menu"
          aria-label="Open admin account menu"
          className="flex items-center gap-3 rounded-lg px-3 py-2 hover:bg-surface-muted transition-colors"
        >
          {/* Avatar — canonical: photo or person icon. No initials,
              including for the read-only admin account itself. */}
          <Avatar
            src={user?.avatarUrl}
            userId={user?.username || user?.email}
            name={user?.username || user?.email}
            size="sm"
          />

          {/* User Info — canonical username from user_profiles */}
          <div className="hidden text-left sm:block">
            <p className="type-label">
              {user?.username ? `@${user.username}` : 'Admin'}
            </p>
            <p className="type-caption">
              {user?.email}
            </p>
          </div>

          <ChevronDown className="h-4 w-4 text-muted-foreground" />
        </button>

        {/* Dropdown Menu */}
        {isDropdownOpen && (
          <>
            {/* Backdrop */}
            <div
              className="fixed inset-0 z-40"
              onClick={() => setIsDropdownOpen(false)}
            />

            {/* Disclosure panel (not an ARIA menu — it contains a radiogroup). */}
            <div
              id="admin-account-menu"
              className="absolute right-0 top-full mt-2 w-56 rounded-lg border border-border bg-surface shadow-lg z-50"
            >
              <div className="p-3 border-b border-border">
                <p className="type-label">
                  {user?.username ? `@${user.username}` : 'Admin'}
                </p>
                <p className="type-caption">{user?.email}</p>
              </div>

              <div className="p-1">
                <div className="border-b border-border px-3 py-2">
                  <p className="mb-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">Theme</p>
                  <div className="grid grid-cols-3 gap-1" role="radiogroup" aria-label="Theme preference">
                    {([
                      ['system', Monitor, 'System'],
                      ['light', Sun, 'Light'],
                      ['dark', Moon, 'Dark'],
                    ] as const).map(([value, Icon, label]) => (
                      <button
                        key={value}
                        type="button"
                        role="radio"
                        aria-checked={mode === value}
                        aria-label={label}
                        onClick={() => setMode(value as ThemeMode)}
                        className={`flex flex-col items-center gap-1 rounded-md px-1 py-2 text-xs ${mode === value ? 'bg-surface-muted text-foreground' : 'text-muted-foreground hover:bg-surface-muted'}`}
                      >
                        <Icon className="h-4 w-4" />
                        {label}
                      </button>
                    ))}
                  </div>
                </div>

                <button
                  type="button"
                  onClick={() => {
                    setIsDropdownOpen(false)
                    navigate('/profile')
                  }}
                  className="flex w-full items-center gap-3 rounded-lg px-3 py-2 type-body hover:bg-surface-muted"
                >
                  <User className="h-4 w-4" />
                  Profile
                </button>

                <button
                  type="button"
                  onClick={() => {
                    setIsDropdownOpen(false)
                    handleSignOut()
                  }}
                  className="flex w-full items-center gap-3 rounded-lg px-3 py-2 text-sm text-destructive hover:bg-destructive-bg"
                >
                  <LogOut className="h-4 w-4" />
                  Sign Out
                </button>
              </div>
            </div>
          </>
        )}
      </div>
    </header>
  )
}
