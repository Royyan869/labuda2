import { useId, type ReactNode } from 'react'
import { cn } from '@/lib/utils'

export interface FieldControlProps {
  id: string
  disabled?: boolean
  'aria-invalid'?: true
  'aria-describedby'?: string
}

interface FieldProps {
  label?: ReactNode
  error?: string | null
  help?: string
  required?: boolean
  disabled?: boolean
  /** Extra classes for the field wrapper (label + control + message). */
  className?: string
  /** Optional control id; auto-generated when omitted. */
  id?: string
  /** Render prop receiving the canonical wiring for the control. */
  children: (control: FieldControlProps) => ReactNode
}

/**
 * Canonical relationship layer: label -> control -> error/help.
 * Owns the id, `htmlFor`, `aria-describedby` and `aria-invalid` wiring so no
 * consumer has to hand-maintain label association.
 *
 * The required marker is rendered as a CSS pseudo-element so it never becomes
 * part of the accessible name (keeps `getByLabelText('Label')` working).
 */
export function Field({
  label,
  error,
  help,
  required,
  disabled,
  className,
  id,
  children,
}: FieldProps) {
  const autoId = useId()
  const controlId = id ?? autoId
  const errorId = `${controlId}-error`
  const helpId = `${controlId}-help`
  const describedBy = error ? errorId : help ? helpId : undefined

  return (
    <div className={cn(className)}>
      {label && (
        <label
          htmlFor={controlId}
          className={cn(
            'block text-sm font-medium mb-1',
            disabled ? 'text-muted-foreground' : 'text-foreground',
            required &&
              "after:ml-0.5 after:text-destructive after:content-['*']"
          )}
        >
          {label}
        </label>
      )}
      {children({
        id: controlId,
        disabled,
        'aria-invalid': error ? true : undefined,
        'aria-describedby': describedBy,
      })}
      {error ? (
        <p id={errorId} className="mt-1 text-sm text-destructive">
          {error}
        </p>
      ) : help ? (
        <p id={helpId} className="mt-1 text-sm text-muted-foreground">
          {help}
        </p>
      ) : null}
    </div>
  )
}
