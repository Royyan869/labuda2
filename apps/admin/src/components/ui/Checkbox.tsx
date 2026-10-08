import { forwardRef, useId, type InputHTMLAttributes, type ReactNode } from 'react'
import { cn } from '@/lib/utils'

interface CheckboxProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'type'> {
  /** Inline label rendered next to the box (implicit native association). */
  label?: ReactNode
  /** Extra classes for the wrapping label when a label is provided. */
  containerClassName?: string
}

/**
 * Canonical native checkbox. Checked color is the brand primary via
 * `accent-color`; focus is the dedicated ring token. Business status belongs
 * in the label/context, never in the checked color.
 */
export const Checkbox = forwardRef<HTMLInputElement, CheckboxProps>(
  ({ className, label, containerClassName, id, disabled, ...props }, ref) => {
    const autoId = useId()
    const checkboxId = id ?? autoId

    const box = (
      <input
        ref={ref}
        id={checkboxId}
        type="checkbox"
        disabled={disabled}
        className={cn(
          'h-4 w-4 shrink-0 rounded border-border accent-primary',
          'focus:outline-none focus:ring-2 focus:ring-ring',
          'disabled:cursor-not-allowed disabled:opacity-50',
          className
        )}
        {...props}
      />
    )

    if (!label) return box

    return (
      <label
        htmlFor={checkboxId}
        className={cn(
          'flex items-center gap-2 text-sm font-medium',
          disabled
            ? 'text-muted-foreground cursor-not-allowed'
            : 'text-foreground cursor-pointer',
          containerClassName
        )}
      >
        {box}
        {label}
      </label>
    )
  }
)

Checkbox.displayName = 'Checkbox'
