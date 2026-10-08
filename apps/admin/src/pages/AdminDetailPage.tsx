import { useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, Shield, RefreshCw, AlertTriangle, Check, X, Users, UserCog } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { Checkbox } from '@/components/ui/Checkbox'
import { useUserDetail, useUserActions } from '@/hooks/useUsers'
import { useCapabilities, useUserCapabilities, useCapabilityActions } from '@/hooks/useCapabilities'
import { groupCapabilitiesByCategory, capabilityGroupDescription } from '@/types/capability'
import { formatDate } from '@/lib/utils'
import { hasCapability } from '@/lib/permissions'
import { useAuth } from '@/hooks/useAuth'
import { AdminLoadingState, AdminErrorState, PageHeader } from '@/components/common'

export function AdminDetailPage() {
  const { id: userId } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { user: currentUser } = useAuth()

  const { user, loading: userLoading, error: userError, refetch: refetchUser } = useUserDetail(userId || null)
  const { capabilities, loading: capsLoading } = useCapabilities()
  const {
    userCapabilities,
    loading: userCapsLoading,
    refetch: refetchUserCaps,
    total,
    role,
    fullAccess,
    missingCapabilities,
  } = useUserCapabilities(userId || null)
  const { assign, revoke, error: actionError, clearError } = useCapabilityActions(userId || null)
  const { setRole, error: roleError, clearError: clearRoleError } = useUserActions(userId || null)

  const [updatingCaps, setUpdatingCaps] = useState<Set<string>>(new Set())
  const [updatingRole, setUpdatingRole] = useState(false)
  const hasCapabilityAssignPermission = hasCapability(currentUser?.capabilities, 'governance.capability.assign')
  const hasRoleAssignPermission = hasCapability(currentUser?.capabilities, 'governance.role.assign')
  const isSelf = currentUser?.id === userId
  const isReadOnly = !hasCapabilityAssignPermission

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
    return <AdminLoadingState />
  }

  if (userError || !user) {
    return (
      <div className="space-y-6">
        <PageHeader
          title="Admin Management"
          description={isReadOnly ? 'Viewing capabilities' : 'Manage capabilities'}
          leading={
            <Button variant="ghost" onClick={() => navigate(-1)} className="gap-2">
              <ArrowLeft className="h-4 w-4" />
              Back
            </Button>
          }
        />
        <AdminErrorState
          title="Failed to load admin"
          message={userError?.message || 'Admin not found'}
          onRetry={refetchUser}
        />
      </div>
    )
  }

  // Group capabilities by the cluster the BACKEND reported. Clusters are open
  // by design: every cluster the canonical universe contains is rendered, so no
  // cluster can be hidden by a stale frontend list.
  const groupedCapabilities = groupCapabilitiesByCategory(capabilities)

  const isOwnProfile = currentUser?.id === userId

  const handleSetRole = async (next: 'user' | 'admin') => {
    setUpdatingRole(true)
    clearRoleError()
    const result = await setRole(next)
    if (result) {
      await Promise.all([refetchUser(), refetchUserCaps()])
    }
    setUpdatingRole(false)
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <PageHeader
        title="Admin Management"
        description={isReadOnly ? 'Viewing capabilities' : 'Manage capabilities'}
        leading={
          <Button variant="ghost" onClick={() => navigate(-1)} className="gap-2">
            <ArrowLeft className="h-4 w-4" />
            Back
          </Button>
        }
        actions={
          <Button variant="secondary" onClick={() => refetchUserCaps()} className="gap-2">
            <RefreshCw className="h-4 w-4" />
            Refresh
          </Button>
        }
      />

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
                  <div className="w-24 h-24 rounded-full bg-border flex items-center justify-center">
                    <Users className="h-12 w-12 text-muted-foreground" />
                  </div>
                )}
              </div>

              {/* Name */}
              <div className="text-center">
                <h3 className="text-xl font-semibold text-foreground">@{user.username}</h3>
              </div>

              {/* Email */}
              <div className="border-t pt-4">
                <p className="type-secondary">Email</p>
                <p className="text-sm font-mono text-foreground break-all">{user.email}</p>
              </div>

              {/* Status */}
              <div className="border-t pt-4">
                <p className="type-secondary">Status</p>
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
                  <p className="type-secondary">Joined</p>
                  <p className="type-body">{formatDate(user.created_at)}</p>
                </div>
                <div>
                  <p className="type-secondary">Last Active</p>
                  <p className="type-body">{user.last_active_at ? formatDate(user.last_active_at) : 'Never'}</p>
                </div>
              </div>
            </CardContent>
          </Card>

          {/* Admin authority: role + DERIVED full access. */}
          <Card>
            <CardHeader>
              <CardTitle>Admin Authority</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div>
                <p className="type-secondary">Role</p>
                <Badge variant={role === 'admin' ? 'info' : 'default'} className="mt-1">
                  {role === 'admin' ? 'Admin' : 'User'}
                </Badge>
              </div>

              <div className="border-t pt-4">
                <p className="type-secondary">Full access</p>
                {fullAccess ? (
                  <Badge variant="success" className="mt-1">Full access</Badge>
                ) : (
                  <div className="mt-1">
                    <Badge variant="warning">Not full access</Badge>
                    <p className="type-caption mt-1">
                      {role !== 'admin'
                        ? 'Requires admin membership.'
                        : `${missingCapabilities.length} capability${missingCapabilities.length === 1 ? '' : 'ies'} not granted.`}
                    </p>
                  </div>
                )}
              </div>

              {hasRoleAssignPermission && !isSelf && (
                <div className="border-t pt-4 space-y-2">
                  {role === 'admin' ? (
                    <Button
                      variant="warning"
                      size="sm"
                      className="w-full"
                      disabled={updatingRole}
                      onClick={() => {
                        const ok = window.confirm(
                          'Demote this admin to user? Capabilities stay stored but become inert without admin membership. The system refuses this if it would remove the last full-access admin.'
                        )
                        if (ok) void handleSetRole('user')
                      }}
                    >
                      <UserCog className="h-4 w-4 mr-2" />
                      Demote to User
                    </Button>
                  ) : (
                    <Button
                      variant="primary"
                      size="sm"
                      className="w-full"
                      disabled={updatingRole}
                      onClick={() => void handleSetRole('admin')}
                    >
                      <UserCog className="h-4 w-4 mr-2" />
                      Promote to Admin
                    </Button>
                  )}
                  <p className="type-caption">
                    Promoting grants admin membership only. Grant capabilities separately — a new admin starts with
                    none.
                  </p>
                </div>
              )}

              {isSelf && (
                <p className="border-t pt-4 text-xs text-info bg-info-bg rounded p-2">
                  This is your own account. Role changes for yourself must be made by another authorized admin.
                </p>
              )}
            </CardContent>
          </Card>
        </div>

        {/* CENTER COLUMN - Capabilities (6 cols) */}
        <div className="lg:col-span-6 space-y-6">
          {/* Read-only warning */}
          {isReadOnly && (
            <Card className="border-warning bg-warning-bg">
              <CardContent className="p-4">
                <div className="flex items-start gap-3">
                  <AlertTriangle className="h-5 w-5 text-warning flex-shrink-0 mt-0.5" />
                  <div>
                    <p className="text-sm font-medium text-warning">Read-Only Mode</p>
                    <p className="text-sm text-warning">
                      You don't have permission to modify capabilities. You need the{' '}
                      <code className="px-1 py-0.5 bg-warning-bg rounded text-xs">governance.capability.assign</code>{' '}
                      capability.
                    </p>
                  </div>
                </div>
              </CardContent>
            </Card>
          )}

          {/* Role action error */}
          {roleError && (
            <Card className="border-destructive bg-destructive-bg">
              <CardContent className="p-4">
                <div className="flex items-start gap-3">
                  <X className="h-5 w-5 text-destructive flex-shrink-0 mt-0.5" />
                  <div>
                    <p className="text-sm font-medium text-destructive">Role change rejected</p>
                    <p className="text-sm text-destructive">{roleError}</p>
                  </div>
                </div>
              </CardContent>
            </Card>
          )}

          {/* Action Error */}
          {actionError && (
            <Card className="border-destructive bg-destructive-bg">
              <CardContent className="p-4">
                <div className="flex items-start gap-3">
                  <X className="h-5 w-5 text-destructive flex-shrink-0 mt-0.5" />
                  <div>
                    <p className="text-sm font-medium text-destructive">Error</p>
                    <p className="text-sm text-destructive">{actionError}</p>
                  </div>
                </div>
              </CardContent>
            </Card>
          )}

          {/* Capability Groups */}
          {groupedCapabilities.map(([category, categoryCaps]) => (
            <Card key={category}>
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <Shield className="h-5 w-5" />
                  {category}
                </CardTitle>
                <p className="type-secondary">
                  {capabilityGroupDescription(category)}
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
                        isDisabled ? 'bg-surface-muted border-border' : 'bg-surface border-border hover:border-border'
                      }`}
                    >
                      <Checkbox
                        id={cap.capability}
                        checked={isAssigned}
                        onChange={() => handleToggleCapability(cap.capability)}
                        disabled={isDisabled || isOwnLastCritical}
                        className="mt-0.5"
                      />
                      <div className="flex-1 min-w-0">
                        <label
                          htmlFor={cap.capability}
                          className={`font-medium text-sm ${
                            isDisabled ? 'text-muted-foreground' : 'text-foreground cursor-pointer'
                          }`}
                        >
                          {cap.capability}
                          {cap.critical && (
                            <Badge variant="warning" className="ml-2 text-xs">
                              CRITICAL
                            </Badge>
                          )}
                        </label>
                        <p className="type-caption mt-1">{cap.description}</p>
                        {isOwnLastCritical && (
                          <p className="text-xs text-warning mt-1 flex items-center gap-1">
                            <AlertTriangle className="h-3 w-3" />
                            Cannot revoke your own last critical capability
                          </p>
                        )}
                      </div>
                      {isUpdating && (
                        <div className="h-4 w-4 animate-spin rounded-full border-2 border-solid border-primary border-r-transparent"></div>
                      )}
                      {isAssigned && !isUpdating && !isDisabled && (
                        <Check className="h-4 w-4 text-success flex-shrink-0" />
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
                <p className="type-secondary">Total Capabilities</p>
                <p className="type-metric text-primary">{total}</p>
              </div>

              {/* Critical Capabilities */}
              <div className="border-t pt-4">
                <p className="type-secondary">Critical Capabilities</p>
                <p className="type-metric text-warning">
                  {userCapabilities.filter(uc => {
                    const def = capabilities.find(c => c.capability === uc.capability)
                    return def?.critical
                  }).length}
                </p>
              </div>

              {/* Last Updated */}
              {userCapabilities.length > 0 && (
                <div className="border-t pt-4">
                  <p className="type-secondary">Last Updated</p>
                  <p className="type-body">
                    {formatDate(userCapabilities[userCapabilities.length - 1].granted_at)}
                  </p>
                </div>
              )}

              {/* Profile Warning */}
              {isOwnProfile && (
                <div className="border-t pt-4">
                  <div className="p-3 bg-info-bg rounded-lg">
                    <p className="text-xs font-medium text-info flex items-center gap-1">
                      <Shield className="h-3 w-3" />
                      Your Own Profile
                    </p>
                    <p className="text-xs text-info mt-1">
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
              {groupedCapabilities.map(([category, categoryCaps]) => {
                const assigned = categoryCaps.filter(c => isCapabilityAssigned(c.capability)).length
                const total = categoryCaps.length
                return (
                  <div key={category} className="flex items-center justify-between">
                    <span className="type-secondary">{category}</span>
                    <span className="type-label">
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
