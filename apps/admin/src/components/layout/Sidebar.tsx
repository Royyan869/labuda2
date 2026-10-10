import { forwardRef } from 'react'
import { NavLink } from 'react-router-dom'
import {
  LayoutDashboard,
  Shield,
  FileText,
  AlertTriangle,
  ShoppingBag,
  Scale,
  Wallet,
  History,
  Users,
  LifeBuoy,
  BarChart3,
  BadgeCheck,
  Bell,
  MailWarning,
  ShieldCheck,
  ClipboardList,
  BookOpen,
  ClipboardCheck,
  Settings,
  Package,
  Gavel,
  CreditCard,
} from 'lucide-react'
import { cn } from '@/lib/utils'
import { hasCapability } from '@/lib/permissions'
import { useAuth } from '@/hooks/useAuth'

interface NavItem {
  name: string
  path: string
  icon: React.ComponentType<{ className?: string }>
  requiredCapability?: string
  /**
   * Match only the exact path. Needed for a parent route that is also the
   * prefix of a more specific sibling (e.g. `/users` vs `/users/admins`), so
   * that a nested route never highlights two nav items at once.
   */
  end?: boolean
}

// Single navigation authority: every Admin route reachable from the shell is
// declared here exactly once, in shell order. `App.tsx` owns the route table;
// this list owns its presentation.
const navItems: NavItem[] = [
  { name: 'Dashboard', path: '/', icon: LayoutDashboard, requiredCapability: 'governance.dashboard.view', end: true },
  { name: 'Orders', path: '/orders', icon: ShoppingBag, requiredCapability: 'order.read' },
  { name: 'Disputes', path: '/disputes', icon: Scale, requiredCapability: 'finance.dispute.resolve' },
  { name: 'Withdrawals', path: '/finance/withdrawals', icon: Wallet, requiredCapability: 'finance.withdraw.read' },
  { name: 'Finance Verifier', path: '/finance/verifier', icon: ShieldCheck, requiredCapability: 'finance.withdraw.read' },
  { name: 'Finance Ledger', path: '/finance/ledger', icon: BookOpen, requiredCapability: 'finance.withdraw.read' },
  { name: 'Reconciliation', path: '/finance/reconciliation', icon: History, requiredCapability: 'finance.withdraw.read' },
  { name: 'Whitelist Audit', path: '/payouts/whitelist-audit', icon: ClipboardCheck, requiredCapability: 'finance.withdraw.read' },
  { name: 'Support Overview', path: '/support/overview', icon: ClipboardList, requiredCapability: 'support.admin.read' },
  { name: 'Support Tickets', path: '/support/tickets', icon: LifeBuoy, requiredCapability: 'support.ticket.read' },
  { name: 'Moderation', path: '/moderation/cases', icon: Shield, requiredCapability: 'moderation.case.read' },
  { name: 'Appeals', path: '/moderation/appeals', icon: FileText, requiredCapability: 'moderation.appeal.read' },
  { name: 'Warnings', path: '/moderation/warnings', icon: AlertTriangle, requiredCapability: 'moderation.case.read' },
  { name: 'Auction Emergency Cancel', path: '/governance/auction-cancel', icon: Gavel, requiredCapability: 'governance.auction.cancel' },
  { name: 'Verifications', path: '/sellers/verifications', icon: BadgeCheck, requiredCapability: 'seller.verification.review' },
  { name: 'Ext. Products', path: '/promotions/external-products', icon: Package, requiredCapability: 'promotion.external_product.review' },
  { name: 'Users', path: '/users', icon: Users, requiredCapability: 'governance.user.read', end: true },
  { name: 'Admins', path: '/users/admins', icon: Users, requiredCapability: 'governance.user.read' },
  { name: 'Alerts', path: '/alerts', icon: Bell, requiredCapability: 'governance.alert.read' },
  { name: 'Failed Deliveries', path: '/notifications/failed-deliveries', icon: MailWarning, requiredCapability: 'governance.dashboard.view' },
  { name: 'Audit Logs', path: '/audit-logs', icon: History, requiredCapability: 'governance.audit.read' },
  { name: 'SLA Analytics', path: '/analytics/sla', icon: BarChart3, requiredCapability: 'governance.dashboard.view' },
  { name: 'Platform Config', path: '/platform/config', icon: Settings, requiredCapability: 'config.view' },
  { name: 'Payment Methods', path: '/platform/payment-methods', icon: CreditCard, requiredCapability: 'finance.payment_method.view' },
]

