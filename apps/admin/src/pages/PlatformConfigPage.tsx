import { useState, useEffect, useCallback } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { Modal } from '@/components/ui/Modal'
import {
  getPlatformConfigs,
  getSellerSubscriptionConfig,
  updateSellerSubscriptionConfig,
  updatePlatformConfig,
} from '@/lib/api'
import type { PlatformConfigItem, SellerSubscriptionConfig } from '@/types/platform-config'
import { useAuth } from '@/hooks/useAuth'
import { hasCapability } from '@/lib/permissions'
import { RefreshCw, AlertTriangle, Settings, Edit2, X, Check, Lock, ShieldAlert } from 'lucide-react'

// ============================================================================
// Config key metadata — what the UI knows about each key
// ============================================================================

type ConfigCapability = 'config.update.financial' | 'config.update.general'

interface ConfigMeta {
  /** Capability required to edit this key */
  cap: ConfigCapability
  /** Validation: return error string or null */
  validate: (v: string) => string | null
  /** Short hint shown next to the input */
  hint: string
  /** Display category override */
  category: 'Financial' | 'Withdrawal' | 'General'
}

function isPercent(v: string): string | null {
  const n = parseFloat(v)
  if (isNaN(n) || n < 0 || n > 100) return 'Must be a number between 0 and 100'
  return null
}

/**
 * Keys that are safe to edit inline.
 * Keys not listed here are shown read-only (dangerous or future-only).
 */
const EDITABLE_KEYS: Record<string, ConfigMeta> = {
  listing_commission_percent: {
    cap: 'config.update.financial',
    validate: isPercent,
    hint: 'Percent 0–100',
    category: 'Financial',
  },
  auction_commission_percent: {
    cap: 'config.update.financial',
    validate: isPercent,
    hint: 'Percent 0–100',
    category: 'Financial',
  },
}

/**
 * Keys that are dangerous or have no runtime consumer — shown read-only with a lock badge.
 */
const DANGEROUS_KEYS = new Set(['min_withdrawal', 'max_withdrawal', 'withdrawal_threshold'])

/** Classify a config key into a display category. */
function getCategory(key: string): string {
  const meta = EDITABLE_KEYS[key]
  if (meta) return meta.category
  if (DANGEROUS_KEYS.has(key)) return 'Withdrawal'
  if (key.includes('withdrawal')) return 'Withdrawal'
  if (key.includes('commission') || key.includes('subscription')) return 'Financial'
  return 'General'
}

/** Render the display value of a config item. */
function displayValue(item: PlatformConfigItem): string {
  if (item.value_numeric !== undefined) return item.value_numeric
  if (item.value_text !== undefined) return item.value_text
  return '(not set)'
}

/** Format a Rupiah integer as an IDR display string. */
function formatIdr(amount: number): string {
  return `Rp${amount.toLocaleString('id-ID')}`
}

// ============================================================================
// Confirmation modal for financial config edits
// ============================================================================

interface FinancialConfirmModalProps {
  isOpen: boolean
  configKey: string
  oldValue: string
  newValue: string
  onConfirm: () => void
  onCancel: () => void
  saving: boolean
}

function FinancialConfirmModal({
  isOpen,
  configKey,
  oldValue,
  newValue,
  onConfirm,
  onCancel,
  saving,
}: FinancialConfirmModalProps) {
  return (
    <Modal isOpen={isOpen} onClose={onCancel} title="Confirm Financial Config Change">
      <div className="space-y-4">
        <div className="flex items-start gap-3 p-3 bg-amber-50 border border-amber-200 rounded-lg">
          <ShieldAlert className="h-5 w-5 text-amber-600 mt-0.5 flex-shrink-0" />
          <div className="text-sm text-amber-800">
            <p className="font-semibold">This change affects platform revenue calculations.</p>
            <p className="mt-1">
              The new value will apply to all future orders. Existing orders are unaffected
              (pricing snapshot is immutable at order creation).
            </p>
          </div>
        </div>

        <dl className="grid grid-cols-2 gap-x-4 gap-y-2 text-sm bg-gray-50 rounded-lg p-3">
          <dt className="font-medium text-gray-600">Config Key</dt>
          <dd className="font-mono text-gray-900">{configKey}</dd>
          <dt className="font-medium text-gray-600">Current Value</dt>
          <dd className="font-mono text-gray-700">{oldValue}</dd>
          <dt className="font-medium text-gray-600">New Value</dt>
          <dd className="font-mono font-bold text-gray-900">{newValue}</dd>
        </dl>

        <p className="text-xs text-gray-500">
          This action will be recorded in the audit log.
        </p>

        <div className="flex items-center gap-2 pt-1">
          <Button onClick={onConfirm} disabled={saving}>
            {saving ? 'Saving…' : 'Confirm Change'}
          </Button>
          <Button variant="ghost" onClick={onCancel} disabled={saving}>
            Cancel
          </Button>
        </div>
      </div>
    </Modal>
  )
}

