library;

import 'package:labuda/core/core.dart' as core;
import 'package:labuda/shared/widgets/app_dialog.dart';

// =============================================================================
// ORDER ACTION HANDLER - Decision V2 Contract
// =============================================================================
//
// Routes actions from backend Decision V2 contract to appropriate handlers.
// Each action contains:
// - type: Action type enum
// - endpoint: API endpoint to call
// - method: HTTP method
// - input_schema: Structured input definition
//
// The handler executes actions using backend-provided metadata,
// NOT hardcoded routing logic.
// =============================================================================

import 'package:flutter/material.dart';
import 'package:labuda/core/src/theme/app_theme.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/domain.dart'
    as order_domain;
import 'package:labuda/domains/commerce/transaction/order/order.dart';
import 'package:labuda/domains/commerce/transaction/order/presentation/widgets/order_action_label_resolver.dart';
import 'package:labuda/generated/app_localizations.dart';

/// Order Action Handler - routes backend actions to appropriate handlers
class OrderActionHandler {
  final Order order;
  final BuildContext context;

  // Seller action handler
  final void Function(
    String orderId,
    String sellerId,
    ShippingProofData proofData,
  )
  onShipOrder;

  // Buyer action handlers
  final void Function(String orderId, String buyerId) onConfirmDelivery;
  final void Function(String orderId) onExtendConfirmation;
  final void Function({
    required String orderId,
    required double orderSubtotal,
    required String buyerId,
    required String sellerId,
  })
  onRefundRequestRequest;
  final void Function(
    String orderId,
    String fromUserId,
    String toUserId,
    int rating,
    String? review,
  )
  onRate;
  final void Function(Order order) onPayNow;
  final void Function(String orderId, String reason) onCancelOrder;

  // Dispute handler
  final void Function({required String orderId}) onOpenDispute;

  // Support handler
  final VoidCallback onRequestSupport;

  // Chat-seller destination is owned by the order detail flow. The backend
  // action type `contact_seller` (label_key `action.chat_seller`) delegates
  // here — no new navigation architecture is introduced.
  final VoidCallback? onChatSeller;

  const OrderActionHandler({
    required this.order,
    required this.context,
    required this.onShipOrder,
    required this.onConfirmDelivery,
    required this.onExtendConfirmation,
    required this.onRefundRequestRequest,
    required this.onRate,
    required this.onPayNow,
    required this.onCancelOrder,
    required this.onOpenDispute,
    required this.onRequestSupport,
    this.onChatSeller,
  });

  /// Handle action based on backend-provided action type
  Future<void> handleAction(order_domain.Action action) async {
    // Check if action is blocked
    if (!action.enabled) {
      _showBlockedMessage(action);
      return;
    }

    // Route action based on type
    switch (action.type) {
      // Seller actions
      case 'mark_shipped':
        return _handleMarkShipped(action);
      case 'update_tracking':
        return _handleUpdateTracking(action);

      // B4A: "Terima Barang" = complete (single click, final acceptance + escrow release)
      case 'complete':
        return _handleCompleteOrder();
      case 'request_refund':
        return _handleRequestRefund();
      case 'pay':
        return _handlePayNow();
      case 'cancel':
        return _handleCancelOrder(action);

      // Common actions
      case 'open_dispute':
        return _handleOpenDispute();
      case 'extend_confirmation':
        return _handleExtendConfirmation();

      // Backend secondary actions with an existing destination in this flow
      // (I18N-07): labels stay presentation-only, routing stays on `type`.
      case 'contact_seller':
        return _handleContactSeller(action);
      case 'contact_support':
        return onRequestSupport();

      // Unknown action - show info (localized label, never a raw `action.*`)
      default:
        _showUnknownActionInfo(action);
    }
  }

  void _handleMarkShipped(order_domain.Action action) {
    // Show shipping proof dialog
    // This dialog will collect tracking number and call onShipOrder
    _showShippingDialog(action);
  }

  void _handleUpdateTracking(order_domain.Action action) {
    // Show update shipping reference dialog
    _showUpdateShippingReferenceDialog(action);
  }