interface SidebarProps {
  /** Mobile drawer visibility. Ignored on desktop, where the sidebar is docked. */
  isOpen?: boolean
  /** Called when a nav link is chosen, so the shell can close the mobile drawer. */
  onNavigate?: () => void
  /** True on desktop widths (`lg` and up). */
  isDesktop?: boolean
}

/**
 * The one Admin navigation surface.
 *
 * Desktop: permanently docked (`lg:translate-x-0`) and never inert.
 * Narrow viewports: the same element slides in as an overlay drawer. There is
 * no second sidebar and no separate mobile menu list.
 */
export const Sidebar = forwardRef<HTMLElement, SidebarProps>(function Sidebar(
  { isOpen = false, onNavigate, isDesktop = false },
  ref,
) {
  const { capabilities } = useAuth()
  const visible = isDesktop || isOpen

  return (
    <>
      {/* Drawer scrim — mobile only, only while open. */}
      {isOpen && !isDesktop && (
        <div
          className="fixed inset-0 z-40 bg-black/50"
          aria-hidden="true"
          onClick={onNavigate}
        />
      )}

      <aside
        id="admin-sidebar"
        ref={ref}
        aria-label="Admin navigation"
        // When off-canvas on mobile, keep the links out of the tab order and
        // the accessibility tree; on desktop it is always interactive.
        inert={!visible}
        className={cn(
          'fixed left-0 top-0 z-50 flex h-screen w-64 flex-col border-r border-border bg-surface transition-transform duration-200 ease-out',
          visible ? 'translate-x-0' : '-translate-x-full'
        )}
      >
        {/* Logo */}
        <div className="flex h-16 shrink-0 items-center border-b border-border px-6">
          {/* Brand, not a heading: the page owns the single h1. */}
          <span className="text-xl font-bold text-primary">HiShumi Admin</span>
        </div>

        {/* Navigation — scrolls independently so items below the fold (below
            the viewport height) stay reachable instead of being clipped by
            the fixed-height aside. */}
        <nav className="min-h-0 flex-1 overflow-y-auto space-y-1 px-3 py-4">
          {navItems.map((item) => {
            const Icon = item.icon
            const allowed = item.requiredCapability ? hasCapability(capabilities, item.requiredCapability) : true

            return (
              <NavLink
                key={item.path}
                to={item.path}
                end={item.end}
                onClick={onNavigate}
                aria-disabled={!allowed}
                className={({ isActive }) =>
                  cn(
                    'flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium transition-colors',
                    !allowed && 'opacity-50 pointer-events-none',
                    isActive
                      ? 'bg-primary/10 text-primary'
                      : 'text-foreground hover:bg-surface-muted hover:text-foreground'
                  )
                }
                title={!allowed ? `Requires: ${item.requiredCapability}` : ''}
              >
                {({ isActive }) => (
                  <>
                    <Icon className={cn('h-5 w-5', isActive ? 'text-primary' : 'text-muted-foreground')} />
                    {item.name}
                  </>
                )}
              </NavLink>
            )
          })}
        </nav>

        {/* Footer */}
        <div className="shrink-0 border-t border-border p-4">
          <p className="type-caption text-center">
            HiShumi Admin Dashboard
            <br />
            v1.0.0
          </p>
        </div>
      </aside>
    </>
  )
})
