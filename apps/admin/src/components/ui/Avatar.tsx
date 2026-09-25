import { User } from 'lucide-react'
import { cn } from '@/lib/utils'

// Size map — the only allowed avatar sizes in the admin app.
const SIZES = {
  sm: 'h-8 w-8',
  md: 'h-10 w-10',
  lg: 'h-16 w-16',
} as const

// Icon scales with the circle, mirroring mobile's ProfileAvatar ratio.
const ICON_SIZES = {
  sm: 'h-4 w-4',
  md: 'h-5 w-5',
  lg: 'h-8 w-8',
} as const

export type AvatarSize = keyof typeof SIZES

interface AvatarProps {
  /** Canonical user photo URL (user_profiles.avatar_url / photo_url). */
  src?: string | null
  /** Stable user id — used as the img fallback alt, never rendered as text. */
  userId?: string
  /** Display name for the img alt attribute only. */
  name?: string
  size?: AvatarSize
  className?: string
}

/**
 * CANONICAL avatar for the admin web app.
 *
 * Business truth (Owner decision 2026-09-24, same doctrine as mobile):
 * - A user avatar is ALWAYS the user's photo, or the person icon when there
 *   is no photo. Initials do not exist anywhere — not even for admins.
 * - The dashboard admin account is read-only here (no avatar upload), but it
 *   still renders through this component: one truth, zero exceptions.
 */
export function Avatar({ src, userId, name, size = 'sm', className }: AvatarProps) {
  const normalized = src?.trim() || null

  if (normalized) {
    return (
      <img
        src={normalized}
        alt={name || userId || 'user avatar'}
        className={cn(
          'rounded-full object-cover shrink-0',
          SIZES[size],
          className
        )}
      />
    )
  }

  return (
    <div
      className={cn(
        'flex items-center justify-center rounded-full bg-gray-200 shrink-0',
        SIZES[size],
        className
      )}
      aria-label={name || userId || 'user avatar'}
      role="img"
    >
      <User className={cn('text-gray-500', ICON_SIZES[size])} />
    </div>
  )
}
