import { useState } from 'react'
import { Users, Eye, Filter, RefreshCw, Shield, Search } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { Avatar } from '@/components/ui/Avatar'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/Table'
import { Input } from '@/components/ui/Input'
import { Select } from '@/components/ui/Select'
import { AdminLoadingState, AdminErrorState, AdminEmptyState, AdminPagination, PageHeader } from '@/components/common'
import { UserDetailModal } from '@/components/users/UserDetailModal'
import { useUsers } from '@/hooks/useUsers'
import { formatDate } from '@/lib/utils'
import type {
  UserListItem,
  AccountStatus,
} from '@/types'
import {
  accountStatusLabels,
  accountStatusVariants,
} from '@/types'

const USER_STATUSES: { value: AccountStatus | ''; label: string }[] = [
  { value: '', label: 'All Statuses' },
  { value: 'active', label: 'Active' },
  { value: 'suspended', label: 'Suspended' },
  { value: 'banned', label: 'Banned' },
]

// Canonical role filter. users.role is exactly user|admin — "buyer" and
// "seller" are not roles (seller authority is a seller_profiles + active
// subscription concern), and the backend rejects them.
const USER_ROLES: { value: 'user' | 'admin' | ''; label: string }[] = [
  { value: '', label: 'All Roles' },
  { value: 'user', label: 'User' },
  { value: 'admin', label: 'Admin' },
]

const VERIFICATION_OPTIONS: { value: 'true' | 'false' | ''; label: string }[] = [
  { value: '', label: 'All Users' },
  { value: 'true', label: 'Verified Only' },
  { value: 'false', label: 'Unverified Only' },
]

