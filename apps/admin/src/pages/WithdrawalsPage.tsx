import { useState } from 'react'
import { DollarSign, Eye, Filter, RefreshCw, Wallet } from 'lucide-react'
import { AdminLoadingState, AdminErrorState, AdminEmptyState, AdminPagination, PageHeader } from '@/components/common'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/Table'
import { Select } from '@/components/ui/Select'
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

  const handleClearFilters = () => {
    setStatusFilter('')
    setPage(1)
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
          {farmName && <p className="type-caption">{farmName}</p>}
        </div>
      )
    }

    return (
      <div className="text-sm">
        <p className="font-medium text-muted-foreground">{withdrawal.seller_id}</p>
      </div>
    )
  }

  if (loading && withdrawals.length === 0) {
    return <AdminLoadingState />
  }

  if (error) {
    return (
      <div className="space-y-6">
        <PageHeader title="Withdrawals" description="Manage seller withdrawal requests" />
        <AdminErrorState title="Failed to load withdrawals" message={error.message} onRetry={refetch} />
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <PageHeader
        title="Withdrawals"
        description="Manage seller withdrawal requests"
        actions={
          <Button variant="secondary" onClick={refetch} className="gap-2">
            <RefreshCw className="h-4 w-4" />
            Refresh
          </Button>
        }
      />

      {/* Stats Cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        <Card>
          <CardContent className="pt-6">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-medium text-muted-foreground">Total Requests</p>
                <p className="type-metric-lg text-primary mt-1">{total}</p>
              </div>
              <div className="p-4 rounded-lg bg-info-bg">
                <Wallet className="h-8 w-8 text-info" />
              </div>
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="pt-6">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-medium text-muted-foreground">Pending Amount</p>
                <p className="type-metric-lg text-warning mt-1">{formatRupiah(pendingAmount)}</p>
              </div>
              <div className="p-4 rounded-lg bg-warning-bg">
                <DollarSign className="h-8 w-8 text-warning" />
              </div>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Filters */}
      <Card>
        <CardContent className="pt-6">
          <div className="flex items-center gap-6 flex-wrap">
            <div className="flex items-end gap-4">
              <Filter className="h-5 w-5 text-muted-foreground mb-2" />
              <Select
                label="Status:"
                value={statusFilter}
                onChange={(e) => handleStatusChange(e.target.value as WithdrawalStatus | '')}
              >
                {WITHDRAWAL_STATUSES.map((status) => (
                  <option key={status.value} value={status.value}>
                    {status.label}
                  </option>
                ))}
              </Select>
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
            <AdminEmptyState
              icon={Wallet}
              title="No Withdrawals Found"
              description={
                statusFilter
                  ? 'No withdrawals match the current filter.'
                  : 'No withdrawal requests in the system.'
              }
              filtered={Boolean(statusFilter)}
              onClearFilters={handleClearFilters}
            />
          ) : (
            <div className="border border-border rounded-lg overflow-hidden">
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
                            <div className="w-6 h-6 rounded-full bg-border" />
                          )}
                          <div className="min-w-0 max-w-[180px]">
                            {renderSellerIdentity(withdrawal)}
                          </div>
                        </div>
                      </TableCell>
                      <TableCell>
                        <div className="text-sm">
                          <p className="font-medium">{withdrawal.bank_name_snapshot}</p>
                          <p className="text-muted-foreground">{withdrawal.account_number_snapshot}</p>
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
                      <TableCell className="type-secondary">
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
        <AdminPagination
          page={page}
          totalPages={totalPages}
          onPageChange={setPage}
          disabled={loading}
        />
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
