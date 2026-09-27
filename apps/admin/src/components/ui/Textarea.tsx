import { type TextareaHTMLAttributes, forwardRef } from 'react'
import { cn } from '@/lib/utils'

interface TextareaProps extends TextareaHTMLAttributes<HTMLTextAreaElement> {
  label?: string
  error?: string
}

export const Textarea = forwardRef<HTMLTextAreaElement, TextareaProps>(
  ({ className, label, error, disabled, ...props }, ref) => {
    return (
      <div className="w-full">
        {label && (
          <label className="block text-sm font-medium text-[hsl(var(--foreground))] mb-1">
            {label}
          </label>
        )}
        <textarea
          ref={ref}
          className={cn(
            'w-full px-3 py-2 border border-[hsl(var(--border))] rounded-lg bg-[hsl(var(--surface))]',
            'text-sm text-[hsl(var(--foreground))] placeholder:text-[hsl(var(--muted-foreground))]',
            'focus:outline-none focus:ring-2 focus:ring-[hsl(var(--ring))] focus:border-transparent',
            'disabled:bg-[hsl(var(--surface-muted))] disabled:cursor-not-allowed',
            error && 'border-[hsl(var(--destructive))] focus:ring-[hsl(var(--destructive))]',
            className
          )}
          disabled={disabled}
          {...props}
        />
        {error && (
          <p className="mt-1 text-sm text-[hsl(var(--destructive))]">{error}</p>
        )}
      </div>
    )
  }
)

Textarea.displayName = 'Textarea'
