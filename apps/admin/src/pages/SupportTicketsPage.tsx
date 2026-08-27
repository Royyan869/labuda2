import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { LifeBuoy, Eye, RefreshCw, Filter, Clock, AlertCircle } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/Table'
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

const SUPPORT_CATEGORIES: { value: SupportCategory | ''; label: string }[] = [
  { value: '', label: 'All Categories' },
  { value: 'order_issue', label: 'Order Issue' },
  { value: 'payment_issue', label: 'Payment Issue' },
  { value: 'account_issue', label: 'Account Issue' },
  { value: 'listing_issue', label: 'Listing Issue' },
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

  const { tickets, loading, error, total, refetch } = useSupportTickets(
    statusFilter || categoryFilter || isOverdueFilter !== undefined || isUnassignedFilter !== undefined
      ? {
          ...(statusFilter && { status: statusFilter }),
          ...(categoryFilter && { category: categoryFilter }),
          ...(isOverdueFilter !== undefined && { is_overdue: isOverdueFilter }),
          ...(isUnassignedFilter !== undefined && { is_unassigned: isUnassignedFilter }),
        }
      : {}
  )

  // STEP 1: Priority sorting (SLA overdue tickets first)
  const sortedTickets = [...tickets].sort((a, b) => {
    // Priority 0: SLA overdue (most critical)
    if (a.sla.is_overdue && !b.sla.is_overdue) return -1
    if (!a.sla.is_overdue && b.sla.is_overdue) return 1

    // Priority 1: escalation === "dispute"
    if (a.escalation === 'dispute' && b.escalation !== 'dispute') return -1
    if (a.escalation !== 'dispute' && b.escalation === 'dispute') return 1

    // Priority 2: priority === "urgent"
    if (a.priority === 'urgent' && b.priority !== 'urgent') return -1
    if (a.priority !== 'urgent' && b.priority === 'urgent') return 1

    // Priority 3: priority === "high"
    if (a.priority === 'high' && b.priority !== 'high') return -1
    if (a.priority !== 'high' && b.priority === 'high') return 1

    // Priority 4: newest created_at
    return new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
  })

  const handleViewTicket = (ticketId: string) => {
    navigate(`/support/tickets/${ticketId}`)
  }

  // STEP 2: Get border color based on escalation/priority/SLA
  const getBorderClass = (ticket: typeof tickets[0]): string => {
    if (ticket.sla.is_overdue) return 'border-l-4 border-red-600 bg-red-50'
    if (ticket.escalation === 'dispute') return 'border-l-4 border-red-500'
    if (ticket.priority === 'urgent') return 'border-l-4 border-orange-500'
    if (ticket.priority === 'high') return 'border-l-4 border-yellow-500'
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

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <div className="text-center">
          <div className="inline-block h-8 w-8 animate-spin rounded-full border-4 border-solid border-primary border-r-transparent"></div>
          <p className="mt-4 text-gray-600">Loading support tickets...</p>
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="space-y-6">
        <div>
          <h1 className="text-3xl font-bold text-gray-900">Support Tickets</h1>
          <p className="text-gray-600 mt-1">Manage customer support requests</p>
        </div>
        <Card>
          <CardContent className="p-6">
            <div className="text-center text-red-600">
              <p>Error loading tickets: {error.message}</p>
            </div>
          </CardContent>
        </Card>
      </div>
    )
  }

  const openCount = tickets.filter(t => t.status === 'open').length
  const overdueCount = tickets.filter(t => t.sla.is_overdue).length

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold text-gray-900">Support Tickets</h1>
          <p className="text-gray-600 mt-1">Manage customer support requests</p>
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
            <div className="flex-1">
              <p className="text-sm font-medium text-gray-600">Total Tickets</p>
              <p className="text-3xl font-bold text-primary mt-1">{total}</p>
              <p className="text-xs text-gray-500 mt-1">{openCount} open tickets</p>
            </div>
            <div className="flex-1 text-center">
              <p className="text-sm font-medium text-gray-600">SLA Overdue</p>
              <p className={`text-3xl font-bold mt-1 ${overdueCount > 0 ? 'text-red-600' : 'text-green-600'}`}>
                {overdueCount}
              </p>
              <p className="text-xs text-gray-500 mt-1">
                {overdueCount > 0 ? 'tickets overdue' : 'all on track'}
              </p>
            </div>
            <div className="p-4 rounded-lg bg-blue-100">
              <LifeBuoy className="h-8 w-8 text-blue-600" />
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
              <label htmlFor="status-filter" className="text-sm font-medium text-gray-700">
                Status:
              </label>
              <select
                id="status-filter"
                value={statusFilter}
                onChange={(e) => setStatusFilter(e.target.value as SupportTicketStatus | '')}
                className="px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-primary"
              >
                {SUPPORT_STATUSES.map((status) => (
                  <option key={status.value} value={status.value}>
                    {status.label}
                  </option>
                ))}
              </select>
            </div>
            <div className="flex items-center gap-4">
              <label htmlFor="category-filter" className="text-sm font-medium text-gray-700">
                Category:
              </label>
              <select
                id="category-filter"
                value={categoryFilter}
                onChange={(e) => setCategoryFilter(e.target.value as SupportCategory | '')}
                className="px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-primary"
              >
                {SUPPORT_CATEGORIES.map((category) => (
                  <option key={category.value} value={category.value}>
                    {category.label}
                  </option>
                ))}
              </select>
            </div>
            <div className="flex items-center gap-4">
              <AlertCircle className="h-5 w-5 text-gray-500" />
              <label htmlFor="sla-filter" className="text-sm font-medium text-gray-700">
                SLA:
              </label>
              <select
                id="sla-filter"
                value={isOverdueFilter === undefined ? 'undefined' : isOverdueFilter.toString()}
                onChange={(e) => {
                  const val = e.target.value
                  setIsOverdueFilter(val === 'undefined' ? undefined : val === 'true')
                }}
                className="px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-primary"
              >
                {SLA_FILTERS.map((filter) => (
                  <option key={filter.value?.toString() || 'undefined'} value={filter.value?.toString() || 'undefined'}>
                    {filter.label}
                  </option>
                ))}
              </select>
            </div>
            <div className="flex items-center gap-4">
              <label htmlFor="assignment-filter" className="text-sm font-medium text-gray-700">
                Assignment:
              </label>
              <select
                id="assignment-filter"
                value={isUnassignedFilter === undefined ? 'undefined' : isUnassignedFilter.toString()}
                onChange={(e) => {
                  const val = e.target.value
                  setIsUnassignedFilter(val === 'undefined' ? undefined : val === 'true')
                }}
                className="px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-primary"
              >
                {ASSIGNMENT_FILTERS.map((filter) => (
                  <option key={filter.value?.toString() || 'undefined'} value={filter.value?.toString() || 'undefined'}>
                    {filter.label}
                  </option>
                ))}
              </select>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Tickets Table */}
      <Card>
        <CardHeader>
          <CardTitle>Tickets Queue</CardTitle>
        </CardHeader>
        <CardContent>
          {sortedTickets.length === 0 ? (
            <div className="text-center py-12">
              <LifeBuoy className="h-12 w-12 text-gray-400 mx-auto mb-4" />
              <h3 className="text-lg font-semibold text-gray-900 mb-2">No Tickets Found</h3>
              <p className="text-gray-600">
                {statusFilter || categoryFilter || isOverdueFilter !== undefined || isUnassignedFilter !== undefined
                  ? 'No tickets match the current filters.'
                  : 'No support tickets in the system.'}
              </p>
            </div>
          ) : (
            <div className="border border-gray-200 rounded-lg overflow-hidden">
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
                  {sortedTickets.map((ticket) => {
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
                              <div className="w-6 h-6 rounded-full bg-gray-200" />
                            )}
                            <div className="min-w-0">
                              <div className="text-sm truncate max-w-[140px] font-medium">
                                {ticket.username ? `@${ticket.username}` : ticket.user_id.slice(0, 8)}
                              </div>
                              {ticket.username && ticket.seller_farm_name ? (
                                <div className="text-xs text-gray-500 truncate max-w-[140px]">
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
                                    <AlertCircle className="h-4 w-4 text-red-600" aria-label="First Response Overdue" />
                                  )}
                                  {slaDisplay.resolutionOverdue && !slaDisplay.firstResponseOverdue && (
                                    <AlertCircle className="h-4 w-4 text-orange-600" aria-label="Resolution Overdue" />
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
                        <TableCell className="text-sm text-gray-600">
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
    </div>
  )
}