// ============================================================================
// Inline-edit row component
// ============================================================================

interface EditableRowProps {
  item: PlatformConfigItem
  meta: ConfigMeta
  canEdit: boolean
  onSaved: (updated: PlatformConfigItem) => void
}

function EditableRow({ item, meta, canEdit, onSaved }: EditableRowProps) {
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState('')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [confirmOpen, setConfirmOpen] = useState(false)

  function startEdit() {
    setDraft(displayValue(item) === '(not set)' ? '' : displayValue(item))
    setError(null)
    setEditing(true)
  }

  function cancelEdit() {
    setEditing(false)
    setError(null)
    setConfirmOpen(false)
  }

  function requestSave() {
    const trimmed = draft.trim()
    if (!trimmed) { setError('Value cannot be empty'); return }
    const validationError = meta.validate(trimmed)
    if (validationError) { setError(validationError); return }

    // Financial keys require explicit confirmation before writing
    if (meta.cap === 'config.update.financial') {
      setConfirmOpen(true)
      return
    }
    void doSave(trimmed)
  }

  async function doSave(value: string) {
    setSaving(true)
    setError(null)
    setConfirmOpen(false)
    try {
      const result = await updatePlatformConfig(item.key, value)
      onSaved(result.config)
      setEditing(false)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save')
    } finally {
      setSaving(false)
    }
  }

  const valueType = item.value_numeric !== undefined ? 'numeric' : 'text'

  if (editing) {
    return (
      <tr className="bg-blue-50">
        <td className="px-6 py-3 font-mono text-xs text-gray-900 font-medium">{item.key}</td>
        <td className="px-6 py-3" colSpan={2}>
          <div className="flex items-center gap-2">
            <input
              type="text"
              value={draft}
              onChange={(e) => { setDraft(e.target.value); setError(null) }}
              className="border border-blue-400 rounded px-2 py-1 text-sm font-mono w-40 focus:outline-none focus:ring-2 focus:ring-blue-500"
              placeholder={meta.hint}
              autoFocus
              onKeyDown={(e) => { if (e.key === 'Enter') requestSave(); if (e.key === 'Escape') cancelEdit() }}
            />
            <span className="text-xs text-gray-400">{meta.hint}</span>
            {error && (
              <span className="text-xs text-red-600 flex items-center gap-1">
                <AlertTriangle className="h-3 w-3" />
                {error}
              </span>
            )}
          </div>
        </td>
        <td className="px-6 py-3 font-mono text-xs text-gray-600">
          {item.updated_by ? `${item.updated_by.slice(0, 8)}...` : '-'}
        </td>
        <td className="px-6 py-3 text-xs text-gray-600 whitespace-nowrap">
          {new Date(item.updated_at * 1000).toLocaleString()}
        </td>
        <td className="px-6 py-3">
          <div className="flex items-center gap-1">
            <Button size="sm" onClick={requestSave} disabled={saving}>
              <Check className="h-3 w-3 mr-1" />
              {saving ? 'Saving…' : 'Save'}
            </Button>
            <Button variant="ghost" size="sm" onClick={cancelEdit} disabled={saving}>
              <X className="h-3 w-3" />
            </Button>
          </div>
          <FinancialConfirmModal
            isOpen={confirmOpen}
            configKey={item.key}
            oldValue={displayValue(item)}
            newValue={draft.trim()}
            onConfirm={() => doSave(draft.trim())}
            onCancel={() => setConfirmOpen(false)}
            saving={saving}
          />
        </td>
      </tr>
    )
  }

  return (
    <tr className="hover:bg-gray-50">
      <td className="px-6 py-3 font-mono text-xs text-gray-900 font-medium">{item.key}</td>
      <td className="px-6 py-3 font-mono text-sm text-gray-900">{displayValue(item)}</td>
      <td className="px-6 py-3">
        <Badge variant={valueType === 'numeric' ? 'info' : 'pending'}>{valueType}</Badge>
      </td>
      <td className="px-6 py-3 font-mono text-xs text-gray-600">
        {item.updated_by ? `${item.updated_by.slice(0, 8)}...` : '-'}
      </td>
      <td className="px-6 py-3 text-xs text-gray-600 whitespace-nowrap">
        {new Date(item.updated_at * 1000).toLocaleString()}
      </td>
      <td className="px-6 py-3">
        {canEdit ? (
          <Button variant="ghost" size="sm" onClick={startEdit}>
            <Edit2 className="h-3 w-3 mr-1" />
            Edit
          </Button>
        ) : (
          <span className="text-xs text-gray-400 italic">
            Requires {meta.cap === 'config.update.financial' ? 'financial' : 'general'} cap
          </span>
        )}
      </td>
    </tr>
  )
}

