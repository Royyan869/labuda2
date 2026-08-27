import { useState, useEffect, useCallback } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { Modal } from '@/components/ui/Modal'
import {
  getPaymentMethods,
  updatePaymentMethod,
  previewPaymentMethodFee,
} from '@/lib/api'
import type {
  PaymentMethodItem,
  PaymentMethodFeeType,
  PaymentMethodRateSource,
  UpdatePaymentMethodRequest,
  PaymentMethodPreviewResponse,
} from '@/types/payment-methods'
import { ALLOWED_MIDTRANS_CHANNELS } from '@/types/payment-methods'
import { useAuth } from '@/hooks/useAuth'
import { hasCapability } from '@/lib/permissions'
import { RefreshCw, AlertTriangle, CreditCard, Edit2, X, Check, ShieldAlert, PlayCircle } from 'lucide-react'

// ============================================================================
// Helpers
// ============================================================================

function formatIdr(amount: number): string {
  return `Rp ${amount.toLocaleString('id-ID')}`
}

function formatBps(bps: number): string {
  return `${(bps / 100).toFixed(2)}%`
}

/** Short, human-readable summary of a method's fee formula for the table. */
function formulaSummary(m: PaymentMethodItem): string {
  switch (m.fee_type) {
    case 'flat':
      return formatIdr(m.flat_amount_rupiah)
    case 'percent':
      return formatBps(m.percent_bps)
    case 'percent_plus_flat':
      return `${formatBps(m.percent_bps)} + ${formatIdr(m.flat_amount_rupiah)}`
    default:
      return '—'
  }
}

const FEE_TYPE_LABELS: Record<PaymentMethodFeeType, string> = {
  flat: 'Flat',
  percent: 'Percent',
  percent_plus_flat: 'Percent + Flat',
}

// ============================================================================
// Rate source (PASS_19A)
// ============================================================================

const RATE_SOURCE_LABELS: Record<PaymentMethodRateSource, string> = {
  public_baseline: 'Public Baseline',
  merchant_verified: 'Merchant Verified',
  manual_override: 'Manual Override',
}

const RATE_SOURCE_BADGE_VARIANT: Record<PaymentMethodRateSource, 'warning' | 'success' | 'default'> = {
  public_baseline: 'warning',
  merchant_verified: 'success',
  manual_override: 'default',
}

function RateSourceBadge({ rateSource }: { rateSource: PaymentMethodRateSource }) {
  return <Badge variant={RATE_SOURCE_BADGE_VARIANT[rateSource]}>{RATE_SOURCE_LABELS[rateSource]}</Badge>
}

// ============================================================================
// Edit + Preview Modal
// ============================================================================

interface EditModalProps {
  method: PaymentMethodItem
  onClose: () => void
  onSaved: (updated: PaymentMethodItem) => void
}

