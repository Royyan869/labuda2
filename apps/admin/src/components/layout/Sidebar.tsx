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
}

const navItems: NavItem[] = [
  { name: 'Dashboard', path: '/', icon: LayoutDashboard, requiredCapability: 'governance.dashboard.view' },
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
  { name: 'Promo Packages', path: '/promotions/packages', icon: Package, requiredCapability: 'promotion.package.manage' },
  { name: 'Campaigns', path: '/promotions/campaigns', icon: Package, requiredCapability: 'promotion.campaign.view' },
  { name: 'Users', path: '/users', icon: Users, requiredCapability: 'governance.user.read' },
  { name: 'Admins', path: '/users/admins', icon: Users, requiredCapability: 'governance.user.read' },
  { name: 'Alerts', path: '/alerts', icon: Bell, requiredCapability: 'governance.alert.read' },
  { name: 'Failed Deliveries', path: '/notifications/failed-deliveries', icon: MailWarning, requiredCapability: 'governance.dashboard.view' },
  { name: 'Audit Logs', path: '/audit-logs', icon: History, requiredCapability: 'governance.audit.read' },
  { name: 'SLA Analytics', path: '/analytics/sla', icon: BarChart3, requiredCapability: 'governance.dashboard.view' },
  { name: 'Platform Config', path: '/platform/config', icon: Settings, requiredCapability: 'config.view' },
  { name: 'Payment Methods', path: '/platform/payment-methods', icon: CreditCard, requiredCapability: 'finance.payment_method.view' },
]

export function Sidebar() {
  const { capabilities } = useAuth()

  return (
    <aside className="fixed left-0 top-0 z-40 flex h-screen w-64 flex-col border-r border-gray-200 bg-white">
      {/* Logo */}
      <div className="flex h-16 shrink-0 items-center border-b border-gray-200 px-6">
        <h1 className="text-xl font-bold text-primary">LABUDA Admin</h1>
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
              end={item.path === '/'}
              className={({ isActive }) =>
                cn(
                  'flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium transition-colors',
                  !allowed && 'opacity-50 pointer-events-none',
                  isActive
                    ? 'bg-primary/10 text-primary'
                    : 'text-gray-700 hover:bg-gray-100 hover:text-gray-900'
                )
              }
              title={!allowed ? `Requires: ${item.requiredCapability}` : ''}
            >
              {({ isActive }) => (
                <>
                  <Icon className={cn('h-5 w-5', isActive ? 'text-primary' : 'text-gray-500')} />
                  {item.name}
                </>
              )}
            </NavLink>
          )
        })}
      </nav>

      {/* Footer */}
      <div className="shrink-0 border-t border-gray-200 p-4">
        <p className="text-xs text-gray-500 text-center">
          LABUDA Admin Dashboard
          <br />
          v1.0.0
        </p>
      </div>
    </aside>
  )
}
