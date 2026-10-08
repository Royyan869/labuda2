import { forwardRef, type ReactNode, type TextareaHTMLAttributes } from 'react'
import { Field } from './Field'
import { fieldControlClasses, type FieldSize } from './formStyles'

interface TextareaProps extends TextareaHTMLAttributes<HTMLTextAreaElement> {
  label?: ReactNode
  error?: string | null
  help?: string
  size?: FieldSize
  /** Extra classes for the field wrapper (label + control + message). */
  containerClassName?: string
}

export const Textarea = forwardRef<HTMLTextAreaElement, TextareaProps>(
  (
    { className, label, error, help, size, containerClassName, disabled, required, id, ...props },
    ref
  ) => (
    <Field
      label={label}
      error={error}
      help={help}
      required={required}
      disabled={disabled}
      className={containerClassName}
      id={id}
    >
      {(control) => (
        <textarea
          ref={ref}
          {...props}
          {...control}
          required={required}
          className={fieldControlClasses({ size, invalid: Boolean(error), className })}
        />
      )}
    </Field>
  )
)

Textarea.displayName = 'Textarea'
