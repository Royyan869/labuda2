/// <reference types="node" />
import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

function read(relative: string): string {
  return readFileSync(new URL(relative, import.meta.url), 'utf8')
}

const indexCss = read('./index.css')
const indexHtml = read('../index.html')
const tailwindConfig = read('../tailwind.config.js')

/**
 * The canonical typography role layer. Each @apply must encode the exact visual
 * values that were previously inlined across consumers, so convergence is a
 * pure rename with zero visual change.
 */
const ROLES: Record<string, string> = {
  'type-page-title': '@apply text-3xl font-bold text-foreground;',
  'type-section-title': '@apply text-lg font-semibold text-foreground;',
  'type-body': '@apply text-sm text-foreground;',
  'type-secondary': '@apply text-sm text-muted-foreground;',
  'type-label': '@apply text-sm font-medium text-foreground;',
  'type-caption': '@apply text-xs text-muted-foreground;',
  'type-metric': '@apply text-2xl font-bold;',
  'type-metric-lg': '@apply text-3xl font-bold;',
}

describe('typography foundation', () => {
  describe('canonical role authority (src/index.css)', () => {
    for (const [name, apply] of Object.entries(ROLES)) {
      it(`defines .${name} with its canonical values`, () => {
        expect(indexCss).toContain(`.${name} {`)
        expect(indexCss).toContain(apply)
      })
    }
  })

  describe('font authority', () => {
    it('loads Inter as the single canonical UI font', () => {
      expect(indexHtml).toContain('fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700')
    })

    it('declares Inter in the Tailwind sans stack', () => {
      expect(tailwindConfig).toContain("sans: ['Inter', 'system-ui', 'sans-serif']")
    })
  })

  describe('component-owned typography contracts remain intact', () => {
    it('Button owns its size scale', () => {
      const button = read('./components/ui/Button.tsx')
      expect(button).toContain('text-sm')
      expect(button).toContain('text-base')
      expect(button).toContain('text-lg')
    })

    it('Card/CardTitle owns its title typography', () => {
      expect(read('./components/ui/Card.tsx')).toContain('text-lg font-semibold text-foreground')
    })

    it('Modal owns its title typography', () => {
      expect(read('./components/ui/Modal.tsx')).toContain('text-xl font-semibold text-foreground')
    })

    it('Table owns its body + header typography', () => {
      const table = read('./components/ui/Table.tsx')
      expect(table).toContain('text-sm')
      expect(table).toContain('font-medium text-foreground')
    })

    it('Field owns form label typography', () => {
      expect(read('./components/ui/Field.tsx')).toContain('block text-sm font-medium')
    })
  })
})
