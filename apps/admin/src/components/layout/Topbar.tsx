import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { LogOut, User, ChevronDown, Sun, Moon, Monitor } from 'lucide-react'
import { useAuthStore } from '@/store/authStore'
import { useTheme, type ThemeMode } from '@/lib/theme'
import { logoutAdmin } from '@/lib/api'
import { Avatar } from '@/components/ui/Avatar'

export function Topbar() {
  const { user } = useAuthStore()
  const navigate = useNavigate()
  const { mode, setMode } = useTheme()
  const [isDropdownOpen, setIsDropdownOpen] = useState(false)

  const handleSignOut = async () => {
    try {
      logoutAdmin()
      window.location.href = '/login'
    } catch (error) {
      console.error('Sign out error:', error)
    }
  }

  return (
    <header className="fixed left-64 right-0 top-0 z-30 h-16 border-b border-[hsl(var(--border))] bg-[hsl(var(--surface))]">
      <div className="flex h-full items-center justify-between px-6">
        {/* Search or Breadcrumbs */}
        <div className="flex items-center gap-2">
          <h2 className="text-lg font-semibold text-[hsl(var(--foreground))]">Admin Dashboard</h2>
        </div>

        {/* User Menu */}
        <div className="relative">
          <button
            onClick={() => setIsDropdownOpen(!isDropdownOpen)}
            aria-expanded={isDropdownOpen}
            aria-haspopup="menu"
            aria-label="Open admin account menu"
            className="flex items-center gap-3 rounded-lg px-3 py-2 hover:bg-[hsl(var(--surface-muted))] transition-colors"
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
            <div className="text-left">
              <p className="text-sm font-medium text-[hsl(var(--foreground))]">
                {user?.username ? `@${user.username}` : 'Admin'}
              </p>
              <p className="text-xs text-[hsl(var(--muted-foreground))]">
                {user?.email}
              </p>
            </div>

            <ChevronDown className="h-4 w-4 text-[hsl(var(--muted-foreground))]" />
          </button>

          {/* Dropdown Menu */}
          {isDropdownOpen && (
            <>
              {/* Backdrop */}
              <div
                className="fixed inset-0 z-40"
                onClick={() => setIsDropdownOpen(false)}
              />

              {/* Menu */}
              <div
                role="menu"
                className="absolute right-0 top-full mt-2 w-56 rounded-lg border border-[hsl(var(--border))] bg-[hsl(var(--surface))] shadow-lg z-50"
              >
                <div className="p-3 border-b border-[hsl(var(--border))]">
                  <p className="text-sm font-medium text-[hsl(var(--foreground))]">
                    {user?.username ? `@${user.username}` : 'Admin'}
                  </p>
                  <p className="text-xs text-[hsl(var(--muted-foreground))]">{user?.email}</p>
                </div>

                <div className="p-1">
                  <div className="border-b border-[hsl(var(--border))] px-3 py-2">
                    <p className="mb-2 text-xs font-medium uppercase tracking-wide text-[hsl(var(--muted-foreground))]">Theme</p>
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
                          className={`flex flex-col items-center gap-1 rounded-md px-1 py-2 text-xs ${mode === value ? 'bg-muted text-[hsl(var(--foreground))]' : 'text-[hsl(var(--muted-foreground))] hover:bg-muted'}`}
                        >
                          <Icon className="h-4 w-4" />
                          {label}
                        </button>
                      ))}
                    </div>
                  </div>

                  <button
                    onClick={() => {
                      setIsDropdownOpen(false)
                      navigate('/profile')
                    }}
                    role="menuitem"
                    className="flex w-full items-center gap-3 rounded-lg px-3 py-2 text-sm text-[hsl(var(--foreground))] hover:bg-[hsl(var(--surface-muted))]"
                  >
                    <User className="h-4 w-4" />
                    Profile
                  </button>

                  <button
                    onClick={() => {
                      setIsDropdownOpen(false)
                      handleSignOut()
                    }}
                    className="flex w-full items-center gap-3 rounded-lg px-3 py-2 text-sm text-[hsl(var(--destructive))] hover:bg-[hsl(var(--destructive-bg))]"
                  >
                    <LogOut className="h-4 w-4" />
                    Sign Out
                  </button>
                </div>
              </div>
            </>
          )}
        </div>
      </div>
    </header>
  )
}
