import { useState } from 'react'
import { Clock, ChevronDown, ChevronRight, User, Calendar, RefreshCw, AlertTriangle, CheckCircle, Globe } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import type { TimelineEvent } from '@/types'
import { formatRupiah } from '@/lib/utils'

type WebhookEventType = 'webhook.received' | 'webhook.retry' | 'webhook.processed' | 'webhook.failed'

const WEBHOOK_EVENT_TYPES: readonly WebhookEventType[] = [
  'webhook.received',
  'webhook.retry',
  'webhook.processed',
  'webhook.failed',
]

function isWebhookEventType(eventType: string): eventType is WebhookEventType {
  return WEBHOOK_EVENT_TYPES.includes(eventType as WebhookEventType)
}

function getWebhookEventDescription(event: WebhookEventType, metadata?: Record<string, unknown> | null): string {
  switch (event) {
    case 'webhook.received':
      return 'Webhook received from payment provider'
    case 'webhook.retry': {
      const attempt = metadata?.attempt as number | undefined
      return attempt ? `Webhook delivery retry (attempt ${attempt})` : 'Webhook delivery retry'
    }
    case 'webhook.processed':
      return 'Webhook processed successfully'
    case 'webhook.failed': {
      const error = metadata?.error as string | undefined
      return error ? `Webhook delivery failed: ${error}` : 'Webhook delivery failed'
    }
    default:
      return event
  }
}

function renderEventIcon(eventType: string, iconColor: string) {
  if (!isWebhookEventType(eventType)) {
    return <div className="w-3 h-3 rounded-full bg-primary border-2 border-white shadow-sm" />
  }

  switch (eventType) {
    case 'webhook.received':
      return (
        <div className="p-1 rounded-full bg-white border-2 border-gray-200 shadow-sm">
          <Globe className={`h-3.5 w-3.5 ${iconColor}`} />
        </div>
      )
    case 'webhook.retry':
      return (
        <div className="p-1 rounded-full bg-white border-2 border-gray-200 shadow-sm">
          <RefreshCw className={`h-3.5 w-3.5 ${iconColor}`} />
        </div>
      )
    case 'webhook.failed':
      return (
        <div className="p-1 rounded-full bg-white border-2 border-gray-200 shadow-sm">
          <AlertTriangle className={`h-3.5 w-3.5 ${iconColor}`} />
        </div>
      )
    case 'webhook.processed':
      return (
        <div className="p-1 rounded-full bg-white border-2 border-gray-200 shadow-sm">
          <CheckCircle className={`h-3.5 w-3.5 ${iconColor}`} />
        </div>
      )
    default:
      return <div className="w-3 h-3 rounded-full bg-primary border-2 border-white shadow-sm" />
  }
}

interface TimelinePanelProps {
  events: TimelineEvent[]
  loading: boolean
}

interface TimelineItemProps {
  event: TimelineEvent
  isLast: boolean
}

// Get icon color class
function getEventIconColor(eventType: string): string {
  if (eventType.startsWith('webhook.')) {
    switch (eventType) {
      case 'webhook.received':
        return 'text-blue-500'
      case 'webhook.retry':
        return 'text-amber-500'
      case 'webhook.failed':
        return 'text-red-500'
      case 'webhook.processed':
        return 'text-green-500'
      default:
        return 'text-gray-500'
    }
  }
  return 'text-primary'
}

// Human-readable descriptions for timeline events
function getEventDescription(event: TimelineEvent): string {
  const { event: eventType, metadata } = event

  if (isWebhookEventType(eventType)) {
    return getWebhookEventDescription(eventType, metadata)
  }

  switch (eventType) {
    case 'order.created':
      return 'Order was created'

    case 'payment.settled': {
      const amount = metadata?.amount as number | undefined
      return amount
        ? `Payment of ${formatRupiah(amount)} completed`
        : 'Payment completed'
    }

    case 'payment.failed':
      return 'Payment failed'

    case 'payment.pending':
      return 'Payment initiated'

    case 'order.shipped':
      return 'Seller submitted shipping proof'

    case 'shipment.delivered':
      return 'Package marked as delivered'

    case 'order.completed':
      return 'Order completed'

    case 'order.cancelled':
      return 'Order was cancelled'

    case 'order.expired':
      return 'Order expired'

    case 'order.refunded':
      return 'Order was refunded'

    case 'dispute.opened':
      return 'Dispute was opened'

    case 'dispute.resolved':
      return metadata?.resolution === 'refund'
        ? 'Dispute resolved - refund issued'
        : 'Dispute resolved - released to seller'

    case 'escrow.released':
      return 'Payment released to seller'

    case 'escrow.refunded':
      return 'Payment refunded to buyer'

    default:
      // Fallback: capitalize first letter and replace dots with spaces
      // Also replace 'escrow' with 'payment' for user-friendly display
      return eventType
        .replace(/^escrow\./, 'payment.')
        .split('.')
        .map(word => word.charAt(0).toUpperCase() + word.slice(1))
        .join(' ')
  }
}