/** Read-only row for dangerous/future-only keys */
function DangerousRow({ item }: { item: PlatformConfigItem }) {
  const valueType = item.value_numeric !== undefined ? 'numeric' : 'text'
  return (
    <tr className="hover:bg-gray-50 opacity-75">
      <td className="px-6 py-3 font-mono text-xs text-gray-900 font-medium">
        <span className="inline-flex items-center gap-1">
          {item.key}
          <span title="Not editable — no runtime consumer">
            <Lock className="inline h-3 w-3 text-gray-400" />
          </span>
        </span>
      </td>
      <td className="px-6 py-3 font-mono text-sm text-gray-900">{displayValue(item)}</td>
      <td className="px-6 py-3">
        <Badge variant={valueType === 'numeric' ? 'info' : 'pending'}>{valueType}</Badge>
      </td>
      <td className="px-6 py-3 font-mono text-xs text-gray-600">
        {item.updated_by ? `${item.updated_by.slice(0, 8)}...` : '-'}
      </td>
      <td className="px-6 py-3 text-xs text-gray-600 whitespace-nowrap">
        {new Date(item.updated_at * 1000).toLocaleString()}
      </td>
      <td className="px-6 py-3">
        <span className="text-xs text-amber-600 font-medium">future-only</span>
      </td>
    </tr>
  )
}

/** Read-only row for unrecognised keys */
function ReadOnlyRow({ item }: { item: PlatformConfigItem }) {
  const valueType = item.value_numeric !== undefined ? 'numeric' : 'text'
  return (
    <tr className="hover:bg-gray-50">
      <td className="px-6 py-3 font-mono text-xs text-gray-900 font-medium">{item.key}</td>
      <td className="px-6 py-3 font-mono text-sm text-gray-900">{displayValue(item)}</td>
      <td className="px-6 py-3">
        <Badge variant={valueType === 'numeric' ? 'info' : 'pending'}>{valueType}</Badge>
      </td>
      <td className="px-6 py-3 font-mono text-xs text-gray-600">
        {item.updated_by ? `${item.updated_by.slice(0, 8)}...` : '-'}
      </td>
      <td className="px-6 py-3 text-xs text-gray-600 whitespace-nowrap">
        {new Date(item.updated_at * 1000).toLocaleString()}
      </td>
      <td className="px-6 py-3">
        <span className="text-xs text-gray-400">-</span>
      </td>
    </tr>
  )
}

// ============================================================================
// Seller Subscription Config Card
// ============================================================================

interface SellerSubscriptionCardProps {
  onRefreshParent: () => void
  canEdit: boolean
}

