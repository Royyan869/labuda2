import { useState, useEffect, useRef, useCallback } from 'react'
import {
  User,
  Mail,
  Phone,
  Calendar,
  Shield,
  AlertTriangle,
  RefreshCw,
  CheckCircle,
  XCircle,
  Ban,
  AlertOctagon,
  UserCheck,
  Store,
  ShoppingBag,
  Key,
  ShieldBan,
  UserCog,
  Activity,
  RotateCcw,
} from 'lucide-react'
import { Modal, ModalFooter } from '@/components/ui/Modal'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { useUserDetail, useUserActions } from '@/hooks/useUsers'
import { formatDate, formatRupiah } from '@/lib/utils'
import { hasCapability } from '@/lib/permissions'
import { useAuth } from '@/hooks/useAuth'
import { api, resetBNRByUser, recoverSellerSubscription } from '@/lib/api'
import type {
  UserListItem,
  UserRole,
  AccountStatus,
} from '@/types'
import {
  accountStatusLabels,
  accountStatusVariants,
} from '@/types'

interface UserDetailModalProps {
  isOpen: boolean
  onClose: () => void
  userData: UserListItem | null
  onSuccess?: () => void
}

type ActionState =
  | 'idle'
  | 'confirm-suspend'
  | 'confirm-activate'
  | 'confirm-ban'
  | 'confirm-unban'
  | 'confirm-role-change'
  | 'confirm-bnr-reset'
  | 'confirm-subscription-recover'

const ROLE_OPTIONS: { value: UserRole; label: string }[] = [
  { value: 'user', label: 'User' },
  { value: 'seller', label: 'Seller' },
  { value: 'admin', label: 'Admin' },
]

// Action confirmation configurations
const ACTION_CONFIRMATIONS = {
  'confirm-suspend': {
    title: 'Suspend User Account',
    icon: AlertTriangle,
    iconColor: 'text-amber-600',
    bgColor: 'bg-amber-50',
    borderColor: 'border-amber-200',
    message: (
      <>
        <p className="font-semibold text-lg mb-2">You are about to suspend this user account.</p>
        <p className="text-sm">The user will be temporarily restricted from accessing the platform. You may optionally specify when the suspension ends.</p>
      </>
    ),
  },
  'confirm-activate': {
    title: 'Activate User Account',
    icon: UserCheck,
    iconColor: 'text-green-600',
    bgColor: 'bg-green-50',
    borderColor: 'border-green-200',
    message: (
      <>
        <p className="font-semibold text-lg mb-2">You are about to activate this user account.</p>
        <p className="text-sm">The user will regain full access to the platform. Any suspension will be removed.</p>
      </>
    ),
  },
  'confirm-ban': {
    title: 'Ban User Account',
    icon: Ban,
    iconColor: 'text-red-600',
    bgColor: 'bg-red-50',
    borderColor: 'border-red-200',
    message: (
      <>
        <p className="font-semibold text-lg mb-2">You are about to PERMANENTLY BAN this user.</p>
        <p className="text-sm text-red-700 font-medium">This is a severe action. Use unban to reverse if needed.</p>
        <p className="text-sm mt-2">The user will be blocked from accessing the platform.</p>
      </>
    ),
  },
  'confirm-unban': {
    title: 'Unban User Account',
    icon: UserCheck,
    iconColor: 'text-emerald-600',
    bgColor: 'bg-emerald-50',
    borderColor: 'border-emerald-200',
    message: (
      <>
        <p className="font-semibold text-lg mb-2">You are about to unban this user account.</p>
        <p className="text-sm">The user will be restored to active status and regain access to the platform.</p>
      </>
    ),
  },
  'confirm-role-change': {
    title: 'Change User Role',
    icon: UserCog,
    iconColor: 'text-blue-600',
    bgColor: 'bg-blue-50',
    borderColor: 'border-blue-200',
    message: (
      <>
        <p className="font-semibold text-lg mb-2">You are about to change this user&apos;s role.</p>
        <p className="text-sm">This will change the user&apos;s permissions and access level on the platform.</p>
      </>
    ),
  },
  'confirm-bnr-reset': {
    title: 'Reset BNR Strikes',
    icon: Activity,
    iconColor: 'text-orange-600',
    bgColor: 'bg-orange-50',
    borderColor: 'border-orange-200',
    message: (
      <>
        <p className="font-semibold text-lg mb-2">You are about to reset all active BNR strikes for this buyer.</p>
        <p className="text-sm">This will clear active BNR restrictions. Audit trail remains.</p>
      </>
    ),
  },
  'confirm-subscription-recover': {
    title: 'Recover Subscription',
    icon: RotateCcw,
    iconColor: 'text-blue-600',
    bgColor: 'bg-blue-50',
    borderColor: 'border-blue-200',
    message: (
      <>
        <p className="font-semibold text-lg mb-2">You are about to recover a missed subscription payment.</p>
        <p className="text-sm">This calls the canonical activation pipeline (idempotent). If the subscription already exists this is a no-op.</p>
      </>
    ),
  },
} as const

