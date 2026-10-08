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
import { Input } from '@/components/ui/Input'
import { Select } from '@/components/ui/Select'
import { Textarea } from '@/components/ui/Textarea'
import { useGovernanceCase, useCreateDecision, useGovernanceCaseAudit } from '@/hooks/useGovernance'
import { useAuth } from '@/hooks/useAuth'
import { hasCapability } from '@/lib/permissions'
import { formatDate } from '@/lib/utils'
import { AdminLoadingState, AdminErrorState, AdminEmptyState, PageHeader } from '@/components/common'
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
    return <AdminLoadingState />
  }

  if (error) {
    return (
      <div className="space-y-6">
        <PageHeader
          title="Governance Case"
          leading={
            <Button variant="secondary" onClick={() => navigate('/moderation/cases')}>
              <ArrowLeft className="h-4 w-4 mr-2" />
              Back to Cases
            </Button>
          }
        />
        <AdminErrorState title="Failed to load case" message={error.message} onRetry={refetch} />
      </div>
    )
  }

  if (!data) {
    return (
      <div className="space-y-6">
        <PageHeader
          title="Governance Case"
          leading={
            <Button variant="secondary" onClick={() => navigate('/moderation/cases')}>
              <ArrowLeft className="h-4 w-4 mr-2" />
              Back to Cases
            </Button>
          }
        />
        <AdminErrorState
          title="Case not found"
          message="The requested governance case could not be found."
        />
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
      <PageHeader
        title="Governance Case"
        leading={
          <Button variant="secondary" onClick={() => navigate('/moderation/cases')}>
            <ArrowLeft className="h-4 w-4 mr-2" />
            Back to Cases
          </Button>
        }
        actions={
          isOpen && canCreateDecision ? (
            <Button onClick={() => setShowDecisionForm(!showDecisionForm)}>
              <Gavel className="h-4 w-4 mr-2" />
              {showDecisionForm ? 'Cancel' : 'Create Decision'}
            </Button>
          ) : undefined
        }
      />

      {/* Read-only notice for admins without decision authority */}
      {isOpen && !canCreateDecision && (
        <div className="bg-surface-muted border border-border text-muted-foreground p-3 rounded-lg flex items-center gap-2">
          <AlertTriangle className="h-4 w-4 flex-shrink-0" />
          <span className="text-sm">
            You can view this case but do not have permission to create decisions
            (requires moderation.case.resolve).
          </span>
        </div>
      )}

      {/* Success banner */}
      {decisionSuccess && (
        <div className="bg-success-bg border border-success text-success p-4 rounded-lg flex items-center gap-2">
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
              <p className="type-secondary">Case ID</p>
              <p className="font-mono text-sm">{kase.id}</p>
            </div>
            <div>
              <p className="type-secondary">Subject Type</p>
              <Badge variant="default">{targetTypeLabels[kase.subject_type]}</Badge>
            </div>
            <div>
              <p className="type-secondary">Subject ID</p>
              <p className="font-mono text-sm">{kase.subject_id}</p>
            </div>
            <div>
              <p className="type-secondary">Status</p>
              <Badge variant={caseStatusVariants[kase.status]}>
                {caseStatusLabels[kase.status]}
              </Badge>
            </div>
          </div>
          <div className="grid grid-cols-2 md:grid-cols-3 gap-4">
            <div>
              <p className="type-secondary">Created</p>
              <p className="text-sm">{formatDate(kase.created_at)}</p>
            </div>
            <div>
              <p className="type-secondary">Updated</p>
              <p className="text-sm">{formatDate(kase.updated_at)}</p>
            </div>
            {kase.closed_at && (
              <div>
                <p className="type-secondary">Closed</p>
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
              <div className="bg-destructive-bg border border-destructive text-destructive p-3 rounded-lg flex items-center gap-2">
                <AlertTriangle className="h-4 w-4" />
                <span className="text-sm">{decisionError}</span>
              </div>
            )}

            {/* Outcome */}
            <Select
              label="Outcome"
              required
              value={decisionOutcome}
              onChange={(e) => setDecisionOutcome(e.target.value as DecisionOutcome)}
              help={
                decisionOutcome === 'violation'
                  ? 'Policy was violated — enforcement will be created'
                  : 'Content complies with policy — no enforcement needed'
              }
            >
              <option value="no_violation">No Violation</option>
              <option value="violation">Violation</option>
            </Select>

            {/* Target (violation only) */}
            {decisionOutcome === 'violation' && (
              <>
                <Select
                  label="Target Type"
                  required
                  value={targetType}
                  onChange={(e) => setTargetType(e.target.value as GovernanceTargetType)}
                >
                  <option value="content">Content</option>
                  <option value="comment">Comment</option>
                  <option value="for_sale">For Sale</option>
                  <option value="auction">Auction</option>
                  <option value="user">User</option>
                </Select>
                <Input
                  type="text"
                  label="Target ID"
                  required
                  value={targetId}
                  onChange={(e) => setTargetId(e.target.value)}
                  placeholder="UUID of the target to enforce against"
                  help="The subject ID to apply enforcement to (often the Case's subject_id)"
                />
              </>
            )}

            {/* Decision Note */}
            <div>
              <Textarea
                label="Decision Note (optional)"
                value={decisionNote}
                onChange={(e) => setDecisionNote(e.target.value)}
                placeholder="Reason or note for this decision..."
                rows={3}
                maxLength={2000}
                className="resize-none"
              />
              <p className="type-caption mt-1">{decisionNote.length}/2000 characters</p>
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
            <p className="type-secondary">No reports associated with this case.</p>
          ) : (
            <div className="space-y-3">
              {reports.map((report) => (
                <div key={report.id} className="border border-border rounded-lg p-4">
                  <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
                    <div>
                      <p className="type-caption">Report ID</p>
                      <p className="font-mono text-xs">{report.id.slice(0, 8)}</p>
                    </div>
                    <div>
                      <p className="type-caption">Reporter</p>
                      <p className="font-mono text-xs">{report.reporter_id.slice(0, 8)}</p>
                    </div>
                    <div>
                      <p className="type-caption">Reason</p>
                      <p className="text-xs font-medium">{report.reason_code}</p>
                    </div>
                    <div>
                      <p className="type-caption">Created</p>
                      <p className="text-xs">{formatDate(report.created_at)}</p>
                    </div>
                  </div>
                  {report.reason_note && (
                    <div className="mt-2">
                      <p className="type-caption">Note</p>
                      <p className="text-sm bg-surface-muted p-2 rounded">{report.reason_note}</p>
                    </div>
                  )}
                  {report.evidence_snapshot && (
                    <div className="mt-2 type-caption">
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
            <AdminEmptyState
              icon={Clock}
              title="No decisions made yet."
              description={
                isOpen
                  ? 'Click "Create Decision" to make a governance decision.'
                  : undefined
              }
            />
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
          <AdminLoadingState embedded label="Loading audit events" />
        </CardContent>
      </Card>
    )
  }

  if (error) {
    return <AdminErrorState title="Failed to load audit events" message={error.message} />
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
          <AdminEmptyState icon={Clock} title="No audit events recorded for this case." />
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
    <div className="border border-border rounded-lg p-4">
      <div className="flex items-start justify-between">
        <div className="space-y-1">
          <div className="flex items-center gap-3">
            <Badge variant="default">{event.event_type}</Badge>
            {event.outcome && (
              <Badge variant={outcomeVariant}>{outcomeLabel}</Badge>
            )}
          </div>
          <div className="flex items-center gap-2 type-caption">
            <span className="font-medium capitalize">{event.actor_type}</span>
            {event.actor_name && (
              <span>({event.actor_name})</span>
            )}
            {event.actor_id && !event.actor_name && (
              <span className="font-mono">{event.actor_id.slice(0, 8)}</span>
            )}
          </div>
          {event.target_type && (
            <div className="type-caption">
              Target: {targetTypeLabels[event.target_type] || event.target_type}
              {event.target_id && (
                <span className="font-mono ml-1">{event.target_id.slice(0, 8)}</span>
              )}
            </div>
          )}
          {event.decision_note && (
            <p className="text-sm bg-surface-muted p-2 rounded mt-1">{event.decision_note}</p>
          )}
        </div>
        <span className="type-caption whitespace-nowrap">
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
    <div className="border border-border rounded-lg p-4">
      <div className="flex items-start justify-between">
        <div className="space-y-2">
          <div className="flex items-center gap-3">
            <Badge variant={decisionOutcomeVariants[decision.outcome]}>
              {decisionOutcomeLabels[decision.outcome]}
            </Badge>
            <span className="type-caption font-mono">{decision.id.slice(0, 8)}</span>
            <span className="type-caption">·</span>
            <span className="type-caption">
              by {decision.decided_by.slice(0, 8)}
            </span>
          </div>
          <div className="type-caption">
            {formatDate(decision.created_at)}
          </div>
          {decision.decision_note && (
            <p className="text-sm bg-surface-muted p-2 rounded">{decision.decision_note}</p>
          )}
        </div>
      </div>

      {/* Enforcements for this Decision */}
      {decision.enforcements && decision.enforcements.length > 0 && (
        <div className="mt-3 pt-3 border-t border-border">
          <p className="text-xs font-medium text-muted-foreground mb-2">Enforcement</p>
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
      <span className="text-muted-foreground">
        {targetTypeLabels[enforcement.target_type]}
      </span>
      <span className="font-mono type-caption">
        {enforcement.target_id.slice(0, 8)}
      </span>
      <span className="text-muted-foreground">·</span>
      <span className="type-caption">
        attempt {enforcement.attempt_count}
      </span>
      {enforcement.last_error && (
        <>
          <span className="text-muted-foreground">·</span>
          <span className="text-xs text-destructive" title={enforcement.last_error}>
            Error: {enforcement.last_error.length > 50
              ? enforcement.last_error.slice(0, 50) + '...'
              : enforcement.last_error}
          </span>
        </>
      )}
    </div>
  )
}
