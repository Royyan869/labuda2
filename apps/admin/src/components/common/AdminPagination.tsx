import { Button } from '@/components/ui/Button'
import { cn } from '@/lib/utils'

interface AdminPaginationProps {
  page: number
  totalPages: number
  onPageChange: (page: number) => void
  /** Disables both buttons (e.g. while a background refetch is in progress). */
  disabled?: boolean
  className?: string
}

/**
 * Simple previous/next pagination bar for admin list pages.
 * Presentation-only — no domain semantics.
 */
export function AdminPagination({
  page,
  totalPages,
  onPageChange,
  disabled,
  className,
}: AdminPaginationProps) {
  return (
    <div className={cn('flex items-center justify-between', className)}>
      <Button
        variant="secondary"
        size="sm"
        onClick={() => onPageChange(Math.max(1, page - 1))}
        disabled={disabled || page <= 1}
      >
        Previous
      </Button>
      <span className="text-sm text-gray-600">
        Page {page} of {totalPages}
      </span>
      <Button
        variant="secondary"
        size="sm"
        onClick={() => onPageChange(Math.min(totalPages, page + 1))}
        disabled={disabled || page >= totalPages}
      >
        Next
      </Button>
    </div>
  )
}