function PaymentMethodEditModal({ method, onClose, onSaved }: EditModalProps) {
  const [displayName, setDisplayName] = useState(method.display_name)
  const [enabled, setEnabled] = useState(method.enabled)
  const [feeType, setFeeType] = useState<PaymentMethodFeeType>(method.fee_type)
  const [flatAmount, setFlatAmount] = useState(String(method.flat_amount_rupiah))
  const [percentBps, setPercentBps] = useState(String(method.percent_bps))
  const [minFee, setMinFee] = useState(method.min_fee_rupiah != null ? String(method.min_fee_rupiah) : '')
  const [maxFee, setMaxFee] = useState(method.max_fee_rupiah != null ? String(method.max_fee_rupiah) : '')
  const [channels, setChannels] = useState<string[]>(method.midtrans_channels)
  const [sortOrder, setSortOrder] = useState(String(method.sort_order))
  const [rateSource, setRateSource] = useState<PaymentMethodRateSource>(method.rate_source)
  const [rateSourceNote, setRateSourceNote] = useState(method.rate_source_note ?? '')

  const [saving, setSaving] = useState(false)
  const [saveError, setSaveError] = useState<string | null>(null)

  const [previewBase, setPreviewBase] = useState('100000')
  const [previewResult, setPreviewResult] = useState<PaymentMethodPreviewResponse | null>(null)
  const [previewing, setPreviewing] = useState(false)
  const [previewError, setPreviewError] = useState<string | null>(null)

  function toggleChannel(ch: string) {
    setChannels((prev) => (prev.includes(ch) ? prev.filter((c) => c !== ch) : [...prev, ch]))
  }

  function buildPayload(): UpdatePaymentMethodRequest {
    return {
      display_name: displayName.trim(),
      enabled,
      fee_type: feeType,
      flat_amount_rupiah: parseInt(flatAmount || '0', 10),
      percent_bps: parseInt(percentBps || '0', 10),
      min_fee_rupiah: minFee.trim() === '' ? null : parseInt(minFee, 10),
      max_fee_rupiah: maxFee.trim() === '' ? null : parseInt(maxFee, 10),
      midtrans_channels: channels,
      sort_order: parseInt(sortOrder || '0', 10),
      rate_source: rateSource,
      rate_source_note: rateSourceNote.trim() === '' ? undefined : rateSourceNote.trim(),
    }
  }

  async function handlePreview() {
    setPreviewing(true)
    setPreviewError(null)
    setPreviewResult(null)
    try {
      const payload = buildPayload()
      const result = await previewPaymentMethodFee(method.method_code, {
        fee_type: payload.fee_type,
        flat_amount_rupiah: payload.flat_amount_rupiah,
        percent_bps: payload.percent_bps,
        min_fee_rupiah: payload.min_fee_rupiah,
        max_fee_rupiah: payload.max_fee_rupiah,
        base_amount_rupiah: parseInt(previewBase || '0', 10),
      })
      setPreviewResult(result)
    } catch (err) {
      setPreviewError(err instanceof Error ? err.message : 'Failed to preview')
    } finally {
      setPreviewing(false)
    }
  }

  async function handleSave() {
    setSaving(true)
    setSaveError(null)
    try {
      const result = await updatePaymentMethod(method.method_code, buildPayload())
      onSaved(result.method)
      onClose()
    } catch (err) {
      setSaveError(err instanceof Error ? err.message : 'Failed to save')
    } finally {
      setSaving(false)
    }
  }

  return (
    <Modal isOpen onClose={onClose} title={`Edit ${method.method_code}`}>
      <div className="space-y-5 max-h-[70vh] overflow-y-auto pr-1">
        {/* Safety copy */}
        <div className="flex items-start gap-3 p-3 bg-amber-50 border border-amber-200 rounded-lg">
          <ShieldAlert className="h-5 w-5 text-amber-600 mt-0.5 flex-shrink-0" />
          <div className="text-sm text-amber-800 space-y-1">
            <p>Fee dihitung backend saat buyer membuat pembayaran.</p>
            <p>Perubahan hanya berlaku untuk payment baru — order/payment lama tidak berubah.</p>
            <p>Jangan isi rate sebelum cocok dengan kontrak Midtrans merchant.</p>
          </div>
        </div>

        {/* Basic fields */}
        <div className="grid grid-cols-2 gap-4">
          <div className="col-span-2">
            <label className="block text-sm font-medium text-gray-700 mb-1">Display Name</label>
            <input
              type="text"
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              className="w-full border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
          </div>

          <div className="flex items-end pb-2">
            <label className="flex items-center gap-2 text-sm font-medium text-gray-700 cursor-pointer">
              <input
                type="checkbox"
                checked={enabled}
                onChange={(e) => setEnabled(e.target.checked)}
                className="h-4 w-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
              />
              Enabled
            </label>
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Sort Order</label>
            <input
              type="number"
              value={sortOrder}
              onChange={(e) => setSortOrder(e.target.value)}
              className="w-full border border-gray-300 rounded px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
          </div>

          <div className="col-span-2">
            <label className="block text-sm font-medium text-gray-700 mb-1">Fee Type</label>
            <select
              value={feeType}
              onChange={(e) => setFeeType(e.target.value as PaymentMethodFeeType)}
              className="w-full border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
            >
              <option value="flat">Flat</option>
              <option value="percent">Percent</option>
              <option value="percent_plus_flat">Percent + Flat</option>
            </select>
          </div>

          {(feeType === 'flat' || feeType === 'percent_plus_flat') && (
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Flat Amount (Rupiah)</label>
              <input
                type="number"
                min={0}
                value={flatAmount}
                onChange={(e) => setFlatAmount(e.target.value)}
                className="w-full border border-gray-300 rounded px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-blue-500"
                placeholder="e.g. 4000"
              />
            </div>
          )}

          {(feeType === 'percent' || feeType === 'percent_plus_flat') && (
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Percent (basis points)</label>
              <input
                type="number"
                min={0}
                max={2000}
                value={percentBps}
                onChange={(e) => setPercentBps(e.target.value)}
                className="w-full border border-gray-300 rounded px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-blue-500"
                placeholder="e.g. 70 = 0.7%"
              />
              {percentBps && !isNaN(parseInt(percentBps)) && (
                <p className="mt-1 text-xs text-gray-500">= {formatBps(parseInt(percentBps))}</p>
              )}
            </div>
          )}

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Min Fee (Rupiah, optional)</label>
            <input
              type="number"
              min={0}
              value={minFee}
              onChange={(e) => setMinFee(e.target.value)}
              className="w-full border border-gray-300 rounded px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-blue-500"
              placeholder="none"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Max Fee (Rupiah, optional)</label>
            <input
              type="number"
              min={0}
              value={maxFee}
              onChange={(e) => setMaxFee(e.target.value)}
              className="w-full border border-gray-300 rounded px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-blue-500"
              placeholder="none"
            />
          </div>
        </div>

        {/* Midtrans channels */}
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">
            Midtrans Channels {enabled && <span className="text-red-600">(required when enabled)</span>}
          </label>
          <div className="flex flex-wrap gap-2">
            {ALLOWED_MIDTRANS_CHANNELS.map((ch) => (
              <button
                key={ch}
                type="button"
                onClick={() => toggleChannel(ch)}
                className={`text-xs px-2 py-1 rounded border font-mono ${
                  channels.includes(ch)
                    ? 'bg-blue-100 border-blue-400 text-blue-800'
                    : 'bg-gray-50 border-gray-200 text-gray-500'
                }`}
              >
                {ch}
              </button>
            ))}
          </div>
        </div>

        {/* Rate source (PASS_19A) */}
        <div className="border border-gray-200 rounded-lg p-3 space-y-3 bg-gray-50">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Rate Source</label>
            <select
              value={rateSource}
              onChange={(e) => setRateSource(e.target.value as PaymentMethodRateSource)}
              className="w-full border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
            >
              <option value="public_baseline">Public Baseline</option>
              <option value="merchant_verified">Merchant Verified</option>
              <option value="manual_override">Manual Override</option>
            </select>
            <p className="mt-1 text-xs text-gray-500">
              Public baseline bukan rate kontrak merchant Labuda. Jika Anda mengubah nilai fee dari
              baseline tanpa memilih &quot;Merchant Verified&quot;, backend otomatis akan menandainya
              sebagai &quot;Manual Override&quot;.
            </p>
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">
              Rate Source Note {rateSource === 'merchant_verified' && <span className="text-red-600">(wajib diisi)</span>}
            </label>
            <textarea
              value={rateSourceNote}
              onChange={(e) => setRateSourceNote(e.target.value)}
              rows={2}
              className="w-full border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
              placeholder="mis. Dikonfirmasi dari dashboard merchant Midtrans tanggal ..."
            />
          </div>
          {method.merchant_verified_at && (
            <p className="text-xs text-gray-500">
              Terakhir diverifikasi merchant: {new Date(method.merchant_verified_at).toLocaleString('id-ID')}
            </p>
          )}
        </div>

        {/* Preview simulation */}
        <div className="border border-gray-200 rounded-lg p-3 space-y-2 bg-gray-50">
          <p className="text-sm font-medium text-gray-700 flex items-center gap-1">
            <PlayCircle className="h-4 w-4" /> Preview Simulation
          </p>
          <div className="flex items-center gap-2">
            <input
              type="number"
              min={1}
              value={previewBase}
              onChange={(e) => setPreviewBase(e.target.value)}
              className="border border-gray-300 rounded px-3 py-1.5 text-sm font-mono w-40 focus:outline-none focus:ring-2 focus:ring-blue-500"
              placeholder="Base amount (Rupiah)"
            />
            <Button size="sm" variant="secondary" onClick={handlePreview} disabled={previewing}>
              {previewing ? 'Menghitung…' : 'Simulasikan'}
            </Button>
          </div>
          {previewError && (
            <p className="text-xs text-red-600 flex items-center gap-1">
              <AlertTriangle className="h-3 w-3" /> {previewError}
            </p>
          )}
          {previewResult && (
            <dl className="grid grid-cols-2 gap-x-4 gap-y-1 text-sm mt-2">
              <dt className="text-gray-600">Buyer Payment Fee</dt>
              <dd className="font-mono font-semibold">{formatIdr(previewResult.buyer_payment_fee_rupiah)}</dd>
              <dt className="text-gray-600">Gross Amount</dt>
              <dd className="font-mono font-semibold">{formatIdr(previewResult.gross_amount_rupiah)}</dd>
              <dt className="text-gray-600">Formula</dt>
              <dd className="font-mono text-xs text-gray-700">{previewResult.formula}</dd>
              {previewResult.clamped && (
                <>
                  <dt className="text-gray-600">Note</dt>
                  <dd className="text-amber-700 text-xs">min/max clamp applied</dd>
                </>
              )}
            </dl>
          )}
        </div>

        {saveError && (
          <p className="text-sm text-red-600 flex items-center gap-1">
            <AlertTriangle className="h-4 w-4" /> {saveError}
          </p>
        )}

        <div className="flex items-center gap-2 pt-3 border-t border-gray-100">
          <Button onClick={handleSave} disabled={saving}>
            <Check className="h-4 w-4 mr-1" />
            {saving ? 'Saving…' : 'Save'}
          </Button>
          <Button variant="ghost" onClick={onClose} disabled={saving}>
            <X className="h-4 w-4 mr-1" />
            Cancel
          </Button>
        </div>
      </div>
    </Modal>
  )
}

