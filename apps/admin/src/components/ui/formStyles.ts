import { cn } from '@/lib/utils'

export type FieldSize = 'normal' | 'compact'

const SIZE_CLASSES: Record<FieldSize, string> = {
  normal: 'px-3 py-2 rounded-lg',
  compact: 'px-3 py-1.5 rounded-md',
}

/**
 * The single canonical style authority for text-like form controls
 * (Input / Select / Textarea). Field state tokens: background, foreground,
 * placeholder, border, focus ring, disabled and invalid.
 */
export function fieldControlClasses({
  size = 'normal',
  invalid,
  className,
}: {
  size?: FieldSize
  invalid?: boolean
  className?: string
} = {}) {
  return cn(
    'w-full border border-border bg-background text-foreground text-sm',
    'placeholder:text-muted-foreground',
    'focus:outline-none focus:ring-2 focus:ring-ring focus:border-transparent',
    'disabled:bg-surface-muted disabled:cursor-not-allowed',
    SIZE_CLASSES[size],
    invalid && 'border-destructive focus:ring-destructive',
    className
  )
}