export function UserDetailModal({ isOpen, onClose, userData, onSuccess }: UserDetailModalProps) {
  const [actionState, setActionState] = useState<ActionState>('idle')
  const [error, setError] = useState<string | null>(null)
  const [reason, setReason] = useState('')
  const [suspensionEndDate, setSuspensionEndDate] = useState('')
  const [isDataStale, setIsDataStale] = useState(false)
  const lastKnownStatus = useRef<AccountStatus | null>(null)

  // Role change state
  const [selectedRole, setSelectedRole] = useState<UserRole>('user')

  // Block list state
  const [blockList, setBlockList] = useState<string[] | null>(null)
  const [blockListLoading, setBlockListLoading] = useState(false)
  const [blockListExpanded, setBlockListExpanded] = useState(false)

  const { user, loading, refetch } = useUserDetail(userData?.id || null)
  const { suspend, activate, ban, unban, setRole, loading: actionLoading } = useUserActions(userData?.id || null)
  const { capabilities } = useAuth()

  const canBanUsers = hasCapability(capabilities, 'governance.user.ban')
  const canSuspendUsers = hasCapability(capabilities, 'governance.user.suspend')
  const canActivateUsers = hasCapability(capabilities, 'governance.user.activate')
  const canUnbanUsers = hasCapability(capabilities, 'governance.user.unban')
  const canAssignRoles = hasCapability(capabilities, 'governance.role.assign')
  const canReadUsers = hasCapability(capabilities, 'governance.user.read')
  const canResetBNR = hasCapability(capabilities, 'governance.bnr.reset')
  const canRecoverSubscription = hasCapability(capabilities, 'seller.subscription.recover')

  const [bnrResetting, setBnrResetting] = useState(false)
  const [subscriptionRecovering, setSubscriptionRecovering] = useState(false)

  // Fetch block list on demand
  const fetchBlockList = useCallback(async () => {
    if (!userData?.id || !canReadUsers) return
    setBlockListLoading(true)
    try {
      const resp = await api.get<{ blocked: string[]; limit: number }>(
        `/api/v1/admin/users/${userData.id}/blocks?limit=50`
      )
      setBlockList(resp.blocked || [])
    } catch {
      setBlockList([])
    } finally {
      setBlockListLoading(false)
    }
  }, [userData?.id, canReadUsers])

  // Track status changes for staleness detection
  useEffect(() => {
    if (user) {
      if (lastKnownStatus.current && lastKnownStatus.current !== user.account_status) {
        setIsDataStale(true)
      }
      lastKnownStatus.current = user.account_status
    } else if (userData) {
      lastKnownStatus.current = userData.account_status
    }
  }, [user, userData])

  // Reset state when modal closes
  useEffect(() => {
    if (!isOpen) {
      setActionState('idle')
      setError(null)
      setReason('')
      setSuspensionEndDate('')
      setSelectedRole('user')
      setIsDataStale(false)
      setBlockList(null)
      setBlockListExpanded(false)
    }
  }, [isOpen])

  // Refresh data before showing action confirmation
  const prepareAction = async (action: 'suspend' | 'activate' | 'ban' | 'unban') => {
    setError(null)
    setIsDataStale(false)
    await refetch()
    setActionState(`confirm-${action}`)
  }

  const prepareBNRReset = async () => {
    setError(null)
    await refetch()
    setActionState('confirm-bnr-reset')
  }

  const handleConfirmBNRReset = async () => {
    if (!userData?.id) return
    setError(null)
    setBnrResetting(true)
    try {
      await resetBNRByUser(userData.id)
      await refetch()
      setActionState('idle')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to reset BNR strikes')
    } finally {
      setBnrResetting(false)
    }
  }

  const prepareSubscriptionRecover = async () => {
    setError(null)
    await refetch()
    setActionState('confirm-subscription-recover')
  }

  const handleConfirmSubscriptionRecover = async () => {
    if (!user?.recoverable_subscription_payment_id) return
    setError(null)
    setSubscriptionRecovering(true)
    try {
      await recoverSellerSubscription(user.recoverable_subscription_payment_id)
      await refetch()
      setActionState('idle')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to recover subscription')
    } finally {
      setSubscriptionRecovering(false)
    }
  }

  const prepareRoleChange = async () => {
    setError(null)
    setIsDataStale(false)
    await refetch()
    // Pre-select current role
    if (user?.role) {
      setSelectedRole(user.role)
    }
    setActionState('confirm-role-change')
  }

  const handleConfirmSuspend = async () => {
    if (!reason.trim()) {
      setError('Please provide a reason for suspension')
      return
    }
    setError(null)
    const result = await suspend(reason.trim(), suspensionEndDate || undefined)
    if (result) {
      onSuccess?.()
      onClose()
    }
  }

  const handleConfirmActivate = async () => {
    setError(null)
    const result = await activate()
    if (result) {
      onSuccess?.()
      onClose()
    }
  }

  const handleConfirmBan = async () => {
    if (!reason.trim()) {
      setError('Please provide a reason for banning')
      return
    }
    setError(null)
    const result = await ban(reason.trim())
    if (result) {
      onSuccess?.()
      onClose()
    }
  }

  const handleConfirmUnban = async () => {
    if (!reason.trim()) {
      setError('Please provide a reason for unbanning')
      return
    }
    setError(null)
    const result = await unban(reason.trim())
    if (result) {
      onSuccess?.()
      onClose()
    }
  }

  const handleConfirmRoleChange = async () => {
    if (!selectedRole) {
      setError('Please select a role')
      return
    }
    if (user?.role === selectedRole) {
      setError('Selected role is the same as current role')
      return
    }
    setError(null)
    const result = await setRole(selectedRole)
    if (result) {
      onSuccess?.()
      onClose()
    }
  }

  const handleCancelConfirm = () => {
    setActionState('idle')
    setReason('')
    setSuspensionEndDate('')
    setSelectedRole('user')
    setError(null)
  }

  if (!userData) return null

  const displayData = user || userData
  const primaryIdentity = displayData.username ? `@${displayData.username}` : `@${displayData.id.slice(0, 8)}`
  const isBanned = displayData.account_status === 'banned'
  const isSuspended = displayData.account_status === 'suspended'
  const isActive = displayData.account_status === 'active'
  const confirmConfig = actionState !== 'idle' ? ACTION_CONFIRMATIONS[actionState] : undefined
  const isSubmitting = actionLoading
    || (actionState === 'confirm-bnr-reset' && bnrResetting)
    || (actionState === 'confirm-subscription-recover' && subscriptionRecovering)

  // Determine which confirm handler to call
  const confirmHandlers: Record<string, () => Promise<void>> = {
    'confirm-suspend': handleConfirmSuspend,
    'confirm-activate': handleConfirmActivate,
    'confirm-ban': handleConfirmBan,
    'confirm-unban': handleConfirmUnban,
    'confirm-role-change': handleConfirmRoleChange,
    'confirm-bnr-reset': handleConfirmBNRReset,
    'confirm-subscription-recover': handleConfirmSubscriptionRecover,
  }

  // States that require reason input
  const requiresReason = actionState === 'confirm-suspend' || actionState === 'confirm-ban' || actionState === 'confirm-unban'

  return (
    <Modal isOpen={isOpen} onClose={onClose} title="User Details" size="xl">
      {loading ? (
        <div className="flex items-center justify-center py-12">
          <div className="inline-block h-8 w-8 animate-spin rounded-full border-4 border-solid border-primary border-r-transparent" />
        </div>
      ) : (
        <div className="space-y-6">
          {/* Action confirmation state */}
          {confirmConfig ? (
            <div className={`p-4 rounded-lg border ${confirmConfig.bgColor} ${confirmConfig.borderColor}`}>
              <div className="flex items-start gap-3">
                <confirmConfig.icon className={`h-6 w-6 flex-shrink-0 mt-0.5 ${confirmConfig.iconColor}`} />
                <div className="flex-1">
                  <h3 className={`font-semibold ${confirmConfig.iconColor}`}>{confirmConfig.title}</h3>
                  <div className="mt-2 text-gray-700">
                    {confirmConfig.message}
                  </div>

                  {/* User being affected */}
                  <div className="mt-4 p-3 bg-white rounded border border-gray-200">
                    <p className="text-sm font-medium">{primaryIdentity}</p>
                    <p className="text-xs text-gray-500 font-mono">{displayData.email}</p>
                  </div>

                  {/* Reason input for suspend/ban/unban */}
                  {requiresReason && (
                    <div className="mt-4">
                      <label htmlFor="action-reason" className="block text-sm font-medium text-gray-700 mb-1">
                        Reason <span className="text-red-600">*</span>
                      </label>
                      <textarea
                        id="action-reason"
                        value={reason}
                        onChange={(e) => setReason(e.target.value)}
                        placeholder="Provide a clear reason for this action..."
                        rows={3}
                        className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
                        autoFocus
                      />
                    </div>
                  )}

                  {/* Optional suspension end date */}
                  {actionState === 'confirm-suspend' && (
                    <div className="mt-4">
                      <label htmlFor="suspension-until" className="block text-sm font-medium text-gray-700 mb-1">
                        Suspension End Date <span className="text-gray-500">(optional)</span>
                      </label>
                      <input
                        type="datetime-local"
                        id="suspension-until"
                        value={suspensionEndDate}
                        onChange={(e) => setSuspensionEndDate(e.target.value)}
                        className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-amber-500 focus:border-amber-500"
                      />
                      <p className="text-xs text-gray-500 mt-1">Leave empty for indefinite suspension</p>
                    </div>
                  )}

                  {/* Role selector for role change */}
                  {actionState === 'confirm-role-change' && (
                    <div className="mt-4">
                      <label htmlFor="role-select" className="block text-sm font-medium text-gray-700 mb-1">
                        New Role <span className="text-red-600">*</span>
                      </label>
                      <select
                        id="role-select"
                        value={selectedRole}
                        onChange={(e) => setSelectedRole(e.target.value as UserRole)}
                        className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
                      >
                        {ROLE_OPTIONS.map((opt) => (
                          <option key={opt.value} value={opt.value}>
                            {opt.label}{user?.role === opt.value ? ' (current)' : ''}
                          </option>
                        ))}
                      </select>
                      {user?.role === selectedRole && (
                        <p className="text-xs text-amber-600 mt-1">This is already the current role</p>
                      )}
                    </div>
                  )}

                  {/* Error inline */}
                  {error && (
                    <div className="mt-3 bg-red-50 border border-red-200 text-red-700 p-2 rounded text-sm flex items-center gap-2">
                      <AlertOctagon className="h-4 w-4 flex-shrink-0" />
                      {error}
                    </div>
                  )}

                  {/* Action confirmation buttons */}
                  <div className="mt-4 flex items-center gap-3">
                    <Button
                      variant="secondary"
                      onClick={handleCancelConfirm}
                      disabled={isSubmitting}
                    >
                      Cancel
                    </Button>
                    {actionState === 'confirm-suspend' && (
                      <Button
                        variant="warning"
                        onClick={handleConfirmSuspend}
                        isLoading={isSubmitting}
                        disabled={isSubmitting || !reason.trim()}
                      >
                        Confirm Suspension
                      </Button>
                    )}
                    {actionState === 'confirm-activate' && (
                      <Button
                        variant="success"
                        onClick={handleConfirmActivate}
                        isLoading={isSubmitting}
                        disabled={isSubmitting}
                      >
                        Confirm Activation
                      </Button>
                    )}
                    {actionState === 'confirm-ban' && (
                      <Button
                        variant="danger"
                        onClick={handleConfirmBan}
                        isLoading={isSubmitting}
                        disabled={isSubmitting || !reason.trim()}
                      >
                        PERMANENTLY BAN USER
                      </Button>
                    )}
                    {actionState === 'confirm-unban' && (
                      <Button
                        variant="success"
                        onClick={handleConfirmUnban}
                        isLoading={isSubmitting}
                        disabled={isSubmitting || !reason.trim()}
                      >
                        Confirm Unban
                      </Button>
                    )}
                    {actionState === 'confirm-role-change' && (
                      <Button
                        variant="primary"
                        onClick={confirmHandlers['confirm-role-change']}
                        isLoading={isSubmitting}
                        disabled={isSubmitting || user?.role === selectedRole}
                      >
                        Confirm Role Change
                      </Button>
                    )}
                    {actionState === 'confirm-bnr-reset' && (
                      <Button
                        variant="warning"
                        onClick={handleConfirmBNRReset}
                        isLoading={isSubmitting}
                        disabled={isSubmitting}
                      >
                        <RotateCcw className="h-4 w-4 mr-2" />
                        Reset BNR Strikes
                      </Button>
                    )}
                    {actionState === 'confirm-subscription-recover' && (
                      <Button
                        variant="secondary"
                        onClick={handleConfirmSubscriptionRecover}
                        isLoading={isSubmitting}
                        disabled={isSubmitting}
                      >
                        <RotateCcw className="h-4 w-4 mr-2" />
                        Recover Subscription
                      </Button>
                    )}
                  </div>
                </div>
              </div>
            </div>
          ) : (
            <>
              {/* Stale data warning */}
              {isDataStale && (
                <div className="bg-amber-50 border border-amber-200 text-amber-800 p-3 rounded-lg flex items-center gap-2">
                  <AlertTriangle className="h-4 w-4 flex-shrink-0" />
                  <span className="text-sm">This user&apos;s status has changed. Refresh to see the latest data.</span>
                  <Button
                    size="sm"
                    variant="secondary"
                    onClick={() => {
                      refetch()
                      setIsDataStale(false)
                    }}
                    className="ml-auto"
                  >
                    <RefreshCw className="h-3 w-3 mr-1" />
                    Refresh
                  </Button>
                </div>
              )}

              {/* Error Message */}
              {error && (
                <div className="bg-red-50 border border-red-200 text-red-700 p-3 rounded-lg flex items-center gap-2">
                  <AlertOctagon className="h-4 w-4 flex-shrink-0" />
                  <span className="text-sm">{error}</span>
                </div>
              )}

              {/* Header with Status Badges */}
              <div className="flex items-center justify-between flex-wrap gap-3">
                <div className="flex items-center gap-3 flex-wrap">
                  <Badge variant={accountStatusVariants[displayData.account_status] || 'info'}>
                    {accountStatusLabels[displayData.account_status] || displayData.account_status}
                  </Badge>
                  {user != null && user.active_warning_count > 0 && (
                    <Badge variant="warning">
                      {user.active_warning_count} Active Warning{user.active_warning_count > 1 ? 's' : ''}
                    </Badge>
                  )}
                  {user != null && user.severe_warning_count > 0 && (
                    <Badge variant="error">
                      {user.severe_warning_count} Severe
                    </Badge>
                  )}
                  {displayData.is_admin && (
                    <Badge variant="info">Admin</Badge>
                  )}
                  {displayData.is_seller && (
                    <Badge variant="default">Seller</Badge>
                  )}
                  {displayData.is_buyer && (
                    <Badge variant="default">Buyer</Badge>
                  )}
                  {user?.role && (
                    <Badge variant="info">Role: {user.role}</Badge>
                  )}
                </div>
                <div className="flex items-center gap-3">
                  <span className="text-sm text-gray-500">
                    User ID: <span className="font-mono">{displayData.id}</span>
                  </span>
                  <button
                    onClick={() => {
                      setError(null)
                      setIsDataStale(false)
                      refetch()
                    }}
                    className="text-gray-400 hover:text-gray-600 transition-colors"
                    title="Refresh user data"
                  >
                    <RefreshCw className="h-4 w-4" />
                  </button>
                </div>
              </div>

              {/* User Profile Card */}
              <Card>
                <CardHeader>
                  <CardTitle className="text-lg flex items-center gap-2">
                    <User className="h-5 w-5" />
                    Profile Information
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="flex items-start gap-4">
                    {displayData.photo_url ? (
                      <img
                        src={displayData.photo_url}
                        alt=""
                        className="w-16 h-16 rounded-full object-cover"
                      />
                    ) : (
                      <div className="w-16 h-16 rounded-full bg-gray-200 flex items-center justify-center">
                        <User className="h-8 w-8 text-gray-500" />
                      </div>
                    )}
                    <div className="flex-1 space-y-3">
                      <div>
                        <p className="font-medium text-lg">{primaryIdentity}</p>
                      </div>
                      <div className="flex items-center gap-2 text-sm">
                        <Mail className="h-4 w-4 text-gray-500" />
                        <span className="text-gray-600">{displayData.email}</span>
                        {user?.email_verified !== undefined && (
                          user.email_verified ? (
                            <CheckCircle className="h-4 w-4 text-green-500" aria-label="Email verified" />
                          ) : (
                            <XCircle className="h-4 w-4 text-red-500" aria-label="Email not verified" />
                          )
                        )}
                      </div>
                      {user?.phone_number && (
                        <div className="flex items-center gap-2 text-sm">
                          <Phone className="h-4 w-4 text-gray-500" />
                          <span className="text-gray-600">{user.phone_number}</span>
                          {user.phone_verified ? (
                            <CheckCircle className="h-4 w-4 text-green-500" aria-label="Phone verified" />
                          ) : (
                            <XCircle className="h-4 w-4 text-red-500" aria-label="Phone not verified" />
                          )}
                        </div>
                      )}
                      {user?.kyc_verified !== undefined && (
                        <div className="flex items-center gap-2 text-sm">
                          <Shield className="h-4 w-4 text-gray-500" />
                          <span className="text-gray-600">KYC Status:</span>
                          {user.kyc_verified ? (
                            <Badge variant="success">Verified</Badge>
                          ) : (
                            <Badge variant="warning">Not Verified</Badge>
                          )}
                        </div>
                      )}
                    </div>
                  </div>

                  {/* Additional Profile Info */}
                  {(user?.bio || user?.location || user?.date_of_birth) && (
                    <div className="mt-4 pt-4 border-t border-gray-100 grid grid-cols-2 gap-4 text-sm">
                      {user?.bio && (
                        <div>
                          <p className="text-gray-600">Bio</p>
                          <p className="text-gray-900">{user.bio}</p>
                        </div>
                      )}
                      {user?.location && (
                        <div>
                          <p className="text-gray-600">Location</p>
                          <p className="text-gray-900">{user.location}</p>
                        </div>
                      )}
                    </div>
                  )}
                </CardContent>
              </Card>

              {/* Capabilities (read-only) */}
              {user?.capabilities && user.capabilities.length > 0 && (
                <Card>
                  <CardHeader>
                    <CardTitle className="text-lg flex items-center gap-2">
                      <Key className="h-5 w-5" />
                      Capabilities
                      <span className="text-sm font-normal text-gray-500">({user.capabilities.length})</span>
                    </CardTitle>
                  </CardHeader>
                  <CardContent>
                    <div className="flex flex-wrap gap-2">
                      {user.capabilities.map((cap) => (
                        <Badge key={cap} variant="default">
                          {cap}
                        </Badge>
                      ))}
                    </div>
                  </CardContent>
                </Card>
              )}

              {/* Warning Summary (read-only governance visibility) */}
              {user != null && (user.warning_count > 0 || user.active_warning_count > 0) && (
                <Card className={user.severe_warning_count > 0 ? 'border-red-200' : 'border-amber-200'}>
                  <CardHeader>
                    <CardTitle className="text-lg flex items-center gap-2">
                      <AlertOctagon className={`h-5 w-5 ${user.severe_warning_count > 0 ? 'text-red-500' : 'text-amber-500'}`} />
                      Warnings
                    </CardTitle>
                  </CardHeader>
                  <CardContent>
                    <div className="grid grid-cols-3 gap-4 text-sm">
                      <div>
                        <p className="text-gray-600">Total Issued</p>
                        <p className="font-medium">{user.warning_count}</p>
                      </div>
                      <div>
                        <p className="text-gray-600">Currently Active</p>
                        <p className={`font-medium ${user.active_warning_count > 0 ? 'text-amber-700' : ''}`}>
                          {user.active_warning_count}
                        </p>
                      </div>
                      <div>
                        <p className="text-gray-600">Severe (active)</p>
                        <p className={`font-medium ${user.severe_warning_count > 0 ? 'text-red-600' : ''}`}>
                          {user.severe_warning_count}
                        </p>
                      </div>
                    </div>
                  </CardContent>
                </Card>
              )}

              {/* Block List */}
              {canReadUsers && (
                <Card>
                  <CardHeader>
                    <CardTitle className="text-lg flex items-center gap-2">
                      <ShieldBan className="h-5 w-5" />
                      Block List
                      <button
                        onClick={() => {
                          if (!blockListExpanded) {
                            setBlockListExpanded(true)
                            fetchBlockList()
                          } else {
                            setBlockListExpanded(false)
                          }
                        }}
                        className="text-sm font-normal text-blue-600 hover:text-blue-800 ml-auto"
                      >
                        {blockListExpanded ? 'Hide' : 'Show'}
                      </button>
                    </CardTitle>
                  </CardHeader>
                  {blockListExpanded && (
                    <CardContent>
                      {blockListLoading ? (
                        <div className="flex items-center gap-2 text-sm text-gray-500">
                          <div className="inline-block h-4 w-4 animate-spin rounded-full border-2 border-solid border-primary border-r-transparent" />
                          Loading...
                        </div>
                      ) : blockList && blockList.length > 0 ? (
                        <div className="space-y-1">
                          {blockList.map((blockedId) => (
                            <div key={blockedId} className="text-sm font-mono text-gray-600 py-1 px-2 bg-gray-50 rounded">
                              {blockedId}
                            </div>
                          ))}
                        </div>
                      ) : (
                        <p className="text-sm text-gray-500">No blocked users</p>
                      )}
                    </CardContent>
                  )}
                </Card>
              )}

              {/* Seller Info (if applicable) */}
              {user?.is_seller && (
                <Card>
                  <CardHeader>
                    <CardTitle className="text-lg flex items-center gap-2">
                      <Store className="h-5 w-5" />
                      Seller Information
                    </CardTitle>
                  </CardHeader>
                  <CardContent>
                    {/* Seller Authority Status */}
                    <div className="flex flex-wrap gap-3 mb-4">
                      <div>
                        <p className="text-xs text-gray-500 mb-1">Subscription</p>
                        <Badge variant={
                          user.subscription_status === 'active' ? 'success'
                            : user.subscription_status === 'expired' ? 'error'
                            : 'default'
                        }>
                          {user.subscription_status === 'active' ? 'Active'
                            : user.subscription_status === 'expired' ? 'Expired'
                            : 'Inactive'}
                        </Badge>
                        {user.recoverable_subscription_payment_id && canRecoverSubscription && (
                          <Button
                            variant="ghost"
                            size="sm"
                            className="mt-1 text-blue-600 hover:text-blue-800"
                            onClick={prepareSubscriptionRecover}
                            title="Recover missed subscription payment (webhook miss)"
                          >
                            <RotateCcw className="h-3 w-3 mr-1" />
                            Recover
                          </Button>
                        )}
                      </div>
                      <div>
                        <p className="text-xs text-gray-500 mb-1">Verification</p>
                        <Badge variant={
                          user.verification_status === 'approved' ? 'success'
                            : user.verification_status === 'suspended' || user.verification_status === 'revoked' || user.verification_status === 'rejected' ? 'error'
                            : user.verification_status === 'pending_review' || user.verification_status === 'needs_resubmission' || user.verification_status === 'under_investigation' ? 'warning'
                            : 'default'
                        }>
                          {({
                            approved: 'Approved',
                            pending_review: 'Pending Review',
                            needs_resubmission: 'Needs Resubmission',
                            rejected: 'Rejected',
                            suspended: 'Suspended',
                            revoked: 'Revoked',
                            under_investigation: 'Under Investigation',
                            not_submitted: 'Not Submitted',
                          } as Record<string, string>)[user.verification_status ?? ''] ?? 'Not Submitted'}
                        </Badge>
                      </div>
                      <div>
                        <p className="text-xs text-gray-500 mb-1">Seller Tier</p>
                        {user.seller_tier === 'pro' && (
                          <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full border border-amber-300 bg-amber-50 text-amber-900 text-xs font-semibold">
                            ⭐ Pro
                          </span>
                        )}
                        {user.seller_tier === 'elite' && (
                          <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full border border-indigo-300 bg-indigo-50 text-indigo-900 text-xs font-semibold">
                            👑 Elite
                          </span>
                        )}
                        {(!user.seller_tier || user.seller_tier === 'basic') && (
                          <Badge variant="default">Basic</Badge>
                        )}
                      </div>
                    </div>
                    <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                      {user.farm_name && (
                        <div>
                          <p className="text-sm text-gray-600">Farm Name</p>
                          <p className="font-medium">{user.farm_name}</p>
                        </div>
                      )}
                      {user.seller_rating !== undefined && (
                        <div>
                          <p className="text-sm text-gray-600">Rating</p>
                          <p className="font-medium">{user.seller_rating.toFixed(1)} / 5.0</p>
                        </div>
                      )}
                      {user.total_sales !== undefined && (
                        <div>
                          <p className="text-sm text-gray-600">Total Sales</p>
                          <p className="font-medium">{formatRupiah(user.total_sales)}</p>
                        </div>
                      )}
                      {user.total_orders_sold !== undefined && (
                        <div>
                          <p className="text-sm text-gray-600">Orders Sold</p>
                          <p className="font-medium">{user.total_orders_sold}</p>
                        </div>
                      )}
                      {user.seller_payable != null && (
                        <div>
                          <p className="text-sm text-gray-600">Seller Payable</p>
                          <p className="font-medium">{formatRupiah(user.seller_payable)}</p>
                        </div>
                      )}
                      {user.frozen_payable_balance !== undefined && user.frozen_payable_balance > 0 && (
                        <div>
                          <p className="text-sm text-gray-600">Frozen Balance</p>
                          <p className="font-medium text-amber-600">{formatRupiah(user.frozen_payable_balance)}</p>
                        </div>
                      )}
                    </div>
                  </CardContent>
                </Card>
              )}

              {/* Buyer Info (if applicable) */}
              {user?.is_buyer && (
                <Card>
                  <CardHeader>
                    <CardTitle className="text-lg flex items-center gap-2">
                      <ShoppingBag className="h-5 w-5" />
                      Buyer Information
                    </CardTitle>
                  </CardHeader>
                  <CardContent>
                    <div className="grid grid-cols-2 gap-4">
                      {user.total_orders_bought !== undefined && (
                        <div>
                          <p className="text-sm text-gray-600">Orders Purchased</p>
                          <p className="font-medium">{user.total_orders_bought}</p>
                        </div>
                      )}
                      {user.total_spent !== undefined && (
                        <div>
                          <p className="text-sm text-gray-600">Total Spent</p>
                          <p className="font-medium">{formatRupiah(user.total_spent)}</p>
                        </div>
                      )}
                    </div>
                  </CardContent>
                </Card>
              )}

              {/* BNR Status (buyers with auction history) */}
              {(user?.total_bnr !== undefined || user?.banned_from_bidding !== undefined) && (
                <Card className={user?.banned_from_bidding ? 'border-orange-200' : undefined}>
                  <CardHeader>
                    <CardTitle className="text-lg flex items-center gap-2">
                      <Activity className="h-5 w-5" />
                      BNR Status
                    </CardTitle>
                  </CardHeader>
                  <CardContent>
                    <div className="grid grid-cols-2 gap-4 text-sm">
                      <div>
                        <p className="text-gray-600">Total Strikes</p>
                        <p className="font-medium">
                          {user?.total_bnr ?? 0}
                          {(user?.total_bnr ?? 0) > 0 && (
                            <span className="ml-2 text-orange-600 text-xs font-normal">active</span>
                          )}
                        </p>
                      </div>
                      <div>
                        <p className="text-gray-600">Bidding Restriction</p>
                        <p className="font-medium">
                          {user?.banned_from_bidding
                            ? <span className="text-red-600">Restricted</span>
                            : <span className="text-green-600">None</span>}
                        </p>
                      </div>
                      {user?.bid_reliability !== undefined && (
                        <div>
                          <p className="text-gray-600">Bid Reliability</p>
                          <p className="font-medium">{user.bid_reliability}%</p>
                        </div>
                      )}
                    </div>
                    {(user?.total_bnr ?? 0) > 0 && canResetBNR && (
                      <div className="mt-4 pt-4 border-t border-gray-100">
                        <Button
                          size="sm"
                          variant="warning"
                          onClick={prepareBNRReset}
                          disabled={loading}
                        >
                          <RotateCcw className="h-3 w-3 mr-1" />
                          Reset All BNR Strikes
                        </Button>
                        <p className="text-xs text-gray-500 mt-1">
                          Clears active restrictions. Audit trail remains.
                        </p>
                      </div>
                    )}
                  </CardContent>
                </Card>
              )}

              {/* Account Status Details (if suspended or banned) */}
              {(user?.suspended_reason || user?.banned_reason) && (
                <Card className="border-amber-200">
                  <CardHeader>
                    <CardTitle className="text-lg text-amber-700 flex items-center gap-2">
                      <AlertTriangle className="h-5 w-5" />
                      Account Status Details
                    </CardTitle>
                  </CardHeader>
                  <CardContent className="space-y-3">
                    {user.suspended_reason && (
                      <div className="bg-amber-50 p-3 rounded border border-amber-200">
                        <p className="text-sm font-medium text-amber-800">Suspension Reason</p>
                        <p className="text-sm text-gray-700 mt-1">{user.suspended_reason}</p>
                        {user.suspended_until && (
                          <p className="text-xs text-amber-600 mt-2">
                            Until: {formatDate(user.suspended_until)}
                          </p>
                        )}
                      </div>
                    )}
                    {user.banned_reason && (
                      <div className="bg-red-50 p-3 rounded border border-red-200">
                        <p className="text-sm font-medium text-red-800">Ban Reason</p>
                        <p className="text-sm text-gray-700 mt-1">{user.banned_reason}</p>
                        {user.banned_at && (
                          <p className="text-xs text-red-600 mt-2">
                            Banned on: {formatDate(user.banned_at)}
                          </p>
                        )}
                      </div>
                    )}
                  </CardContent>
                </Card>
              )}

              {/* Timestamps */}
              <Card>
                <CardHeader>
                  <CardTitle className="text-lg flex items-center gap-2">
                    <Calendar className="h-5 w-5" />
                    Timeline
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="grid grid-cols-2 gap-4 text-sm">
                    <div>
                      <p className="text-gray-600">Joined At</p>
                      <p className="font-medium">{formatDate(displayData.created_at)}</p>
                    </div>
                    <div>
                      <p className="text-gray-600">Last Updated</p>
                      <p className="font-medium">
                        {displayData.updated_at ? formatDate(displayData.updated_at) : 'N/A'}
                      </p>
                    </div>
                    <div>
                      <p className="text-gray-600">Last Active</p>
                      <p className="font-medium">
                        {displayData.last_active_at ? formatDate(displayData.last_active_at) : 'Unknown'}
                      </p>
                    </div>
                    <div>
                      <p className="text-gray-600">Auth Provider</p>
                      <p className="font-medium capitalize">
                        {user?.auth_provider || 'email'}
                      </p>
                    </div>
                  </div>
                </CardContent>
              </Card>
            </>
          )}

          {/* Footer with action buttons (only in idle state) */}
          {actionState === 'idle' && (
            <ModalFooter className="flex items-center justify-between">
              <div className="text-sm text-gray-500">
                <span className="flex items-center gap-1">
                  <AlertTriangle className="h-4 w-4 text-amber-500" />
                  Review all information before taking action
                </span>
              </div>
              <div className="flex items-center gap-3 flex-wrap">
                <Button variant="secondary" onClick={onClose}>
                  Close
                </Button>

                {/* Role change button - always available if capable */}
                {canAssignRoles && (
                  <Button
                    variant="secondary"
                    onClick={prepareRoleChange}
                    disabled={loading}
                    title="Change user role"
                  >
                    <UserCog className="h-4 w-4 mr-2" />
                    Change Role
                  </Button>
                )}

                {/* Status-specific actions */}
                {isBanned ? (
                  // Banned users can only be unbanned
                  canUnbanUsers && (
                    <Button
                      variant="success"
                      onClick={() => prepareAction('unban')}
                      disabled={loading}
                      title="Unban this user (requires governance.user.unban)"
                    >
                      <UserCheck className="h-4 w-4 mr-2" />
                      Unban
                    </Button>
                  )
                ) : (
                  <>
                    {isSuspended && (
                      <Button
                        variant="success"
                        onClick={() => prepareAction('activate')}
                        disabled={loading || !canActivateUsers}
                        title={!canActivateUsers ? 'Requires: governance.user.activate' : ''}
                      >
                        <UserCheck className="h-4 w-4 mr-2" />
                        Activate
                      </Button>
                    )}
                    {isActive && (
                      <Button
                        variant="warning"
                        onClick={() => prepareAction('suspend')}
                        disabled={loading || !canSuspendUsers}
                        title={!canSuspendUsers ? 'Requires: governance.user.suspend' : ''}
                      >
                        <AlertTriangle className="h-4 w-4 mr-2" />
                        Suspend
                      </Button>
                    )}
                    <Button
                      variant="danger"
                      onClick={() => prepareAction('ban')}
                      disabled={loading || !canBanUsers}
                      title={!canBanUsers ? 'Requires: governance.user.ban' : ''}
                    >
                      <Ban className="h-4 w-4 mr-2" />
                      Ban
                    </Button>
                  </>
                )}
              </div>
            </ModalFooter>
          )}
        </div>
      )}
    </Modal>
  )
}
