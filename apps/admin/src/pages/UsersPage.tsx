import { useState } from 'react'
import { Users, Eye, Filter, RefreshCw, Shield, Search } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/Table'
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

const USER_ROLES: { value: 'buyer' | 'seller' | 'admin' | ''; label: string }[] = [
  { value: '', label: 'All Roles' },
  { value: 'buyer', label: 'Buyer' },
  { value: 'seller', label: 'Seller' },
  { value: 'admin', label: 'Admin' },
]

const VERIFICATION_OPTIONS: { value: 'true' | 'false' | ''; label: string }[] = [
  { value: '', label: 'All Users' },
  { value: 'true', label: 'Verified Only' },
  { value: 'false', label: 'Unverified Only' },
]

export function UsersPage() {
  const [statusFilter, setStatusFilter] = useState<AccountStatus | ''>('')
  const [roleFilter, setRoleFilter] = useState<'buyer' | 'seller' | 'admin' | ''>('')
  const [verifiedFilter, setVerifiedFilter] = useState<'true' | 'false' | ''>('')
  const [searchQuery, setSearchQuery] = useState('')
  const [selectedUser, setSelectedUser] = useState<UserListItem | null>(null)
  const [isDetailModalOpen, setIsDetailModalOpen] = useState(false)

  const { users, loading, error, total, refetch } = useUsers(
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
    refetch()
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <div className="text-center">
          <div className="inline-block h-8 w-8 animate-spin rounded-full border-4 border-solid border-primary border-r-transparent"></div>
          <p className="mt-4 text-gray-600">Loading users...</p>
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="space-y-6">
        <div>
          <h1 className="text-3xl font-bold text-gray-900">Users</h1>
          <p className="text-gray-600 mt-1">Manage user accounts</p>
        </div>
        <Card>
          <CardContent className="p-6">
            <div className="text-center text-red-600">
              <p>Error loading users: {error.message}</p>
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
          <h1 className="text-3xl font-bold text-gray-900">Users</h1>
          <p className="text-gray-600 mt-1">Manage user accounts and permissions</p>
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
              <p className="text-sm font-medium text-gray-600">Total Users</p>
              <p className="text-3xl font-bold text-primary mt-1">{total}</p>
            </div>
            <div className="p-4 rounded-lg bg-blue-100">
              <Users className="h-8 w-8 text-blue-600" />
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
                onChange={(e) => setStatusFilter(e.target.value as AccountStatus | '')}
                className="px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-primary"
              >
                {USER_STATUSES.map((status) => (
                  <option key={status.value} value={status.value}>
                    {status.label}
                  </option>
                ))}
              </select>
            </div>

            {/* Role Filter */}
            <div className="flex items-center gap-2">
              <label htmlFor="role-filter" className="text-sm font-medium text-gray-700">
                Role:
              </label>
              <select
                id="role-filter"
                value={roleFilter}
                onChange={(e) => setRoleFilter(e.target.value as 'buyer' | 'seller' | 'admin' | '')}
                className="px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-primary"
              >
                {USER_ROLES.map((role) => (
                  <option key={role.value} value={role.value}>
                    {role.label}
                  </option>
                ))}
              </select>
            </div>

            {/* Verification Filter */}
            <div className="flex items-center gap-2">
              <label htmlFor="verified-filter" className="text-sm font-medium text-gray-700">
                KYC:
              </label>
              <select
                id="verified-filter"
                value={verifiedFilter}
                onChange={(e) => setVerifiedFilter(e.target.value as 'true' | 'false' | '')}
                className="px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-primary"
              >
                {VERIFICATION_OPTIONS.map((option) => (
                  <option key={option.value} value={option.value}>
                    {option.label}
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
                  placeholder="Search by name, email, or username..."
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

      {/* Users Table */}
      <Card>
        <CardHeader>
          <CardTitle>User Accounts</CardTitle>
        </CardHeader>
        <CardContent>
          {users.length === 0 ? (
            <div className="text-center py-12">
              <Users className="h-12 w-12 text-gray-400 mx-auto mb-4" />
              <h3 className="text-lg font-semibold text-gray-900 mb-2">No Users Found</h3>
              <p className="text-gray-600">
                {statusFilter || roleFilter || verifiedFilter || searchQuery
                  ? 'No users match the current filters.'
                  : 'No users in the system.'}
              </p>
            </div>
          ) : (
            <div className="border border-gray-200 rounded-lg overflow-hidden">
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
                            <p className="font-medium text-sm">@{user.username}</p>
                          </div>
                        </div>
                      </TableCell>
                      <TableCell>
                        <p className="text-sm font-mono text-gray-600 truncate max-w-[150px]">{user.email}</p>
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
                          <span className="text-sm text-gray-400">-</span>
                        )}
                      </TableCell>
                      <TableCell className="text-sm text-gray-600">
                        {formatDate(user.created_at)}
                      </TableCell>
                      <TableCell className="text-sm text-gray-600">
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
