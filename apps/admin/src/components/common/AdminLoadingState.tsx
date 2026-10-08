import { Card, CardContent } from '@/components/ui/Card'

interface AdminLoadingStateProps {
  /** Number of skeleton rows. */
  rows?: number
  /** Label announced to assistive technology. Defaults to 'Loading'. */
  label?: string
  /**
   * Render only the skeleton body (no Card wrapper) so it can compose inside
   * an existing Card/panel. Use for panel-scoped loading where the panel
   * chrome must stay visible.
   */
  embedded?: boolean
  className?: string
}

/**
 * Canonical loading state for admin list/table/page fetches.
 *
 * A layout-preserving skeleton is used instead of a bare spinner because these
 * surfaces replace a repeating row layout. The region is announced once via
 * `role="status"`; the skeleton shapes are decorative (`aria-hidden`).
 *
 * Contract: callers render this only for the INITIAL load (no data yet). A
 * background refetch must keep existing data visible and must not mount this
 * state, so screen readers are not re-announced and the view does not blank.
 */
export function AdminLoadingState({
  rows = 5,
  label = 'Loading',
  embedded,
  className,
}: AdminLoadingStateProps) {
  const body = (
    <>
      <span className="sr-only">{label}…</span>
      <div className="space-y-4" aria-hidden="true">
        {Array.from({ length: rows }).map((_, i) => (
          <div key={i} className="animate-pulse flex items-center gap-4">
            <div className="h-6 w-16 bg-surface-muted rounded-full" />
            <div className="h-4 bg-surface-muted rounded flex-1" />
            <div className="h-6 w-20 bg-surface-muted rounded-full" />
          </div>
        ))}
      </div>
    </>
  )

  if (embedded) {
    return (
      <div role="status" aria-busy="true" className={className}>
        {body}
      </div>
    )
  }

  return (
    <Card className={className} role="status" aria-busy="true">
      <CardContent className="p-8">{body}</CardContent>
    </Card>
  )
}
