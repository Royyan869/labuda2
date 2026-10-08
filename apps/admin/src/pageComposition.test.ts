import { describe, expect, it } from 'vitest'

const pageSources = import.meta.glob('./pages/*.tsx', {
  query: '?raw',
  import: 'default',
  eager: true,
}) as Record<string, string>

const PageHeaderSource = Object.values(
  import.meta.glob('./components/common/PageHeader.tsx', {
    query: '?raw',
    import: 'default',
    eager: true,
  }) as Record<string, string>,
)[0]

const DisputeHeaderSource = Object.values(
  import.meta.glob('./components/disputes/DisputeHeader.tsx', {
    query: '?raw',
    import: 'default',
    eager: true,
  }) as Record<string, string>,
)[0]

// LoginPage is the only standalone page: it is not composed inside the shell
// and renders its own brand heading.
const STANDALONE_PAGES = new Set(['./pages/LoginPage.tsx'])

const pageFiles = Object.keys(pageSources)
  .filter((file) => !file.endsWith('.test.tsx'))
  .sort()

/**
 * Canonical page composition contract.
 *
 * Page title authority lives in `PageHeader` (or a domain header component
 * that consumes the same `type-page-title` role). Pages must not render their
 * own competing title markup, and every shell page shares one root rhythm.
 */
describe('page composition authority', () => {
  it('has a non-vacuous page population to check', () => {
    expect(pageFiles.length).toBeGreaterThan(25)
  })

  it('no page declares its own type-page-title (single title authority)', () => {
    const offenders = pageFiles.filter((file) => pageSources[file].includes('type-page-title'))
    expect(offenders).toEqual([])
  })

  it('no shell page renders a raw h1 outside PageHeader', () => {
    const offenders = pageFiles.filter(
      (file) => !STANDALONE_PAGES.has(file) && /<h1[\s>]/.test(pageSources[file]),
    )
    expect(offenders).toEqual([])
  })

  it('every shell page composes its header through PageHeader', () => {
    const offenders = pageFiles.filter(
      (file) => !STANDALONE_PAGES.has(file) && !pageSources[file].includes('PageHeader'),
    )
    expect(offenders).toEqual([])
  })

  it('every page shares the canonical space-y-6 root rhythm', () => {
    const offenders = pageFiles.filter(
      (file) => !STANDALONE_PAGES.has(file) && !pageSources[file].includes('space-y-6'),
    )
    expect(offenders).toEqual([])
  })

  it('PageHeader is the single primitive that renders type-page-title', () => {
    expect((PageHeaderSource.match(/<h1/g) ?? []).length).toBe(1)
    expect(PageHeaderSource).toContain('type-page-title')
  })

  it('the dispute workspace domain header uses the canonical title role, not ad-hoc typography', () => {
    expect(DisputeHeaderSource).toContain('type-page-title')
    expect(DisputeHeaderSource).not.toContain('text-2xl font-bold text-foreground')
  })
})
