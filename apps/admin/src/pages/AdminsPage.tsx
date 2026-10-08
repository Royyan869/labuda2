import { useState } from 'react'
import { Link } from 'react-router-dom'
import { Users, Eye, Filter, RefreshCw, Shield, Search } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/Table'
import { Input } from '@/components/ui/Input'
import { Select } from '@/components/ui/Select'
import { useUsers } from '@/hooks/useUsers'
import { AdminLoadingState, AdminErrorState, AdminEmptyState, PageHeader } from '@/components/common'
import { PromoteAdminPanel } from '@/components/users/PromoteAdminPanel'
import { hasCapability } from '@/lib/permissions'
import { useAuth } from '@/hooks/useAuth'
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
  const { capabilities } = useAuth()
  const canAssignRoles = hasCapability(capabilities, 'governance.role.assign')

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

  const hasActiveFilters = statusFilter || searchQuery

  const handleClearFilters = () => {
    setStatusFilter('')
    setSearchQuery('')
  }

  if (loading && users.length === 0) {
    return <AdminLoadingState />
  }

  if (error) {
    return (
      <div className="space-y-6">
        <PageHeader title="Admin Management" description="Manage admin accounts and their capabilities" />
        <AdminErrorState title="Failed to load admins" message={error.message} onRetry={refetch} />
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <PageHeader
        title="Admin Management"
        description="Manage admin accounts and their capabilities"
        actions={
          <Button variant="secondary" onClick={refetch} className="gap-2">
            <RefreshCw className="h-4 w-4" />
            Refresh
          </Button>
        }
      />

      {/* Stats Card */}
      <Card>
        <CardContent className="pt-6">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium text-muted-foreground">Total Admins</p>
              <p className="type-metric-lg text-primary mt-1">{total}</p>
            </div>
            <div className="p-4 rounded-lg bg-info-bg">
              <Shield className="h-8 w-8 text-info" />
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Canonical admin recruitment: promote an existing user, then grant
          capabilities on their admin page. Requires governance.role.assign. */}
      {canAssignRoles && <PromoteAdminPanel onPromoted={refetch} />}

      {/* Filters */}
      <Card>
        <CardContent className="pt-6">
          <div className="flex items-end gap-6 flex-wrap">
            <Filter className="h-5 w-5 text-muted-foreground mb-2" />

            {/* Status Filter */}
            <Select
              label="Status:"
              value={statusFilter}
              onChange={(e) => setStatusFilter(e.target.value as 'active' | 'suspended' | 'banned' | '')}
            >
              {ADMIN_STATUSES.map((status) => (
                <option key={status.value} value={status.value}>
                  {status.label}
                </option>
              ))}
            </Select>

            {/* Search */}
            <form onSubmit={handleSearch} className="flex items-end gap-2">
              <div className="relative">
                <Search className="h-4 w-4 absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground z-10" />
                <Input
                  type="text"
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  placeholder="Search by name or email..."
                  className="pl-9 w-64"
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
            <AdminEmptyState
              icon={Shield}
              title="No Admins Found"
              description={
                hasActiveFilters
                  ? 'No admins match the current filters.'
                  : 'No admin accounts in the system.'
              }
              filtered={Boolean(hasActiveFilters)}
              onClearFilters={handleClearFilters}
            />
          ) : (
            <div className="border border-border rounded-lg overflow-hidden">
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
                            <div className="w-8 h-8 rounded-full bg-surface-muted flex items-center justify-center">
                              <Users className="h-4 w-4 text-muted-foreground" />
                            </div>
                          )}
                          <div>
                            <p className="type-caption">@{user.username}</p>
                          </div>
                        </div>
                      </TableCell>
                      <TableCell>
                        <p className="text-sm font-mono text-muted-foreground truncate max-w-[150px]">{user.email}</p>
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
                      <TableCell className="type-secondary">
                        {formatDate(user.created_at)}
                      </TableCell>
                      <TableCell className="type-secondary">
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
