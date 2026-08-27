import { useState, useEffect, useCallback } from 'react'
import { Package, Plus, Pencil, Power, PowerOff, RefreshCw } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/Table'
import { Modal } from '@/components/ui/Modal'
import {
  adminListPackages,
  adminCreatePackage,
  adminUpdatePackage,
  adminEnablePackage,
  adminDisablePackage,
} from '@/lib/api/promotions'
import { formatDate } from '@/lib/utils'
import type { PromotionPackage } from '@/types/promotion'

// ─── Package Form Modal ───────────────────────────────────────────────────────

const TARGET_TYPE_OPTIONS = ['fixed_price_sale', 'auction', 'external_product']

interface PackageFormModalProps {
  isOpen: boolean
  onClose: () => void
  onSuccess: () => void
  editPackage?: PromotionPackage | null
}

function PackageFormModal({ isOpen, onClose, onSuccess, editPackage }: PackageFormModalProps) {
  const isEdit = !!editPackage

  const [name, setName] = useState('')
  const [durationHours, setDurationHours] = useState('')
  const [validityHours, setValidityHours] = useState('')
  const [priceAmount, setPriceAmount] = useState('')
  const [targetTypes, setTargetTypes] = useState<string[]>([])
  const [isActive, setIsActive] = useState(true)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (editPackage) {
      setName(editPackage.name)
      setDurationHours(String(editPackage.total_duration_hours))
      setValidityHours(String(editPackage.validity_window_hours))
      setPriceAmount(String(editPackage.price_amount))
      setTargetTypes(editPackage.allowed_target_types)
      setIsActive(editPackage.is_active)
    } else {
      setName('')
      setDurationHours('')
      setValidityHours('')
      setPriceAmount('')
      setTargetTypes(['fixed_price_sale'])
      setIsActive(true)
    }
    setError(null)
  }, [editPackage, isOpen])

  const toggleTargetType = (tt: string) => {
    setTargetTypes((prev) =>
      prev.includes(tt) ? prev.filter((t) => t !== tt) : [...prev, tt]
    )
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (targetTypes.length === 0) {
      setError('Select at least one allowed target type.')
      return
    }
    setSubmitting(true)
    setError(null)
    try {
      const payload = {
        name,
        total_duration_hours: parseInt(durationHours, 10),
        validity_window_hours: parseInt(validityHours, 10),
        price_amount: parseInt(priceAmount, 10),
        allowed_target_types: targetTypes,
      }
      if (isEdit && editPackage) {
        await adminUpdatePackage(editPackage.id, { ...payload, is_active: isActive })
      } else {
        await adminCreatePackage(payload)
      }
      onSuccess()
      onClose()
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to save package')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Modal isOpen={isOpen} onClose={onClose} title={isEdit ? 'Edit Package' : 'Create Package'}>
      <form onSubmit={handleSubmit} className="space-y-4">
        {error && (
          <div className="rounded bg-red-50 p-3 text-sm text-red-700">{error}</div>
        )}

        <div>
          <label className="mb-1 block text-sm font-medium text-gray-700">Name</label>
          <input
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            required
            className="w-full rounded border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
            placeholder="e.g. Promote Basic (3 Days)"
          />
        </div>

        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="mb-1 block text-sm font-medium text-gray-700">
              Duration (hours)
            </label>
            <input
              type="number"
              min={1}
              value={durationHours}
              onChange={(e) => setDurationHours(e.target.value)}
              required
              className="w-full rounded border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
              placeholder="72"
            />
          </div>
          <div>
            <label className="mb-1 block text-sm font-medium text-gray-700">
              Validity window (hours)
            </label>
            <input
              type="number"
              min={1}
              value={validityHours}
              onChange={(e) => setValidityHours(e.target.value)}
              required
              className="w-full rounded border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
              placeholder="336"
            />
          </div>
        </div>

        <div>
          <label className="mb-1 block text-sm font-medium text-gray-700">
            Price (IDR)
          </label>
          <input
            type="number"
            min={0}
            value={priceAmount}
            onChange={(e) => setPriceAmount(e.target.value)}
            required
            className="w-full rounded border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
            placeholder="25000"
          />
        </div>

        <div>
          <label className="mb-1 block text-sm font-medium text-gray-700">
            Allowed target types
          </label>
          <div className="flex gap-3">
            {TARGET_TYPE_OPTIONS.map((tt) => (
              <label key={tt} className="flex cursor-pointer items-center gap-1.5 text-sm">
                <input
                  type="checkbox"
                  checked={targetTypes.includes(tt)}
                  onChange={() => toggleTargetType(tt)}
                  className="h-4 w-4 rounded border-gray-300"
                />
                {tt.replace('_', ' ')}
              </label>
            ))}
          </div>
        </div>

        {isEdit && (
          <div>
            <label className="flex cursor-pointer items-center gap-2 text-sm font-medium text-gray-700">
              <input
                type="checkbox"
                checked={isActive}
                onChange={(e) => setIsActive(e.target.checked)}
                className="h-4 w-4 rounded border-gray-300"
              />
              Active (visible to buyers)
            </label>
          </div>
        )}

        <div className="flex justify-end gap-3 pt-2">
          <Button type="button" variant="secondary" onClick={onClose} disabled={submitting}>
            Cancel
          </Button>
          <Button type="submit" disabled={submitting}>
            {submitting ? 'Saving…' : isEdit ? 'Save Changes' : 'Create Package'}
          </Button>
        </div>
      </form>
    </Modal>
  )
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export function PromotionPackagesPage() {
  const [packages, setPackages] = useState<PromotionPackage[]>([])
  const [loading, setLoading] = useState(true)
  const [fetchError, setFetchError] = useState<string | null>(null)

  const [formOpen, setFormOpen] = useState(false)
  const [editTarget, setEditTarget] = useState<PromotionPackage | null>(null)

  const [toggling, setToggling] = useState<string | null>(null)

  const fetchPackages = useCallback(async () => {
    setLoading(true)
    setFetchError(null)
    try {
      const data = await adminListPackages()
      setPackages(data.packages ?? [])
    } catch (err: unknown) {
      setFetchError(err instanceof Error ? err.message : 'Failed to load packages')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchPackages()
  }, [fetchPackages])

  const handleToggleActive = async (pkg: PromotionPackage) => {
    setToggling(pkg.id)
    try {
      if (pkg.is_active) {
        await adminDisablePackage(pkg.id)
      } else {
        await adminEnablePackage(pkg.id)
      }
      await fetchPackages()
    } catch (err: unknown) {
      alert(err instanceof Error ? err.message : 'Failed to update package')
    } finally {
      setToggling(null)
    }
  }

  const openCreate = () => {
    setEditTarget(null)
    setFormOpen(true)
  }

  const openEdit = (pkg: PromotionPackage) => {
    setEditTarget(pkg)
    setFormOpen(true)
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Promotion Packages</h1>
          <p className="mt-1 text-sm text-gray-500">
            Manage purchasable promotion packages available to sellers.
          </p>
        </div>
        <div className="flex gap-3">
          <Button variant="secondary" onClick={fetchPackages} disabled={loading}>
            <RefreshCw className={`mr-2 h-4 w-4 ${loading ? 'animate-spin' : ''}`} />
            Refresh
          </Button>
          <Button onClick={openCreate}>
            <Plus className="mr-2 h-4 w-4" />
            Create Package
          </Button>
        </div>
      </div>

      {fetchError && (
        <div className="rounded border border-red-200 bg-red-50 p-4 text-sm text-red-700">
          {fetchError}
        </div>
      )}

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Package className="h-5 w-5" />
            All Packages ({packages.length})
          </CardTitle>
        </CardHeader>
        <CardContent className="p-0">
          {loading ? (
            <div className="p-8 text-center text-sm text-gray-500">Loading…</div>
          ) : packages.length === 0 ? (
            <div className="p-8 text-center text-sm text-gray-500">No packages found.</div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Duration</TableHead>
                  <TableHead>Validity</TableHead>
                  <TableHead>Price</TableHead>
                  <TableHead>Target Types</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Created</TableHead>
                  <TableHead>Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {packages.map((pkg) => (
                  <TableRow key={pkg.id}>
                    <TableCell className="font-medium">{pkg.name}</TableCell>
                    <TableCell>{pkg.total_duration_hours}h</TableCell>
                    <TableCell>{pkg.validity_window_hours}h</TableCell>
                    <TableCell>
                      {pkg.price_amount.toLocaleString('id-ID', {
                        style: 'currency',
                        currency: 'IDR',
                        maximumFractionDigits: 0,
                      })}
                    </TableCell>
                    <TableCell>
                      <div className="flex flex-wrap gap-1">
                        {pkg.allowed_target_types.map((tt) => (
                          <Badge key={tt} variant="info" className="text-xs">
                            {tt.replace('_', ' ')}
                          </Badge>
                        ))}
                      </div>
                    </TableCell>
                    <TableCell>
                      <Badge variant={pkg.is_active ? 'success' : 'default'}>
                        {pkg.is_active ? 'Active' : 'Inactive'}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-sm text-gray-500">
                      {formatDate(pkg.created_at)}
                    </TableCell>
                    <TableCell>
                      <div className="flex items-center gap-2">
                        <Button
                          variant="secondary"
                          size="sm"
                          onClick={() => openEdit(pkg)}
                          title="Edit"
                        >
                          <Pencil className="h-3.5 w-3.5" />
                        </Button>
                        <Button
                          variant="secondary"
                          size="sm"
                          onClick={() => handleToggleActive(pkg)}
                          disabled={toggling === pkg.id}
                          title={pkg.is_active ? 'Disable' : 'Enable'}
                        >
                          {pkg.is_active ? (
                            <PowerOff className="h-3.5 w-3.5 text-red-500" />
                          ) : (
                            <Power className="h-3.5 w-3.5 text-green-600" />
                          )}
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      <PackageFormModal
        isOpen={formOpen}
        onClose={() => setFormOpen(false)}
        onSuccess={fetchPackages}
        editPackage={editTarget}
      />
    </div>
  )
}
