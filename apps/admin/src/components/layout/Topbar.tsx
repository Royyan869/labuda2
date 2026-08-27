import { useState } from 'react'
import { LogOut, User, ChevronDown } from 'lucide-react'
import { useAuthStore } from '@/store/authStore'
import { logoutAdmin } from '@/lib/api'

export function Topbar() {
  const { user } = useAuthStore()
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
    <header className="fixed left-64 right-0 top-0 z-30 h-16 border-b border-gray-200 bg-white">
      <div className="flex h-full items-center justify-between px-6">
        {/* Search or Breadcrumbs */}
        <div className="flex items-center gap-2">
          <h2 className="text-lg font-semibold text-gray-900">Admin Dashboard</h2>
        </div>

        {/* User Menu */}
        <div className="relative">
          <button
            onClick={() => setIsDropdownOpen(!isDropdownOpen)}
            className="flex items-center gap-3 rounded-lg px-3 py-2 hover:bg-gray-100 transition-colors"
          >
            {/* Avatar */}
            <div className="flex h-8 w-8 items-center justify-center rounded-full bg-primary text-white text-sm font-medium">
              {(user?.username || user?.email)?.charAt(0).toUpperCase() || 'A'}
            </div>

            {/* User Info */}
            <div className="text-left">
              <p className="text-sm font-medium text-gray-900">
                {user?.username ? `@${user.username}` : user?.id ? `@${user.id.slice(0, 8)}` : 'Admin'}
              </p>
              <p className="text-xs text-gray-500">
                {user?.email}
              </p>
            </div>

            <ChevronDown className="h-4 w-4 text-gray-500" />
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
              <div className="absolute right-0 top-full mt-2 w-56 rounded-lg border border-gray-200 bg-white shadow-lg z-50">
                <div className="p-3 border-b border-gray-100">
                  <p className="text-sm font-medium text-gray-900">
                    {user?.username ? `@${user.username}` : user?.id ? `@${user.id.slice(0, 8)}` : 'Admin'}
                  </p>
                  <p className="text-xs text-gray-500">{user?.email}</p>
                </div>

                <div className="p-1">
                  <button
                    onClick={() => {
                      setIsDropdownOpen(false)
                      // Navigate to profile
                    }}
                    className="flex w-full items-center gap-3 rounded-lg px-3 py-2 text-sm text-gray-700 hover:bg-gray-100"
                  >
                    <User className="h-4 w-4" />
                    Profile
                  </button>

                  <button
                    onClick={() => {
                      setIsDropdownOpen(false)
                      handleSignOut()
                    }}
                    className="flex w-full items-center gap-3 rounded-lg px-3 py-2 text-sm text-red-600 hover:bg-red-50"
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
