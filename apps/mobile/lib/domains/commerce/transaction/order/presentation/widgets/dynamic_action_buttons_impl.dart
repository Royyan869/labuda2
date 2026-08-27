library;

// =============================================================================
// DYNAMIC ACTION BUTTONS - Decision V2 Contract
// =============================================================================
//
// Backend is the SINGLE SOURCE OF TRUTH for all UI actions.
//
// This widget:
// 1. Loops through primary_action + secondary_actions from backend
// 2. Renders buttons based on action metadata (label, endpoint, method)
// 3. Executes actions using backend-provided endpoint + method
// 4. NO hardcoded button visibility logic
// 5. NO fallback to status-based checks
//
// Action structure from backend:
// - type: Action type enum
// - label_key: Localization key for UI
// - enabled: Whether action is currently enabled
// - blocked: Why action is disabled (with resolution)
// - endpoint: API endpoint to call
// - method: HTTP method (POST, PATCH, etc.)
// - requires_idempotency: Whether action requires idempotency key
// - financial: Whether action affects money (ledger validation)
// - input_schema: Structured input definition with validation
// =============================================================================

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart' as core;
import 'package:labuda/domains/commerce/transaction/order/domain/domain.dart'
    as order_domain;

/// Action Button Callbacks - handlers for different action types
class ActionCallbacks {
  final void Function(order_domain.Action action) onAction;
  final VoidCallback? onRequestSupport;
  final VoidCallback? onChatSeller;

  const ActionCallbacks({
    required this.onAction,
    this.onRequestSupport,
    this.onChatSeller,
  });
}

/// Dynamic Action Buttons Widget
///
/// Renders buttons dynamically based on backend Decision V2 contract.
/// NO hardcoded logic - all button visibility and behavior comes from backend.
class DynamicActionButtons extends ConsumerWidget {
  final order_domain.DecisionContract decision;
  final ActionCallbacks callbacks;

  const DynamicActionButtons({
    super.key,
    required this.decision,
    required this.callbacks,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    // Collect all actions (primary + secondary)
    final allActions = decision.allActions;

    // Filter to only enabled actions
    final enabledActions = allActions.where((a) => a.enabled).toList();

    // No actions available
    if (enabledActions.isEmpty) {
      return _MinimalSupportAction(
        onRequestSupport: callbacks.onRequestSupport,
      );
    }

    // Separate primary and secondary actions
    final primaryAction = decision.primaryAction?.enabled == true
        ? decision.primaryAction
        : null;
    final selectedPrimaryAction = primaryAction;
    final secondaryActions = enabledActions
        .where(
          (a) =>
              selectedPrimaryAction == null ||
              a.type != selectedPrimaryAction.type,
        )
        .toList();

    return _ActionButtonsContainer(
      primaryAction: primaryAction,
      secondaryActions: secondaryActions,
      callbacks: callbacks,
    );
  }
}

/// Action Buttons Container
class _ActionButtonsContainer extends StatelessWidget {
  final order_domain.Action? primaryAction;
  final List<order_domain.Action> secondaryActions;
  final ActionCallbacks callbacks;

  const _ActionButtonsContainer({
    required this.primaryAction,
    required this.secondaryActions,
    required this.callbacks,
  });

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final selectedPrimaryAction = primaryAction;

    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: isDark ? const Color(0xFF1E1E1E) : Colors.white,
        borderRadius: const BorderRadius.only(
          topLeft: Radius.circular(12),
          topRight: Radius.circular(12),
        ),
        border: Border(
          top: BorderSide(
            color: isDark ? const Color(0xFF333333) : const Color(0xFFE0E0E0),
          ),
        ),
      ),
      child: SafeArea(
        top: false,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            // Primary action (main CTA)
            if (selectedPrimaryAction != null)
              _PrimaryActionButton(
                action: selectedPrimaryAction,
                onPressed: () => callbacks.onAction(selectedPrimaryAction),
              ),

            // Secondary actions
            if (secondaryActions.isNotEmpty) ...[
              if (selectedPrimaryAction != null) const SizedBox(height: 12),
              ...secondaryActions.map(
                (action) => _SecondaryActionButton(
                  action: action,
                  onPressed: () => callbacks.onAction(action),
                ),
              ),
              const SizedBox(height: 8),
            ] else if (primaryAction != null)
              const SizedBox(height: 8),

            // Support button (always available)
            TextButton.icon(
              onPressed: callbacks.onRequestSupport,
              icon: const Icon(Icons.support_agent, size: 16),
              label: const Text('Butuh Bantuan?'),
              style: TextButton.styleFrom(foregroundColor: Colors.grey[600]),
            ),

            // Chat Seller button (BATCH 2B - DIRECT ORDER → CHAT CONTINUITY)
            // Allows buyer↔seller communication through canonical commerce chat
            if (callbacks.onChatSeller != null)
              TextButton.icon(
                onPressed: callbacks.onChatSeller,
                icon: const Icon(Icons.chat_bubble_outline, size: 16),
                label: const Text('Chat Penjual'),
                style: TextButton.styleFrom(
                  foregroundColor: core.AppColors.primaryRed,
                ),
              ),
          ],
        ),
      ),
    );
  }
}

