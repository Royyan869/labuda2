import { Routes, Route } from 'react-router-dom'
import { MainLayout } from '@/components/layout/MainLayout'
import { RequireCapability } from '@/components/auth/RequireCapability'
import { LoginPage } from '@/pages/LoginPage'
import { DashboardPage } from '@/pages/DashboardPage'
import { OrdersPage } from '@/pages/OrdersPage'
import { DisputesPage } from '@/pages/DisputesPage'
import { DisputeWorkspacePage } from '@/pages/DisputeWorkspacePage'
import { ModerationCasesPage } from '@/pages/ModerationCasesPage'
import { AppealsPage } from '@/pages/AppealsPage'
import { WarningsPage } from '@/pages/WarningsPage'
import { WithdrawalsPage } from '@/pages/WithdrawalsPage'
import { UsersPage } from '@/pages/UsersPage'
import { AdminsPage } from '@/pages/AdminsPage'
import { AdminDetailPage } from '@/pages/AdminDetailPage'
import { AuditLogsPage } from '@/pages/AuditLogsPage'
import { SupportTicketsPage } from '@/pages/SupportTicketsPage'
import { SupportTicketDetailPage } from '@/pages/SupportTicketDetailPage'
import { SLADashboardPage } from '@/pages/SLADashboardPage'
import { SellerVerificationsPage } from '@/pages/SellerVerificationsPage'
import { AlertsPage } from '@/pages/AlertsPage'
import { FailedDeliveriesPage } from '@/pages/FailedDeliveriesPage'
import { FinanceVerifierPage } from '@/pages/FinanceVerifierPage'
import { FinanceLedgerPage } from '@/pages/FinanceLedgerPage'
import { FinanceReconciliationPage } from '@/pages/FinanceReconciliationPage'
import { AuctionEmergencyCancelPage } from '@/pages/AuctionEmergencyCancelPage'
import { PayoutWhitelistAuditPage } from '@/pages/PayoutWhitelistAuditPage'
import { PlatformConfigPage } from '@/pages/PlatformConfigPage'
import { PaymentMethodsPage } from '@/pages/PaymentMethodsPage'
import { SupportOverviewPage } from '@/pages/SupportOverviewPage'
import { ExternalProductsPage } from '@/pages/ExternalProductsPage'
import { PromotionPackagesPage } from '@/pages/PromotionPackagesPage'
import { PromotionCampaignsPage } from '@/pages/PromotionCampaignsPage'

