import { useState, useEffect } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, LifeBuoy, CheckCircle, XCircle, Send, User, Clock, AlertCircle, Package, Scale, Gavel, UserCheck, Pause } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { Textarea } from '@/components/ui/Textarea'
import { Modal } from '@/components/ui/Modal'
import { useSupportTicketDetail, useSupportMessages, useSupportTicketActions } from '@/hooks/useSupport'
import { formatDateTime } from '@/lib/utils'
import type {
  EscalateToDisputeRequest,
  SupportPriority,
  SupportCategory,
} from '@/types/support'
import {
  supportTicketStatusLabels,
  supportTicketStatusVariants,
  supportCategoryLabels,
  supportPriorityLabels,
  supportPriorityVariants,
} from '@/types/support'

export function SupportTicketDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [messageText, setMessageText] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [successMessage, setSuccessMessage] = useState('')
  const [escalationModalOpen, setEscalationModalOpen] = useState(false)
  const [escalationReason, setEscalationReason] = useState('')
  const [escalationDescription, setEscalationDescription] = useState('')
  const [escalationReasonCode, setEscalationReasonCode] = useState('')

  const { ticket, loading: ticketLoading, error: ticketError, refetch: refetchTicket } = useSupportTicketDetail(id || null)
  const { messages, loading: messagesLoading, refetch: refetchMessages } = useSupportMessages(id || null)
  const {
    resolveTicket, closeTicket, sendMessage, escalateToDispute,
    claimTicket, setWaitingForUser, updatePriority, updateCategory,
  } = useSupportTicketActions(id || '')
  const [actionError, setActionError] = useState('')

  useEffect(() => {
    if (successMessage) {
      const timer = setTimeout(() => setSuccessMessage(''), 3000)
      return () => clearTimeout(timer)
    }
  }, [successMessage])

  const handleSendMessage = async () => {
    if (!messageText.trim()) return

    setSubmitting(true)
    const result = await sendMessage({ type: 'agent', message: messageText })
    setSubmitting(false)

    if (result.success) {
      setMessageText('')
      refetchMessages()
      refetchTicket()
      setSuccessMessage('Message sent successfully')
    } else {
      alert(result.error?.message || 'Failed to send message')
    }
  }

  const handleResolve = async () => {
    if (!confirm('Mark this ticket as resolved?')) return

    setSubmitting(true)
    setActionError('')
    const result = await resolveTicket()
    setSubmitting(false)

    if (result.success) {
      refetchTicket()
      refetchMessages()
      setSuccessMessage('Ticket resolved successfully')
    } else {
      setActionError(result.error?.message || 'Failed to resolve ticket')
    }
  }

  const handleClose = async () => {
    if (!confirm('Close this ticket?')) return

    setSubmitting(true)
    setActionError('')
    const result = await closeTicket()
    setSubmitting(false)

    if (result.success) {
      refetchTicket()
      refetchMessages()
      setSuccessMessage('Ticket closed successfully')
    } else {
      setActionError(result.error?.message || 'Failed to close ticket')
    }
  }

  const handleClaim = async () => {
    setSubmitting(true)
    setActionError('')
    const result = await claimTicket()
    setSubmitting(false)

    if (result.success) {
      refetchTicket()
      setSuccessMessage('Ticket claimed successfully')
    } else {
      setActionError(result.error?.message || 'Failed to claim ticket')
    }
  }

  const handleSetWaiting = async () => {
    setSubmitting(true)
    setActionError('')
    const result = await setWaitingForUser()
    setSubmitting(false)

    if (result.success) {
      refetchTicket()
      setSuccessMessage('Ticket set to waiting for user')
    } else {
      setActionError(result.error?.message || 'Failed to set waiting')
    }
  }

  const handleUpdatePriority = async (priority: SupportPriority) => {
    setSubmitting(true)
    setActionError('')
    const result = await updatePriority({ priority })
    setSubmitting(false)

    if (result.success) {
      refetchTicket()
      setSuccessMessage('Priority updated')
    } else {
      setActionError(result.error?.message || 'Failed to update priority')
    }
  }

  const handleUpdateCategory = async (category: SupportCategory) => {
    setSubmitting(true)
    setActionError('')
    const result = await updateCategory({ category })
    setSubmitting(false)

    if (result.success) {
      refetchTicket()
      setSuccessMessage('Category updated')
    } else {
      setActionError(result.error?.message || 'Failed to update category')
    }
  }

  const handleEscalateToDispute = async () => {
    if (!escalationReason.trim()) {
      alert('Please provide a reason for escalation')
      return
    }

    if (!escalationReasonCode.trim()) {
      alert('Please provide a reason code')
      return
    }

    setSubmitting(true)
    const data: EscalateToDisputeRequest = {
      reason: escalationReason,
      description: escalationDescription || undefined,
      reason_code: escalationReasonCode,
    }
    const result = await escalateToDispute(data)
    setSubmitting(false)

    if (result.success) {
      setEscalationModalOpen(false)
      setEscalationReason('')
      setEscalationDescription('')
      setEscalationReasonCode('')
      refetchTicket()
      refetchMessages()
      setSuccessMessage('Ticket escalated to dispute successfully')
    } else {
      alert(result.error?.message || 'Failed to escalate ticket to dispute')
    }
  }

  // Check if escalation button should be shown
  const canEscalateToDispute = ticket &&
    ticket.order_id &&
    ticket.escalation !== 'dispute' &&
    ticket.order_info &&
    !ticket.order_info.has_dispute

  if (ticketLoading) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <div className="text-center">
          <div className="inline-block h-8 w-8 animate-spin rounded-full border-4 border-solid border-primary border-r-transparent"></div>
          <p className="mt-4 text-gray-600">Loading ticket details...</p>
        </div>
      </div>
    )
  }

  if (ticketError || !ticket) {
    return (
      <div className="space-y-6">
        <div className="flex items-center gap-4">
          <Button variant="secondary" onClick={() => navigate('/support/tickets')}>
            <ArrowLeft className="h-4 w-4 mr-2" />
            Back to Tickets
          </Button>
        </div>
        <Card>
          <CardContent className="p-6">
            <div className="text-center text-red-600">
              <p>Error loading ticket: {ticketError?.message || 'Ticket not found'}</p>
            </div>
          </CardContent>
        </Card>
      </div>
    )
  }

  // Backend canonical open states: open, in_progress, waiting_user
  const isActive = ticket.status === 'open' || ticket.status === 'in_progress' || ticket.status === 'waiting_user'
  const canResolve = ticket.status === 'in_progress' || ticket.status === 'waiting_user'
  const canClose = ticket.status === 'resolved'
  const canClaim = ticket.status === 'open'
  const canSetWaiting = ticket.status === 'in_progress'

  // STEP 7: Status hint based on last message
  const getLastMessageHint = () => {
    if (!messages || messages.length === 0) return null

    const lastMessage = messages[messages.length - 1]
    if (lastMessage.sender_type === 'user') {
      return {
        text: 'Waiting for admin response',
        variant: 'info' as const
      }
    } else if (lastMessage.sender_type === 'admin') {
      return {
        text: 'Waiting for user response',
        variant: 'warning' as const
      }
    }
    return null
  }

  const statusHint = getLastMessageHint()

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <Button variant="secondary" onClick={() => navigate('/support/tickets')}>
            <ArrowLeft className="h-4 w-4 mr-2" />
            Back
          </Button>
          <div>
            <h1 className="text-3xl font-bold text-gray-900">Ticket #{ticket.id.slice(0, 8)}</h1>
            <p className="text-gray-600 mt-1">{ticket.subject}</p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          {canClaim && (
            <Button
              variant="primary"
              onClick={handleClaim}
              disabled={submitting}
              className="gap-2"
            >
              <UserCheck className="h-4 w-4" />
              Claim
            </Button>
          )}
          {canSetWaiting && (
            <Button
              variant="secondary"
              onClick={handleSetWaiting}
              disabled={submitting}
              className="gap-2"
            >
              <Pause className="h-4 w-4" />
              Waiting for User
            </Button>
          )}
          {canResolve && (
            <Button
              variant="secondary"
              onClick={handleResolve}
              disabled={submitting}
              className="gap-2"
            >
              <CheckCircle className="h-4 w-4" />
              Resolve
            </Button>
          )}
          {canClose && (
            <Button
              variant="secondary"
              onClick={handleClose}
              disabled={submitting}
              className="gap-2"
            >
              <XCircle className="h-4 w-4" />
              Close
            </Button>
          )}
        </div>
      </div>

      {/* STEP 7: Status hint */}
      {statusHint && (
        <div className={`bg-${statusHint.variant === 'info' ? 'blue' : 'yellow'}-50 border border-${statusHint.variant === 'info' ? 'blue' : 'yellow'}-200 text-${statusHint.variant === 'info' ? 'blue' : 'yellow'}-700 px-4 py-3 rounded-lg flex items-center gap-2`}>
          <AlertCircle className="h-4 w-4" />
          <span className="text-sm font-medium">{statusHint.text}</span>
        </div>
      )}

      {/* Success Message */}
      {successMessage && (
        <div className="bg-green-50 border border-green-200 text-green-700 px-4 py-3 rounded-lg">
          {successMessage}
        </div>
      )}

      {/* Action Error */}
      {actionError && (
        <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded-lg">
          {actionError}
        </div>
      )}

      {/* Ticket Info */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Main Ticket Details */}
        <div className="lg:col-span-2 space-y-6">
          {/* User & Category Info */}
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <LifeBuoy className="h-5 w-5" />
                Ticket Information
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <p className="text-sm font-medium text-gray-600">User</p>
                  <div className="flex items-start gap-2 mt-1">
                    {ticket.user_avatar ? (
                      <img
                        src={ticket.user_avatar}
                        alt=""
                        className="w-8 h-8 rounded-full object-cover"
                      />
                    ) : (
                      <div className="w-8 h-8 rounded-full bg-gray-200 flex items-center justify-center">
                        <User className="h-4 w-4 text-gray-500" />
                      </div>
                    )}
                    <div className="min-w-0">
                      <div className="text-sm font-medium truncate max-w-[220px]">
                        {ticket.username ? `@${ticket.username}` : ticket.user_id.slice(0, 8)}
                      </div>
                      {ticket.username && ticket.seller_farm_name ? (
                        <div className="text-xs text-gray-500 truncate max-w-[220px]">
                          {ticket.seller_farm_name}
                        </div>
                      ) : null}
                    </div>
                  </div>
                </div>
                <div>
                  <p className="text-sm font-medium text-gray-600">User ID</p>
                  <p className="text-sm font-mono text-gray-900 mt-1">{ticket.user_id.slice(0, 8)}</p>
                </div>
                <div>
                  <p className="text-sm font-medium text-gray-600">Category</p>
                  <p className="text-sm text-gray-900 mt-1">
                    {supportCategoryLabels[ticket.category] || ticket.category}
                  </p>
                </div>
                <div>
                  <p className="text-sm font-medium text-gray-600">Priority</p>
                  <Badge variant={supportPriorityVariants[ticket.priority] || 'info'} className="mt-1">
                    {supportPriorityLabels[ticket.priority] || ticket.priority}
                  </Badge>
                </div>
                <div>
                  <p className="text-sm font-medium text-gray-600">Status</p>
                  <Badge variant={supportTicketStatusVariants[ticket.status] || 'info'} className="mt-1">
                    {supportTicketStatusLabels[ticket.status] || ticket.status}
                  </Badge>
                </div>
                {ticket.order_id && (
                  <div>
                    <p className="text-sm font-medium text-gray-600">Linked Order</p>
                    <p className="text-sm font-mono text-gray-900 mt-1">{ticket.order_id.slice(0, 8)}</p>
                  </div>
                )}
                <div>
                  <p className="text-sm font-medium text-gray-600">Created At</p>
                  <p className="text-sm text-gray-900 mt-1 flex items-center gap-1">
                    <Clock className="h-3 w-3" />
                    {formatDateTime(ticket.created_at)}
                  </p>
                </div>
                <div>
                  <p className="text-sm font-medium text-gray-600">Last Updated</p>
                  <p className="text-sm text-gray-900 mt-1 flex items-center gap-1">
                    <Clock className="h-3 w-3" />
                    {formatDateTime(ticket.updated_at)}
                  </p>
                </div>
              </div>

              {/* Description */}
              <div className="pt-4 border-t">
                <p className="text-sm font-medium text-gray-600 mb-2">Description</p>
                <p className="text-sm text-gray-900 bg-gray-50 p-3 rounded-lg">
                  {ticket.description || 'No description provided'}
                </p>
              </div>

              {/* Admin Info */}
              {ticket.admin_name && (
                <div className="pt-4 border-t">
                  <p className="text-sm font-medium text-gray-600 mb-2">Assigned To</p>
                  <p className="text-sm text-gray-900">{ticket.admin_name}</p>
                </div>
              )}
            </CardContent>
          </Card>

          {/* Order Information */}
          {ticket.order_info && (
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <Package className="h-5 w-5" />
                  Order Information
                </CardTitle>
              </CardHeader>
              <CardContent>
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <p className="text-sm font-medium text-gray-600">Order ID</p>
                    <p className="text-sm font-mono text-gray-900 mt-1">{ticket.order_info.order_id.slice(0, 8)}</p>
                  </div>
                  <div>
                    <p className="text-sm font-medium text-gray-600">Status</p>
                    <Badge variant="info" className="mt-1">
                      {ticket.order_info.status}
                    </Badge>
                  </div>
                  <div>
                    <p className="text-sm font-medium text-gray-600">Escrow Status</p>
                    <p className="text-sm text-gray-900 mt-1">{ticket.order_info.escrow_status}</p>
                  </div>
                  <div>
                    <p className="text-sm font-medium text-gray-600">Has Dispute</p>
                    <Badge variant={ticket.order_info.has_dispute ? 'warning' : 'success'} className="mt-1">
                      {ticket.order_info.has_dispute ? 'Yes' : 'No'}
                    </Badge>
                  </div>
                </div>
              </CardContent>
            </Card>
          )}

          {/* Dispute Information */}
          {ticket.dispute_info && (
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <Scale className="h-5 w-5" />
                  Dispute Information
                </CardTitle>
              </CardHeader>
              <CardContent>
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <p className="text-sm font-medium text-gray-600">Dispute ID</p>
                    <p className="text-sm font-mono text-gray-900 mt-1">{ticket.dispute_info.dispute_id.slice(0, 8)}</p>
                  </div>
                  <div>
                    <p className="text-sm font-medium text-gray-600">Status</p>
                    <Badge variant="warning" className="mt-1">
                      {ticket.dispute_info.status}
                    </Badge>
                  </div>
                  <div>
                    <p className="text-sm font-medium text-gray-600">Opened At</p>
                    <p className="text-sm text-gray-900 mt-1 flex items-center gap-1">
                      <Clock className="h-3 w-3" />
                      {formatDateTime(ticket.dispute_info.opened_at)}
                    </p>
                  </div>
                  {ticket.dispute_info.resolved_at && (
                    <div>
                      <p className="text-sm font-medium text-gray-600">Resolved At</p>
                      <p className="text-sm text-gray-900 mt-1 flex items-center gap-1">
                        <Clock className="h-3 w-3" />
                        {formatDateTime(ticket.dispute_info.resolved_at)}
                      </p>
                    </div>
                  )}
                </div>
              </CardContent>
            </Card>
          )}

          {/* Messages Thread */}
          <Card>
            <CardHeader>
              <CardTitle>Conversation</CardTitle>
            </CardHeader>
            <CardContent>
              {messagesLoading ? (
                <div className="text-center py-8">
                  <div className="inline-block h-6 w-6 animate-spin rounded-full border-4 border-solid border-primary border-r-transparent"></div>
                  <p className="mt-2 text-sm text-gray-600">Loading messages...</p>
                </div>
              ) : messages.length === 0 ? (
                <div className="text-center py-8">
                  <AlertCircle className="h-12 w-12 text-gray-400 mx-auto mb-3" />
                  <p className="text-sm text-gray-600">No messages yet</p>
                </div>
              ) : (
                <div className="space-y-4">
                  {messages.map((message, index) => (
                    <div
                      key={message.id || index}
                      className={`flex ${message.sender_type === 'admin' ? 'justify-end' : 'justify-start'}`}
                    >
                      <div
                        className={`max-w-[80%] rounded-lg p-3 ${
                          message.sender_type === 'admin'
                            ? 'bg-primary text-white'
                            : message.sender_type === 'user'
                            ? 'bg-gray-100 text-gray-900'
                            : 'bg-blue-50 text-gray-700'
                        }`}
                      >
                        <div className="flex items-center gap-2 mb-1">
                          <span className="text-xs font-medium">
                            {message.sender_type === 'admin'
                              ? 'Admin'
                              : message.sender_type === 'user'
                              ? 'User'
                              : 'System'}
                          </span>
                          <span className="text-xs opacity-75">
                            {formatDateTime(message.created_at)}
                          </span>
                        </div>
                        {message.body && (
                          <p className="text-sm">{message.body}</p>
                        )}
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>

          {/* Reply Form */}
          {isActive && (
            <Card>
              <CardHeader>
                <CardTitle>Reply</CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <Textarea
                  placeholder="Type your reply here..."
                  value={messageText}
                  onChange={(e) => setMessageText(e.target.value)}
                  rows={4}
                  disabled={submitting}
                />
                <div className="flex justify-end">
                  <Button
                    onClick={handleSendMessage}
                    disabled={!messageText.trim() || submitting}
                    className="gap-2"
                  >
                    <Send className="h-4 w-4" />
                    Send Reply
                  </Button>
                </div>
              </CardContent>
            </Card>
          )}
        </div>

        {/* Sidebar */}
        <div className="space-y-6">
          {/* Ticket Status Timeline */}
          <Card>
            <CardHeader>
              <CardTitle className="text-sm">Timeline</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3">
              <div className="flex items-start gap-3">
                <div className="w-2 h-2 rounded-full bg-primary mt-2"></div>
                <div>
                  <p className="text-sm font-medium">Created</p>
                  <p className="text-xs text-gray-600">{formatDateTime(ticket.created_at)}</p>
                </div>
              </div>
              {ticket.claimed_at && (
                <div className="flex items-start gap-3">
                  <div className="w-2 h-2 rounded-full bg-blue-500 mt-2"></div>
                  <div>
                    <p className="text-sm font-medium">Claimed</p>
                    <p className="text-xs text-gray-600">{formatDateTime(ticket.claimed_at)}</p>
                  </div>
                </div>
              )}
              {ticket.resolved_at && (
                <div className="flex items-start gap-3">
                  <div className="w-2 h-2 rounded-full bg-green-500 mt-2"></div>
                  <div>
                    <p className="text-sm font-medium">Resolved</p>
                    <p className="text-xs text-gray-600">{formatDateTime(ticket.resolved_at)}</p>
                  </div>
                </div>
              )}
              {ticket.closed_at && (
                <div className="flex items-start gap-3">
                  <div className="w-2 h-2 rounded-full bg-red-500 mt-2"></div>
                  <div>
                    <p className="text-sm font-medium">Closed</p>
                    <p className="text-xs text-gray-600">{formatDateTime(ticket.closed_at)}</p>
                  </div>
                </div>
              )}
            </CardContent>
          </Card>

          {/* Quick Actions */}
          {(isActive || canClose) && (
            <Card>
              <CardHeader>
                <CardTitle className="text-sm">Quick Actions</CardTitle>
              </CardHeader>
              <CardContent className="space-y-2">
                {canClaim && (
                  <Button
                    variant="primary"
                    size="sm"
                    onClick={handleClaim}
                    disabled={submitting}
                    className="w-full gap-2"
                  >
                    <UserCheck className="h-4 w-4" />
                    Claim Ticket
                  </Button>
                )}
                {canSetWaiting && (
                  <Button
                    variant="secondary"
                    size="sm"
                    onClick={handleSetWaiting}
                    disabled={submitting}
                    className="w-full gap-2"
                  >
                    <Pause className="h-4 w-4" />
                    Set Waiting for User
                  </Button>
                )}
                {canResolve && (
                  <Button
                    variant="secondary"
                    size="sm"
                    onClick={handleResolve}
                    disabled={submitting}
                    className="w-full gap-2"
                  >
                    <CheckCircle className="h-4 w-4" />
                    Mark Resolved
                  </Button>
                )}
                {canClose && (
                  <Button
                    variant="secondary"
                    size="sm"
                    onClick={handleClose}
                    disabled={submitting}
                    className="w-full gap-2"
                  >
                    <XCircle className="h-4 w-4" />
                    Close Ticket
                  </Button>
                )}
                {canEscalateToDispute && (
                  <Button
                    variant="danger"
                    size="sm"
                    onClick={() => setEscalationModalOpen(true)}
                    disabled={submitting}
                    className="w-full gap-2"
                  >
                    <Gavel className="h-4 w-4" />
                    Escalate to Dispute
                  </Button>
                )}
              </CardContent>
            </Card>
          )}

          {/* Priority & Category */}
          {ticket.status !== 'closed' && (
            <Card>
              <CardHeader>
                <CardTitle className="text-sm">Ticket Settings</CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <div>
                  <label className="block text-xs font-medium text-gray-600 mb-1">Priority</label>
                  <select
                    value={ticket.priority}
                    onChange={(e) => handleUpdatePriority(e.target.value as SupportPriority)}
                    disabled={submitting}
                    className="w-full rounded-lg border border-gray-300 px-3 py-1.5 text-sm focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary"
                  >
                    <option value="low">{supportPriorityLabels.low}</option>
                    <option value="medium">{supportPriorityLabels.medium}</option>
                    <option value="high">{supportPriorityLabels.high}</option>
                    <option value="urgent">{supportPriorityLabels.urgent}</option>
                  </select>
                </div>
                <div>
                  <label className="block text-xs font-medium text-gray-600 mb-1">Category</label>
                  <select
                    value={ticket.category}
                    onChange={(e) => handleUpdateCategory(e.target.value as SupportCategory)}
                    disabled={submitting}
                    className="w-full rounded-lg border border-gray-300 px-3 py-1.5 text-sm focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary"
                  >
                    <option value="order_issue">{supportCategoryLabels.order_issue}</option>
                    <option value="payment_issue">{supportCategoryLabels.payment_issue}</option>
                    <option value="account_issue">{supportCategoryLabels.account_issue}</option>
                    <option value="listing_issue">{supportCategoryLabels.listing_issue}</option>
                    <option value="other">{supportCategoryLabels.other}</option>
                  </select>
                </div>
              </CardContent>
            </Card>
          )}
        </div>
      </div>

      {/* Escalation Modal */}
      {escalationModalOpen && (
        <Modal
          isOpen={escalationModalOpen}
          onClose={() => setEscalationModalOpen(false)}
          title="Escalate to Dispute"
        >
          <div className="space-y-4">
            {/* STEP 6: Warning message */}
            <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded-lg flex items-start gap-2">
              <AlertCircle className="h-5 w-5 mt-0.5 flex-shrink-0" />
              <div>
                <p className="text-sm font-medium">⚠️ Warning</p>
                <p className="text-sm mt-1">This will escalate the case to finance team for decision. This action cannot be undone.</p>
              </div>
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                Reason *
              </label>
              <input
                type="text"
                value={escalationReason}
                onChange={(e) => setEscalationReason(e.target.value)}
                className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-primary"
                placeholder="e.g., Product not received, Item damaged"
                disabled={submitting}
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                Reason Code *
              </label>
              <input
                type="text"
                value={escalationReasonCode}
                onChange={(e) => setEscalationReasonCode(e.target.value)}
                className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-primary"
                placeholder="e.g., PRODUCT_NOT_RECEIVED"
                disabled={submitting}
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                Description
              </label>
              <Textarea
                value={escalationDescription}
                onChange={(e) => setEscalationDescription(e.target.value)}
                rows={4}
                placeholder="Additional details about the escalation..."
                disabled={submitting}
              />
            </div>

            <div className="flex justify-end gap-2 pt-4 border-t">
              <Button
                variant="secondary"
                onClick={() => setEscalationModalOpen(false)}
                disabled={submitting}
              >
                Cancel
              </Button>
              <Button
                variant="danger"
                onClick={handleEscalateToDispute}
                disabled={submitting || !escalationReason.trim() || !escalationReasonCode.trim()}
                className="gap-2"
              >
                <Gavel className="h-4 w-4" />
                Escalate to Dispute
              </Button>
            </div>
          </div>
        </Modal>
      )}
    </div>
  )
}