// Extract auto-release date from shipment.proof_submitted event
function getAutoReleaseDate(event: TimelineEvent): Date | null {
  if (event.event === 'shipment.proof_submitted' && event.metadata?.auto_release_at) {
    const date = new Date(event.metadata.auto_release_at as string)
    if (!isNaN(date.getTime())) {
      return date
    }
  }
  return null
}

function TimelineItem({ event, isLast }: TimelineItemProps) {
  const [isExpanded, setIsExpanded] = useState(false)
  const hasMetadata = event.metadata && Object.keys(event.metadata).length > 0
  const description = getEventDescription(event)
  const autoReleaseDate = getAutoReleaseDate(event)
  const iconColor = getEventIconColor(event.event)
  const isWebhookEvent = event.event.startsWith('webhook.')

  return (
    <div className="flex gap-3">
      {/* Timeline connector */}
      <div className="flex flex-col items-center">
        {renderEventIcon(event.event, iconColor)}
        {!isLast && <div className="w-0.5 flex-1 bg-gray-200 min-h-[48px]" />}
      </div>

      {/* Timeline content */}
      <div className="flex-1 pb-6">
        {/* Event header */}
        <div className="flex items-start justify-between gap-2">
          <div className="flex-1">
            {/* Human-readable description (primary) */}
            <p className={`text-sm font-medium ${isWebhookEvent ? 'text-gray-900' : 'text-gray-900'}`}>
              {description}
            </p>

            {/* Event type code (secondary, smaller) */}
            <p className="text-xs text-gray-400 mt-0.5 font-mono">{event.event}</p>

            {/* Timestamp */}
            <p className="text-xs text-gray-500 mt-0.5">
              {new Date(event.timestamp).toLocaleString('id-ID')}
            </p>

            {/* Actor */}
            {event.actor_name && (
              <div className="flex items-center gap-1 mt-1">
                <User className="h-3 w-3 text-gray-400" />
                <p className="text-xs text-gray-500">{event.actor_name}</p>
              </div>
            )}

            {/* Auto-release date for proof_submitted event */}
            {autoReleaseDate && (
              <div className="flex items-center gap-1.5 mt-2 p-2 bg-blue-50 rounded-md">
                <Calendar className="h-3 w-3 text-blue-600" />
                <p className="text-xs text-blue-700">
                  Auto-release: {autoReleaseDate.toLocaleDateString('id-ID')}
                </p>
              </div>
            )}

            {/* Webhook-specific info */}
            {isWebhookEvent && event.metadata && (
              <div className="mt-2 space-y-1">
                {!!event.metadata.http_status && (
                  <div className="text-xs text-gray-600">
                    HTTP Status: <span className="font-mono">{event.metadata.http_status as string}</span>
                  </div>
                )}
                {!!event.metadata.retry_count && (
                  <div className="text-xs text-amber-600">
                    Retry attempt: {event.metadata.retry_count as string}
                  </div>
                )}
                {!!event.metadata.error && (
                  <div className="text-xs text-red-600">
                    Error: {event.metadata.error as string}
                  </div>
                )}
              </div>
            )}
          </div>

          {/* Expand button for metadata */}
          {hasMetadata && (
            <button
              onClick={() => setIsExpanded(!isExpanded)}
              className="p-1 hover:bg-gray-100 rounded transition-colors"
              aria-label={isExpanded ? 'Collapse payload' : 'Expand payload'}
            >
              {isExpanded ? (
                <ChevronDown className="h-4 w-4 text-gray-500" />
              ) : (
                <ChevronRight className="h-4 w-4 text-gray-500" />
              )}
            </button>
          )}
        </div>

        {/* Expandable metadata payload (secondary, collapsible) */}
        {hasMetadata && isExpanded && (
          <div className="mt-3 p-3 bg-gray-50 rounded-lg border border-gray-200">
            <p className="text-xs font-medium text-gray-500 mb-2">Raw Payload</p>
            <pre className="text-xs font-mono text-gray-600 overflow-x-auto whitespace-pre-wrap">
              {JSON.stringify(event.metadata, null, 2)}
            </pre>
          </div>
        )}
      </div>
    </div>
  )
}

export function TimelinePanel({ events, loading }: TimelinePanelProps) {
  if (loading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="text-lg flex items-center gap-2">
            <Clock className="h-5 w-5" />
            Timeline
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex items-center justify-center py-8">
            <div className="inline-block h-6 w-6 animate-spin rounded-full border-3 border-solid border-primary border-r-transparent" />
          </div>
        </CardContent>
      </Card>
    )
  }

  if (!events || events.length === 0) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="text-lg flex items-center gap-2">
            <Clock className="h-5 w-5" />
            Timeline
          </CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-gray-500">No timeline events available</p>
        </CardContent>
      </Card>
    )
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-lg flex items-center gap-2">
          <Clock className="h-5 w-5" />
          Timeline
          <span className="text-sm font-normal text-gray-500">({events.length} events)</span>
        </CardTitle>
      </CardHeader>
      <CardContent>
        <div className="space-y-1">
          {events.map((event, index) => (
            <TimelineItem
              key={index}
              event={event}
              isLast={index === events.length - 1}
            />
          ))}
        </div>
      </CardContent>
    </Card>
  )
}
