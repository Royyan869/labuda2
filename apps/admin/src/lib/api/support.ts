import { api } from './client'
import type {
  EscalateToDisputeRequest,
  SendMessageRequest,
  SupportMessage,
  SupportTicketDetail,
  SupportTicketListItem,
  SupportTicketsQueryParams,
  UpdateCategoryRequest,
  UpdatePriorityRequest,
} from '@/types/support'

export interface SupportStats {
  total_tickets: number
  open_tickets: number
  in_progress_tickets: number
  waiting_user_tickets: number
  resolved_tickets: number
  closed_tickets: number
  unassigned_tickets: number
}

function buildSupportTicketQuery(params: SupportTicketsQueryParams = {}) {
  const queryParams = new URLSearchParams()
  if (params.status) queryParams.append('status', params.status)
  if (params.category) queryParams.append('category', params.category)
  if (params.is_overdue !== undefined) queryParams.append('is_overdue', params.is_overdue.toString())
  if (params.is_unassigned !== undefined) queryParams.append('is_unassigned', params.is_unassigned.toString())
  if (params.date_from) queryParams.append('date_from', params.date_from)
  if (params.date_to) queryParams.append('date_to', params.date_to)
  if (params.page) queryParams.append('page', params.page.toString())
  if (params.page_size) queryParams.append('page_size', params.page_size.toString())
  return queryParams.toString()
}

export async function listSupportTickets(params: SupportTicketsQueryParams = {}) {
  const query = buildSupportTicketQuery(params)
  const resp = await api.get<{
    data: {
      data: SupportTicketListItem[]
    }
  }>(`/api/v1/admin/support/tickets${query ? `?${query}` : ''}`)
  return { tickets: resp.data.data ?? [] }
}

export async function getSupportTicket(ticketId: string) {
  const resp = await api.get<{
    data: SupportTicketDetail
  }>(`/api/v1/admin/support/tickets/${encodeURIComponent(ticketId)}`)
  return resp.data
}

export async function getSupportTicketMessages(ticketId: string) {
  // Canonical backend envelope: { success, data: { data: SupportMessage[] } }.
  // The message array is nested one level inside the envelope's `data` key.
  const resp = await api.get<{
    data: {
      data: SupportMessage[]
    }
  }>(`/api/v1/admin/support/tickets/${encodeURIComponent(ticketId)}/messages`)
  return resp.data?.data ?? []
}

export async function claimSupportTicket(ticketId: string) {
  const resp = await api.put<{
    data: SupportTicketDetail
  }>(`/api/v1/admin/support/tickets/${encodeURIComponent(ticketId)}/claim`, {})
  return resp.data
}

export async function resolveSupportTicket(ticketId: string, notes?: string) {
  const resp = await api.put<{
    data: { ticket_id: string }
  }>(`/api/v1/admin/support/tickets/${encodeURIComponent(ticketId)}/resolve`, notes ? { notes } : {})
  return resp.data
}

export async function reopenSupportTicket(ticketId: string) {
  const resp = await api.put<{
    data: SupportTicketDetail
  }>(`/api/v1/admin/support/tickets/${encodeURIComponent(ticketId)}/reopen`, {})
  return resp.data
}

export async function closeSupportTicket(ticketId: string, reason?: string) {
  const resp = await api.put<{
    data: { ticket_id: string }
  }>(`/api/v1/admin/support/tickets/${encodeURIComponent(ticketId)}/close`, reason ? { reason } : {})
  return resp.data
}

export async function sendSupportTicketMessage(ticketId: string, data: SendMessageRequest) {
  const resp = await api.post<{
    data: { ticket_id: string; chat_room_id: string }
  }>(`/api/v1/admin/support/tickets/${encodeURIComponent(ticketId)}/messages`, data)
  return resp.data
}

export async function escalateSupportTicketToDispute(ticketId: string, data: EscalateToDisputeRequest) {
  const resp = await api.post<{
    data: SupportTicketDetail
  }>(`/api/v1/admin/support/tickets/${encodeURIComponent(ticketId)}/escalate-to-dispute`, data)
  return resp.data
}

export async function setSupportTicketWaitingForUser(ticketId: string) {
  const resp = await api.put<{
    data: SupportTicketDetail
  }>(`/api/v1/admin/support/tickets/${encodeURIComponent(ticketId)}/waiting`, {})
  return resp.data
}

export async function updateSupportTicketPriority(ticketId: string, data: UpdatePriorityRequest) {
  const resp = await api.put<{
    data: SupportTicketDetail
  }>(`/api/v1/admin/support/tickets/${encodeURIComponent(ticketId)}/priority`, data)
  return resp.data
}

export async function updateSupportTicketCategory(ticketId: string, data: UpdateCategoryRequest) {
  const resp = await api.put<{
    data: SupportTicketDetail
  }>(`/api/v1/admin/support/tickets/${encodeURIComponent(ticketId)}/category`, data)
  return resp.data
}

export async function getSupportStatistics() {
  const resp = await api.get<{
    data: SupportStats
  }>('/api/v1/admin/support/statistics')
  return resp.data
}
