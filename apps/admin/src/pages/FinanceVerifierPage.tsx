import { useState } from 'react'
import { ShieldCheck, ShieldAlert, Play, ChevronDown, ChevronRight } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { useFinanceVerifier } from '@/hooks/useFinanceVerifier'
import type { VerifierSection } from '@/hooks/useFinanceVerifier'

function SectionRow({ section }: { section: VerifierSection }) {
  const [expanded, setExpanded] = useState(!section.passed)
  const findings = section.findings || []

  return (
    <div className="border border-gray-200 rounded-lg">
      <button
        type="button"
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center justify-between px-4 py-3 text-left hover:bg-gray-50"
      >
        <div className="flex items-center gap-3">
          {expanded ? (
            <ChevronDown className="h-4 w-4 text-gray-500" />
          ) : (
            <ChevronRight className="h-4 w-4 text-gray-500" />
          )}
          <span className="font-medium text-sm">{section.name}</span>
        </div>
        <Badge variant={section.passed ? 'success' : 'error'}>
          {section.passed ? 'PASS' : 'FAIL'}
        </Badge>
      </button>
      {expanded && findings.length > 0 && (
        <div className="border-t border-gray-200 px-4 py-3 space-y-2">
          {findings.map((f, i) => (
            <div key={i} className="flex items-start gap-3 text-sm">
              <Badge variant={f.level === 'error' ? 'error' : 'warning'} className="shrink-0">
                {f.level}
              </Badge>
              <div className="min-w-0">
                <span className="font-mono text-xs text-gray-500">[{f.code}]</span>{' '}
                <span className="text-gray-700">{f.detail}</span>
                {f.class && (
                  <span className="ml-2 text-xs text-gray-400">({f.class})</span>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
      {expanded && findings.length === 0 && (
        <div className="border-t border-gray-200 px-4 py-3 text-sm text-gray-500">
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
      <div>
        <h1 className="text-3xl font-bold text-gray-900">Finance Verifier</h1>
        <p className="text-gray-600 mt-1">Run financial invariant checks (read-only)</p>
      </div>

      {/* Controls */}
      <Card>
        <CardContent className="pt-6">
          <div className="flex items-center gap-4 flex-wrap">
            <label htmlFor="mode-select" className="text-sm font-medium text-gray-700">
              Mode:
            </label>
            <select
              id="mode-select"
              value={mode}
              onChange={(e) => setMode(e.target.value as 'forensic' | 'strict')}
              className="px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-primary"
              disabled={loading}
            >
              <option value="forensic">Forensic (default)</option>
              <option value="strict">Strict (all findings = error)</option>
            </select>
            <Button onClick={() => run(mode)} disabled={loading} className="gap-2">
              <Play className="h-4 w-4" />
              {loading ? 'Running...' : 'Run Verification'}
            </Button>
          </div>
        </CardContent>
      </Card>

      {/* Error state */}
      {error && (
        <Card>
          <CardContent className="p-6">
            <div className="text-center text-red-600">
              <p>Error: {error}</p>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Loading state */}
      {loading && (
        <div className="flex items-center justify-center min-h-[200px]">
          <div className="text-center">
            <div className="inline-block h-8 w-8 animate-spin rounded-full border-4 border-solid border-primary border-r-transparent"></div>
            <p className="mt-4 text-gray-600">Running invariant checks...</p>
          </div>
        </div>
      )}

      {/* Result */}
      {result && !loading && (
        <>
          {/* Summary */}
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-3">
                {result.passed ? (
                  <ShieldCheck className="h-6 w-6 text-green-600" />
                ) : (
                  <ShieldAlert className="h-6 w-6 text-red-600" />
                )}
                Verification {result.passed ? 'Passed' : 'Failed'}
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 text-sm">
                <div>
                  <span className="text-gray-500">Mode:</span>{' '}
                  <span className="font-medium">{result.mode}</span>
                </div>
                <div>
                  <span className="text-gray-500">Errors:</span>{' '}
                  <span className="font-bold text-red-600">{result.error_count}</span>
                </div>
                <div>
                  <span className="text-gray-500">Warnings:</span>{' '}
                  <span className="font-bold text-amber-600">{result.warning_count}</span>
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
            <div className="text-center text-gray-500">
              <ShieldCheck className="h-12 w-12 mx-auto mb-4 text-gray-300" />
              <p>Click "Run Verification" to check financial invariants.</p>
              <p className="text-xs mt-2">This is a read-only operation that inspects ledger integrity.</p>
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  )
}
