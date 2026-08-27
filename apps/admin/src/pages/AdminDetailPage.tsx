import { useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, Shield, RefreshCw, AlertTriangle, Check, X, Users } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { useUserDetail } from '@/hooks/useUsers'
import { useCapabilities, useUserCapabilities, useCapabilityActions } from '@/hooks/useCapabilities'
import { CAPABILITY_GROUPS, type CapabilityCategory } from '@/types/capability'
import { formatDate } from '@/lib/utils'
import { hasCapability } from '@/lib/permissions'
import { useAuth } from '@/hooks/useAuth'

export function AdminDetailPage() {
  const { id: userId } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { user: currentUser } = useAuth()

  const { user, loading: userLoading, error: userError } = useUserDetail(userId || null)
  const { capabilities, loading: capsLoading } = useCapabilities()
  const { userCapabilities, loading: userCapsLoading, refetch: refetchUserCaps, total } = useUserCapabilities(userId || null)
  const { assign, revoke, error: actionError, clearError } = useCapabilityActions(userId || null)

  const [updatingCaps, setUpdatingCaps] = useState<Set<string>>(new Set())
  const hasCapabilityAssignPermission = hasCapability(currentUser?.capabilities, 'governance.capability.assign')

  // Get user's capability strings for quick lookup
  const userCapabilitySet = new Set(userCapabilities.map(uc => uc.capability))

  // Check if a capability is currently assigned
  const isCapabilityAssigned = (capability: string) => userCapabilitySet.has(capability)

  // Handle capability toggle
  const handleToggleCapability = async (capability: string) => {
    if (!userId) return

    // Safety check: cannot revoke own last critical capability
    if (isCapabilityAssigned(capability) && isOwnLastCriticalCapability(capability)) {
      alert('Cannot revoke your own last critical capability')
      return
    }

    setUpdatingCaps(prev => new Set(prev).add(capability))
    clearError()

    const success = isCapabilityAssigned(capability)
      ? await revoke(capability)
      : await assign(capability)

    if (success) {
      await refetchUserCaps()
    }

    setUpdatingCaps(prev => {
      const next = new Set(prev)
      next.delete(capability)
      return next
    })
  }

  // Check if this is the user's own last critical capability
  const isOwnLastCriticalCapability = (capability: string) => {
    if (currentUser?.id !== userId) return false

    const capDef = capabilities.find(c => c.capability === capability)
    if (!capDef?.critical) return false

    // Count user's critical capabilities
    const criticalCaps = userCapabilities.filter(uc => {
      const def = capabilities.find(c => c.capability === uc.capability)
      return def?.critical
    })

    return criticalCaps.length <= 1
  }

  const isLoading = userLoading || capsLoading || userCapsLoading

  if (isLoading) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <div className="text-center">
          <div className="inline-block h-8 w-8 animate-spin rounded-full border-4 border-solid border-primary border-r-transparent"></div>
          <p className="mt-4 text-gray-600">Loading admin details...</p>
        </div>
      </div>
    )
  }

  if (userError || !user) {
    return (
      <div className="space-y-6">
        <div className="flex items-center gap-4">
          <Button variant="ghost" onClick={() => navigate(-1)} className="gap-2">
            <ArrowLeft className="h-4 w-4" />
            Back
          </Button>
        </div>
        <Card>
          <CardContent className="p-6">
            <div className="text-center text-red-600">
              <p>Error loading admin: {userError?.message || 'Admin not found'}</p>
            </div>
          </CardContent>
        </Card>
      </div>
    )
  }

  // Group capabilities by category
  const capabilitiesByCategory: Record<CapabilityCategory, typeof capabilities> = {
    Finance: capabilities.filter(c => c.category === 'Finance'),
    Governance: capabilities.filter(c => c.category === 'Governance'),
    Moderation: capabilities.filter(c => c.category === 'Moderation'),
    Support: capabilities.filter(c => c.category === 'Support'),
    Other: capabilities.filter(c => c.category === 'Other'),
  }

  const isReadOnly = !hasCapabilityAssignPermission
  const isOwnProfile = currentUser?.id === userId

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <Button variant="ghost" onClick={() => navigate(-1)} className="gap-2">
            <ArrowLeft className="h-4 w-4" />
            Back
          </Button>
          <div>
            <h1 className="text-3xl font-bold text-gray-900">Admin Management</h1>
            <p className="text-gray-600 mt-1">
              {isReadOnly ? 'Viewing capabilities' : 'Manage capabilities'}
            </p>
          </div>
        </div>
        <Button
          variant="secondary"
          onClick={() => refetchUserCaps()}
          className="gap-2"
        >
          <RefreshCw className="h-4 w-4" />
          Refresh
        </Button>
      </div>

      {/* Main Layout: 3 Columns */}
      <div className="grid grid-cols-1 lg:grid-cols-12 gap-6">
        {/* LEFT COLUMN - User Info (3 cols) */}
        <div className="lg:col-span-3 space-y-6">
          <Card>
            <CardHeader>
              <CardTitle>Admin Info</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              {/* Avatar */}
              <div className="flex justify-center">
                {user.photo_url ? (
                  <img
                    src={user.photo_url}
                    alt={user.username}
                    className="w-24 h-24 rounded-full object-cover"
                  />
                ) : (
                  <div className="w-24 h-24 rounded-full bg-gray-200 flex items-center justify-center">
                    <Users className="h-12 w-12 text-gray-500" />
                  </div>
                )}
              </div>

              {/* Name */}
              <div className="text-center">
                <h3 className="text-xl font-semibold text-gray-900">@{user.username}</h3>
              </div>

              {/* Email */}
              <div className="border-t pt-4">
                <p className="text-sm text-gray-500">Email</p>
                <p className="text-sm font-mono text-gray-900 break-all">{user.email}</p>
              </div>

              {/* Status */}
              <div className="border-t pt-4">
                <p className="text-sm text-gray-500">Status</p>
                <Badge variant={
                  user.account_status === 'active' ? 'success' :
                  user.account_status === 'suspended' ? 'warning' :
                  user.account_status === 'banned' ? 'error' :
                  'info'
                } className="mt-1">
                  {user.account_status.charAt(0).toUpperCase() + user.account_status.slice(1)}
                </Badge>
              </div>

              {/* Dates */}
              <div className="border-t pt-4 space-y-2">
                <div>
                  <p className="text-sm text-gray-500">Joined</p>
                  <p className="text-sm text-gray-900">{formatDate(user.created_at)}</p>
                </div>
                <div>
                  <p className="text-sm text-gray-500">Last Active</p>
                  <p className="text-sm text-gray-900">{user.last_active_at ? formatDate(user.last_active_at) : 'Never'}</p>
                </div>
              </div>
            </CardContent>
          </Card>
        </div>

        {/* CENTER COLUMN - Capabilities (6 cols) */}
        <div className="lg:col-span-6 space-y-6">
          {/* Read-only warning */}
          {isReadOnly && (
            <Card className="border-yellow-200 bg-yellow-50">
              <CardContent className="p-4">
                <div className="flex items-start gap-3">
                  <AlertTriangle className="h-5 w-5 text-yellow-600 flex-shrink-0 mt-0.5" />
                  <div>
                    <p className="text-sm font-medium text-yellow-900">Read-Only Mode</p>
                    <p className="text-sm text-yellow-700">
                      You don't have permission to modify capabilities. You need the{' '}
                      <code className="px-1 py-0.5 bg-yellow-100 rounded text-xs">governance.capability.assign</code>{' '}
                      capability.
                    </p>
                  </div>
                </div>
              </CardContent>
            </Card>
          )}

          {/* Action Error */}
          {actionError && (
            <Card className="border-red-200 bg-red-50">
              <CardContent className="p-4">
                <div className="flex items-start gap-3">
                  <X className="h-5 w-5 text-red-600 flex-shrink-0 mt-0.5" />
                  <div>
                    <p className="text-sm font-medium text-red-900">Error</p>
                    <p className="text-sm text-red-700">{actionError}</p>
                  </div>
                </div>
              </CardContent>
            </Card>
          )}

          {/* Capability Groups */}
          {Object.entries(capabilitiesByCategory).map(([category, categoryCaps]) => (
            <Card key={category}>
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <Shield className="h-5 w-5" />
                  {CAPABILITY_GROUPS[category as CapabilityCategory].label}
                </CardTitle>
                <p className="text-sm text-gray-600">
                  {CAPABILITY_GROUPS[category as CapabilityCategory].description}
                </p>
              </CardHeader>
              <CardContent className="space-y-3">
                {categoryCaps.map((cap) => {
                  const isAssigned = isCapabilityAssigned(cap.capability)
                  const isUpdating = updatingCaps.has(cap.capability)
                  const isDisabled = isReadOnly || isUpdating
                  const isOwnLastCritical = isOwnLastCriticalCapability(cap.capability)

                  return (
                    <div
                      key={cap.capability}
                      className={`flex items-start gap-3 p-3 rounded-lg border transition-colors ${
                        isDisabled ? 'bg-gray-50 border-gray-200' : 'bg-white border-gray-200 hover:border-gray-300'
                      }`}
                    >
                      <input
                        type="checkbox"
                        id={cap.capability}
                        checked={isAssigned}
                        onChange={() => handleToggleCapability(cap.capability)}
                        disabled={isDisabled || isOwnLastCritical}
                        className={`mt-0.5 h-4 w-4 rounded border-gray-300 ${
                          cap.critical ? 'text-orange-600 focus:ring-orange-500' : 'text-primary focus:ring-primary'
                        } ${isDisabled ? 'cursor-not-allowed opacity-50' : 'cursor-pointer'}`}
                      />
                      <div className="flex-1 min-w-0">
                        <label
                          htmlFor={cap.capability}
                          className={`font-medium text-sm ${
                            isDisabled ? 'text-gray-500' : 'text-gray-900 cursor-pointer'
                          }`}
                        >
                          {cap.capability}
                          {cap.critical && (
                            <Badge variant="warning" className="ml-2 text-xs">
                              CRITICAL
                            </Badge>
                          )}
                        </label>
                        <p className="text-xs text-gray-500 mt-1">{cap.description}</p>
                        {isOwnLastCritical && (
                          <p className="text-xs text-orange-600 mt-1 flex items-center gap-1">
                            <AlertTriangle className="h-3 w-3" />
                            Cannot revoke your own last critical capability
                          </p>
                        )}
                      </div>
                      {isUpdating && (
                        <div className="h-4 w-4 animate-spin rounded-full border-2 border-solid border-primary border-r-transparent"></div>
                      )}
                      {isAssigned && !isUpdating && !isDisabled && (
                        <Check className="h-4 w-4 text-green-600 flex-shrink-0" />
                      )}
                    </div>
                  )
                })}
              </CardContent>
            </Card>
          ))}
        </div>

        {/* RIGHT COLUMN - Summary (3 cols) */}
        <div className="lg:col-span-3 space-y-6">
          <Card>
            <CardHeader>
              <CardTitle>Summary</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              {/* Total Capabilities */}
              <div>
                <p className="text-sm text-gray-500">Total Capabilities</p>
                <p className="text-2xl font-bold text-primary">{total}</p>
              </div>

              {/* Critical Capabilities */}
              <div className="border-t pt-4">
                <p className="text-sm text-gray-500">Critical Capabilities</p>
                <p className="text-2xl font-bold text-orange-600">
                  {userCapabilities.filter(uc => {
                    const def = capabilities.find(c => c.capability === uc.capability)
                    return def?.critical
                  }).length}
                </p>
              </div>

              {/* Last Updated */}
              {userCapabilities.length > 0 && (
                <div className="border-t pt-4">
                  <p className="text-sm text-gray-500">Last Updated</p>
                  <p className="text-sm text-gray-900">
                    {formatDate(userCapabilities[userCapabilities.length - 1].granted_at)}
                  </p>
                </div>
              )}

              {/* Profile Warning */}
              {isOwnProfile && (
                <div className="border-t pt-4">
                  <div className="p-3 bg-blue-50 rounded-lg">
                    <p className="text-xs font-medium text-blue-900 flex items-center gap-1">
                      <Shield className="h-3 w-3" />
                      Your Own Profile
                    </p>
                    <p className="text-xs text-blue-700 mt-1">
                      You cannot revoke your own last critical capability
                    </p>
                  </div>
                </div>
              )}
            </CardContent>
          </Card>

          {/* Quick Stats */}
          <Card>
            <CardHeader>
              <CardTitle>Capability Stats</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3">
              {Object.entries(capabilitiesByCategory).map(([category, categoryCaps]) => {
                const assigned = categoryCaps.filter(c => isCapabilityAssigned(c.capability)).length
                const total = categoryCaps.length
                return (
                  <div key={category} className="flex items-center justify-between">
                    <span className="text-sm text-gray-600">{category}</span>
                    <span className="text-sm font-medium text-gray-900">
                      {assigned} / {total}
                    </span>
                  </div>
                )
              })}
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  )
}