/// Primary Action Button
class _PrimaryActionButton extends StatelessWidget {
  final order_domain.Action action;
  final VoidCallback onPressed;

  const _PrimaryActionButton({required this.action, required this.onPressed});

  @override
  Widget build(BuildContext context) {
    return ElevatedButton.icon(
      onPressed: onPressed,
      style: ElevatedButton.styleFrom(
        backgroundColor: core.AppColors.primaryRed,
        foregroundColor: Colors.white,
        padding: const EdgeInsets.symmetric(vertical: 16),
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
      ),
      icon: _getIconForAction(action.type),
      label: Text(
        _getLabelForAction(action),
        style: const TextStyle(fontSize: 16, fontWeight: FontWeight.w600),
      ),
    );
  }

  Icon _getIconForAction(String actionType) {
    switch (actionType) {
      case 'mark_shipped':
      case 'update_tracking':
        return const Icon(Icons.local_shipping, size: 20);
      case 'complete':
        return const Icon(Icons.check_circle, size: 20);
      case 'request_refund':
        return const Icon(Icons.currency_exchange, size: 20);
      case 'open_dispute':
        return const Icon(Icons.report_problem, size: 20);
      case 'cancel':
        return const Icon(Icons.cancel, size: 20);
      case 'pay':
        return const Icon(Icons.payment, size: 20);
      case 'extend_confirmation':
        return const Icon(Icons.add_alarm, size: 20);
      default:
        return const Icon(Icons.arrow_forward, size: 20);
    }
  }

  String _getLabelForAction(order_domain.Action action) {
    // Use label_key from backend as the source of truth
    // Backend provides localized labels via label_key
    // For now, use the label_key directly - in production this would go through i18n
    // TODO: Use proper localization service with action.labelKey
    return _localizeLabel(action.labelKey);
  }

  /// Localize label using backend-provided label_key
  /// In production, this would use a proper i18n service
  String _localizeLabel(String labelKey) {
    // Map backend label keys to localized strings
    // Backend is the source of truth for these keys
    // B4A: action.confirm_receipt = "Terima Barang" (final acceptance).
    // action.complete_order is no longer used (was the old second-step).
    const labelMap = {
      'action.mark_shipped': 'Kirim Pesanan',
      'action.confirm_receipt': 'Terima Barang',
      'action.complete_order': 'Terima Barang',
      'action.request_refund': 'Ajukan Pengembalian',
      'action.open_dispute': 'Buka Dispute',
      'action.cancel_order': 'Batalkan Pesanan',
      'action.update_tracking': 'Update Resi',
      'action.extend_confirmation': 'Perpanjang Konfirmasi',
      'action.pay_now': 'Bayar Sekarang',
      'action.payment_continue': 'Lanjutkan Pembayaran',
      'action.payment_check_status': 'Cek Status Pembayaran',
      'action.pay_again': 'Bayar Ulang',
      'action.accept': 'Terima',
      'action.reject': 'Tolak',
    };

    return labelMap[labelKey] ??
        _formatLabelKey(labelKey); // Fallback to formatted key if not mapped
  }

  /// Format label key as fallback (e.g., "action.mark_shipped" -> "Mark Shipped")
  String _formatLabelKey(String key) {
    // Extract the part after the last dot
    final parts = key.split('.');
    final lastPart = parts.isNotEmpty ? parts.last : key;

    // Convert snake_case to Title Case
    return lastPart
        .split('_')
        .map(
          (word) => word.isNotEmpty
              ? '${word[0].toUpperCase()}${word.substring(1)}'
              : '',
        )
        .join(' ');
  }
}

