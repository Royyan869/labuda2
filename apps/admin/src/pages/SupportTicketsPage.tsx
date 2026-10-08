import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { LifeBuoy, Eye, RefreshCw, Filter, Clock, AlertCircle } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/Table'
import { Select } from '@/components/ui/Select'
import { AdminLoadingState, AdminErrorState, AdminEmptyState, AdminPagination, PageHeader } from '@/components/common'
import { useSupportTickets } from '@/hooks/useSupport'
import { formatDate } from '@/lib/utils'
import type {
  SupportTicketStatus,
  SupportCategory,
} from '@/types/support'
import {
  supportTicketStatusLabels,
  supportTicketStatusVariants,
  supportCategoryLabels,
  supportPriorityLabels,
  supportPriorityVariants,
} from '@/types/support'

// Helper function to format duration in human-readable format
function formatDuration(seconds: number): string {
  const hours = Math.floor(seconds / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)

  if (hours > 0) {
    return `${hours}h ${minutes}m`
  }
  return `${minutes}m`
}

// Helper function to get time since creation
function getTimeSinceCreation(createdAt: string): number {
  return Math.floor((Date.now() - new Date(createdAt).getTime()) / 1000)
}

const SUPPORT_STATUSES: { value: SupportTicketStatus | ''; label: string }[] = [
  { value: '', label: 'All Statuses' },
  { value: 'open', label: 'Open' },
  { value: 'in_progress', label: 'In Progress' },
  { value: 'waiting_user', label: 'Waiting for User' },
  { value: 'resolved', label: 'Resolved' },
  { value: 'closed', label: 'Closed' },
]

// Canonical category vocabulary — identical to the backend enum and the
// mobile client. Kept in the same order as the backend taxonomy.
const SUPPORT_CATEGORIES: { value: SupportCategory | ''; label: string }[] = [
  { value: '', label: 'All Categories' },
  { value: 'order_issue', label: 'Order Issue' },
  { value: 'payment_issue', label: 'Payment Issue' },
  { value: 'account_issue', label: 'Account Issue' },
  { value: 'listing_issue', label: 'Listing Issue' },
  { value: 'shipping_issue', label: 'Shipping Issue' },
  { value: 'refund_request', label: 'Refund Request' },
  { value: 'dispute', label: 'Dispute' },
  { value: 'technical_issue', label: 'Technical Issue' },
  { value: 'other', label: 'Other' },
]

const SLA_FILTERS: { value: boolean | undefined; label: string }[] = [
  { value: undefined, label: 'All Tickets' },
  { value: true, label: 'Overdue Only' },
  { value: false, label: 'Not Overdue' },
]

const ASSIGNMENT_FILTERS: { value: boolean | undefined; label: string }[] = [
  { value: undefined, label: 'All Tickets' },
  { value: true, label: 'Unassigned Only' },
  { value: false, label: 'Assigned' },
]

