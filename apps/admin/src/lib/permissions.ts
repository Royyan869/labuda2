export function hasCapability(
  userCapabilities: string[] | undefined,
  required: string
): boolean {
  if (!userCapabilities) return false
  return userCapabilities.includes(required)
}

export function hasAnyCapability(
  userCapabilities: string[] | undefined,
  required: string[]
): boolean {
  if (!userCapabilities) return false
  return required.some(cap => userCapabilities.includes(cap))
}

/**
 * Formats a capability string into a human-readable format
 * @example
 * formatCapability('finance.withdraw.review')
 * // → "Finance → Withdraw → Review"
 */
export function formatCapability(capability: string): string {
  return capability
    .split('.')
    .map(part => part.charAt(0).toUpperCase() + part.slice(1).replace(/_/g, ' '))
    .join(' → ')
}
