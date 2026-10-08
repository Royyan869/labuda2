import { type ComponentType } from 'react'
import { Button } from '@/components/ui/Button'
import { cn } from '@/lib/utils'

interface AdminEmptyStateProps {
  /** Lucide (or any) icon component. */
  icon?: ComponentType<{ className?: string }>
  title: string
  description?: string
  /**
   * True when the request succeeded but the active filters/search matched
   * nothing. This is the NO-RESULT state and is distinct from a genuinely
   * empty dataset, which is `filtered` omitted/false.
   */
  filtered?: boolean
  /**
   * Clears the active filters/search. Rendered as a reset action only when a
   * filtered no-result state is shown.
   */
  onClearFilters?: () => void
  className?: string
}

/**
 * Canonical empty / no-result state for admin lists, tables, and panels.
 *
 * Content-only by contract: the caller owns the surrounding container
 * (Card/CardContent) so this composes identically inside a list card and a
 * standalone section. Do not wrap it in another Card.
 *
 * Semantics:
 * - filtered=false → request succeeded, dataset is genuinely empty ("No data yet").
 * - filtered=true  → request succeeded, data exists, filters matched nothing
 *   ("No results match the current filters"), with a reset affordance.
 */
export function AdminEmptyState({
  icon: Icon,
  title,
  description,
  filtered,
  onClearFilters,
  className,
}: AdminEmptyStateProps) {
  return (
    <div className={cn('text-center py-12', className)} role="status">
      {Icon && (
        <Icon className="h-12 w-12 text-muted-foreground mx-auto mb-4" aria-hidden="true" />
      )}
      <h3 className="type-section-title mb-2">{title}</h3>
      {description && <p className="text-muted-foreground">{description}</p>}
      {filtered && onClearFilters && (
        <Button variant="secondary" size="sm" onClick={onClearFilters} className="mt-4">
          Clear filters
        </Button>
      )}
    </div>
  )
}
