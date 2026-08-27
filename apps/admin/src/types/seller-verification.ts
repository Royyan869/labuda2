export type SellerVerificationStatus =
  | 'not_submitted'
  | 'pending_review'
  | 'needs_resubmission'
  | 'approved'
  | 'rejected'
  | 'suspended'
  | 'revoked'
  | 'under_investigation'

export type DocumentReviewStatus = 'not_submitted' | 'pending' | 'approved' | 'rejected'

// KYC scope (owner decision): individual identity only — migration 000205.
// Business docs (NPWP/SIUP/NIB/business_other) removed.
export type DocumentType = 'identity_ktp' | 'identity_selfie'

export interface SellerVerificationListItem {
  id: string
  seller_id: string
  /** Public display handle from user_profiles. Absent when no profile row exists. */
  seller_username?: string
  seller_farm_name?: string | null
  status: SellerVerificationStatus
  submitted_at?: string
  created_at: string
  updated_at: string
}

export interface AdminVerificationDocument {
  id: string
  document_type: DocumentType
  document_name: string
  /** Short-lived presigned S3 GET URL (5 min TTL). Refresh via the view-url endpoint. */
  view_url: string
  status: DocumentReviewStatus
  rejection_note?: string
  submitted_at: string
  reviewed_at?: string
}

export interface BankAccountInfo {
  id: string
  bank_name: string
  bank_code: string
  account_number: string
  account_holder_name: string
  is_default: boolean
  /** True when this account was captured in reviewed_bank_account_ids at KYC approval time.
   *  False = post-approval addition; GUARD 5 blocks withdrawal until admin marks it reviewed. */
  is_reviewed_for_payout: boolean
}

export interface SellerVerificationDetail {
  seller_id: string
  seller_username?: string | null
  seller_farm_name?: string | null
  status: SellerVerificationStatus
  submitted_at?: string
  reviewed_at?: string
  reason?: string
  documents: AdminVerificationDocument[]
  bank_accounts: BankAccountInfo[]
}

export const verificationStatusLabels: Record<SellerVerificationStatus, string> = {
  not_submitted: 'Not Submitted',
  pending_review: 'Pending Review',
  needs_resubmission: 'Needs Resubmission',
  approved: 'Approved',
  rejected: 'Rejected',
  suspended: 'Suspended',
  revoked: 'Revoked',
  under_investigation: 'Under Investigation',
}

export const verificationStatusVariants: Record<
  SellerVerificationStatus,
  'default' | 'success' | 'warning' | 'error' | 'info'
> = {
  not_submitted: 'default',
  pending_review: 'warning',
  needs_resubmission: 'warning',
  approved: 'success',
  rejected: 'error',
  suspended: 'error',
  revoked: 'error',
  under_investigation: 'info',
}

export const documentTypeLabels: Record<DocumentType, string> = {
  identity_ktp: 'KTP (National ID)',
  identity_selfie: 'Selfie with ID',
}

export const documentReviewStatusVariants: Record<
  DocumentReviewStatus,
  'default' | 'success' | 'warning' | 'error'
> = {
  not_submitted: 'default',
  pending: 'warning',
  approved: 'success',
  rejected: 'error',
}