export function UsersPage() {
  const [statusFilter, setStatusFilter] = useState<AccountStatus | ''>('')
  const [roleFilter, setRoleFilter] = useState<'user' | 'admin' | ''>('')
  const [verifiedFilter, setVerifiedFilter] = useState<'true' | 'false' | ''>('')
  const [searchQuery, setSearchQuery] = useState('')
  const [selectedUser, setSelectedUser] = useState<UserListItem | null>(null)
  const [isDetailModalOpen, setIsDetailModalOpen] = useState(false)

  const { users, loading, error, total, page, setPage, totalPages, refetch } = useUsers(
    statusFilter || roleFilter || verifiedFilter || searchQuery
      ? {
          status: statusFilter || undefined,
          role: roleFilter || undefined,
          is_verified: verifiedFilter || undefined,
          search: searchQuery || undefined,
        }
      : {}
  )

  const handleViewDetail = (user: UserListItem) => {
    setSelectedUser(user)
    setIsDetailModalOpen(true)
  }

  const handleCloseModal = () => {
    setIsDetailModalOpen(false)
    setSelectedUser(null)
  }

  const handleSuccess = () => {
    refetch()
  }

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault()
    setPage(1)
    refetch()
  }

  const hasActiveFilters = statusFilter || roleFilter || verifiedFilter || searchQuery

  const handleClearFilters = () => {
    setStatusFilter('')
    setRoleFilter('')
    setVerifiedFilter('')
    setSearchQuery('')
    setPage(1)
  }

  if (loading && users.length === 0) {
    return <AdminLoadingState />
  }

  if (error) {
    return (
      <div className="space-y-6">
        <PageHeader title="Users" description="Manage user accounts and permissions" />
        <AdminErrorState title="Failed to load users" message={error.message} onRetry={refetch} />
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <PageHeader
        title="Users"
        description="Manage user accounts and permissions"
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
              <p className="text-sm font-medium text-muted-foreground">Total Users</p>
              <p className="type-metric-lg text-primary mt-1">{total}</p>
            </div>
            <div className="p-4 rounded-lg bg-info-bg">
              <Users className="h-8 w-8 text-info" />
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Filters */}
      <Card>
        <CardContent className="pt-6">
          <div className="flex items-end gap-6 flex-wrap">
            <Filter className="h-5 w-5 text-muted-foreground mb-2" />

            {/* Status Filter */}
            <Select
              label="Status:"
              value={statusFilter}
              onChange={(e) => { setStatusFilter(e.target.value as AccountStatus | ''); setPage(1) }}
            >
              {USER_STATUSES.map((status) => (
                <option key={status.value} value={status.value}>
                  {status.label}
                </option>
              ))}
            </Select>

            {/* Role Filter */}
            <Select
              label="Role:"
              value={roleFilter}
              onChange={(e) => { setRoleFilter(e.target.value as 'user' | 'admin' | ''); setPage(1) }}
            >
              {USER_ROLES.map((role) => (
                <option key={role.value} value={role.value}>
                  {role.label}
                </option>
              ))}
            </Select>

            {/* Verification Filter */}
            <Select
              label="KYC:"
              value={verifiedFilter}
              onChange={(e) => { setVerifiedFilter(e.target.value as 'true' | 'false' | ''); setPage(1) }}
            >
              {VERIFICATION_OPTIONS.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
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
                  onChange={(e) => { setSearchQuery(e.target.value); setPage(1) }}
                  placeholder="Search by name, email, or username..."
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

      {/* Users Table */}
      <Card>
        <CardHeader>
          <CardTitle>User Accounts</CardTitle>
        </CardHeader>
        <CardContent>
          {users.length === 0 ? (
            <AdminEmptyState
              icon={Users}
              title="No Users Found"
              description={
                hasActiveFilters
                  ? 'No users match the current filters.'
                  : 'No users in the system.'
              }
              filtered={Boolean(hasActiveFilters)}
              onClearFilters={handleClearFilters}
            />
          ) : (
            <div className="border border-border rounded-lg overflow-hidden">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>User</TableHead>
                    <TableHead>Email</TableHead>
                    <TableHead>Roles</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>Warnings</TableHead>
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
                          <Avatar
                            src={user.photo_url}
                            userId={user.id}
                            name={user.username}
                            size="sm"
                          />
                          <div>
                            <p className="font-medium text-sm">@{user.username}</p>
                          </div>
                        </div>
                      </TableCell>
                      <TableCell>
                        <p className="text-sm font-mono text-muted-foreground truncate max-w-[150px]">{user.email}</p>
                      </TableCell>
                      <TableCell>
                        <div className="flex items-center gap-1 flex-wrap">
                          {user.is_admin && (
                            <Badge variant="info" className="text-xs">Admin</Badge>
                          )}
                          {user.is_seller && (
                            <Badge variant="default" className="text-xs">Seller</Badge>
                          )}
                          {user.is_buyer && (
                            <Badge variant="default" className="text-xs">Buyer</Badge>
                          )}
                        </div>
                      </TableCell>
                      <TableCell>
                        <Badge variant={accountStatusVariants[user.account_status] || 'info'}>
                          {accountStatusLabels[user.account_status] || user.account_status}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        {user.warning_count !== undefined && user.warning_count > 0 ? (
                          <Badge variant="warning" className="gap-1">
                            <Shield className="h-3 w-3" />
                            {user.warning_count}
                          </Badge>
                        ) : (
                          <span className="type-secondary">-</span>
                        )}
                      </TableCell>
                      <TableCell className="type-secondary">
                        {formatDate(user.created_at)}
                      </TableCell>
                      <TableCell className="type-secondary">
                        {user.last_active_at ? formatDate(user.last_active_at) : 'Never'}
                      </TableCell>
                      <TableCell className="text-right">
                        <Button
                          size="sm"
                          onClick={() => handleViewDetail(user)}
                        >
                          <Eye className="h-4 w-4 mr-1" />
                          View
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Pagination */}
      {totalPages > 1 && (
        <AdminPagination
          page={page}
          totalPages={totalPages}
          onPageChange={setPage}
          disabled={loading}
        />
      )}

      {/* User Detail Modal */}
      <UserDetailModal
        isOpen={isDetailModalOpen}
        onClose={handleCloseModal}
        userData={selectedUser}
        onSuccess={handleSuccess}
      />
    </div>
  )
}