// ============================================================================
// Table row
// ============================================================================

interface RowProps {
  method: PaymentMethodItem
  canEdit: boolean
  onEdit: () => void
}

function MethodRow({ method, canEdit, onEdit }: RowProps) {
  return (
    <tr className="hover:bg-gray-50">
      <td className="px-6 py-3 font-mono text-xs text-gray-900 font-medium">{method.method_code}</td>
      <td className="px-6 py-3 text-sm text-gray-900">{method.display_name}</td>
      <td className="px-6 py-3">
        <Badge variant={method.enabled ? 'success' : 'default'}>
          {method.enabled ? 'Enabled' : 'Disabled'}
        </Badge>
      </td>
      <td className="px-6 py-3 text-sm text-gray-700">{FEE_TYPE_LABELS[method.fee_type]}</td>
      <td className="px-6 py-3 font-mono text-sm text-gray-900">{formulaSummary(method)}</td>
      <td className="px-6 py-3">
        <RateSourceBadge rateSource={method.rate_source} />
      </td>
      <td className="px-6 py-3 font-mono text-xs text-gray-600">
        {method.min_fee_rupiah != null || method.max_fee_rupiah != null
          ? `${method.min_fee_rupiah != null ? formatIdr(method.min_fee_rupiah) : '—'} / ${
              method.max_fee_rupiah != null ? formatIdr(method.max_fee_rupiah) : '—'
            }`
          : '—'}
      </td>
      <td className="px-6 py-3 text-xs text-gray-600">{method.sort_order}</td>
      <td className="px-6 py-3">
        {canEdit ? (
          <Button variant="ghost" size="sm" onClick={onEdit}>
            <Edit2 className="h-3 w-3 mr-1" />
            Edit
          </Button>
        ) : (
          <span className="text-xs text-gray-400 italic">Requires manage capability</span>
        )}
      </td>
    </tr>
  )
}

