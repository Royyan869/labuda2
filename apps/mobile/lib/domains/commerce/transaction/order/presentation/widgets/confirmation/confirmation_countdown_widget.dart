import 'dart:async';
import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart' as core;
import 'package:labuda/domains/commerce/transaction/order/domain/domain.dart';

/// Confirmation Countdown Widget (Dumb Widget)
///
/// Real-time countdown display for order protection window.
/// Updates every second, color-coded based on time remaining.
///
/// Color scheme:
/// - Blue: > 1 day remaining
/// - Orange: < 1 day remaining (urgent)
/// - Gray: Expired
///
/// This is a "dumb" widget - only renders the provided confirmation data.
/// No business logic, no state management.
class ConfirmationCountdownWidget extends StatefulWidget {
  final OrderConfirmation confirmation;

  const ConfirmationCountdownWidget({super.key, required this.confirmation});

  @override
  State<ConfirmationCountdownWidget> createState() =>
      _ConfirmationCountdownWidgetState();
}

class _ConfirmationCountdownWidgetState
    extends State<ConfirmationCountdownWidget> {
  Timer? _timer;
  Duration? _remainingDuration;

  @override
  void initState() {
    super.initState();
    _updateDuration();
    _startTimer();
  }

  @override
  void dispose() {
    _timer?.cancel();
    super.dispose();
  }

  void _startTimer() {
    _timer = Timer.periodic(const Duration(seconds: 1), (_) {
      if (mounted) {
        _updateDuration();
      }
    });
  }

  void _updateDuration() {
    final now = DateTime.now();
    final endDate = widget.confirmation.activeEndDate;

    setState(() {
      if (now.isBefore(endDate)) {
        _remainingDuration = endDate.difference(now);
      } else {
        _remainingDuration = Duration.zero;
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final isExpired = widget.confirmation.isExpired;
    final isUrgent =
        _remainingDuration != null && _remainingDuration!.inHours < 24;

    // Determine color
    final Color backgroundColor;
    final Color borderColor;
    final Color textColor;
    final IconData icon;

    if (isExpired) {
      backgroundColor = core.AppColors.neutralGray400.withValues(alpha: 0.1);
      borderColor = core.AppColors.neutralGray400.withValues(alpha: 0.3);
      textColor = core.AppColors.neutralGray400;
      icon = Icons.info_outline;
    } else if (isUrgent) {
      backgroundColor = core.AppColors.koiOrange.withValues(alpha: 0.1);
      borderColor = core.AppColors.koiOrange.withValues(alpha: 0.3);
      textColor = core.AppColors.koiOrange;
      icon = Icons.warning_amber_rounded;
    } else {
      backgroundColor = core.AppColors.primaryBlue.withValues(alpha: 0.1);
      borderColor = core.AppColors.primaryBlue.withValues(alpha: 0.3);
      textColor = core.AppColors.primaryBlue;
      icon = Icons.schedule;
    }

    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: backgroundColor,
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: borderColor),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(icon, color: textColor, size: 18),
              const SizedBox(width: 8),
              Text(
                isExpired
                    ? 'Waktu Pemeriksaan Barang Berakhir'
                    : widget.confirmation.extensionUsed
                    ? 'Perpanjangan'
                    : 'Menunggu Konfirmasi',
                style: TextStyle(
                  fontSize: 13,
                  fontWeight: FontWeight.w600,
                  color: isDark
                      ? core.AppColors.neutralGray200
                      : core.AppColors.neutralGray800,
                ),
              ),
            ],
          ),
          const SizedBox(height: 8),
          if (isExpired)
            _buildExpiredInfo(isDark)
          else
            _buildCountdownInfo(textColor, isDark),
        ],
      ),
    );
  }

  Widget _buildExpiredInfo(bool isDark) {
    return Text(
      'Proses penjualan telah selesai',
      style: TextStyle(
        fontSize: 12,
        color: isDark
            ? core.AppColors.neutralGray400
            : core.AppColors.neutralGray600,
      ),
    );
  }

  Widget _buildCountdownInfo(Color textColor, bool isDark) {
    final timeText = _formatDuration(_remainingDuration);
    final isUrgent = _remainingDuration!.inHours < 24;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Text(
              'Berakhir dalam:',
              style: TextStyle(
                fontSize: 11,
                color: isDark
                    ? core.AppColors.neutralGray400
                    : core.AppColors.neutralGray600,
              ),
            ),
            const SizedBox(width: 6),
            Text(
              timeText,
              style: TextStyle(
                fontSize: 14,
                fontWeight: FontWeight.w700,
                color: textColor,
              ),
            ),
          ],
        ),
        if (isUrgent) ...[
          const SizedBox(height: 4),
          Text(
            'Konfirmasi penerimaan atau perpanjang waktu',
            style: TextStyle(
              fontSize: 10,
              fontStyle: FontStyle.italic,
              color: textColor,
            ),
          ),
        ],
      ],
    );
  }

  String _formatDuration(Duration? duration) {
    if (duration == null) return '0 menit';

    final days = duration.inDays;
    final hours = duration.inHours % 24;
    final minutes = duration.inMinutes % 60;

    if (days > 0) {
      return '$days hari $hours jam';
    } else if (hours > 0) {
      return '$hours jam $minutes menit';
    } else {
      return '$minutes menit';
    }
  }
}
