/**
 * Governance Case Detail Page
 *
 * Canonical admin governance case detail view.
 * Shows Case + Reports + Decisions + Enforcement + Decision creation form.
 *
 * Authority: REPORT_GOVERNANCE_ADMIN_BACKEND_IMPLEMENTATION_SLICE_6.md
 */
import { useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import {
  ArrowLeft,
  Shield,
  FileText,
  Gavel,
  AlertTriangle,
  CheckCircle,
  Clock,
} from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { useGovernanceCase, useCreateDecision, useGovernanceCaseAudit } from '@/hooks/useGovernance'
import { useAuth } from '@/hooks/useAuth'
import { hasCapability } from '@/lib/permissions'
import { formatDate } from '@/lib/utils'
import {
  caseStatusLabels,
  decisionOutcomeLabels,
  enforcementStatusLabels,
  targetTypeLabels,
  caseStatusVariants,
  decisionOutcomeVariants,
  enforcementStatusVariants,
} from '@/types/governance'
import type {
  GovernanceDecision,
  GovernanceEnforcement,
  GovernanceAuditEvent,
  DecisionOutcome,
  GovernanceTargetType,
  CreateDecisionRequest,
} from '@/types/governance'

export function GovernanceCaseDetailPage() {
  const { id: caseId } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { data, loading, error, refetch } = useGovernanceCase(caseId || null)
  const { events: auditEvents, loading: auditLoading, error: auditError } = useGovernanceCaseAudit(caseId || null)
  const { createDecision, loading: isCreating } = useCreateDecision()
  const { capabilities } = useAuth()
  const canCreateDecision = hasCapability(capabilities, 'moderation.case.resolve')

  // Decision form state
  const [showDecisionForm, setShowDecisionForm] = useState(false)
  const [decisionOutcome, setDecisionOutcome] = useState<DecisionOutcome>('no_violation')
  const [targetType, setTargetType] = useState<GovernanceTargetType>('content')
  const [targetId, setTargetId] = useState('')
  const [decisionNote, setDecisionNote] = useState('')
  const [decisionError, setDecisionError] = useState<string | null>(null)
  const [decisionSuccess, setDecisionSuccess] = useState(false)

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <div className="text-center">
          <div className="inline-block h-8 w-8 animate-spin rounded-full border-4 border-solid border-primary border-r-transparent"></div>
          <p className="mt-4 text-gray-600">Loading case details...</p>
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="space-y-6">
        <Button variant="secondary" onClick={() => navigate('/moderation/cases')}>
          <ArrowLeft className="h-4 w-4 mr-2" />
          Back to Cases
        </Button>
        <Card>
          <CardContent className="p-6">
            <div className="text-center text-red-600">
              <p className="font-medium">Error loading case</p>
              <p className="text-sm mt-1">{error.message}</p>
            </div>
          </CardContent>
        </Card>
      </div>
    )
  }

  if (!data) {
    return (
      <div className="space-y-6">
        <Button variant="secondary" onClick={() => navigate('/moderation/cases')}>
          <ArrowLeft className="h-4 w-4 mr-2" />
          Back to Cases
        </Button>
        <Card>
          <CardContent className="p-6">
            <div className="text-center text-gray-600">
              <p>Case not found.</p>
            </div>
          </CardContent>
        </Card>
      </div>
    )
  }

  const { case: kase, reports, decisions } = data
  const isOpen = kase.status === 'open'

  const handleCreateDecision = async () => {
    setDecisionError(null)
    setDecisionSuccess(false)

    // Validate
    if (decisionOutcome === 'violation') {
      if (!targetId.trim()) {
        setDecisionError('Target ID is required for violation decisions')
        return
      }
    }

    const request: CreateDecisionRequest = {
      outcome: decisionOutcome,
      decision_note: decisionNote.trim() || undefined,
    }

    if (decisionOutcome === 'violation') {
      request.target_type = targetType
      request.target_id = targetId.trim()
    }

    try {
      await createDecision(kase.id, request)
      setDecisionSuccess(true)
      setShowDecisionForm(false)
      setDecisionNote('')
      setTargetId('')
      // Refresh to show new Decision and updated Case status
      refetch()
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to create decision'
      setDecisionError(message)
    }
  }

  return (
    <div className="space-y-6">
      {/* Navigation */}
      <div className="flex items-center justify-between">
        <Button variant="secondary" onClick={() => navigate('/moderation/cases')}>
          <ArrowLeft className="h-4 w-4 mr-2" />
          Back to Cases
        </Button>
        {isOpen && canCreateDecision && (
          <Button onClick={() => setShowDecisionForm(!showDecisionForm)}>
            <Gavel className="h-4 w-4 mr-2" />
            {showDecisionForm ? 'Cancel' : 'Create Decision'}
          </Button>
        )}
      </div>

      {/* Read-only notice for admins without decision authority */}
      {isOpen && !canCreateDecision && (
        <div className="bg-gray-50 border border-gray-200 text-gray-600 p-3 rounded-lg flex items-center gap-2">
          <AlertTriangle className="h-4 w-4 flex-shrink-0" />
          <span className="text-sm">
            You can view this case but do not have permission to create decisions
            (requires moderation.case.resolve).
          </span>
        </div>
      )}

      {/* Success banner */}
      {decisionSuccess && (
        <div className="bg-green-50 border border-green-200 text-green-700 p-4 rounded-lg flex items-center gap-2">
          <CheckCircle className="h-5 w-5" />
          <span className="font-medium">Decision created successfully. Case has been refreshed.</span>
        </div>
      )}

      {/* Case Info */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Shield className="h-5 w-5" />
            Case Detail
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            <div>
              <p className="text-sm text-gray-500">Case ID</p>
              <p className="font-mono text-sm">{kase.id}</p>
            </div>
            <div>
              <p className="text-sm text-gray-500">Subject Type</p>
              <Badge variant="default">{targetTypeLabels[kase.subject_type]}</Badge>
            </div>
            <div>
              <p className="text-sm text-gray-500">Subject ID</p>
              <p className="font-mono text-sm">{kase.subject_id}</p>
            </div>
            <div>
              <p className="text-sm text-gray-500">Status</p>
              <Badge variant={caseStatusVariants[kase.status]}>
                {caseStatusLabels[kase.status]}
              </Badge>
            </div>
          </div>
          <div className="grid grid-cols-2 md:grid-cols-3 gap-4">
            <div>
              <p className="text-sm text-gray-500">Created</p>
              <p className="text-sm">{formatDate(kase.created_at)}</p>
            </div>
            <div>
              <p className="text-sm text-gray-500">Updated</p>
              <p className="text-sm">{formatDate(kase.updated_at)}</p>
            </div>
            {kase.closed_at && (
              <div>
                <p className="text-sm text-gray-500">Closed</p>
                <p className="text-sm">{formatDate(kase.closed_at)}</p>
              </div>
            )}
          </div>
        </CardContent>
      </Card>

      {/* Create Decision Form */}
      {showDecisionForm && (
        <Card className="border-primary/30">
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-primary">
              <Gavel className="h-5 w-5" />
              Create Decision
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            {decisionError && (
              <div className="bg-red-50 border border-red-200 text-red-700 p-3 rounded-lg flex items-center gap-2">
                <AlertTriangle className="h-4 w-4" />
                <span className="text-sm">{decisionError}</span>
              </div>
            )}

            {/* Outcome */}
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                Outcome <span className="text-red-500">*</span>
              </label>
              <select
                value={decisionOutcome}
                onChange={(e) => setDecisionOutcome(e.target.value as DecisionOutcome)}
                className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-primary"
              >
                <option value="no_violation">No Violation</option>
                <option value="violation">Violation</option>
              </select>
              <p className="text-xs text-gray-500 mt-1">
                {decisionOutcome === 'violation'
                  ? 'Policy was violated — enforcement will be created'
                  : 'Content complies with policy — no enforcement needed'}
              </p>
            </div>

            {/* Target (violation only) */}
            {decisionOutcome === 'violation' && (
              <>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">
                    Target Type <span className="text-red-500">*</span>
                  </label>
                  <select
                    value={targetType}
                    onChange={(e) => setTargetType(e.target.value as GovernanceTargetType)}
                    className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-primary"
                  >
                    <option value="content">Content</option>
                    <option value="comment">Comment</option>
                    <option value="for_sale">For Sale</option>
                    <option value="auction">Auction</option>
                    <option value="user">User</option>
                  </select>
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">
                    Target ID <span className="text-red-500">*</span>
                  </label>
                  <input
                    type="text"
                    value={targetId}
                    onChange={(e) => setTargetId(e.target.value)}
                    placeholder="UUID of the target to enforce against"
                    className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-primary"
                  />
                  <p className="text-xs text-gray-500 mt-1">
                    The subject ID to apply enforcement to (often the Case's subject_id)
                  </p>
                </div>
              </>
            )}

            {/* Decision Note */}
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                Decision Note <span className="text-gray-400">(optional)</span>
              </label>
              <textarea
                value={decisionNote}
                onChange={(e) => setDecisionNote(e.target.value)}
                placeholder="Reason or note for this decision..."
                rows={3}
                maxLength={2000}
                className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-primary resize-none"
              />
              <p className="text-xs text-gray-500 mt-1">{decisionNote.length}/2000 characters</p>
            </div>

            {/* Submit */}
            <div className="flex justify-end gap-3 pt-2">
              <Button
                variant="secondary"
                onClick={() => {
                  setShowDecisionForm(false)
                  setDecisionError(null)
                }}
                disabled={isCreating}
              >
                Cancel
              </Button>
              <Button
                onClick={handleCreateDecision}
                disabled={isCreating}
                isLoading={isCreating}
              >
                Create Decision
              </Button>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Reports */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <FileText className="h-5 w-5" />
            Reports ({reports.length})
          </CardTitle>
        </CardHeader>
        <CardContent>
          {reports.length === 0 ? (
            <p className="text-gray-500 text-sm">No reports associated with this case.</p>
          ) : (
            <div className="space-y-3">
              {reports.map((report) => (
                <div key={report.id} className="border border-gray-200 rounded-lg p-4">
                  <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
                    <div>
                      <p className="text-xs text-gray-500">Report ID</p>
                      <p className="font-mono text-xs">{report.id.slice(0, 8)}</p>
                    </div>
                    <div>
                      <p className="text-xs text-gray-500">Reporter</p>
                      <p className="font-mono text-xs">{report.reporter_id.slice(0, 8)}</p>
                    </div>
                    <div>
                      <p className="text-xs text-gray-500">Reason</p>
                      <p className="text-xs font-medium">{report.reason_code}</p>
                    </div>
                    <div>
                      <p className="text-xs text-gray-500">Created</p>
                      <p className="text-xs">{formatDate(report.created_at)}</p>
                    </div>
                  </div>
                  {report.reason_note && (
                    <div className="mt-2">
                      <p className="text-xs text-gray-500">Note</p>
                      <p className="text-sm bg-gray-50 p-2 rounded">{report.reason_note}</p>
                    </div>
                  )}
                  {report.evidence_snapshot && (
                    <div className="mt-2 text-xs text-gray-500">
                      {report.evidence_snapshot.author_username && (
                        <span>Author: {report.evidence_snapshot.author_username} · </span>
                      )}
                      {report.evidence_snapshot.title && (
                        <span>Title: {report.evidence_snapshot.title} · </span>
                      )}
                      {report.evidence_snapshot.status && (
                        <span>Status: {report.evidence_snapshot.status}</span>
                      )}
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      {/* Decisions */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Gavel className="h-5 w-5" />
            Decisions ({decisions.length})
          </CardTitle>
        </CardHeader>
        <CardContent>
          {decisions.length === 0 ? (
            <div className="text-center py-8 text-gray-500">
              <Clock className="h-8 w-8 mx-auto mb-2 text-gray-400" />
              <p className="text-sm">No decisions made yet.</p>
              {isOpen && (
                <p className="text-xs text-gray-400 mt-1">
                  Click &quot;Create Decision&quot; to make a governance decision.
                </p>
              )}
            </div>
          ) : (
            <div className="space-y-4">
              {decisions.map((decision) => (
                <DecisionCard key={decision.id} decision={decision} />
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      {/* Audit Timeline */}
      <AuditTimeline events={auditEvents} loading={auditLoading} error={auditError} />
    </div>
  )
}

// ============================================================================
// AUDIT TIMELINE COMPONENT
// ============================================================================

function AuditTimeline({
  events,
  loading,
  error,
}: {
  events: GovernanceAuditEvent[]
  loading: boolean
  error: Error | null
}) {
  if (loading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Clock className="h-5 w-5" />
            Audit Timeline
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex items-center justify-center py-8">
            <div className="inline-block h-6 w-6 animate-spin rounded-full border-4 border-solid border-primary border-r-transparent"></div>
            <span className="ml-3 text-sm text-gray-500">Loading audit events...</span>
          </div>
        </CardContent>
      </Card>
    )
  }

  if (error) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Clock className="h-5 w-5" />
            Audit Timeline
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="text-center py-8">
            <p className="text-sm text-red-600">Failed to load audit events: {error.message}</p>
          </div>
        </CardContent>
      </Card>
    )
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Clock className="h-5 w-5" />
          Audit Timeline ({events.length})
        </CardTitle>
      </CardHeader>
      <CardContent>
        {events.length === 0 ? (
          <div className="text-center py-8">
            <Clock className="h-8 w-8 mx-auto mb-2 text-gray-400" />
            <p className="text-sm text-gray-500">No audit events recorded for this case.</p>
          </div>
        ) : (
          <div className="space-y-3">
            {events.map((event) => (
              <AuditEventRow key={event.id} event={event} />
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  )
}

function AuditEventRow({ event }: { event: GovernanceAuditEvent }) {
  const outcomeLabel = event.outcome === 'violation' ? 'Violation' : event.outcome === 'no_violation' ? 'No Violation' : event.outcome
  const outcomeVariant = event.outcome === 'violation' ? 'warning' as const : 'success' as const

  return (
    <div className="border border-gray-200 rounded-lg p-4">
      <div className="flex items-start justify-between">
        <div className="space-y-1">
          <div className="flex items-center gap-3">
            <Badge variant="default">{event.event_type}</Badge>
            {event.outcome && (
              <Badge variant={outcomeVariant}>{outcomeLabel}</Badge>
            )}
          </div>
          <div className="flex items-center gap-2 text-xs text-gray-500">
            <span className="font-medium capitalize">{event.actor_type}</span>
            {event.actor_name && (
              <span>({event.actor_name})</span>
            )}
            {event.actor_id && !event.actor_name && (
              <span className="font-mono">{event.actor_id.slice(0, 8)}</span>
            )}
          </div>
          {event.target_type && (
            <div className="text-xs text-gray-500">
              Target: {targetTypeLabels[event.target_type] || event.target_type}
              {event.target_id && (
                <span className="font-mono ml-1">{event.target_id.slice(0, 8)}</span>
              )}
            </div>
          )}
          {event.decision_note && (
            <p className="text-sm bg-gray-50 p-2 rounded mt-1">{event.decision_note}</p>
          )}
        </div>
        <span className="text-xs text-gray-400 whitespace-nowrap">
          {formatDate(event.created_at)}
        </span>
      </div>
    </div>
  )
}

// ============================================================================
// DECISION CARD COMPONENT
// ============================================================================

function DecisionCard({ decision }: { decision: GovernanceDecision }) {
  return (
    <div className="border border-gray-200 rounded-lg p-4">
      <div className="flex items-start justify-between">
        <div className="space-y-2">
          <div className="flex items-center gap-3">
            <Badge variant={decisionOutcomeVariants[decision.outcome]}>
              {decisionOutcomeLabels[decision.outcome]}
            </Badge>
            <span className="text-xs text-gray-400 font-mono">{decision.id.slice(0, 8)}</span>
            <span className="text-xs text-gray-400">·</span>
            <span className="text-xs text-gray-500">
              by {decision.decided_by.slice(0, 8)}
            </span>
          </div>
          <div className="text-xs text-gray-500">
            {formatDate(decision.created_at)}
          </div>
          {decision.decision_note && (
            <p className="text-sm bg-gray-50 p-2 rounded">{decision.decision_note}</p>
          )}
        </div>
      </div>

      {/* Enforcements for this Decision */}
      {decision.enforcements && decision.enforcements.length > 0 && (
        <div className="mt-3 pt-3 border-t border-gray-100">
          <p className="text-xs font-medium text-gray-500 mb-2">Enforcement</p>
          {decision.enforcements.map((enf) => (
            <EnforcementRow key={enf.id} enforcement={enf} />
          ))}
        </div>
      )}
    </div>
  )
}

// ============================================================================
// ENFORCEMENT ROW COMPONENT
// ============================================================================

function EnforcementRow({ enforcement }: { enforcement: GovernanceEnforcement }) {
  return (
    <div className="flex items-center gap-3 text-sm">
      <Badge variant={enforcementStatusVariants[enforcement.status]}>
        {enforcementStatusLabels[enforcement.status]}
      </Badge>
      <span className="text-gray-500">
        {targetTypeLabels[enforcement.target_type]}
      </span>
      <span className="font-mono text-xs text-gray-400">
        {enforcement.target_id.slice(0, 8)}
      </span>
      <span className="text-gray-400">·</span>
      <span className="text-xs text-gray-500">
        attempt {enforcement.attempt_count}
      </span>
      {enforcement.last_error && (
        <>
          <span className="text-gray-400">·</span>
          <span className="text-xs text-red-500" title={enforcement.last_error}>
            Error: {enforcement.last_error.length > 50
              ? enforcement.last_error.slice(0, 50) + '...'
              : enforcement.last_error}
          </span>
        </>
      )}
    </div>
  )
}
