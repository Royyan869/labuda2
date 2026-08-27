import { useAuth } from '@/hooks/useAuth'
import { hasCapability, formatCapability } from '@/lib/permissions'
import { Navigate } from 'react-router-dom'
import { Lock, AlertCircle } from 'lucide-react'

interface RequireCapabilityProps {
  cap: string
  children: React.ReactNode
}

export function RequireCapability({ cap, children }: RequireCapabilityProps) {
  const { user, capabilities } = useAuth()

  if (!user) return <Navigate to="/login" replace />

  if (!hasCapability(capabilities, cap)) {
    return (
      <div className="flex items-center justify-center h-screen bg-gray-50">
        <div className="text-center max-w-md">
          <div className="inline-flex items-center justify-center w-16 h-16 rounded-full bg-red-100 mb-4">
            <Lock className="w-8 h-8 text-red-600" />
          </div>
          <h1 className="text-2xl font-bold text-gray-900">Access Denied</h1>
          <p className="mt-2 text-gray-600">You don't have permission to access this page.</p>
          <div className="mt-6 p-4 bg-amber-50 border border-amber-200 rounded-lg">
            <div className="flex items-start gap-3">
              <AlertCircle className="w-5 h-5 text-amber-600 flex-shrink-0 mt-0.5" />
              <div className="text-left">
                <p className="text-sm font-medium text-amber-900">Required Capability</p>
                <p className="text-xs text-amber-700 mt-1 font-mono">{cap}</p>
                <p className="text-xs text-amber-700 mt-1">{formatCapability(cap)}</p>
              </div>
            </div>
          </div>
        </div>
      </div>
    )
  }

  return <>{children}</>
}
