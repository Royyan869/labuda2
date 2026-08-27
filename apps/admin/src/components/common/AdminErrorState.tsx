import { AlertTriangle } from 'lucide-react'
import { Card, CardContent } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'

interface AdminErrorStateProps {
  /** Heading text. Defaults to 'Failed to load'. */
  title?: string
  /** Error detail message (e.g. error.message from the hook). */
  message: string
  onRetry?: () => void
  className?: string
}

/**
 * Generic error card for admin pages that fail to load data.
 * Presentation-only — no domain semantics.
 */
export function AdminErrorState({
  title = 'Failed to load',
  message,
  onRetry,
  className,
}: AdminErrorStateProps) {
  return (
    <Card className={className}>
      <CardContent className="p-8 text-center">
        <AlertTriangle className="h-10 w-10 text-red-400 mx-auto mb-3" />
        <p className="text-gray-900 font-medium">{title}</p>
        <p className="text-gray-600 text-sm mt-1">{message}</p>
        {onRetry && (
          <Button variant="secondary" size="sm" onClick={onRetry} className="mt-4">
            Retry
          </Button>
        )}
      </CardContent>
    </Card>
  )
}
