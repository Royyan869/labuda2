import { useState } from 'react'
import { Link } from 'react-router-dom'
import { Users, Eye, Filter, RefreshCw, Shield, Search } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/Table'
import { useUsers } from '@/hooks/useUsers'
import { formatDate } from '@/lib/utils'

const ADMIN_STATUSES: { value: 'active' | 'suspended' | 'banned' | ''; label: string }[] = [
  { value: '', label: 'All Statuses' },
  { value: 'active', label: 'Active' },
  { value: 'suspended', label: 'Suspended' },
  { value: 'banned', label: 'Banned' },
]

export function AdminsPage() {
  const [statusFilter, setStatusFilter] = useState<'active' | 'suspended' | 'banned' | ''>('')
  const [searchQuery, setSearchQuery] = useState('')

  const { users, loading, error, total, refetch } = useUsers(
    statusFilter || searchQuery
      ? {
          role: 'admin',
          status: statusFilter || undefined,
          search: searchQuery || undefined,
        }
      : { role: 'admin' }
  )

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault()
    refetch()
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <div className="text-center">
          <div className="inline-block h-8 w-8 animate-spin rounded-full border-4 border-solid border-primary border-r-transparent"></div>
          <p className="mt-4 text-gray-600">Loading admins...</p>
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="space-y-6">
        <div>
          <h1 className="text-3xl font-bold text-gray-900">Admin Management</h1>
          <p className="text-gray-600 mt-1">Manage admin accounts and capabilities</p>
        </div>
        <Card>
          <CardContent className="p-6">
            <div className="text-center text-red-600">
              <p>Error loading admins: {error.message}</p>
            </div>
          </CardContent>
        </Card>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold text-gray-900">Admin Management</h1>
          <p className="text-gray-600 mt-1">Manage admin accounts and their capabilities</p>
        </div>
        <Button
          variant="secondary"
          onClick={refetch}
          className="gap-2"
        >
          <RefreshCw className="h-4 w-4" />
          Refresh
        </Button>
      </div>

      {/* Stats Card */}
      <Card>
        <CardContent className="pt-6">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium text-gray-600">Total Admins</p>
              <p className="text-3xl font-bold text-primary mt-1">{total}</p>
            </div>
            <div className="p-4 rounded-lg bg-blue-100">
              <Shield className="h-8 w-8 text-blue-600" />
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Filters */}
      <Card>
        <CardContent className="pt-6">
          <div className="flex items-center gap-6 flex-wrap">
            <div className="flex items-center gap-4">
              <Filter className="h-5 w-5 text-gray-500" />
            </div>

            {/* Status Filter */}
            <div className="flex items-center gap-2">
              <label htmlFor="status-filter" className="text-sm font-medium text-gray-700">
                Status:
              </label>
              <select
                id="status-filter"
                value={statusFilter}
                onChange={(e) => setStatusFilter(e.target.value as 'active' | 'suspended' | 'banned' | '')}
                className="px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-primary"
              >
                {ADMIN_STATUSES.map((status) => (
                  <option key={status.value} value={status.value}>
                    {status.label}
                  </option>
                ))}
              </select>
            </div>

            {/* Search */}
            <form onSubmit={handleSearch} className="flex items-center gap-2">
              <div className="relative">
                <Search className="h-4 w-4 absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
                <input
                  type="text"
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  placeholder="Search by name or email..."
                  className="pl-9 pr-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-primary w-64"
                />
              </div>
              <Button type="submit" size="sm" variant="secondary">
                Search
              </Button>
            </form>
          </div>
        </CardContent>
      </Card>

      {/* Admins Table */}
      <Card>
        <CardHeader>
          <CardTitle>Admin Accounts</CardTitle>
        </CardHeader>
        <CardContent>
          {users.length === 0 ? (
            <div className="text-center py-12">
              <Shield className="h-12 w-12 text-gray-400 mx-auto mb-4" />
              <h3 className="text-lg font-semibold text-gray-900 mb-2">No Admins Found</h3>
              <p className="text-gray-600">
                {statusFilter || searchQuery
                  ? 'No admins match the current filters.'
                  : 'No admin accounts in the system.'}
              </p>
            </div>
          ) : (
            <div className="border border-gray-200 rounded-lg overflow-hidden">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Admin</TableHead>
                    <TableHead>Email</TableHead>
                    <TableHead>Capabilities</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>Joined</TableHead>
                    <TableHead>Last Active</TableHead>
                    <TableHead className="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {users.map((user) => (
                    <TableRow key={user.id}>
                      <TableCell>
                        <div className="flex items-center gap-2">
                          {user.photo_url ? (
                            <img
                              src={user.photo_url}
                              alt=""
                              className="w-8 h-8 rounded-full object-cover"
                            />
                          ) : (
                            <div className="w-8 h-8 rounded-full bg-gray-200 flex items-center justify-center">
                              <Users className="h-4 w-4 text-gray-500" />
                            </div>
                          )}
                          <div>
                            <p className="text-xs text-gray-500">@{user.username}</p>
                          </div>
                        </div>
                      </TableCell>
                      <TableCell>
                        <p className="text-sm font-mono text-gray-600 truncate max-w-[150px]">{user.email}</p>
                      </TableCell>
                      <TableCell>
                        <Badge variant="info" className="gap-1">
                          <Shield className="h-3 w-3" />
                          {user.total_capabilities ?? '-'}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        <Badge variant={
                          user.account_status === 'active' ? 'success' :
                          user.account_status === 'suspended' ? 'warning' :
                          user.account_status === 'banned' ? 'error' :
                          'info'
                        }>
                          {user.account_status.charAt(0).toUpperCase() + user.account_status.slice(1)}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-sm text-gray-600">
                        {formatDate(user.created_at)}
                      </TableCell>
                      <TableCell className="text-sm text-gray-600">
                        {user.last_active_at ? formatDate(user.last_active_at) : 'Never'}
                      </TableCell>
                      <TableCell className="text-right">
                        <Link to={`/users/admins/${user.id}`}>
                          <Button
                            size="sm"
                            variant="secondary"
                            className="w-full"
                          >
                            <Eye className="h-4 w-4 mr-1" />
                            Manage
                          </Button>
                        </Link>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
