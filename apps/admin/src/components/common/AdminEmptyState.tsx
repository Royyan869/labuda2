import { type ComponentType } from 'react'
import { Card, CardContent } from '@/components/ui/Card'

interface AdminEmptyStateProps {
  /** Lucide (or any) icon component. */
  icon?: ComponentType<{ className?: string }>
  title: string
  description?: string
  className?: string
}

/**
 * Generic empty-state card for admin list/table pages.
 * Presentation-only — no domain semantics.
 */
export function AdminEmptyState({ icon: Icon, title, description, className }: AdminEmptyStateProps) {
  return (
    <Card className={className}>
      <CardContent className="p-12 text-center">
        {Icon && <Icon className="h-12 w-12 text-muted-foreground mx-auto mb-4" />}
        <h2 className="text-lg font-semibold text-foreground">{title}</h2>
        {description && <p className="text-muted-foreground mt-1">{description}</p>}
      </CardContent>
    </Card>
  )
}