/// Secondary Action Button
class _SecondaryActionButton extends StatelessWidget {
  final order_domain.Action action;
  final VoidCallback onPressed;

  const _SecondaryActionButton({required this.action, required this.onPressed});

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final isDestructive = _isDestructiveAction(action.type);

    return Padding(
      padding: const EdgeInsets.only(bottom: 12),
      child: OutlinedButton.icon(
        onPressed: onPressed,
        style: OutlinedButton.styleFrom(
          foregroundColor: isDestructive
              ? core.AppColors.statusError
              : (isDark ? Colors.white : Colors.black87),
          side: BorderSide(
            color: isDestructive
                ? core.AppColors.statusError.withValues(alpha: 0.3)
                : (isDark ? const Color(0xFF333333) : const Color(0xFFE0E0E0)),
          ),
          padding: const EdgeInsets.symmetric(vertical: 12),
          shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
        ),
        icon: _getIconForAction(action.type),
        label: Text(
          _getLabelForAction(action),
          style: const TextStyle(fontSize: 14, fontWeight: FontWeight.w500),
        ),
      ),
    );
  }

  Icon _getIconForAction(String actionType) {
    switch (actionType) {
      case 'mark_shipped':
      case 'update_tracking':
        return const Icon(Icons.local_shipping, size: 18);
      case 'complete':
        return const Icon(Icons.check_circle, size: 18);
      case 'request_refund':
        return const Icon(Icons.currency_exchange, size: 18);
      case 'open_dispute':
        return const Icon(Icons.report_problem, size: 18);
      case 'cancel':
        return const Icon(Icons.cancel, size: 18);
      case 'extend_confirmation':
        return const Icon(Icons.add_alarm, size: 18);
      case 'reject':
        return const Icon(Icons.close, size: 18);
      case 'accept':
        return const Icon(Icons.check, size: 18);
      default:
        return const Icon(Icons.arrow_forward, size: 18);
    }
  }

  String _getLabelForAction(order_domain.Action action) {
    // Use label_key from backend as the source of truth
    return _localizeLabel(action.labelKey);
  }

  /// Localize label using backend-provided label_key
  String _localizeLabel(String labelKey) {
    // B4A: Unified labels — "Terima Barang" for both confirm_receipt and complete_order.
    const labelMap = {
      'action.mark_shipped': 'Kirim Pesanan',
      'action.confirm_receipt': 'Terima Barang',
      'action.complete_order': 'Terima Barang',
      'action.request_refund': 'Ajukan Pengembalian',
      'action.open_dispute': 'Buka Dispute',
      'action.cancel_order': 'Batalkan',
      'action.update_tracking': 'Update Nomor Resi',
      'action.extend_confirmation': 'Perpanjang Konfirmasi',
      'action.pay_now': 'Bayar',
      'action.payment_continue': 'Lanjutkan Pembayaran',
      'action.payment_check_status': 'Cek Status Pembayaran',
      'action.pay_again': 'Bayar Ulang',
      'action.accept': 'Terima',
      'action.reject': 'Tolak',
    };

    return labelMap[labelKey] ?? _formatLabelKey(labelKey);
  }

  /// Format label key as fallback
  String _formatLabelKey(String key) {
    final parts = key.split('.');
    final lastPart = parts.isNotEmpty ? parts.last : key;
    return lastPart
        .split('_')
        .map(
          (word) => word.isNotEmpty
              ? '${word[0].toUpperCase()}${word.substring(1)}'
              : '',
        )
        .join(' ');
  }

  bool _isDestructiveAction(String actionType) {
    return actionType == 'cancel' ||
        actionType == 'reject' ||
        actionType == 'request_refund';
  }
}

/// Minimal Support Action - shown when no other actions are available
class _MinimalSupportAction extends StatelessWidget {
  final VoidCallback? onRequestSupport;

  const _MinimalSupportAction({required this.onRequestSupport});

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: isDark ? const Color(0xFF1E1E1E) : Colors.white,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: isDark ? const Color(0xFF333333) : const Color(0xFFE0E0E0),
        ),
      ),
      child: TextButton.icon(
        onPressed: onRequestSupport,
        icon: const Icon(Icons.support_agent, size: 16),
        label: const Text('Butuh Bantuan?'),
        style: TextButton.styleFrom(foregroundColor: Colors.grey[600]),
      ),
    );
  }
}
