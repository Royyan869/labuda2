library;

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
import 'package:labuda/domains/commerce/transaction/order/domain/domain.dart'
    as order_domain;
import 'package:labuda/domains/commerce/transaction/order/order.dart';

/// Order Action Handler - routes backend actions to appropriate handlers
class OrderActionHandler {
  final Order order;
  final BuildContext context;

  // Seller action handlers
  final void Function(String orderId, String sellerId) onAcceptOrder;
  final void Function(String orderId, String sellerId, String reason)
  onRejectOrder;
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
  final void Function(Order order) onChangePaymentMethod;
  final void Function(String orderId, String reason) onCancelOrder;

  // Dispute handler
  final void Function({required String orderId}) onOpenDispute;

  // Support handler
  final VoidCallback onRequestSupport;

  const OrderActionHandler({
    required this.order,
    required this.context,
    required this.onAcceptOrder,
    required this.onRejectOrder,
    required this.onShipOrder,
    required this.onConfirmDelivery,
    required this.onExtendConfirmation,
    required this.onRefundRequestRequest,
    required this.onRate,
    required this.onPayNow,
    required this.onChangePaymentMethod,
    required this.onCancelOrder,
    required this.onOpenDispute,
    required this.onRequestSupport,
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
      case 'accept':
        return _handleAcceptOrder();
      case 'reject':
        return _handleRejectOrder(action);

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

      // Unknown action - show info
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

  void _handleAcceptOrder() {
    // Show confirmation dialog before accepting order
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Terima Pesanan'),
        content: const Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Icon(Icons.check_circle_outline, color: Colors.green, size: 48),
            SizedBox(height: 16),
            Text(
              'Anda yakin ingin menerima pesanan ini?',
              style: TextStyle(fontSize: 16),
            ),
            SizedBox(height: 8),
            Text(
              'Dengan menerima pesanan, Anda berkewajiban untuk memproses dan mengirim produk sesuai dengan pesanan.',
              style: TextStyle(fontSize: 13, color: Colors.grey),
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
              Navigator.pop(context);
              onAcceptOrder(order.id, order.sellerId);
            },
            style: ElevatedButton.styleFrom(
              backgroundColor: Colors.green,
              foregroundColor: Colors.white,
            ),
            child: const Text('Ya, Terima Pesanan'),
          ),
        ],
      ),
    );
  }

  void _handleRejectOrder(order_domain.Action action) {
    // Check if action has input schema for reason
    _showRejectDialog(action);
  }

  void _handleCompleteOrder() {
    // B4A: Single-click final acceptance — releases funds to seller.
    // Must show clear confirmation dialog (financial action).
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Terima Barang'),
        content: const Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Icon(Icons.warning_amber_outlined, color: Colors.orange, size: 48),
            SizedBox(height: 16),
            Text(
              'Anda yakin barang sudah diterima dengan baik?',
              style: TextStyle(fontSize: 16),
            ),
            SizedBox(height: 8),
            Text(
              'Dengan menerima barang, pesanan akan selesai dan pembayaran akan diteruskan ke penjual. Tindakan ini tidak dapat dibatalkan.',
              style: TextStyle(fontSize: 13, color: Colors.grey),
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
              Navigator.pop(context);
              onConfirmDelivery(order.id, order.buyerId);
            },
            style: ElevatedButton.styleFrom(
              backgroundColor: Colors.orange,
              foregroundColor: Colors.white,
            ),
            child: const Text('Ya, Terima Barang'),
          ),
        ],
      ),
    );
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

  void _handleExtendConfirmation() {
    // Show confirmation extension dialog
    _showExtendConfirmationDialog();
  }

  void _showBlockedMessage(order_domain.Action action) {
    final blocked = action.blocked;
    if (blocked == null) return;

    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Action Not Available'),
        content: Text(blocked.reason ?? blocked.messageKey),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('OK'),
          ),
        ],
      ),
    );
  }

  void _showUnknownActionInfo(order_domain.Action action) {
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Action Info'),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('Type: ${action.type}'),
            const SizedBox(height: 8),
            Text('Label: ${action.labelKey}'),
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
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('Close'),
          ),
        ],
      ),
    );
  }

  void _showShippingDialog(order_domain.Action action) {
    final referenceController = TextEditingController();
    final noteController = TextEditingController();
    String? referenceType = 'tracking'; // Default to tracking

    showDialog(
      context: context,
      builder: (context) => StatefulBuilder(
        builder: (context, setDialogState) => AlertDialog(
          title: const Text('Konfirmasi Pengiriman'),
          content: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                // Read-only shipping method from checkout
                const Text(
                  'Metode Pengiriman',
                  style: TextStyle(fontWeight: FontWeight.bold, fontSize: 12),
                ),
                const SizedBox(height: 4),
                Container(
                  padding: const EdgeInsets.all(12),
                  decoration: BoxDecoration(
                    color: Colors.grey.shade100,
                    borderRadius: BorderRadius.circular(8),
                    border: Border.all(color: Colors.grey.shade300),
                  ),
                  child: Row(
                    children: [
                      const Icon(Icons.local_shipping, size: 16),
                      const SizedBox(width: 8),
                      Expanded(
                        child: Text(
                          _formatShippingMethod(),
                          style: const TextStyle(fontSize: 13),
                        ),
                      ),
                    ],
                  ),
                ),
                const SizedBox(height: 16),

                // Reference type selector
                const Text(
                  'Jenis Referensi',
                  style: TextStyle(fontWeight: FontWeight.bold, fontSize: 12),
                ),
                const SizedBox(height: 8),
                SegmentedButton<String>(
                  segments: const [
                    ButtonSegment(
                      value: 'tracking',
                      label: Text('Resi Kurir'),
                      icon: Icon(Icons.qr_code, size: 16),
                    ),
                    ButtonSegment(
                      value: 'phone',
                      label: Text('No. HP/WA'),
                      icon: Icon(Icons.phone, size: 16),
                    ),
                    ButtonSegment(
                      value: 'other',
                      label: Text('Lainnya'),
                      icon: Icon(Icons.more_horiz, size: 16),
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
                  style: const TextStyle(
                    fontWeight: FontWeight.bold,
                    fontSize: 12,
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
                    border: const OutlineInputBorder(),
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
                  decoration: const InputDecoration(
                    labelText: 'Catatan (opsional)',
                    hintText: 'Catatan pengiriman untuk pembeli...',
                    border: OutlineInputBorder(),
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
              style: ElevatedButton.styleFrom(
                backgroundColor: Theme.of(context).colorScheme.primary,
              ),
              child: const Text('Konfirmasi Pengiriman'),
            ),
          ],
        ),
      ),
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
          decoration: const InputDecoration(
            labelText: 'Referensi Pengiriman',
            hintText: 'Masukkan referensi pengiriman baru',
            border: OutlineInputBorder(),
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

  void _showRejectDialog(order_domain.Action action) {
    final reasonController = TextEditingController();

    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Tolak Pesanan'),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const Text('Mohon berikan alasan penolakan:'),
            const SizedBox(height: 12),
            TextField(
              controller: reasonController,
              decoration: const InputDecoration(
                hintText: 'Contoh: Stok habis, lokasi jauh, dll.',
                border: OutlineInputBorder(),
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
              onRejectOrder(
                order.id,
                order.sellerId,
                reason.isEmpty ? 'Ditolak oleh penjual' : reason,
              );
            },
            style: ElevatedButton.styleFrom(
              backgroundColor: Theme.of(context).colorScheme.error,
            ),
            child: const Text('Tolak Pesanan'),
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
              decoration: const InputDecoration(
                hintText: 'Alasan pembatalan...',
                border: OutlineInputBorder(),
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

  void _showExtendConfirmationDialog() {
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Perpanjang Konfirmasi'),
        content: const Text('Perpanjang masa konfirmasi penerimaan pesanan?'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('Batal'),
          ),
          ElevatedButton(
            onPressed: () {
              Navigator.pop(context);
              // Call extend confirmation API
              onExtendConfirmation(order.id);
            },
            child: const Text('Perpanjang'),
          ),
        ],
      ),
    );
  }
}
