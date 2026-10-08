import { forwardRef, type ReactNode, type SelectHTMLAttributes } from 'react'
import { Field } from './Field'
import { fieldControlClasses, type FieldSize } from './formStyles'

interface SelectProps extends Omit<SelectHTMLAttributes<HTMLSelectElement>, 'size'> {
  label?: ReactNode
  error?: string | null
  help?: string
  size?: FieldSize
  /** Extra classes for the field wrapper (label + control + message). */
  containerClassName?: string
}

export const Select = forwardRef<HTMLSelectElement, SelectProps>(
  (
    { className, label, error, help, size, containerClassName, disabled, required, id, children, ...props },
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
        <select
          ref={ref}
          {...props}
          {...control}
          required={required}
          className={fieldControlClasses({ size, invalid: Boolean(error), className })}
        >
          {children}
        </select>
      )}
    </Field>
  )
)

Select.displayName = 'Select'