export function SupportTicketsPage() {
  const navigate = useNavigate()
  const [statusFilter, setStatusFilter] = useState<SupportTicketStatus | ''>('')
  const [categoryFilter, setCategoryFilter] = useState<SupportCategory | ''>('')
  const [isOverdueFilter, setIsOverdueFilter] = useState<boolean | undefined>(undefined)
  const [isUnassignedFilter, setIsUnassignedFilter] = useState<boolean | undefined>(undefined)

  const { tickets, loading, error, total, page, setPage, totalPages, refetch } = useSupportTickets(
    statusFilter || categoryFilter || isOverdueFilter !== undefined || isUnassignedFilter !== undefined
      ? {
          ...(statusFilter && { status: statusFilter }),
          ...(categoryFilter && { category: categoryFilter }),
          ...(isOverdueFilter !== undefined && { is_overdue: isOverdueFilter }),
          ...(isUnassignedFilter !== undefined && { is_unassigned: isUnassignedFilter }),
        }
      : {}
  )

  // Ordering authority is the SERVER (canonical SLA-urgency order). The page
  // renders the server-ordered slice verbatim — no client-side re-sort, no
  // per-page sort.

  const handleViewTicket = (ticketId: string) => {
    navigate(`/support/tickets/${ticketId}`)
  }

  const hasActiveFilters =
    Boolean(statusFilter) ||
    Boolean(categoryFilter) ||
    isOverdueFilter !== undefined ||
    isUnassignedFilter !== undefined

  const handleClearFilters = () => {
    setStatusFilter('')
    setCategoryFilter('')
    setIsOverdueFilter(undefined)
    setIsUnassignedFilter(undefined)
    setPage(1)
  }

  // STEP 2: Get border color based on escalation/priority/SLA
  const getBorderClass = (ticket: typeof tickets[0]): string => {
    if (ticket.sla.is_overdue) return 'border-l-4 border-destructive bg-destructive-bg'
    if (ticket.escalation === 'dispute') return 'border-l-4 border-destructive'
    if (ticket.priority === 'urgent') return 'border-l-4 border-warning'
    if (ticket.priority === 'high') return 'border-l-4 border-warning'
    return ''
  }

  // Get SLA display information
  const getSLADisplay = (ticket: typeof tickets[0]) => {
    const timeSinceCreation = getTimeSinceCreation(ticket.created_at)

    if (ticket.status === 'resolved' || ticket.status === 'closed') {
      // Ticket is resolved/closed - show resolution time
      if (ticket.sla.resolution_time_seconds) {
        const duration = formatDuration(ticket.sla.resolution_time_seconds)
        return {
          text: duration,
          overdue: ticket.sla.resolution_overdue,
          variant: (ticket.sla.resolution_overdue ? 'error' : 'success') as 'error' | 'success',
          firstResponseOverdue: false,
          resolutionOverdue: ticket.sla.resolution_overdue
        }
      }
    } else {
      // Ticket is open - show first response or active time
      if (ticket.sla.first_response_time_seconds) {
        // First response already made
        const duration = formatDuration(ticket.sla.first_response_time_seconds)
        return {
          text: duration,
          overdue: ticket.sla.first_response_overdue,
          variant: (ticket.sla.first_response_overdue ? 'error' : 'success') as 'error' | 'success',
          firstResponseOverdue: ticket.sla.first_response_overdue,
          resolutionOverdue: ticket.sla.resolution_overdue
        }
      } else {
        // No first response yet
        const duration = formatDuration(timeSinceCreation)
        return {
          text: duration,
          overdue: ticket.sla.first_response_overdue,
          variant: (ticket.sla.first_response_overdue ? 'error' : 'pending') as 'error' | 'pending',
          firstResponseOverdue: ticket.sla.first_response_overdue,
          resolutionOverdue: ticket.sla.resolution_overdue
        }
      }
    }

    return {
      text: '-',
      overdue: false,
      variant: 'info' as const,
      firstResponseOverdue: false,
      resolutionOverdue: false
    }
  }

  // Get next action display
  const getNextActionDisplay = (ticket: typeof tickets[0]) => {
    const action = ticket.sla.next_action

    switch (action) {
      case 'reply':
        return {
          text: 'Reply Needed',
          variant: 'error' as const,
          icon: <AlertCircle className="h-4 w-4" />
        }
      case 'wait':
        return {
          text: 'Waiting for User',
          variant: 'info' as const,
          icon: <Clock className="h-4 w-4" />
        }
      case 'resolve':
        return {
          text: 'Resolve Ticket',
          variant: 'warning' as const,
          icon: <AlertCircle className="h-4 w-4" />
        }
      case 'none':
        return {
          text: 'Completed',
          variant: 'success' as const,
          icon: null
        }
      default:
        return {
          text: '-',
          variant: 'info' as const,
          icon: null
        }
    }
  }

  // STEP 3: Get status display (stronger for escalated)
  const getStatusDisplay = (ticket: typeof tickets[0]) => {
    if (ticket.escalation === 'dispute') {
      return { text: 'ESCALATED', variant: 'error' as const, bold: true }
    }
    return {
      text: supportTicketStatusLabels[ticket.status] || ticket.status,
      variant: supportTicketStatusVariants[ticket.status] || 'info',
      bold: false
    }
  }

  if (loading && tickets.length === 0) {
    return <AdminLoadingState />
  }

  if (error) {
    return (
      <div className="space-y-6">
        <PageHeader title="Support Tickets" description="Manage customer support requests" />
        <AdminErrorState title="Failed to load tickets" message={error.message} onRetry={refetch} />
      </div>
    )
  }

  const openCount = tickets.filter(t => t.status === 'open').length
  const overdueCount = tickets.filter(t => t.sla.is_overdue).length

  return (
    <div className="space-y-6">
      {/* Header */}
      <PageHeader
        title="Support Tickets"
        description="Manage customer support requests"
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
            <div className="flex-1">
              <p className="text-sm font-medium text-muted-foreground">Total Tickets</p>
              <p className="type-metric-lg text-primary mt-1">{total}</p>
              <p className="type-caption mt-1">{openCount} open on this page</p>
            </div>
            <div className="flex-1 text-center">
              <p className="text-sm font-medium text-muted-foreground">SLA Overdue</p>
              <p className={`type-metric-lg mt-1 ${overdueCount > 0 ? 'text-destructive' : 'text-success'}`}>
                {overdueCount}
              </p>
              <p className="type-caption mt-1">
                {overdueCount > 0 ? 'overdue on this page' : 'none overdue on this page'}
              </p>
            </div>
            <div className="p-4 rounded-lg bg-info-bg">
              <LifeBuoy className="h-8 w-8 text-info" />
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Filters */}
      <Card>
        <CardContent className="pt-6">
          <div className="flex items-end gap-6 flex-wrap">
            <div className="flex items-end gap-2">
              <Filter className="h-5 w-5 text-muted-foreground mb-2" />
              <Select
                label="Status:"
                value={statusFilter}
                onChange={(e) => { setStatusFilter(e.target.value as SupportTicketStatus | ''); setPage(1) }}
              >
                {SUPPORT_STATUSES.map((status) => (
                  <option key={status.value} value={status.value}>
                    {status.label}
                  </option>
                ))}
              </Select>
            </div>
            <Select
              label="Category:"
              value={categoryFilter}
              onChange={(e) => { setCategoryFilter(e.target.value as SupportCategory | ''); setPage(1) }}
            >
              {SUPPORT_CATEGORIES.map((category) => (
                <option key={category.value} value={category.value}>
                  {category.label}
                </option>
              ))}
            </Select>
            <div className="flex items-end gap-2">
              <AlertCircle className="h-5 w-5 text-muted-foreground mb-2" />
              <Select
                label="SLA:"
                value={isOverdueFilter === undefined ? 'undefined' : isOverdueFilter.toString()}
                onChange={(e) => {
                  const val = e.target.value
                  setIsOverdueFilter(val === 'undefined' ? undefined : val === 'true')
                  setPage(1)
                }}
              >
                {SLA_FILTERS.map((filter) => (
                  <option key={filter.value?.toString() || 'undefined'} value={filter.value?.toString() || 'undefined'}>
                    {filter.label}
                  </option>
                ))}
              </Select>
            </div>
            <Select
              label="Assignment:"
              value={isUnassignedFilter === undefined ? 'undefined' : isUnassignedFilter.toString()}
              onChange={(e) => {
                const val = e.target.value
                setIsUnassignedFilter(val === 'undefined' ? undefined : val === 'true')
                setPage(1)
              }}
            >
              {ASSIGNMENT_FILTERS.map((filter) => (
                <option key={filter.value?.toString() || 'undefined'} value={filter.value?.toString() || 'undefined'}>
                  {filter.label}
                </option>
              ))}
            </Select>
          </div>
        </CardContent>
      </Card>

      {/* Tickets Table */}
      <Card>
        <CardHeader>
          <CardTitle>Tickets Queue</CardTitle>
        </CardHeader>
        <CardContent>
          {tickets.length === 0 ? (
            <AdminEmptyState
              icon={LifeBuoy}
              title="No Tickets Found"
              description={
                hasActiveFilters
                  ? 'No tickets match the current filters.'
                  : 'No support tickets in the system.'
              }
              filtered={hasActiveFilters}
              onClearFilters={handleClearFilters}
            />
          ) : (
            <div className="border border-border rounded-lg overflow-hidden">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Ticket ID</TableHead>
                    <TableHead>User</TableHead>
                    <TableHead>Subject</TableHead>
                    <TableHead>Category</TableHead>
                    <TableHead>Priority</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>
                      <div className="flex items-center gap-1">
                        <Clock className="h-4 w-4" />
                        SLA
                      </div>
                    </TableHead>
                    <TableHead>Next Action</TableHead>
                    <TableHead>Created At</TableHead>
                    <TableHead className="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {tickets.map((ticket) => {
                    const statusDisplay = getStatusDisplay(ticket)
                    return (
                      <TableRow key={ticket.id} className={getBorderClass(ticket)}>
                        <TableCell className="font-mono text-sm">
                          {ticket.id.slice(0, 8)}
                        </TableCell>
                        <TableCell>
                          <div className="flex items-center gap-2">
                            {ticket.user_avatar ? (
                              <img
                                src={ticket.user_avatar}
                                alt=""
                                className="w-6 h-6 rounded-full object-cover"
                              />
                            ) : (
                              <div className="w-6 h-6 rounded-full bg-border" />
                            )}
                            <div className="min-w-0">
                              <div className="text-sm truncate max-w-[140px] font-medium">
                                {ticket.username ? `@${ticket.username}` : ticket.user_id.slice(0, 8)}
                              </div>
                              {ticket.username && ticket.seller_farm_name ? (
                                <div className="type-caption truncate max-w-[140px]">
                                  {ticket.seller_farm_name}
                                </div>
                              ) : null}
                            </div>
                          </div>
                        </TableCell>
                        <TableCell>
                          <span className="text-sm truncate max-w-[200px] block">
                            {ticket.subject}
                          </span>
                        </TableCell>
                        <TableCell>
                          <span className="text-sm">
                            {supportCategoryLabels[ticket.category] || ticket.category}
                          </span>
                        </TableCell>
                        <TableCell>
                          <Badge variant={supportPriorityVariants[ticket.priority] || 'info'}>
                            {supportPriorityLabels[ticket.priority] || ticket.priority}
                          </Badge>
                        </TableCell>
                        <TableCell>
                          <Badge
                            variant={statusDisplay.variant}
                            className={statusDisplay.bold ? 'font-bold' : ''}
                          >
                            {statusDisplay.text}
                          </Badge>
                        </TableCell>
                        <TableCell>
                          <div className="flex items-center gap-2">
                            {(() => {
                              const slaDisplay = getSLADisplay(ticket)
                              return (
                                <>
                                  <Badge variant={slaDisplay.variant} className="font-medium">
                                    {slaDisplay.text}
                                  </Badge>
                                  {slaDisplay.firstResponseOverdue && (
                                    <AlertCircle className="h-4 w-4 text-destructive" aria-label="First Response Overdue" />
                                  )}
                                  {slaDisplay.resolutionOverdue && !slaDisplay.firstResponseOverdue && (
                                    <AlertCircle className="h-4 w-4 text-warning" aria-label="Resolution Overdue" />
                                  )}
                                </>
                              )
                            })()}
                          </div>
                        </TableCell>
                        <TableCell>
                          {(() => {
                            const actionDisplay = getNextActionDisplay(ticket)
                            return (
                              <div className="flex items-center gap-1">
                                {actionDisplay.icon}
                                <span className="text-sm">
                                  {actionDisplay.text}
                                </span>
                              </div>
                            )
                          })()}
                        </TableCell>
                        <TableCell className="type-secondary">
                          {formatDate(ticket.created_at)}
                        </TableCell>
                        <TableCell className="text-right">
                          <Button
                            size="sm"
                            onClick={() => handleViewTicket(ticket.id)}
                          >
                            <Eye className="h-4 w-4 mr-1" />
                            View
                          </Button>
                        </TableCell>
                      </TableRow>
                    )
                  })}
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
    </div>
  )
}
