import { type HTMLAttributes, forwardRef } from 'react'
import { cn } from '@/lib/utils'

interface BadgeProps extends HTMLAttributes<HTMLSpanElement> {
  variant?: 'success' | 'warning' | 'error' | 'info' | 'pending' | 'default'
}

export const Badge = forwardRef<HTMLSpanElement, BadgeProps>(
  ({ className, variant = 'default', ...props }, ref) => {
    const variants = {
      success: 'bg-[hsl(var(--success-bg))] text-[hsl(var(--success))] border-[hsl(var(--success))]',
      warning: 'bg-[hsl(var(--warning-bg))] text-[hsl(var(--warning))] border-[hsl(var(--warning))]',
      error: 'bg-[hsl(var(--destructive-bg))] text-[hsl(var(--destructive))] border-[hsl(var(--destructive))]',
      info: 'bg-[hsl(var(--info-bg))] text-[hsl(var(--info))] border-[hsl(var(--info))]',
      pending: 'bg-[hsl(var(--warning-bg))] text-[hsl(var(--warning))] border-[hsl(var(--warning))]',
      default: 'bg-[hsl(var(--surface-muted))] text-[hsl(var(--foreground))] border-[hsl(var(--border))]',
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
