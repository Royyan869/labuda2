import { useState, useEffect, useCallback, useMemo } from 'react'
import {
  claimSupportTicket,
  closeSupportTicket,
  escalateSupportTicketToDispute,
  getSupportTicket,
  getSupportTicketMessages,
  listSupportTickets,
  resolveSupportTicket,
  sendSupportTicketMessage,
  setSupportTicketWaitingForUser,
  updateSupportTicketCategory,
  updateSupportTicketPriority,
} from '@/lib/api'
import type {
  SupportTicketListItem,
  SupportTicketDetail,
  SupportMessage,
  SupportTicketsQueryParams,
  SendMessageRequest,
  EscalateToDisputeRequest,
  UpdatePriorityRequest,
  UpdateCategoryRequest,
} from '@/types/support'

/**
 * Hook for fetching support tickets list
 */
export function useSupportTickets(params: SupportTicketsQueryParams = {}) {
  const [tickets, setTickets] = useState<SupportTicketListItem[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<Error | null>(null)
  const [total, setTotal] = useState(0)

  const queryParams = useMemo(
    () => ({
      status: params.status,
      category: params.category,
      is_overdue: params.is_overdue,
      is_unassigned: params.is_unassigned,
      date_from: params.date_from,
      date_to: params.date_to,
    }),
    [
      params.status,
      params.category,
      params.is_overdue,
      params.is_unassigned,
      params.date_from,
      params.date_to,
    ]
  )

  const fetchTickets = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const response = await listSupportTickets(queryParams)
      setTickets(response.tickets || [])
      setTotal(response.tickets?.length || 0)
    } catch (err) {
      setError(err instanceof Error ? err : new Error('Failed to fetch support tickets'))
    } finally {
      setLoading(false)
    }
  }, [queryParams])

  useEffect(() => {
    fetchTickets()
  }, [fetchTickets])

  // Auto-refresh every 30s for operational awareness
  useEffect(() => {
    const interval = setInterval(() => {
      fetchTickets()
    }, 30_000)
    return () => clearInterval(interval)
  }, [fetchTickets])

  return {
    tickets,
    loading,
    error,
    total,
    refetch: fetchTickets,
  }
}

/**
 * Hook for fetching a single support ticket detail
 */
export function useSupportTicketDetail(ticketId: string | null) {
  const [ticket, setTicket] = useState<SupportTicketDetail | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<Error | null>(null)

  const fetchTicket = useCallback(async () => {
    if (!ticketId) return

    setLoading(true)
    setError(null)
    try {
      const response = await getSupportTicket(ticketId)
      setTicket(response)
    } catch (err) {
      setError(err instanceof Error ? err : new Error('Failed to fetch ticket detail'))
    } finally {
      setLoading(false)
    }
  }, [ticketId])

  useEffect(() => {
    fetchTicket()
  }, [fetchTicket])

  return {
    ticket,
    loading,
    error,
    refetch: fetchTicket,
  }
}

/**
 * Hook for fetching support ticket messages
 */
export function useSupportMessages(ticketId: string | null) {
  const [messages, setMessages] = useState<SupportMessage[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<Error | null>(null)

  const fetchMessages = useCallback(async () => {
    if (!ticketId) return

    setLoading(true)
    setError(null)
    try {
      const response = await getSupportTicketMessages(ticketId)
      setMessages(response || [])
    } catch (err) {
      setError(err instanceof Error ? err : new Error('Failed to fetch messages'))
    } finally {
      setLoading(false)
    }
  }, [ticketId])

  useEffect(() => {
    fetchMessages()
  }, [fetchMessages])

  return {
    messages,
    loading,
    error,
    refetch: fetchMessages,
  }
}

/**
 * Hook for support ticket actions (resolve, close, send message)
 */
export function useSupportTicketActions(ticketId: string) {
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<Error | null>(null)

  const resolveTicket = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      await resolveSupportTicket(ticketId)
      return { success: true }
    } catch (err) {
      const error = err instanceof Error ? err : new Error('Failed to resolve ticket')
      setError(error)
      return { success: false, error }
    } finally {
      setLoading(false)
    }
  }, [ticketId])

  const closeTicket = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      await closeSupportTicket(ticketId)
      return { success: true }
    } catch (err) {
      const error = err instanceof Error ? err : new Error('Failed to close ticket')
      setError(error)
      return { success: false, error }
    } finally {
      setLoading(false)
    }
  }, [ticketId])

  const sendMessage = useCallback(async (data: SendMessageRequest) => {
    setLoading(true)
    setError(null)
    try {
      await sendSupportTicketMessage(ticketId, data)
      return { success: true }
    } catch (err) {
      const error = err instanceof Error ? err : new Error('Failed to send message')
      setError(error)
      return { success: false, error }
    } finally {
      setLoading(false)
    }
  }, [ticketId])

  const escalateToDispute = useCallback(async (data: EscalateToDisputeRequest) => {
    setLoading(true)
    setError(null)
    try {
      await escalateSupportTicketToDispute(ticketId, data)
      return { success: true }
    } catch (err) {
      const error = err instanceof Error ? err : new Error('Failed to escalate ticket to dispute')
      setError(error)
      return { success: false, error }
    } finally {
      setLoading(false)
    }
  }, [ticketId])

  const claimTicket = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      await claimSupportTicket(ticketId)
      return { success: true }
    } catch (err) {
      const error = err instanceof Error ? err : new Error('Failed to claim ticket')
      setError(error)
      return { success: false, error }
    } finally {
      setLoading(false)
    }
  }, [ticketId])

  const setWaitingForUser = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      await setSupportTicketWaitingForUser(ticketId)
      return { success: true }
    } catch (err) {
      const error = err instanceof Error ? err : new Error('Failed to set waiting for user')
      setError(error)
      return { success: false, error }
    } finally {
      setLoading(false)
    }
  }, [ticketId])

  const updatePriority = useCallback(async (data: UpdatePriorityRequest) => {
    setLoading(true)
    setError(null)
    try {
      await updateSupportTicketPriority(ticketId, data)
      return { success: true }
    } catch (err) {
      const error = err instanceof Error ? err : new Error('Failed to update priority')
      setError(error)
      return { success: false, error }
    } finally {
      setLoading(false)
    }
  }, [ticketId])

  const updateCategory = useCallback(async (data: UpdateCategoryRequest) => {
    setLoading(true)
    setError(null)
    try {
      await updateSupportTicketCategory(ticketId, data)
      return { success: true }
    } catch (err) {
      const error = err instanceof Error ? err : new Error('Failed to update category')
      setError(error)
      return { success: false, error }
    } finally {
      setLoading(false)
    }
  }, [ticketId])

  return {
    resolveTicket,
    closeTicket,
    sendMessage,
    escalateToDispute,
    claimTicket,
    setWaitingForUser,
    updatePriority,
    updateCategory,
    loading,
    error,
  }
}
