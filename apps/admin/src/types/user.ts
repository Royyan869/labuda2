export type AccountStatus = 'active' | 'suspended' | 'banned'
export type UserRole = 'user' | 'seller' | 'admin'

export const ACCOUNT_STATUS = {
  ACTIVE: 'active',
  SUSPENDED: 'suspended',
  BANNED: 'banned',
} as const

export interface User {
  id: string
  email: string
  username: string
  phoneNumber?: string
  photoURL?: string

  isAdmin: boolean

  // Verification
  emailVerified: boolean
  phoneVerified: boolean
  kycVerified: boolean

  // Account status
  accountStatus: AccountStatus
  suspendedReason?: string
  suspendedUntil?: Date | string
  bannedReason?: string
  bannedAt?: Date | string

  // Profile
  bio?: string
  location?: string
  dateOfBirth?: Date | string

  // Seller info (if isSeller)
  farmName?: string
  farmDescription?: string
  farmSpecialties?: string[]
  farmEstablishedDate?: Date | string
  sellerRating?: number
  totalSales?: number
  totalOrdersSold?: number

  // Buyer info
  totalOrdersBought?: number
  totalSpent?: number

  // Seller Payable (earnings available for payout - SEPARATE from coins)
  sellerPayableBalance?: number
  frozenPayableBalance?: number
  totalBNR?: number
  bidReliability?: number
  bannedFromBidding?: boolean

  // Timestamps
  createdAt: Date | string
  updatedAt?: Date | string
  lastActiveAt?: Date | string

  // Provider info
  authProvider?: 'google' | 'phone' | 'email'
}

// Status labels
export const accountStatusLabels: Record<AccountStatus, string> = {
  active: 'Active',
  suspended: 'Suspended',
  banned: 'Banned',
}

// Status variants for badges
export const accountStatusVariants: Record<AccountStatus, 'success' | 'warning' | 'error'> = {
  active: 'success',
  suspended: 'warning',
  banned: 'error',
}

// ============================================================================
// ADMIN TYPES
// ============================================================================

/**
 * User list item - minimal data for list view
 */
export interface UserListItem {
  id: string
  username: string
  email: string
  photo_url?: string
  account_status: AccountStatus
  is_seller: boolean
  is_buyer: boolean
  is_admin: boolean
  warning_count?: number
  total_capabilities?: number
  created_at: string
  updated_at?: string
  last_active_at?: string
}

/**
 * User detail - full data for detail view
 */
export interface UserDetail extends UserListItem {
  phone_number?: string
  email_verified: boolean
  phone_verified: boolean
  kyc_verified: boolean

  // Account status details
  suspended_reason?: string
  suspended_until?: string
  banned_reason?: string
  banned_at?: string

  // Profile
  bio?: string
  location?: string
  date_of_birth?: string

  // Seller info
  farm_name?: string
  farm_description?: string
  farm_specialties?: string[]
  farm_established_date?: string
  seller_rating?: number
  total_sales?: number
  total_orders_sold?: number

  // Buyer info
  total_orders_bought?: number
  total_spent?: number

  // Financial
  seller_payable_balance?: number
  frozen_payable_balance?: number
  total_bnr?: number
  bid_reliability?: number
  banned_from_bidding?: boolean

  // Seller authority (Batch 52: canonical seller state from backend)
  subscription_status?: 'active' | 'expired' | 'inactive'
  verification_status?: 'not_submitted' | 'pending_review' | 'needs_resubmission' | 'approved' | 'rejected' | 'suspended' | 'revoked' | 'under_investigation'
  seller_payable?: number // Canonical payable from finance ledger (SELLER_PAYABLE)
  seller_tier?: 'basic' | 'pro' | 'elite' // From seller_reputation_state; nil if no row yet
  // Set when a settled subscription payment has no seller_subscriptions row (webhook miss).
  // Non-null signals the UI to show the "Recover Subscription" action.
  recoverable_subscription_payment_id?: string

  // Provider info
  auth_provider?: 'google' | 'phone' | 'email'

  // Role and capabilities (from backend Phase 4)
  role?: UserRole
  capabilities?: string[]

  // Warning counts (governance visibility, read-only)
  warning_count: number
  active_warning_count: number
  severe_warning_count: number
}

/**
 * Query parameters for users list
 */
export interface UsersQueryParams {
  status?: AccountStatus | ''
  role?: 'buyer' | 'seller' | 'admin' | ''
  is_verified?: 'true' | 'false' | ''
  search?: string
  page?: number
  page_size?: number
}

/**
 * Response from users list endpoint
 */
export interface PaginatedUsersResponse {
  users: UserListItem[]
  _meta?: {
    page: number
    per_page: number
    total: number
    total_pages: number
  }
}

/**
 * Response from user action endpoints
 */
export interface UserActionResponse {
  success: boolean
  message: string
  user: UserDetail
}
