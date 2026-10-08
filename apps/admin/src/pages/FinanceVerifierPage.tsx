import { useState } from 'react'
import { ShieldCheck, ShieldAlert, Play, ChevronDown, ChevronRight } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { Select } from '@/components/ui/Select'
import { useFinanceVerifier } from '@/hooks/useFinanceVerifier'
import type { VerifierSection } from '@/hooks/useFinanceVerifier'
import { AdminLoadingState, AdminErrorState, PageHeader } from '@/components/common'

function SectionRow({ section }: { section: VerifierSection }) {
  const [expanded, setExpanded] = useState(!section.passed)
  const findings = section.findings || []

  return (
    <div className="border border-border rounded-lg">
      <button
        type="button"
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center justify-between px-4 py-3 text-left hover:bg-surface-muted"
      >
        <div className="flex items-center gap-3">
          {expanded ? (
            <ChevronDown className="h-4 w-4 text-muted-foreground" />
          ) : (
            <ChevronRight className="h-4 w-4 text-muted-foreground" />
          )}
          <span className="font-medium text-sm">{section.name}</span>
        </div>
        <Badge variant={section.passed ? 'success' : 'error'}>
          {section.passed ? 'PASS' : 'FAIL'}
        </Badge>
      </button>
      {expanded && findings.length > 0 && (
        <div className="border-t border-border px-4 py-3 space-y-2">
          {findings.map((f, i) => (
            <div key={i} className="flex items-start gap-3 text-sm">
              <Badge variant={f.level === 'error' ? 'error' : 'warning'} className="shrink-0">
                {f.level}
              </Badge>
              <div className="min-w-0">
                <span className="font-mono type-caption">[{f.code}]</span>{' '}
                <span className="text-foreground">{f.detail}</span>
                {f.class && (
                  <span className="ml-2 type-caption">({f.class})</span>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
      {expanded && findings.length === 0 && (
        <div className="border-t border-border px-4 py-3 type-secondary">
          No findings.
        </div>
      )}
    </div>
  )
}

export function FinanceVerifierPage() {
  const [mode, setMode] = useState<'forensic' | 'strict'>('forensic')
  const { result, loading, error, run } = useFinanceVerifier()

  return (
    <div className="space-y-6">
      {/* Header */}
      <PageHeader title="Finance Verifier" description="Run financial invariant checks (read-only)" />

      {/* Controls */}
      <Card>
        <CardContent className="pt-6">
          <div className="flex items-end gap-4 flex-wrap">
            <Select
              label="Mode:"
              value={mode}
              onChange={(e) => setMode(e.target.value as 'forensic' | 'strict')}
              disabled={loading}
            >
              <option value="forensic">Forensic (default)</option>
              <option value="strict">Strict (all findings = error)</option>
            </Select>
            <Button onClick={() => run(mode)} disabled={loading} className="gap-2">
              <Play className="h-4 w-4" />
              {loading ? 'Running...' : 'Run Verification'}
            </Button>
          </div>
        </CardContent>
      </Card>

      {/* Error state */}
      {error && (
        <AdminErrorState title="Failed to run verification" message={error} onRetry={() => run(mode)} />
      )}

      {/* Loading state */}
      {loading && <AdminLoadingState label="Running invariant checks" />}

      {/* Result */}
      {result && !loading && (
        <>
          {/* Summary */}
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-3">
                {result.passed ? (
                  <ShieldCheck className="h-6 w-6 text-success" />
                ) : (
                  <ShieldAlert className="h-6 w-6 text-destructive" />
                )}
                Verification {result.passed ? 'Passed' : 'Failed'}
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 text-sm">
                <div>
                  <span className="text-muted-foreground">Mode:</span>{' '}
                  <span className="font-medium">{result.mode}</span>
                </div>
                <div>
                  <span className="text-muted-foreground">Errors:</span>{' '}
                  <span className="font-bold text-destructive">{result.error_count}</span>
                </div>
                <div>
                  <span className="text-muted-foreground">Warnings:</span>{' '}
                  <span className="font-bold text-warning">{result.warning_count}</span>
                </div>
              </div>
            </CardContent>
          </Card>

          {/* Sections */}
          <Card>
            <CardHeader>
              <CardTitle>Sections ({result.sections.length})</CardTitle>
            </CardHeader>
            <CardContent className="space-y-2">
              {result.sections.map((section) => (
                <SectionRow key={section.name} section={section} />
              ))}
            </CardContent>
          </Card>
        </>
      )}

      {/* Idle state */}
      {!result && !loading && !error && (
        <Card>
          <CardContent className="p-12">
            <div className="text-center text-muted-foreground">
              <ShieldCheck className="h-12 w-12 mx-auto mb-4 text-muted-foreground" />
              <p>Click "Run Verification" to check financial invariants.</p>
              <p className="text-xs mt-2">This is a read-only operation that inspects ledger integrity.</p>
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  )
}