// ============================================================================
// Main Page
// ============================================================================

export function PaymentMethodsPage() {
  const [methods, setMethods] = useState<PaymentMethodItem[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [editing, setEditing] = useState<PaymentMethodItem | null>(null)
  const { capabilities } = useAuth()
  const canEdit = hasCapability(capabilities, 'finance.payment_method.manage')

  const fetchMethods = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const resp = await getPaymentMethods()
      setMethods(resp?.methods ?? [])
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch payment methods')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchMethods()
  }, [fetchMethods])

  function handleSaved(updated: PaymentMethodItem) {
    setMethods((prev) => prev.map((m) => (m.method_code === updated.method_code ? updated : m)))
  }

  const noneMerchantVerified = methods.length > 0 && methods.every((m) => m.rate_source !== 'merchant_verified')

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold text-gray-900 flex items-center gap-2">
            <CreditCard className="h-7 w-7" />
            Payment Methods
          </h1>
          <p className="text-gray-600 mt-1">
            Buyer payment method fee configuration. Fee dihitung backend saat buyer membuat
            pembayaran; perubahan di sini hanya berlaku untuk payment baru.
            {!canEdit && ' (View only.)'}
          </p>
        </div>
        <Button variant="ghost" size="sm" onClick={fetchMethods} disabled={loading}>
          <RefreshCw className={`h-4 w-4 mr-1 ${loading ? 'animate-spin' : ''}`} />
          Refresh
        </Button>
      </div>

      {noneMerchantVerified && (
        <div className="flex items-start gap-3 p-4 bg-amber-50 border border-amber-200 rounded-lg">
          <ShieldAlert className="h-5 w-5 text-amber-600 mt-0.5 flex-shrink-0" />
          <div className="text-sm text-amber-800 space-y-1">
            <p className="font-medium">Belum ada metode dengan rate merchant-verified.</p>
            <p>
              Semua rate saat ini adalah public baseline dari dokumentasi/harga publik Midtrans, bukan
              kontrak merchant Labuda. Rate dapat berubah dan bisa berbeda dari kontrak merchant Labuda
              yang sebenarnya. Pemilik perlu memverifikasi rate ini dari dashboard/kontrak/laporan
              Midtrans sebelum melabelinya sebagai &quot;Merchant Verified&quot;.
            </p>
            <p>
              Catatan PPN: harga publik Midtrans umumnya belum termasuk PPN, kecuali QRIS, GoPay, dan
              ShopeePay. Labuda saat ini mengenakan biaya ke buyer sesuai jumlah/formula yang
              dikonfigurasi di sini — pemodelan pajak/settlement merchant final masih menjadi utang
              teknis terpisah.
            </p>
          </div>
        </div>
      )}

      {error && (
        <Card>
          <CardContent className="p-8 text-center">
            <AlertTriangle className="h-10 w-10 text-red-400 mx-auto mb-3" />
            <p className="text-gray-900 font-medium">Failed to load payment methods</p>
            <p className="text-gray-600 text-sm mt-1">{error}</p>
            <Button variant="secondary" size="sm" onClick={fetchMethods} className="mt-4">
              Retry
            </Button>
          </CardContent>
        </Card>
      )}

      {loading && methods.length === 0 && !error && (
        <Card>
          <CardContent className="p-8">
            <div className="space-y-4">
              {Array.from({ length: 3 }).map((_, i) => (
                <div key={i} className="animate-pulse flex items-center gap-4">
                  <div className="h-4 bg-gray-200 rounded w-40" />
                  <div className="h-4 bg-gray-200 rounded flex-1" />
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      )}

      {!loading && !error && methods.length === 0 && (
        <Card>
          <CardContent className="p-12 text-center">
            <CreditCard className="h-12 w-12 text-gray-300 mx-auto mb-4" />
            <h2 className="text-lg font-semibold text-gray-900">No Payment Methods</h2>
            <p className="text-gray-600 mt-1">No canonical payment methods are configured.</p>
          </CardContent>
        </Card>
      )}

      {methods.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle>Canonical Payment Methods</CardTitle>
          </CardHeader>
          <CardContent className="p-0">
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-gray-200 bg-gray-50">
                    <th className="px-6 py-3 text-left font-medium text-gray-600">Code</th>
                    <th className="px-6 py-3 text-left font-medium text-gray-600">Display Name</th>
                    <th className="px-6 py-3 text-left font-medium text-gray-600">Status</th>
                    <th className="px-6 py-3 text-left font-medium text-gray-600">Fee Type</th>
                    <th className="px-6 py-3 text-left font-medium text-gray-600">Formula</th>
                    <th className="px-6 py-3 text-left font-medium text-gray-600">Rate Source</th>
                    <th className="px-6 py-3 text-left font-medium text-gray-600">Min / Max</th>
                    <th className="px-6 py-3 text-left font-medium text-gray-600">Sort</th>
                    <th className="px-6 py-3 text-left font-medium text-gray-600">Action</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-200">
                  {methods.map((m) => (
                    <MethodRow
                      key={m.method_code}
                      method={m}
                      canEdit={canEdit}
                      onEdit={() => setEditing(m)}
                    />
                  ))}
                </tbody>
              </table>
            </div>
          </CardContent>
        </Card>
      )}

      {editing && (
        <PaymentMethodEditModal
          method={editing}
          onClose={() => setEditing(null)}
          onSaved={handleSaved}
        />
      )}
    </div>
  )
}
