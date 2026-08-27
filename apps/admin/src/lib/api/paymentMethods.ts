import { api } from './client'
import type {
  PaymentMethodItem,
  PaymentMethodsListResponse,
  UpdatePaymentMethodRequest,
  UpdatePaymentMethodResponse,
  PaymentMethodPreviewRequest,
  PaymentMethodPreviewResponse,
} from '@/types/payment-methods'

// ============================================================================
// PAYMENT METHOD FEE CONFIG API (PASS_18W)
// ============================================================================

/**
 * List all payment methods (enabled and disabled).
 * GET /api/v1/admin/payment-methods
 * Requires: finance.payment_method.view
 */
export async function getPaymentMethods() {
  const resp = await api.get<{ data: PaymentMethodsListResponse }>(
    '/api/v1/admin/payment-methods'
  )
  return resp.data
}

/**
 * Get a single payment method's full config.
 * GET /api/v1/admin/payment-methods/:code
 * Requires: finance.payment_method.view
 */
export async function getPaymentMethod(code: string) {
  const resp = await api.get<{ data: { method: PaymentMethodItem } }>(
    `/api/v1/admin/payment-methods/${encodeURIComponent(code)}`
  )
  return resp.data.method
}

/**
 * Update a payment method's fee config, enabled status, display name, sort
 * order, or Midtrans channel mapping.
 * PUT /api/v1/admin/payment-methods/:code
 * Requires: finance.payment_method.manage
 *
 * Only affects payments created after this call — existing orders/payments
 * are never recalculated (backend enforced).
 */
export async function updatePaymentMethod(code: string, payload: UpdatePaymentMethodRequest) {
  const resp = await api.put<{ data: UpdatePaymentMethodResponse }>(
    `/api/v1/admin/payment-methods/${encodeURIComponent(code)}`,
    payload
  )
  return resp.data
}

/**
 * Simulate the buyer payment fee/gross for a sample base amount, using
 * either a saved or not-yet-saved fee config. Pure computation — never
 * reads or writes the saved row.
 * POST /api/v1/admin/payment-methods/:code/preview
 * Requires: finance.payment_method.view
 */
export async function previewPaymentMethodFee(
  code: string,
  payload: PaymentMethodPreviewRequest
) {
  const resp = await api.post<{ data: PaymentMethodPreviewResponse }>(
    `/api/v1/admin/payment-methods/${encodeURIComponent(code)}/preview`,
    payload
  )
  return resp.data
}
