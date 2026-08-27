import { describe, expect, it, vi, beforeEach } from 'vitest'

const apiGetMock = vi.hoisted(() => vi.fn())

vi.mock('./client', () => ({
  api: {
    get: apiGetMock,
  },
}))

import {
  getReconciliationResults,
  getReconciliationResult,
  getLatestReconciliationResult,
} from './reconciliation'

describe('reconciliation API client', () => {
  beforeEach(() => {
    apiGetMock.mockReset()
    apiGetMock.mockResolvedValue({ results: [], total: 0, limit: 50, offset: 0 })
  })

  it('lists reconciliation results with default pagination', async () => {
    await getReconciliationResults()

    expect(apiGetMock).toHaveBeenCalledWith(
      '/api/v1/admin/reconciliation?limit=50&offset=0'
    )
  })

  it('includes severity and date filters in the query string', async () => {
    await getReconciliationResults({
      severity: 'high',
      date_from: '2026-01-01T00:00:00Z',
      date_to: '2026-01-31T00:00:00Z',
      limit: 20,
      offset: 40,
    })

    const calledUrl = apiGetMock.mock.calls[0][0] as string
    expect(calledUrl).toContain('severity=high')
    expect(calledUrl).toContain('date_from=2026-01-01T00%3A00%3A00Z')
    expect(calledUrl).toContain('date_to=2026-01-31T00%3A00%3A00Z')
    expect(calledUrl).toContain('limit=20')
    expect(calledUrl).toContain('offset=40')
  })

  it('omits severity from the query string when not provided', async () => {
    await getReconciliationResults()

    const calledUrl = apiGetMock.mock.calls[0][0] as string
    expect(calledUrl).not.toContain('severity=')
  })

  it('fetches a single reconciliation result by id', async () => {
    await getReconciliationResult('result-123')

    expect(apiGetMock).toHaveBeenCalledWith('/api/v1/admin/reconciliation/result-123')
  })

  it('fetches the latest reconciliation result', async () => {
    await getLatestReconciliationResult()

    expect(apiGetMock).toHaveBeenCalledWith('/api/v1/admin/reconciliation/latest')
  })
})
