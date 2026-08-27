import { MemoryRouter, Outlet } from 'react-router-dom'
import { render, screen } from '@testing-library/react'
import type { ReactNode } from 'react'
import { describe, expect, it, vi } from 'vitest'
import App from './App'

vi.mock('./lib/firebase', () => ({
  auth: {},
}))

vi.mock('firebase/auth', () => ({
  onIdTokenChanged: vi.fn((_auth, callback) => {
    callback(null)
    return vi.fn()
  }),
}))

vi.mock('./lib/api', () => ({
  setAuthToken: vi.fn(),
  clearAuthToken: vi.fn(),
}))

vi.mock('./components/layout/MainLayout', () => ({
  MainLayout: () => <Outlet />,
}))

vi.mock('./components/auth/RequireCapability', () => ({
  RequireCapability: ({ cap, children }: { cap: string; children: ReactNode }) => (
    <div data-testid={`guard-${cap}`}>{children}</div>
  ),
}))

vi.mock('./pages/WithdrawalsPage', () => ({
  WithdrawalsPage: () => <div data-testid="withdrawals-page">Withdrawals Page</div>,
}))

vi.mock('./pages/FinanceLedgerPage', () => ({
  FinanceLedgerPage: () => <div data-testid="finance-ledger-page">Finance Ledger Page</div>,
}))

vi.mock('./pages/PayoutWhitelistAuditPage', () => ({
  PayoutWhitelistAuditPage: () => <div data-testid="payout-whitelist-audit-page">Payout Whitelist Audit Page</div>,
}))

vi.mock('./pages/AppealsPage', () => ({
  AppealsPage: () => <div data-testid="appeals-page">Appeals Page</div>,
}))

describe('admin finance routes', () => {
  it.each([
    ['/finance/withdrawals', 'withdrawals-page'],
    ['/finance/ledger', 'finance-ledger-page'],
    ['/payouts/whitelist-audit', 'payout-whitelist-audit-page'],
  ])('renders %s without route compilation errors', (path, testId) => {
    render(
      <MemoryRouter initialEntries={[path]}>
        <App />
      </MemoryRouter>
    )

    expect(screen.getByTestId(testId)).toBeInTheDocument()
  })
})

// PASS_13B: appeal content is escalation/governance content and requires its
// own trust boundary, separate from both generic case reading and appeal
// review authority. The Appeals page/nav is gated on moderation.appeal.read
// (viewing), matching the backend's GET routes — which were moved off the
// looser moderation.case.read in the same pass. Submitting a decision still
// requires the higher-trust moderation.appeal.review, enforced inside
// AppealDetailModal (see AppealDetailModal.test.tsx), not at the route level,
// since a read-only appeal admin must still be able to open the page.
describe('admin moderation routes', () => {
  it('gates /moderation/appeals behind moderation.appeal.read', () => {
    render(
      <MemoryRouter initialEntries={['/moderation/appeals']}>
        <App />
      </MemoryRouter>
    )

    expect(screen.getByTestId('guard-moderation.appeal.read')).toBeInTheDocument()
    expect(screen.getByTestId('appeals-page')).toBeInTheDocument()
  })
})