function SellerSubscriptionCard({ onRefreshParent, canEdit }: SellerSubscriptionCardProps) {
  const [config, setConfig] = useState<SellerSubscriptionConfig | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [editing, setEditing] = useState(false)
  const [saving, setSaving] = useState(false)
  const [saveError, setSaveError] = useState<string | null>(null)
  const [confirmOpen, setConfirmOpen] = useState(false)

  // Edit form — yearly_fee_rupiah is stored as a plain Rupiah integer
  const [feeIdr, setFeeIdr] = useState('')
  const [durationDays, setDurationDays] = useState('')
  const [renewalReminderDays, setRenewalReminderDays] = useState('')
  const [isEnabled, setIsEnabled] = useState(true)

  const fetchConfig = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const cfg = await getSellerSubscriptionConfig()
      setConfig(cfg)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch subscription config')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { fetchConfig() }, [fetchConfig])

  function startEdit() {
    if (!config) return
    setFeeIdr(String(config.yearly_fee_rupiah))
    setDurationDays(String(config.duration_days))
    setRenewalReminderDays(String(config.renewal_reminder_days))
    setIsEnabled(config.enabled)
    setSaveError(null)
    setEditing(true)
  }

  function cancelEdit() {
    setEditing(false)
    setSaveError(null)
  }

  function requestSaveEdit() {
    const feeRupiah = parseInt(feeIdr, 10)
    const dur = parseInt(durationDays, 10)
    const reminder = parseInt(renewalReminderDays, 10)

    if (!feeRupiah || feeRupiah <= 0) { setSaveError('Fee must be > 0'); return }
    if (!dur || dur <= 0) { setSaveError('Duration must be > 0'); return }
    if (isNaN(reminder) || reminder < 0) { setSaveError('Renewal reminder must be >= 0'); return }
    if (reminder >= dur) { setSaveError('Renewal reminder must be < duration'); return }

    setSaveError(null)
    setConfirmOpen(true)
  }

  async function doSaveEdit() {
    const feeRupiah = parseInt(feeIdr, 10)
    const dur = parseInt(durationDays, 10)
    const reminder = parseInt(renewalReminderDays, 10)

    setSaving(true)
    setSaveError(null)
    setConfirmOpen(false)
    try {
      const result = await updateSellerSubscriptionConfig({
        yearly_fee_rupiah: feeRupiah,
        duration_days: dur,
        renewal_reminder_days: reminder,
        enabled: isEnabled,
      })
      if (result.config) {
        setConfig(result.config)
      } else {
        await fetchConfig()
      }
      setEditing(false)
      onRefreshParent()
    } catch (err) {
      setSaveError(err instanceof Error ? err.message : 'Failed to save')
    } finally {
      setSaving(false)
    }
  }

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle>Seller Subscription</CardTitle>
          <div className="flex items-center gap-2">
            {!editing && canEdit && config && (
              <Button variant="ghost" size="sm" onClick={startEdit}>
                <Edit2 className="h-4 w-4 mr-1" />
                Edit
              </Button>
            )}
            {!editing && (
              <Button variant="ghost" size="sm" onClick={fetchConfig} disabled={loading}>
                <RefreshCw className={`h-4 w-4 ${loading ? 'animate-spin' : ''}`} />
              </Button>
            )}
          </div>
        </div>
      </CardHeader>
      <CardContent>
        {loading && !config && (
          <div className="space-y-3">
            {Array.from({ length: 4 }).map((_, i) => (
              <div key={i} className="animate-pulse h-4 bg-gray-200 rounded w-48" />
            ))}
          </div>
        )}

        {error && (
          <div className="flex items-center gap-2 text-red-600 text-sm">
            <AlertTriangle className="h-4 w-4" />
            {error}
          </div>
        )}

        {!loading && !error && config && !editing && (
          <dl className="grid grid-cols-2 gap-x-8 gap-y-3 text-sm">
            <dt className="font-medium text-gray-600">Yearly Fee</dt>
            <dd className="font-mono">
              {formatIdr(config.yearly_fee_rupiah)}
            </dd>

            <dt className="font-medium text-gray-600">Duration</dt>
            <dd className="font-mono">{config.duration_days} days</dd>

            <dt className="font-medium text-gray-600">Renewal Reminder</dt>
            <dd className="font-mono">{config.renewal_reminder_days} days before expiry</dd>

            <dt className="font-medium text-gray-600">Status</dt>
            <dd>
              <Badge variant={config.enabled ? 'success' : 'default'}>
                {config.enabled ? 'Enabled' : 'Disabled'}
              </Badge>
            </dd>

            <dt className="font-medium text-gray-600">Config ID</dt>
            <dd className="font-mono text-xs text-gray-500">{config.id}</dd>

            <dt className="font-medium text-gray-600">Created At</dt>
            <dd className="text-xs text-gray-500">{new Date(config.created_at).toLocaleString()}</dd>
          </dl>
        )}

        {editing && (
          <div className="space-y-4">
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  Yearly Fee (IDR)
                </label>
                <input
                  type="number"
                  min={1}
                  value={feeIdr}
                  onChange={(e) => setFeeIdr(e.target.value)}
                  className="w-full border border-gray-300 rounded px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-blue-500"
                  placeholder="e.g. 70000"
                />
                {feeIdr && !isNaN(parseInt(feeIdr)) && (
                  <p className="mt-1 text-xs text-gray-500">
                    = {formatIdr(parseInt(feeIdr))}
                  </p>
                )}
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Duration (days)</label>
                <input
                  type="number"
                  min={1}
                  value={durationDays}
                  onChange={(e) => setDurationDays(e.target.value)}
                  className="w-full border border-gray-300 rounded px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-blue-500"
                  placeholder="e.g. 365"
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Renewal Reminder (days)</label>
                <input
                  type="number"
                  min={0}
                  value={renewalReminderDays}
                  onChange={(e) => setRenewalReminderDays(e.target.value)}
                  className="w-full border border-gray-300 rounded px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-blue-500"
                  placeholder="e.g. 7"
                />
              </div>

              <div className="flex items-end pb-2">
                <label className="flex items-center gap-2 text-sm font-medium text-gray-700 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={isEnabled}
                    onChange={(e) => setIsEnabled(e.target.checked)}
                    className="h-4 w-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
                  />
                  Enabled
                </label>
              </div>
            </div>

            {saveError && (
              <p className="text-sm text-red-600 flex items-center gap-1">
                <AlertTriangle className="h-4 w-4" />
                {saveError}
              </p>
            )}

            <div className="flex items-center gap-2 pt-1">
              <Button size="sm" onClick={requestSaveEdit} disabled={saving}>
                <Check className="h-4 w-4 mr-1" />
                {saving ? 'Saving…' : 'Save'}
              </Button>
              <Button variant="ghost" size="sm" onClick={cancelEdit} disabled={saving}>
                <X className="h-4 w-4 mr-1" />
                Cancel
              </Button>
            </div>
          </div>
        )}

        <FinancialConfirmModal
          isOpen={confirmOpen}
          configKey="seller_subscription_config"
          oldValue={config ? `Fee: ${formatIdr(config.yearly_fee_rupiah)}, ${config.duration_days}d, reminder ${config.renewal_reminder_days}d` : '—'}
          newValue={`Fee: ${formatIdr(parseInt(feeIdr || '0'))}, ${durationDays}d, reminder ${renewalReminderDays}d, enabled: ${isEnabled}`}
          onConfirm={doSaveEdit}
          onCancel={() => setConfirmOpen(false)}
          saving={saving}
        />
      </CardContent>
    </Card>
  )
}

