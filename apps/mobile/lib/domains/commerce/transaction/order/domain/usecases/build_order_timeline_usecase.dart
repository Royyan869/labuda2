import 'package:flutter/material.dart';
import 'package:labuda/domains/commerce/transaction/order/order.dart';

/// Timeline Step Entity
///
/// Represents a single step in the order timeline
class TimelineStep {
  final String label;
  final String? sublabel;
  final IconData icon;
  final bool isActive;
  final bool isCompleted;
  final DateTime? timestamp;

  const TimelineStep({
    required this.label,
    this.sublabel,
    required this.icon,
    required this.isActive,
    required this.isCompleted,
    this.timestamp,
  });

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is TimelineStep &&
          runtimeType == other.runtimeType &&
          label == other.label &&
          sublabel == other.sublabel &&
          isActive == other.isActive &&
          isCompleted == other.isCompleted;

  @override
  int get hashCode =>
      label.hashCode ^
      sublabel.hashCode ^
      isActive.hashCode ^
      isCompleted.hashCode;
}

/// Build Order Timeline Use Case
///
/// **DOMAIN:** Commerce → Transaction → Order
/// **RESPONSIBILITY:** Build order timeline steps based on order status
/// **BOUNDARY:** Encapsulates timeline progression business logic
class BuildOrderTimelineUseCase {
  /// Execute the use case
  ///
  /// B4A: Timeline progression is now:
  /// - pending -> paid -> shipped -> completed (selesai)
  /// - No separate "Barang Diterima" step — acceptance is implicit in completion.
  /// - cancelled/refunded/expired are terminal states
  List<TimelineStep> call(Order order) {
    final steps = <TimelineStep>[];

    final currentStatus = order.status;

    // Determine which steps are active/completed
    final isPending = currentStatus == OrderStatus.pending;
    final isPaid = currentStatus == OrderStatus.paid;
    final isShipped = currentStatus == OrderStatus.shipped;
    final isDelivered = currentStatus == OrderStatus.delivered;
    final isCompleted = currentStatus == OrderStatus.completed;
    final isCancelled = currentStatus == OrderStatus.cancelled;
    final isCancelledTimeout = currentStatus == OrderStatus.cancelledTimeout;
    final isRefunded = currentStatus == OrderStatus.refunded;
    final isExpired = currentStatus == OrderStatus.expired;

    // Special handling for terminal states (cancelled/cancelledTimeout/refunded/expired)
    if (isCancelled || isCancelledTimeout || isRefunded || isExpired) {
      steps.add(
        TimelineStep(
          label: _getStatusLabel(currentStatus),
          sublabel: _getStatusSublabel(currentStatus),
          icon: (isCancelled || isCancelledTimeout)
              ? Icons.cancel
              : (isRefunded ? Icons.currency_exchange : Icons.timer_off),
          isActive: true,
          isCompleted: false,
          timestamp: order.cancelledAt,
        ),
      );
      return steps;
    }

    // Normal progression steps
    steps.add(
      TimelineStep(
        label: 'Pesanan Dibuat',
        sublabel: 'Menunggu konfirmasi penjual',
        icon: Icons.shopping_cart_outlined,
        isActive: isPending,
        isCompleted: !isPending,
        timestamp: order.createdAt,
      ),
    );

    if (isPaid || isShipped || isDelivered || isCompleted) {
      steps.add(
        TimelineStep(
          label: 'Pembayaran Berhasil',
          sublabel: order.confirmedAt != null
              ? _formatDateTime(order.confirmedAt!)
              : null,
          icon: Icons.check_circle_outline,
          isActive: false,
          isCompleted: true,
          timestamp: order.confirmedAt,
        ),
      );
    }

    // B4A: Shipped step — active while awaiting buyer decision,
    // completed once buyer accepts (→ completed) or order reaches delivered/completed.
    if (isShipped || isDelivered || isCompleted) {
      steps.add(
        TimelineStep(
          label: 'Dalam Pengiriman',
          sublabel: order.shippedAt != null
              ? _formatDateTime(order.shippedAt!)
              : null,
          icon: Icons.local_shipping_outlined,
          isActive: isShipped,
          isCompleted: isDelivered || isCompleted,
          timestamp: order.shippedAt,
        ),
      );
    }

    // B4A: "Selesai" = buyer accepted + escrow released. No separate delivered step.
    if (isDelivered || isCompleted) {
      steps.add(
        TimelineStep(
          label: 'Selesai',
          sublabel: order.completedAt != null
              ? _formatDateTime(order.completedAt!)
              : (order.deliveredAt != null
                    ? _formatDateTime(order.deliveredAt!)
                    : null),
          icon: Icons.done_all,
          isActive: false,
          isCompleted: true,
          timestamp: order.completedAt ?? order.deliveredAt,
        ),
      );
    }

    return steps;
  }

  String _getStatusLabel(OrderStatus status) {
    switch (status) {
      case OrderStatus.pending:
        return 'Menunggu Pembayaran';
      case OrderStatus.paid:
        return 'Pembayaran Berhasil';
      case OrderStatus.shipped:
        return 'Dalam Pengiriman';
      case OrderStatus.delivered:
      case OrderStatus.completed:
        return 'Selesai';
      case OrderStatus.cancelled:
        return 'Dibatalkan';
      case OrderStatus.cancelledTimeout:
        return 'Dibatalkan (Timeout)';
      case OrderStatus.refunded:
        return 'Direfund';
      case OrderStatus.partiallyRefunded:
        return 'Sebagian Direfund';
      case OrderStatus.expired:
        return 'Kedaluwarsa';
      case OrderStatus.disputeOpen:
        return 'Dispute Dibuka';
    }
  }

  String? _getStatusSublabel(OrderStatus status) {
    switch (status) {
      case OrderStatus.cancelled:
        return 'Pesanan dibatalkan';
      case OrderStatus.cancelledTimeout:
        return 'Penjual tidak mengirim dalam batas waktu';
      case OrderStatus.refunded:
        return 'Pengembalian dana diproses';
      case OrderStatus.partiallyRefunded:
        return 'Sebagian dana dikembalikan';
      case OrderStatus.expired:
        return 'Waktu pembayaran habis';
      case OrderStatus.disputeOpen:
        return 'Dispute sedang diproses';
      default:
        return null;
    }
  }

  String _formatDateTime(DateTime dateTime) {
    // Simple date formatting - can be enhanced with proper formatters
    return '${dateTime.day}/${dateTime.month}/${dateTime.year}';
  }
}