function App() {
  return (
    <Routes>
      {/* Public Routes */}
      <Route path="/login" element={<LoginPage />} />

      {/* Protected Routes (Admin Only) */}
      <Route element={<MainLayout />}>
        <Route
          path="/"
          element={
            <RequireCapability cap="governance.dashboard.view">
              <DashboardPage />
            </RequireCapability>
          }
        />
        <Route
          path="/orders"
          element={
            <RequireCapability cap="order.read">
              <OrdersPage />
            </RequireCapability>
          }
        />
        <Route
          path="/disputes"
          element={
            <RequireCapability cap="finance.dispute.resolve">
              <DisputesPage />
            </RequireCapability>
          }
        />
        <Route
          path="/disputes/:id"
          element={
            <RequireCapability cap="finance.dispute.resolve">
              <DisputeWorkspacePage />
            </RequireCapability>
          }
        />
        <Route
          path="/finance/withdrawals"
          element={
            <RequireCapability cap="finance.withdraw.read">
              <WithdrawalsPage />
            </RequireCapability>
          }
        />
        <Route
          path="/moderation/cases"
          element={
            <RequireCapability cap="moderation.case.read">
              <ModerationCasesPage />
            </RequireCapability>
          }
        />
        <Route
          path="/moderation/appeals"
          element={
            <RequireCapability cap="moderation.appeal.read">
              <AppealsPage />
            </RequireCapability>
          }
        />
        <Route
          path="/moderation/warnings"
          element={
            <RequireCapability cap="moderation.case.read">
              <WarningsPage />
            </RequireCapability>
          }
        />
        <Route
          path="/users"
          element={
            <RequireCapability cap="governance.user.read">
              <UsersPage />
            </RequireCapability>
          }
        />
        <Route
          path="/users/admins"
          element={
            <RequireCapability cap="governance.user.read">
              <AdminsPage />
            </RequireCapability>
          }
        />
        <Route
          path="/users/admins/:id"
          element={
            <RequireCapability cap="governance.user.read">
              <AdminDetailPage />
            </RequireCapability>
          }
        />
        <Route
          path="/audit-logs"
          element={
            <RequireCapability cap="governance.audit.read">
              <AuditLogsPage />
            </RequireCapability>
          }
        />
        <Route
          path="/support/overview"
          element={
            <RequireCapability cap="support.admin.read">
              <SupportOverviewPage />
            </RequireCapability>
          }
        />
        <Route
          path="/support/tickets"
          element={
            <RequireCapability cap="support.ticket.read">
              <SupportTicketsPage />
            </RequireCapability>
          }
        />
        <Route
          path="/support/tickets/:id"
          element={
            <RequireCapability cap="support.ticket.read">
              <SupportTicketDetailPage />
            </RequireCapability>
          }
        />
        <Route
          path="/analytics/sla"
          element={
            <RequireCapability cap="governance.dashboard.view">
              <SLADashboardPage />
            </RequireCapability>
          }
        />
        <Route
          path="/sellers/verifications"
          element={
            <RequireCapability cap="seller.verification.review">
              <SellerVerificationsPage />
            </RequireCapability>
          }
        />
        <Route
          path="/alerts"
          element={
            <RequireCapability cap="governance.alert.read">
              <AlertsPage />
            </RequireCapability>
          }
        />
        <Route
          path="/notifications/failed-deliveries"
          element={
            <RequireCapability cap="governance.dashboard.view">
              <FailedDeliveriesPage />
            </RequireCapability>
          }
        />
        <Route
          path="/finance/verifier"
          element={
            <RequireCapability cap="finance.withdraw.read">
              <FinanceVerifierPage />
            </RequireCapability>
          }
        />
        <Route
          path="/finance/ledger"
          element={
            <RequireCapability cap="finance.withdraw.read">
              <FinanceLedgerPage />
            </RequireCapability>
          }
        />
        <Route
          path="/finance/reconciliation"
          element={
            <RequireCapability cap="finance.withdraw.read">
              <FinanceReconciliationPage />
            </RequireCapability>
          }
        />
        <Route
          path="/governance/auction-cancel"
          element={
            <RequireCapability cap="governance.auction.cancel">
              <AuctionEmergencyCancelPage />
            </RequireCapability>
          }
        />
        <Route
          path="/payouts/whitelist-audit"
          element={
            <RequireCapability cap="finance.withdraw.read">
              <PayoutWhitelistAuditPage />
            </RequireCapability>
          }
        />
        <Route
          path="/platform/config"
          element={
            <RequireCapability cap="config.view">
              <PlatformConfigPage />
            </RequireCapability>
          }
        />
        <Route
          path="/platform/payment-methods"
          element={
            <RequireCapability cap="finance.payment_method.view">
              <PaymentMethodsPage />
            </RequireCapability>
          }
        />
        <Route
          path="/promotions/external-products"
          element={
            <RequireCapability cap="promotion.external_product.review">
              <ExternalProductsPage />
            </RequireCapability>
          }
        />
        <Route
          path="/promotions/packages"
          element={
            <RequireCapability cap="promotion.package.manage">
              <PromotionPackagesPage />
            </RequireCapability>
          }
        />
        <Route
          path="/promotions/campaigns"
          element={
            <RequireCapability cap="promotion.campaign.view">
              <PromotionCampaignsPage />
            </RequireCapability>
          }
        />
      </Route>
    </Routes>
  )
}

export default App