// ============================================================================
// Main Page
// ============================================================================

export function PlatformConfigPage() {
  const [configs, setConfigs] = useState<PlatformConfigItem[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const { capabilities } = useAuth()
  const canEditFinancial = hasCapability(capabilities, 'config.update.financial')
  const canEditGeneral = hasCapability(capabilities, 'config.update.general')

  const fetchConfigs = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const response = await getPlatformConfigs()
      setConfigs(response?.configs ?? [])
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch platform config')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchConfigs()
  }, [fetchConfigs])

  /** Replace one item in the list after a successful save. */
  function handleSaved(key: string, updated: PlatformConfigItem) {
    setConfigs((prev) => prev.map((c) => (c.key === key ? updated : c)))
  }

  /** Determine if the actor can edit a given config key. */
  function canEdit(key: string): boolean {
    const meta = EDITABLE_KEYS[key]
    if (!meta) return false
    if (meta.cap === 'config.update.financial') return canEditFinancial
    return canEditGeneral
  }

  // Group configs by category for display
  const grouped = configs.reduce<Record<string, PlatformConfigItem[]>>((acc, item) => {
    const cat = getCategory(item.key)
    if (!acc[cat]) acc[cat] = []
    acc[cat].push(item)
    return acc
  }, {})

  const categories = Object.keys(grouped).sort()

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold text-gray-900">Platform Config</h1>
          <p className="text-gray-600 mt-1">
            Runtime configuration.
            {canEditFinancial && ' Financial keys editable.'}
            {canEditGeneral && !canEditFinancial && ' General keys editable.'}
            {!canEditFinancial && !canEditGeneral && ' View only.'}
          </p>
        </div>
        <Button variant="ghost" size="sm" onClick={fetchConfigs} disabled={loading}>
          <RefreshCw className={`h-4 w-4 mr-1 ${loading ? 'animate-spin' : ''}`} />
          Refresh
        </Button>
      </div>

      {/* Seller Subscription Config — editable card */}
      <SellerSubscriptionCard onRefreshParent={fetchConfigs} canEdit={canEditFinancial} />

      {/* Error State */}
      {error && (
        <Card>
          <CardContent className="p-8 text-center">
            <AlertTriangle className="h-10 w-10 text-red-400 mx-auto mb-3" />
            <p className="text-gray-900 font-medium">Failed to load config</p>
            <p className="text-gray-600 text-sm mt-1">{error}</p>
            <Button variant="secondary" size="sm" onClick={fetchConfigs} className="mt-4">
              Retry
            </Button>
          </CardContent>
        </Card>
      )}

      {/* Loading State */}
      {loading && configs.length === 0 && !error && (
        <Card>
          <CardContent className="p-8">
            <div className="space-y-4">
              {Array.from({ length: 5 }).map((_, i) => (
                <div key={i} className="animate-pulse flex items-center gap-4">
                  <div className="h-4 bg-gray-200 rounded w-40" />
                  <div className="h-4 bg-gray-200 rounded flex-1" />
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      )}

      {/* Empty State */}
      {!loading && !error && configs.length === 0 && (
        <Card>
          <CardContent className="p-12 text-center">
            <Settings className="h-12 w-12 text-gray-300 mx-auto mb-4" />
            <h2 className="text-lg font-semibold text-gray-900">No Config Values</h2>
            <p className="text-gray-600 mt-1">No platform configuration values are set.</p>
          </CardContent>
        </Card>
      )}

      {/* Config Table grouped by category */}
      {categories.map((category) => (
        <Card key={category}>
          <CardHeader>
            <CardTitle>{category}</CardTitle>
          </CardHeader>
          <CardContent className="p-0">
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-gray-200 bg-gray-50">
                    <th className="px-6 py-3 text-left font-medium text-gray-600">Key</th>
                    <th className="px-6 py-3 text-left font-medium text-gray-600">Value</th>
                    <th className="px-6 py-3 text-left font-medium text-gray-600">Type</th>
                    <th className="px-6 py-3 text-left font-medium text-gray-600">Updated By</th>
                    <th className="px-6 py-3 text-left font-medium text-gray-600">Updated At</th>
                    <th className="px-6 py-3 text-left font-medium text-gray-600">Action</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-200">
                  {grouped[category].map((item) => {
                    if (DANGEROUS_KEYS.has(item.key)) {
                      return <DangerousRow key={item.key} item={item} />
                    }
                    const meta = EDITABLE_KEYS[item.key]
                    if (meta) {
                      return (
                        <EditableRow
                          key={item.key}
                          item={item}
                          meta={meta}
                          canEdit={canEdit(item.key)}
                          onSaved={(updated) => handleSaved(item.key, updated)}
                        />
                      )
                    }
                    return <ReadOnlyRow key={item.key} item={item} />
                  })}
                </tbody>
              </table>
            </div>
          </CardContent>
        </Card>
      ))}
    </div>
  )
}
