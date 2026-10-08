import { forwardRef, type InputHTMLAttributes, type ReactNode } from 'react'
import { Field } from './Field'
import { fieldControlClasses, type FieldSize } from './formStyles'

interface InputProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'size'> {
  label?: ReactNode
  error?: string | null
  help?: string
  size?: FieldSize
  /** Extra classes for the field wrapper (label + control + message). */
  containerClassName?: string
}

export const Input = forwardRef<HTMLInputElement, InputProps>(
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
        <input
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

Input.displayName = 'Input'
