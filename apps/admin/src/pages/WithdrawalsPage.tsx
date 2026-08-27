import { useState } from 'react'
import { ChevronLeft, ChevronRight, DollarSign, Eye, Filter, RefreshCw, Wallet } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/Table'
import { WithdrawalDetailModal } from '@/components/finance/WithdrawalDetailModal'
import { useWithdrawals } from '@/hooks/useWithdrawals'
import { formatDate, formatRupiah } from '@/lib/utils'
import type {
  WithdrawalListItem,
  WithdrawalStatus,
} from '@/types'
import {
  withdrawalStatusLabels,
  withdrawalStatusVariants,
} from '@/types'

const WITHDRAWAL_STATUSES: { value: WithdrawalStatus | ''; label: string }[] = [
  { value: '', label: 'All Statuses' },
  { value: 'REQUESTED', label: 'Pending Approval' },
  { value: 'PROCESSING', label: 'Processing' },
  { value: 'SUBMITTED', label: 'Submitted to Gateway' },
  { value: 'SETTLING', label: 'In Transit' },
  { value: 'SETTLED', label: 'Settled by Gateway' },
  { value: 'COMPLETED', label: 'Manually Paid' },
  { value: 'FAILED', label: 'Failed' },
  { value: 'FAILED_RETRYABLE', label: 'Failed (Retrying)' },
  { value: 'FAILED_FINAL', label: 'Failed (Final)' },
  { value: 'PILOT_BLOCKED', label: 'Blocked (Pilot)' },
]