  Future<void> _handleCompleteOrder() async {
    // B4A: Single-click final acceptance — releases funds to seller.
    // Must show clear confirmation dialog (financial action).
    final confirmed = await AppDialog.confirm(
      context: context,
      title: 'Terima Barang',
      content: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(
            Icons.warning_amber_outlined,
            color: context.statusColors.warning,
            size: AppIconSize.display,
          ),
          const SizedBox(height: 16),
          Text(
            'Anda yakin barang sudah diterima dengan baik?',
            style: context.typeRoles.titleCompact,
          ),
          const SizedBox(height: 8),
          Text(
            'Dengan menerima barang, pesanan akan selesai dan pembayaran akan diteruskan ke penjual. Tindakan ini tidak dapat dibatalkan.',
            style: context.typeRoles.bodyDense.copyWith(
              color: Theme.of(context).colorScheme.onSurfaceVariant,
            ),
          ),
        ],
      ),
      confirmLabel: 'Ya, Terima Barang',
      cancelLabel: 'Batal',
    );
    if (confirmed) {
      onConfirmDelivery(order.id, order.buyerId);
    }
  }

  void _handleRequestRefund() {
    onRefundRequestRequest(
      orderId: order.id,
      orderSubtotal: order.pricing.subtotal,
      buyerId: order.buyerId,
      sellerId: order.sellerId,
    );
  }

  void _handlePayNow() {
    onPayNow(order);
  }

  void _handleCancelOrder(order_domain.Action action) {
    // Check if action has input schema for reason
    _showCancelDialog(action);
  }

  void _handleOpenDispute() {
    onOpenDispute(orderId: order.id);
  }

  Future<void> _handleExtendConfirmation() {
    // Show confirmation extension dialog
    return _showExtendConfirmationDialog();
  }

  void _showBlockedMessage(order_domain.Action action) {
    final blocked = action.blocked;
    if (blocked == null) return;

    AppDialog.info(
      context: context,
      title: 'Action Not Available',
      // Never expose a raw backend key: fall back to the smallest existing
      // localized generic message when the backend omits a human reason.
      message: blocked.reason ?? AppLocalizations.of(context)!.anErrorOccurred,
      closeLabel: 'OK',
    );
  }

  /// `contact_seller` delegates to the chat-seller destination that already
  /// exists in the order detail flow. No evidence/API/navigation feature is
  /// created here — only routing of an existing callback.
  void _handleContactSeller(order_domain.Action action) {
    final chatSeller = onChatSeller;
    if (chatSeller != null) {
      chatSeller();
      return;
    }
    // Safe localized presentation while no callback is wired.
    _showUnknownActionInfo(action);
  }

  void _showUnknownActionInfo(order_domain.Action action) {
    final l10n = AppLocalizations.of(context)!;
    AppDialog.info(
      context: context,
      title: 'Action Info',
      content: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('Type: ${action.type}'),
          const SizedBox(height: 8),
          // Canonical localized label — raw `action.*` keys never reach UI.
          Text(resolveOrderActionLabel(l10n, action.labelKey)),
          const SizedBox(height: 8),
          Text('Endpoint: ${action.endpoint}'),
          const SizedBox(height: 8),
          Text('Method: ${action.method}'),
          if (action.financial) ...[
            const SizedBox(height: 8),
            const Text(
              'Financial Action',
              style: TextStyle(fontWeight: FontWeight.bold),
            ),
          ],
        ],
      ),
      closeLabel: 'Close',
    );
  }

  void _showShippingDialog(order_domain.Action action) {
    final referenceController = TextEditingController();
    final noteController = TextEditingController();
    String? referenceType = 'tracking'; // Default to tracking

    showDialog(
      context: context,
      builder: (context) {
        final colorScheme = Theme.of(context).colorScheme;
        return StatefulBuilder(
          builder: (context, setDialogState) => AlertDialog(
            title: const Text('Konfirmasi Pengiriman'),
            content: SingleChildScrollView(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  // Read-only shipping method from checkout
                  Text(
                    'Metode Pengiriman',
                    style: context.typeRoles.labelMicro.copyWith(
                      fontWeight: FontWeight.bold,
                    ),
                  ),
                  const SizedBox(height: 4),
                  Container(
                    padding: const EdgeInsets.all(core.AppMetrics.p12),
                    decoration: BoxDecoration(
                      color: colorScheme.surfaceContainerHigh,
                      borderRadius: BorderRadius.circular(core.AppShape.r8),
                      border: Border.all(color: colorScheme.outlineVariant),
                    ),
                    child: Row(
                      children: [
                        const Icon(
                          Icons.local_shipping,
                          size: AppIconSize.inlineGlyph,
                        ),
                        const SizedBox(width: 8),
                        Expanded(
                          child: Text(
                            _formatShippingMethod(),
                            style: context.typeRoles.bodyDense,
                          ),
                        ),
                      ],
                    ),
                  ),
                  const SizedBox(height: 16),

                  // Reference type selector
                  Text(
                    'Jenis Referensi',
                    style: context.typeRoles.labelMicro.copyWith(
                      fontWeight: FontWeight.bold,
                    ),
                  ),
                  const SizedBox(height: 8),
                  SegmentedButton<String>(
                    segments: const [
                      ButtonSegment(
                        value: 'tracking',
                        label: Text('Resi Kurir'),
                        icon: Icon(
                          Icons.qr_code,
                          size: AppIconSize.inlineGlyph,
                        ),
                      ),
                      ButtonSegment(
                        value: 'phone',
                        label: Text('No. HP/WA'),
                        icon: Icon(Icons.phone, size: AppIconSize.inlineGlyph),
                      ),
                      ButtonSegment(
                        value: 'other',
                        label: Text('Lainnya'),
                        icon: Icon(
                          Icons.more_horiz,
                          size: AppIconSize.inlineGlyph,
                        ),
                      ),
                    ],
                    selected: {referenceType ?? 'tracking'},
                    onSelectionChanged: (Set<String> selected) {
                      setDialogState(() {
                        referenceType = selected.first;
                      });
                    },
                  ),
                  const SizedBox(height: 16),

                  // Shipping reference input
                  Text(
                    referenceType == 'phone'
                        ? 'Nomor HP / WA'
                        : 'Referensi Pengiriman',
                    style: context.typeRoles.labelMicro.copyWith(
                      fontWeight: FontWeight.bold,
                    ),
                  ),
                  const SizedBox(height: 8),
                  TextField(
                    controller: referenceController,
                    decoration: InputDecoration(
                      labelText: referenceType == 'phone'
                          ? 'No. HP / WA'
                          : 'Referensi Pengiriman',
                      hintText: referenceType == 'phone'
                          ? 'Contoh: 08123456789'
                          : referenceType == 'tracking'
                          ? 'Contoh: JNE123456789'
                          : 'Referensi lainnya',
                    ),
                    textCapitalization: TextCapitalization.characters,
                    keyboardType: referenceType == 'phone'
                        ? TextInputType.phone
                        : TextInputType.text,
                  ),
                  const SizedBox(height: 12),

                  // Optional note
                  TextField(
                    controller: noteController,
                    // Border/fill come from `inputDecorationTheme` (AppTheme)
                    // — the one form-field authority.
                    decoration: const InputDecoration(
                      labelText: 'Catatan (opsional)',
                      hintText: 'Catatan pengiriman untuk pembeli...',
                    ),
                    maxLines: 2,
                  ),
                ],
              ),
            ),
            actions: [
              TextButton(
                onPressed: () => Navigator.pop(context),
                child: const Text('Batal'),
              ),
              ElevatedButton(
                onPressed: () {
                  final shippingReference = referenceController.text.trim();
                  if (shippingReference.isEmpty) return;
                  Navigator.pop(context);
                  onShipOrder(
                    order.id,
                    order.sellerId,
                    ShippingProofData(
                      shippingReference: shippingReference,
                      referenceType: referenceType,
                      note: noteController.text.trim().isEmpty
                          ? null
                          : noteController.text.trim(),
                    ),
                  );
                },
                child: const Text('Konfirmasi Pengiriman'),
              ),
            ],
          ),
        );
      },
    );
  }

  /// Formats the shipping method for display
  String _formatShippingMethod() {
    final method = order.shippingInfo.method;
    final courierName = order.shippingInfo.courierName;

    if (courierName != null && courierName.isNotEmpty) {
      return '${method.label} - $courierName';
    }
    return method.label;
  }

  void _showUpdateShippingReferenceDialog(order_domain.Action action) {
    final referenceController = TextEditingController();

    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Update Referensi Pengiriman'),
        content: TextField(
          controller: referenceController,
          // Border/fill come from `inputDecorationTheme` (AppTheme) — the
          // one form-field authority.
          decoration: const InputDecoration(
            labelText: 'Referensi Pengiriman',
            hintText: 'Masukkan referensi pengiriman baru',
          ),
          textCapitalization: TextCapitalization.characters,
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('Batal'),
          ),
          ElevatedButton(
            onPressed: () {
              final shippingReference = referenceController.text.trim();
              if (shippingReference.isEmpty) return;
              Navigator.pop(context);
              onShipOrder(
                order.id,
                order.sellerId,
                ShippingProofData(shippingReference: shippingReference),
              );
            },
            child: const Text('Update'),
          ),
        ],
      ),
    );
  }

  void _showCancelDialog(order_domain.Action action) {
    final reasonController = TextEditingController();

    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Batalkan Pesanan'),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const Text('Mohon berikan alasan pembatalan:'),
            const SizedBox(height: 12),
            TextField(
              controller: reasonController,
              // Border/fill come from `inputDecorationTheme` (AppTheme) — the
              // one form-field authority.
              decoration: const InputDecoration(
                hintText: 'Alasan pembatalan...',
              ),
              maxLines: 3,
            ),
          ],
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('Batal'),
          ),
          ElevatedButton(
            onPressed: () {
              final reason = reasonController.text.trim();
              Navigator.pop(context);
              onCancelOrder(
                order.id,
                reason.isEmpty ? 'User cancelled' : reason,
              );
            },
            style: ElevatedButton.styleFrom(
              backgroundColor: Theme.of(context).colorScheme.error,
            ),
            child: const Text('Batalkan Pesanan'),
          ),
        ],
      ),
    );
  }

  Future<void> _showExtendConfirmationDialog() async {
    final confirmed = await AppDialog.confirm(
      context: context,
      title: 'Perpanjang Konfirmasi',
      message: 'Perpanjang masa konfirmasi penerimaan pesanan?',
      confirmLabel: 'Perpanjang',
      cancelLabel: 'Batal',
    );
    if (confirmed) {
      // Call extend confirmation API
      onExtendConfirmation(order.id);
    }
  }
}
