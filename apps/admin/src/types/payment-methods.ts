// ============================================================================
// PAYMENT METHOD FEE CONFIG TYPES (PASS_18W)
// Based on GET/PUT /api/v1/admin/payment-methods
// ============================================================================
//
// Backend is the sole fee authority. This UI only edits the payment_methods
// table; the actual buyer payment fee/gross amount is computed by the
// backend at payment-creation time. Editing here never touches an existing
// order or payment.

export type PaymentMethodFeeType = 'flat' | 'percent' | 'percent_plus_flat'

/**
 * Where a method's fee rate came from (PASS_19A).
 *
 * - public_baseline: copied from Midtrans's public pricing page/docs, NOT a
 *   merchant dashboard/contract number. This is the default for every method
 *   until an owner merchant-verifies it.
 * - merchant_verified: an owner has confirmed the rate against Labuda's real
 *   Midtrans merchant contract/dashboard/report.
 * - manual_override: an admin edited the fee formula away from its
 *   public-baseline seed value without merchant verification.
 */
export type PaymentMethodRateSource = 'public_baseline' | 'merchant_verified' | 'manual_override'

/**
 * A single canonical payment method and its fee config.
 *
 * flat_amount_rupiah / percent_bps / min_fee_rupiah / max_fee_rupiah are all
 * Rupiah integers (percent_bps is basis points: 100 bps = 1%) — never a
 * cents/sen/minor-unit value.
 */
export interface PaymentMethodItem {
  method_code: string
  display_name: string
  enabled: boolean
  fee_type: PaymentMethodFeeType
  flat_amount_rupiah: number
  percent_bps: number
  min_fee_rupiah?: number
  max_fee_rupiah?: number
  midtrans_channels: string[]
  sort_order: number
  rate_source: PaymentMethodRateSource
  rate_source_note?: string
  merchant_verified_at?: string | null
}

export interface PaymentMethodsListResponse {
  methods: PaymentMethodItem[]
  count: number
}

/**
 * PUT request body. method_code is intentionally absent — it comes from the
 * URL and is immutable.
 */
export interface UpdatePaymentMethodRequest {
  display_name: string
  enabled: boolean
  fee_type: PaymentMethodFeeType
  flat_amount_rupiah: number
  percent_bps: number
  min_fee_rupiah?: number | null
  max_fee_rupiah?: number | null
  midtrans_channels: string[]
  sort_order: number
  /**
   * Required — the admin must always state whether they believe this config
   * is still the unverified public baseline, an owner-confirmed merchant
   * rate, or a manual override. The backend forces public_baseline to
   * manual_override if the fee formula changed and this is left as
   * public_baseline (PASS_19A); it never silently keeps a stale label.
   */
  rate_source: PaymentMethodRateSource
  rate_source_note?: string
}

export interface UpdatePaymentMethodResponse {
  method: PaymentMethodItem
  message: string
}

/** POST .../preview request — a hypothetical fee config + sample base amount. */
export interface PaymentMethodPreviewRequest {
  fee_type: PaymentMethodFeeType
  flat_amount_rupiah?: number
  percent_bps?: number
  min_fee_rupiah?: number | null
  max_fee_rupiah?: number | null
  base_amount_rupiah: number
}

export interface PaymentMethodPreviewResponse {
  method_code: string
  base_amount_rupiah: number
  buyer_payment_fee_rupiah: number
  gross_amount_rupiah: number
  clamped: boolean
  formula: string
}

/**
 * Canonical Midtrans channel allowlist (mirrors backend
 * entity.AllowedMidtransChannels).
 *
 * PASS_19A owner policy: card payment is allowed; PayLater/installment
 * products are not. shopeepay (which also fronts Midtrans's grouped
 * ShopeePay/SPayLater channel), akulaku, and kredivo are deliberately
 * absent — do not add them back without an explicit owner decision.
 */
export const ALLOWED_MIDTRANS_CHANNELS = [
  'bca_va', 'bni_va', 'bri_va', 'permata_va', 'cimb_va', 'bsi_va',
  'danamon_va', 'maybank_va', 'btn_va', 'other_va',
  'gopay', 'dana', 'ovo', 'linkaja',
  'other_qris',
  'credit_card', 'debit_card',
  'alfamart', 'indomaret',
] as const
