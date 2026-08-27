import { useState, useEffect, useCallback } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { AlertTriangle } from 'lucide-react'
import { useDisputeDetail } from '@/hooks/useDisputes'
import { getOrderTimeline, approveDispute, rejectDispute, resolveDisputePartialSplit } from '@/lib/api'
import type { TimelineEvent, DisputeDecision } from '@/types'
import { DisputeHeader } from '@/components/disputes/DisputeHeader'
import { BuyerEvidencePanel } from '@/components/disputes/BuyerEvidencePanel'
import { SellerEvidencePanel } from '@/components/disputes/SellerEvidencePanel'
import { TimelinePanel } from '@/components/orders/TimelinePanel'
import { DecisionPanel } from '@/components/disputes/DecisionPanel'

export function DisputeWorkspacePage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()

  const [timeline, setTimeline] = useState<TimelineEvent[]>([])
  const [timelineLoading, setTimelineLoading] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [refreshing, setRefreshing] = useState(false)

  const { dispute, loading, refetch } = useDisputeDetail(id || null)

  const fetchTimeline = useCallback(async () => {
    if (!dispute?.order_id) return

    setTimelineLoading(true)
    try {
      const data = await getOrderTimeline(dispute.order_id)
      setTimeline(data || [])
    } catch {
      // Error loading timeline - non-critical
    } finally {
      setTimelineLoading(false)
    }
  }, [dispute?.order_id])

  // Fetch order timeline when we have dispute data
  useEffect(() => {
    if (dispute?.order_id) {
      fetchTimeline()
    }
  }, [dispute?.order_id, fetchTimeline])

  const handleRefresh = async () => {
    setRefreshing(true)
    setError(null)
    try {
      await refetch()
      await fetchTimeline()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to refresh')
    } finally {
      setRefreshing(false)
    }
  }

  const handleSubmitDecision = async (decision: DisputeDecision, notes: string) => {
    if (!dispute?.id) return

    setSubmitting(true)
    setError(null)

    try {
      switch (decision) {
        case 'refund_full':
          await approveDispute(dispute.id, notes)
          break
        case 'reject':
          await rejectDispute(dispute.id, notes)
          break
        case 'refund_partial':
          await resolveDisputePartialSplit(dispute.id, notes)
          break
      }
      navigate('/disputes')
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to submit decision'
      setError(message)
      setSubmitting(false)
    }
  }

  const handleBack = () => {
    navigate('/disputes')
  }

  // Loading state
  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-[500px]">
        <div className="text-center">
          <div className="inline-block h-8 w-8 animate-spin rounded-full border-4 border-solid border-primary border-r-transparent"></div>
          <p className="mt-4 text-gray-600">Loading dispute workspace...</p>
        </div>
      </div>
    )
  }

  // Error state (no dispute data)
  if (!dispute) {
    return (
      <div className="space-y-6">
        <div>
          <h1 className="text-3xl font-bold text-gray-900">Dispute Workspace</h1>
        </div>
        <div className="bg-red-50 border border-red-200 text-red-700 p-4 rounded-lg flex items-center gap-2">
          <AlertTriangle className="h-5 w-5 flex-shrink-0" />
          <span className="text-sm">Dispute not found. Please check the dispute ID and try again.</span>
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-6 pb-32">
      {/* Error Banner */}
      {error && (
        <div className="bg-red-50 border border-red-200 text-red-700 p-4 rounded-lg flex items-center gap-2">
          <AlertTriangle className="h-5 w-5 flex-shrink-0" />
          <span className="text-sm">{error}</span>
        </div>
      )}

      {/* Header */}
      <DisputeHeader
        dispute={dispute}
        onRefresh={handleRefresh}
        refreshing={refreshing}
        onBack={handleBack}
      />

      {/* Main Content Grid */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Left Column: Buyer Evidence */}
        <div className="space-y-6">
          <BuyerEvidencePanel dispute={dispute} />
        </div>

        {/* Right Column: Seller Evidence */}
        <div className="space-y-6">
          <SellerEvidencePanel dispute={dispute} />
        </div>
      </div>

      {/* Timeline - Full Width */}
      <TimelinePanel events={timeline} loading={timelineLoading} />

      {/* Decision Panel - Sticky Bottom */}
      <DecisionPanel
        dispute={dispute}
        onSubmit={handleSubmitDecision}
        submitting={submitting}
      />
    </div>
  )
}
