import { describe, expect, it } from 'vitest'
import {
  WITHDRAWAL_STATUS,
  withdrawalStatusLabels,
  withdrawalStatusVariants,
  type WithdrawalStatus,
} from './finance'

const ALL_WITHDRAWAL_STATUSES = Object.values(WITHDRAWAL_STATUS)

describe('finance withdrawal status contract', () => {
  it('exports labels for every backend status', () => {
    for (const status of ALL_WITHDRAWAL_STATUSES) {
      expect(withdrawalStatusLabels[status as WithdrawalStatus]).toBeTruthy()
      expect(withdrawalStatusVariants[status as WithdrawalStatus]).toBeTruthy()
    }
  })

  it('keeps pilot blocked as a visible admin state', () => {
    expect(withdrawalStatusLabels.PILOT_BLOCKED).toBe('Blocked (Pilot)')
    expect(withdrawalStatusVariants.PILOT_BLOCKED).toBe('warning')
  })

  it('keeps manual completion distinct from gateway settlement', () => {
    expect(withdrawalStatusLabels.COMPLETED).toBe('Manually Paid')
    expect(withdrawalStatusLabels.SETTLED).toBe('Settled by Gateway')
    expect(withdrawalStatusLabels.COMPLETED).not.toBe(withdrawalStatusLabels.SETTLED)
  })
})
