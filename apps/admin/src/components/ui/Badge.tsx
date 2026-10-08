import { type HTMLAttributes, forwardRef } from 'react'
import { cn } from '@/lib/utils'

interface BadgeProps extends HTMLAttributes<HTMLSpanElement> {
  variant?: 'success' | 'warning' | 'error' | 'info' | 'pending' | 'default'
}

export const Badge = forwardRef<HTMLSpanElement, BadgeProps>(
  ({ className, variant = 'default', ...props }, ref) => {
    const variants = {
      success: 'bg-success-bg text-success border-success',
      warning: 'bg-warning-bg text-warning border-warning',
      error: 'bg-destructive-bg text-destructive border-destructive',
      info: 'bg-info-bg text-info border-info',
      pending: 'bg-warning-bg text-warning border-warning',
      default: 'bg-surface-muted text-foreground border-border',
    }

    return (
      <span
        ref={ref}
        className={cn(
          'inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium border',
          variants[variant],
          className
        )}
        {...props}
      />
    )
  }
)

Badge.displayName = 'Badge'
