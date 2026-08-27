/// Auto-Release Countdown Widget
///
/// Displays the protection window countdown for shipped/delivered orders.
/// This shows buyers how long they have to inspect items and request help,
/// and shows sellers when funds will be available.
///
/// BUSINESS TRUTH: The 5-day protection window starts when order is marked SHIPPED,
/// not when delivered. We don't have courier tracking, so we use shipped date as
/// the canonical start of the protection period.
library;

import 'dart:async';
import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart' as core;
import 'package:labuda/shared/utils/app_formatters.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/entities/order_status.dart';

/// Helper function to apply opacity to a color (replaces deprecated withOpacity)
Color _withOpacity(Color color, double opacity) {
  return color.withValues(alpha: opacity);
}

/// Auto-release countdown widget for shipped/delivered orders
class AutoReleaseCountdownWidget extends StatefulWidget {
  final DateTime? autoReleaseAt;
  final DateTime? shippedAt;
  final bool isBuyer;
  final OrderStatus status;

  const AutoReleaseCountdownWidget({
    super.key,
    required this.autoReleaseAt,
    this.shippedAt,
    required this.isBuyer,
    required this.status,
  });

  @override
  State<AutoReleaseCountdownWidget> createState() =>
      _AutoReleaseCountdownWidgetState();
}

class _AutoReleaseCountdownWidgetState
    extends State<AutoReleaseCountdownWidget> {
  Timer? _timer;
  Duration? _remaining;

  @override
  void initState() {
    super.initState();
    _calculateRemaining();
    // Update every minute
    _timer = Timer.periodic(const Duration(minutes: 1), (_) {
      setState(() {
        _calculateRemaining();
      });
    });
  }

  @override
  void didUpdateWidget(AutoReleaseCountdownWidget oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.autoReleaseAt != widget.autoReleaseAt) {
      _calculateRemaining();
    }
  }

  void _calculateRemaining() {
    if (widget.autoReleaseAt == null) {
      _remaining = null;
      return;
    }
    final now = DateTime.now();
    final releaseTime = widget.autoReleaseAt!;
    _remaining = releaseTime.isAfter(now)
        ? releaseTime.difference(now)
        : Duration.zero;
  }

  @override
  void dispose() {
    _timer?.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    // Only show for shipped and delivered orders with valid autoReleaseAt
    if (widget.status != OrderStatus.shipped &&
        widget.status != OrderStatus.delivered) {
      return const SizedBox.shrink();
    }

    if (_remaining == null || widget.autoReleaseAt == null) {
      return const SizedBox.shrink();
    }

    final isDark = Theme.of(context).brightness == Brightness.dark;
    final hasExpired = _remaining == Duration.zero;

    return Container(
      padding: const EdgeInsets.all(12),
      margin: const EdgeInsets.only(bottom: 12),
      decoration: BoxDecoration(
        gradient: LinearGradient(
          colors: hasExpired
              ? [
                  _withOpacity(core.AppColors.successGreen, 0.15),
                  _withOpacity(core.AppColors.successGreen, 0.05),
                ]
              : [
                  _withOpacity(core.AppColors.primaryBlue, 0.1),
                  _withOpacity(core.AppColors.primaryBlue, 0.05),
                ],
        ),
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: hasExpired
              ? _withOpacity(core.AppColors.successGreen, 0.3)
              : _withOpacity(core.AppColors.primaryBlue, 0.3),
          width: 1,
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Header row
          Row(
            children: [
              Container(
                width: 32,
                height: 32,
                decoration: BoxDecoration(
                  color: hasExpired
                      ? _withOpacity(core.AppColors.successGreen, 0.15)
                      : _withOpacity(core.AppColors.primaryBlue, 0.15),
                  shape: BoxShape.circle,
                ),
                child: Icon(
                  hasExpired ? Icons.check_circle_outline : Icons.schedule,
                  color: hasExpired
                      ? core.AppColors.successGreen
                      : core.AppColors.primaryBlue,
                  size: 18,
                ),
              ),
              const SizedBox(width: 10),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      hasExpired
                          ? 'Waktu Pemeriksaan Barang Berakhir'
                          : 'Waktu Pemeriksaan Barang',
                      style: TextStyle(
                        fontSize: 13,
                        fontWeight: FontWeight.w600,
                        color: isDark ? Colors.white : Colors.black87,
                      ),
                    ),
                    if (!hasExpired)
                      Text(
                        widget.isBuyer
                            ? 'Selama masa ini Anda masih bisa menghubungi penjual atau mengajukan bantuan'
                            : 'Proses penjualan akan selesai setelah masa ini berakhir',
                        style: TextStyle(fontSize: 11, color: Colors.grey[600]),
                      ),
                  ],
                ),
              ),
            ],
          ),

          // Countdown display
          if (!hasExpired) ...[
            const SizedBox(height: 10),
            _CountdownDisplay(remaining: _remaining!),
          ],

          // Release date info
          const SizedBox(height: 8),
          Row(
            children: [
              Icon(Icons.event_outlined, size: 12, color: Colors.grey[600]),
              const SizedBox(width: 4),
              Text(
                hasExpired
                    ? 'Masa pemeriksaan telah berakhir'
                    : 'Berakhir: ${AppFormatters.formatDate(widget.autoReleaseAt!)}',
                style: TextStyle(fontSize: 10, color: Colors.grey[600]),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

/// Countdown display widget - simplified to show only days
class _CountdownDisplay extends StatelessWidget {
  final Duration remaining;

  const _CountdownDisplay({required this.remaining});

  @override
  Widget build(BuildContext context) {
    final days = remaining.inDays;

    // Color coding based on urgency
    Color getColor() {
      if (days <= 0) {
        return core.AppColors.statusError; // Red - urgent
      } else if (days <= 1) {
        return core.AppColors.statusWarning; // Orange - soon
      } else {
        return core.AppColors.primaryBlue; // Blue - normal
      }
    }

    final color = getColor();

    // Simplified display: only show days
    final daysText = days > 0 ? '$days hari lagi' : 'Hari terakhir';

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
      decoration: BoxDecoration(
        color: _withOpacity(color, 0.1),
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: _withOpacity(color, 0.3), width: 1),
      ),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(Icons.schedule, color: color, size: 20),
          const SizedBox(width: 8),
          Text(
            daysText,
            style: TextStyle(
              fontSize: 16,
              fontWeight: FontWeight.bold,
              color: color,
            ),
          ),
        ],
      ),
    );
  }
}
