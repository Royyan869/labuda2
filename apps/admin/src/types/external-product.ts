// External product review types — matches backend AdminExternalProductResponse shape.

export type ExternalProductReviewStatus =
  | 'draft'
  | 'pending_review'
  | 'approved'
  | 'rejected'
  | 'request_changes'
  | 'hidden'

export interface ExternalProductMedia {
  id: string
  external_product_id: string
  media_type: 'image' | 'video'
  storage_key: string
  url: string
  thumbnail_url?: string
  sort_order: number
  created_at: string
}

export interface ExternalProductReviewHistory {
  id: string
  external_product_id: string
  actor_admin_id?: string
  actor_user_id?: string
  from_status?: string
  to_status: string
  reason?: string
  created_at: string
}

export interface AdminExternalProduct {
  id: string
  owner_user_id: string
  title: string
  description?: string
  external_url: string
  normalized_external_url: string
  review_status: ExternalProductReviewStatus
  rejection_reason?: string
  unsafe_url_flag: boolean
  submitted_at?: string
  approved_at?: string
  rejected_at?: string
  hidden_at?: string
  last_reviewed_by?: string
  created_at: string
  updated_at: string
  media?: ExternalProductMedia[]
  can_edit: boolean
  can_submit: boolean
  can_resubmit: boolean
  public_visible: boolean
  // Admin-only fields
  review_history?: ExternalProductReviewHistory[]
  can_approve: boolean
  can_reject: boolean
  can_hide: boolean
}

export interface AdminExternalProductListResponse {
  items: AdminExternalProduct[]
  count: number
  page: number
  limit: number
}

export const externalProductStatusLabels: Record<ExternalProductReviewStatus, string> = {
  draft: 'Draft',
  pending_review: 'Pending Review',
  approved: 'Approved',
  rejected: 'Rejected',
  request_changes: 'Changes Requested',
  hidden: 'Hidden',
}

export const externalProductStatusVariants: Record<
  ExternalProductReviewStatus,
  'default' | 'pending' | 'success' | 'error' | 'warning' | 'info'
> = {
  draft: 'default',
  pending_review: 'pending',
  approved: 'success',
  rejected: 'error',
  request_changes: 'warning',
  hidden: 'info',
}