export function WithdrawalsPage() {
  const [statusFilter, setStatusFilter] = useState<WithdrawalStatus | ''>('REQUESTED')
  const [selectedWithdrawal, setSelectedWithdrawal] = useState<WithdrawalListItem | null>(null)
  const [isDetailModalOpen, setIsDetailModalOpen] = useState(false)

  const { withdrawals, loading, error, total, refetch, page, setPage, totalPages } = useWithdrawals(
    statusFilter
      ? { status: statusFilter }
      : {}
  )

  // Reset to page 1 when the status filter changes so a stale page number
  // from a previous filter does not carry over to the new result set.
  const handleStatusChange = (newStatus: WithdrawalStatus | '') => {
    setStatusFilter(newStatus)
    setPage(1)
  }

  const handleViewDetail = (withdrawal: WithdrawalListItem) => {
    setSelectedWithdrawal(withdrawal)
    setIsDetailModalOpen(true)
  }

  const handleCloseModal = () => {
    setIsDetailModalOpen(false)
    setSelectedWithdrawal(null)
  }

  const handleSuccess = () => {
    // Refetch the list after a successful action
    refetch()
  }

  // Calculate pending amount for summary
  const pendingAmount = withdrawals
    .filter(w => w.status === 'REQUESTED')
    .reduce((sum, w) => sum + w.amount, 0)

  const renderSellerIdentity = (withdrawal: WithdrawalListItem) => {
    const username = withdrawal.seller_username?.trim()
    const farmName = withdrawal.seller_farm_name?.trim()

    if (username) {
      return (
        <div className="text-sm">
          <p className="font-medium">@{username}</p>
          {farmName && <p className="text-xs text-gray-500">{farmName}</p>}
        </div>
      )
    }

    return (
      <div className="text-sm">
        <p className="font-medium text-gray-500">{withdrawal.seller_id}</p>
      </div>
    )
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <div className="text-center">
          <div className="inline-block h-8 w-8 animate-spin rounded-full border-4 border-solid border-primary border-r-transparent"></div>
          <p className="mt-4 text-gray-600">Loading withdrawals...</p>
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="space-y-6">
        <div>
          <h1 className="text-3xl font-bold text-gray-900">Withdrawals</h1>
          <p className="text-gray-600 mt-1">Manage seller withdrawal requests</p>
        </div>
        <Card>
          <CardContent className="p-6">
            <div className="text-center text-red-600">
              <p>Error loading withdrawals: {error.message}</p>
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
          <h1 className="text-3xl font-bold text-gray-900">Withdrawals</h1>
          <p className="text-gray-600 mt-1">Manage seller withdrawal requests</p>
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

      {/* Stats Cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        <Card>
          <CardContent className="pt-6">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-medium text-gray-600">Total Requests</p>
                <p className="text-3xl font-bold text-primary mt-1">{total}</p>
              </div>
              <div className="p-4 rounded-lg bg-blue-100">
                <Wallet className="h-8 w-8 text-blue-600" />
              </div>
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="pt-6">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-medium text-gray-600">Pending Amount</p>
                <p className="text-3xl font-bold text-amber-600 mt-1">{formatRupiah(pendingAmount)}</p>
              </div>
              <div className="p-4 rounded-lg bg-amber-100">
                <DollarSign className="h-8 w-8 text-amber-600" />
              </div>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Filters */}
      <Card>
        <CardContent className="pt-6">
          <div className="flex items-center gap-6 flex-wrap">
            <div className="flex items-center gap-4">
              <Filter className="h-5 w-5 text-gray-500" />
              <label htmlFor="status-filter" className="text-sm font-medium text-gray-700">
                Status:
              </label>
              <select
                id="status-filter"
                value={statusFilter}
                onChange={(e) => handleStatusChange(e.target.value as WithdrawalStatus | '')}
                className="px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-primary"
              >
                {WITHDRAWAL_STATUSES.map((status) => (
                  <option key={status.value} value={status.value}>
                    {status.label}
                  </option>
                ))}
              </select>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Withdrawals Table */}
      <Card>
        <CardHeader>
          <CardTitle>Withdrawal Requests</CardTitle>
        </CardHeader>
        <CardContent>
          {withdrawals.length === 0 ? (
            <div className="text-center py-12">
              <Wallet className="h-12 w-12 text-gray-400 mx-auto mb-4" />
              <h3 className="text-lg font-semibold text-gray-900 mb-2">No Withdrawals Found</h3>
              <p className="text-gray-600">
                {statusFilter
                  ? 'No withdrawals match the current filter.'
                  : 'No withdrawal requests in the system.'}
              </p>
            </div>
          ) : (
            <div className="border border-gray-200 rounded-lg overflow-hidden">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>ID</TableHead>
                    <TableHead>Seller</TableHead>
                    <TableHead>Bank</TableHead>
                    <TableHead>Amount</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>Requested At</TableHead>
                    <TableHead className="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {withdrawals.map((withdrawal) => (
                    <TableRow key={withdrawal.id}>
                      <TableCell className="font-mono text-sm">
                        {withdrawal.id.slice(0, 8)}
                      </TableCell>
                      <TableCell>
                        <div className="flex items-center gap-2">
                          {withdrawal.seller_avatar ? (
                            <img
                              src={withdrawal.seller_avatar}
                              alt=""
                              className="w-6 h-6 rounded-full object-cover"
                            />
                          ) : (
                            <div className="w-6 h-6 rounded-full bg-gray-200" />
                          )}
                          <div className="min-w-0 max-w-[180px]">
                            {renderSellerIdentity(withdrawal)}
                          </div>
                        </div>
                      </TableCell>
                      <TableCell>
                        <div className="text-sm">
                          <p className="font-medium">{withdrawal.bank_name_snapshot}</p>
                          <p className="text-gray-500">{withdrawal.account_number_snapshot}</p>
                        </div>
                      </TableCell>
                      <TableCell className="font-semibold">
                        {formatRupiah(withdrawal.amount)}
                      </TableCell>
                      <TableCell>
                        <Badge variant={withdrawalStatusVariants[withdrawal.status] || 'info'}>
                          {withdrawalStatusLabels[withdrawal.status] || withdrawal.status}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-sm text-gray-600">
                        {formatDate(withdrawal.created_at)}
                      </TableCell>
                      <TableCell className="text-right">
                        <Button
                          size="sm"
                          onClick={() => handleViewDetail(withdrawal)}
                        >
                          <Eye className="h-4 w-4 mr-1" />
                          Review
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
        <div className="flex items-center justify-center gap-4">
          <Button
            variant="secondary"
            size="sm"
            onClick={() => setPage(p => p - 1)}
            disabled={page <= 1 || loading}
          >
            <ChevronLeft className="h-4 w-4 mr-1" />
            Previous
          </Button>
          <span className="text-sm text-gray-600 font-medium">
            Page {page} of {totalPages}
          </span>
          <Button
            variant="secondary"
            size="sm"
            onClick={() => setPage(p => p + 1)}
            disabled={page >= totalPages || loading}
          >
            Next
            <ChevronRight className="h-4 w-4 ml-1" />
          </Button>
        </div>
      )}

      {/* Withdrawal Detail Modal */}
      <WithdrawalDetailModal
        isOpen={isDetailModalOpen}
        onClose={handleCloseModal}
        withdrawalData={selectedWithdrawal}
        onSuccess={handleSuccess}
      />
    </div>
  )
}
