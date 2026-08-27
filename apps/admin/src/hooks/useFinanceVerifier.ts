import { useState, useCallback } from 'react'
import { api, ApiError } from '@/lib/api'

export interface VerifierFinding {
  code: string
  level: string
  class: string
  detail: string
}

export interface VerifierSection {
  name: string
  passed: boolean
  findings?: VerifierFinding[]
}

export interface VerifierResult {
  mode: string
  passed: boolean
  error_count: number
  warning_count: number
  sections: VerifierSection[]
}

type VerifierMode = 'forensic' | 'strict'

export function useFinanceVerifier() {
  const [result, setResult] = useState<VerifierResult | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const run = useCallback(async (mode: VerifierMode = 'forensic') => {
    setLoading(true)
    setError(null)
    setResult(null)
    try {
      const res = await api.post<VerifierResult>(
        `/api/v1/admin/finance/verify?mode=${mode}`,
        {}
      )
      setResult(res)
    } catch (err) {
      // HTTP 422 = verification ran but found failures; response body is still VerifierResult
      if (err instanceof ApiError && err.status === 422 && err.data) {
        setResult(err.data as VerifierResult)
      } else {
        setError(err instanceof Error ? err.message : 'Verification failed')
      }
    } finally {
      setLoading(false)
    }
  }, [])

  return { result, loading, error, run }
}
