import { useCallback, useState } from 'react'
import { Search, UserPlus, AlertTriangle } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { api, setUserRole } from '@/lib/api'
import type { UserListItem } from '@/types'

interface PromoteAdminPanelProps {
  /** Called after a successful promotion so the admin list can refresh. */
  onPromoted?: () => void
}

/**
 * Canonical admin recruitment surface.
 *
 * This is the normal operational path for growing the admin team: pick an
 * existing user, promote them to admin membership, then grant the capabilities
 * they need. Command-line bootstrap is reserved for initial setup and disaster
 * recovery, NOT for day-to-day recruitment.
 *
 * Promoting grants membership only — never capabilities. A freshly promoted
 * admin has zero capabilities until they are granted explicitly, and is
 * therefore not a full-access admin.
 */
export function PromoteAdminPanel({ onPromoted }: PromoteAdminPanelProps) {
  const [query, setQuery] = useState('')
  const [results, setResults] = useState<UserListItem[]>([])
  const [searching, setSearching] = useState(false)
  const [promotingId, setPromotingId] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)
  const [searched, setSearched] = useState(false)

  const search = useCallback(async () => {
    const term = query.trim()
    setNotice(null)
    if (!term) {
      setError('Enter a name or email to find an existing user.')
      return
    }
    setSearching(true)
    setError(null)
    try {
      const res = await api.get<{ data: { users: UserListItem[] } }>(
        `/api/v1/admin/users?search=${encodeURIComponent(term)}&page=1&page_size=10`
      )
      const users = res.data?.users ?? []
      setResults(users.filter(u => !u.is_admin))
      setSearched(true)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Search failed')
    } finally {
      setSearching(false)
    }
  }, [query])

  const promote = async (target: UserListItem) => {
    const confirmed = window.confirm(
      `Promote ${target.email} to admin?\n\nThis grants admin membership only, not capabilities. Grant capabilities on the admin detail page afterwards.`
    )
    if (!confirmed) return

    setPromotingId(target.id)
    setError(null)
    setNotice(null)
    try {
      await setUserRole(target.id, 'admin')
      setResults(prev => prev.filter(r => r.id !== target.id))
      setNotice(
        `${target.email} is now an admin with no capabilities. Open their admin page to grant capabilities.`
      )
      onPromoted?.()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to promote user')
    } finally {
      setPromotingId(null)
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <UserPlus className="h-5 w-5" />
          Promote an existing user to admin
        </CardTitle>
        <p className="text-sm text-gray-600">
          Normal recruitment path: promote membership here, then grant capabilities. Admin membership alone grants no
          capabilities.
        </p>
      </CardHeader>
      <CardContent className="space-y-4">
        <form
          className="flex items-center gap-2"
          onSubmit={e => {
            e.preventDefault()
            void search()
          }}
        >
          <div className="relative flex-1 max-w-md">
            <Search className="h-4 w-4 absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
            <input
              type="text"
              value={query}
              onChange={e => setQuery(e.target.value)}
              placeholder="Search by name or email..."
              className="pl-9 pr-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-primary w-full"
            />
          </div>
          <Button type="submit" size="sm" variant="secondary" disabled={searching}>
            {searching ? 'Searching...' : 'Search users'}
          </Button>
        </form>

        {error && (
          <div className="bg-red-50 border border-red-200 text-red-700 p-3 rounded-lg flex items-center gap-2">
            <AlertTriangle className="h-4 w-4 flex-shrink-0" />
            <span className="text-sm">{error}</span>
          </div>
        )}

        {notice && (
          <div className="bg-green-50 border border-green-200 text-green-800 p-3 rounded-lg text-sm">{notice}</div>
        )}

        {searched && results.length === 0 && !error && (
          <p className="text-sm text-gray-500">No promotable users match that search.</p>
        )}

        {results.length > 0 && (
          <div className="space-y-2">
            {results.map(u => (
              <div
                key={u.id}
                className="flex items-center justify-between gap-3 p-3 rounded-lg border border-gray-200"
              >
                <div className="min-w-0">
                  <p className="text-sm font-medium text-gray-900 truncate">
                    {u.username ? `@${u.username}` : u.id.slice(0, 8)}
                  </p>
                  <p className="text-xs font-mono text-gray-500 truncate">{u.email}</p>
                </div>
                <div className="flex items-center gap-3 flex-shrink-0">
                  <Badge variant={u.account_status === 'active' ? 'success' : 'warning'}>
                    {u.account_status}
                  </Badge>
                  <Button
                    size="sm"
                    variant="primary"
                    disabled={promotingId === u.id}
                    onClick={() => void promote(u)}
                  >
                    {promotingId === u.id ? 'Promoting...' : 'Promote to Admin'}
                  </Button>
                </div>
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  )
}
