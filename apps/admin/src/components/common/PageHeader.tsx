import { type ReactNode } from 'react'

interface PageHeaderProps {
  /** The page's single `h1`. Pages own their title; the shell does not. */
  title: string
  /** Optional supporting line under the title. */
  description?: ReactNode
  /** Page-level actions, right-aligned on wide viewports. */
  actions?: ReactNode
  /** Optional element before the title block (e.g. a Back button). */
  leading?: ReactNode
  /** Optional decorative glyph rendered before the title text. */
  icon?: ReactNode
}

/**
 * Canonical Admin page header.
 *
 * One composition for every page: title (`type-page-title`), optional
 * description, optional leading control, and optional page actions. It stacks
 * on narrow viewports and becomes a single row from `sm` up, so page controls
 * never collide with the title.
 *
 * Pages own the title authority; this primitive only gives that authority one
 * shape. Do not render a second title in the shell.
 */
export function PageHeader({ title, description, actions, leading, icon }: PageHeaderProps) {
  return (
    <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
      <div className="flex min-w-0 items-center gap-4">
        {leading}
        <div className="min-w-0">
          <h1 className={icon ? 'type-page-title flex items-center gap-2' : 'type-page-title'}>
            {icon && (
              <span aria-hidden="true" className="inline-flex shrink-0">
                {icon}
              </span>
            )}
            {title}
          </h1>
          {description && <p className="text-muted-foreground mt-1">{description}</p>}
        </div>
      </div>
      {actions && (
        <div className="flex flex-wrap items-center gap-2 sm:justify-end">{actions}</div>
      )}
    </div>
  )
}
