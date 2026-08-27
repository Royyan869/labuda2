import { Card, CardContent } from '@/components/ui/Card'

interface AdminLoadingStateProps {
  rows?: number
  className?: string
}

/**
 * Generic skeleton loading card for admin list/table pages.
 * Presentation-only — no domain semantics.
 */
export function AdminLoadingState({ rows = 5, className }: AdminLoadingStateProps) {
  return (
    <Card className={className}>
      <CardContent className="p-8">
        <div className="space-y-4">
          {Array.from({ length: rows }).map((_, i) => (
            <div key={i} className="animate-pulse flex items-center gap-4">
              <div className="h-6 w-16 bg-gray-200 rounded-full" />
              <div className="h-4 bg-gray-200 rounded flex-1" />
              <div className="h-6 w-20 bg-gray-200 rounded-full" />
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  )
}
