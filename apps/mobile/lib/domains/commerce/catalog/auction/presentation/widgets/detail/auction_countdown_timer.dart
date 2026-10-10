/// Auction Countdown Timer
///
/// Shows remaining time for auction
///
/// BOUNDARY NORMALIZATION (PHASE 1D):
/// - Timer displays countdown based on auction.endTime from backend
/// - STATUS is authoritative for business decisions, not time
/// - Time-based display is PRESENTATION ONLY (may differ from backend due to clock drift)
/// - For bid operability, backend decision contract is authoritative
library;

import 'package:flutter/material.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/shared/domain/entities/resource_projection.dart';
import 'package:hishumi/domains/commerce/catalog/auction/domain/entities/auction.dart';
import 'package:hishumi/domains/commerce/catalog/auction/domain/entities/auction_status.dart';

/// Countdown timer widget for auction
///
/// NOTE: This is a PRESENTATION helper only. The actual auction end time
/// and bidding eligibility are determined by the backend via auction.status,
/// not by client-side time calculations.
class AuctionCountdownTimer extends StatelessWidget {
  final Auction auction;
  final String? currentUserId;

  const AuctionCountdownTimer({
    super.key,
    required this.auction,
    this.currentUserId,
  });

  @override
  Widget build(BuildContext context) {
    final now = DateTime.now();
    final endTime = auction.endTime;
    final timeRemaining = endTime.difference(now);

    // BOUNDARY NORMALIZATION: Use status-based check, not time-based
    // Backend auction.status is authoritative, client time is display only
    final hasEnded = auction.status == AuctionStatus.ended;

    // Determine display based on auction state. All tones come from the
    // scheme authority — no light-only hex, no local brightness branch.
    final scheme = Theme.of(context).colorScheme;
    final display = _getDisplay(context, scheme, hasEnded, timeRemaining);

    return Container(
      color: display.backgroundColor,
      padding: const EdgeInsets.symmetric(
        horizontal: AppMetrics.p16,
        vertical: AppMetrics.p12,
      ),
      child: Row(
        children: [
          Icon(
            display.icon,
            size: AppIconSize.action,
            color: display.iconColor,
          ),
          const SizedBox(width: 8),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  display.label,
                  style: context.typeRoles.bodyDense.copyWith(
                    fontWeight: display.fontWeight,
                    color: display.textColor,
                  ),
                ),
                if (display.subtitle.isNotEmpty) ...[
                  const SizedBox(height: 2),
                  Text(
                    display.subtitle,
                    style: context.typeRoles.labelMicro.copyWith(
                      color: display.textColor.withValues(alpha: 0.8),
                    ),
                  ),
                ],
              ],
            ),
          ),
          // Show countdown only for active auctions
          // Time display is PRESENTATION ONLY - backend status determines actual operability
          if (auction.status == AuctionStatus.active && !hasEnded) ...[
            Text(
              _formatDuration(timeRemaining),
              style: context.typeRoles.titleCompact.copyWith(
                fontWeight: FontWeight.bold,
                color: _getTimeColor(context, scheme, timeRemaining),
              ),
            ),
          ],
          // Show time to start for scheduled auctions
          if (auction.status == AuctionStatus.scheduled) ...[
            Text(
              _formatDuration(timeRemaining),
              style: context.typeRoles.titleCompact.copyWith(
                fontWeight: FontWeight.bold,
                color: scheme.secondary,
              ),
            ),
          ],
        ],
      ),
    );
  }

  _TimerDisplay _getDisplay(
    BuildContext context,
    ColorScheme scheme,
    bool hasEnded,
    Duration timeRemaining,
  ) {
    // Active auction
    if (auction.status == AuctionStatus.active && !hasEnded) {
      return _TimerDisplay(
        label: 'Berakhir dalam:',
        subtitle: '',
        icon: Icons.access_time,
        iconColor: scheme.onSurfaceVariant,
        textColor: scheme.onSurface,
        backgroundColor: scheme.surface,
        fontWeight: FontWeight.normal,
      );
    }

    // Scheduled auction
    if (auction.status == AuctionStatus.scheduled) {
      return _TimerDisplay(
        label: 'Lelang Dimulai:',
        subtitle:
            '${auction.startTime.day}/${auction.startTime.month}/${auction.startTime.year} '
            '${auction.startTime.hour.toString().padLeft(2, '0')}:'
            '${auction.startTime.minute.toString().padLeft(2, '0')}',
        icon: Icons.schedule,
        iconColor: scheme.secondary,
        textColor: scheme.onSurface,
        backgroundColor: scheme.secondary.withValues(alpha: 0.12),
        fontWeight: FontWeight.w500,
      );
    }

    // Cancelled auction
    if (auction.status == AuctionStatus.cancelled) {
      return _TimerDisplay(
        label: 'Lelang Dibatalkan',
        subtitle: 'Lelang ini telah dibatalkan oleh penjual',
        icon: Icons.cancel,
        iconColor: scheme.onSurfaceVariant,
        textColor: scheme.onSurface,
        backgroundColor: scheme.surfaceContainerHighest,
        fontWeight: FontWeight.w500,
      );
    }

    // Waiting for Settlement - winner must complete purchase
    if (auction.status == AuctionStatus.waitingSettlement) {
      // Only show if current user is the winner
      if (currentUserId != null && auction.isUserWinner(currentUserId!)) {
        // Canonical deadline derivation: end_at + 24h (backend
        // Auction.SettlementDeadline()). Non-null by definition.
        final deadline = auction.settlementDeadline;

        // TRANSACTION CLARITY: Show countdown to settlement deadline
        final currentTime = DateTime.now();
        final timeRemaining = deadline.difference(currentTime);
        final hoursRemaining = timeRemaining.inHours;

        // Format: "Harus diselesaikan dalam X jam" or "X jam Y menit"
        final timeRemainingText = hoursRemaining > 0
            ? '$hoursRemaining jam'
            : '${timeRemaining.inMinutes.remainder(60)} menit';

        return _TimerDisplay(
          label: '🏆 Anda Menang!',
          subtitle: 'Harus diselesaikan dalam $timeRemainingText',
          icon: Icons.emoji_events,
          iconColor: context.statusColors.warning,
          textColor: scheme.onSurface,
          backgroundColor: context.statusColors.warning.withValues(alpha: 0.12),
          fontWeight: FontWeight.bold,
        );
      }

      // Non-winner view
      return _TimerDisplay(
        label: 'Menunggu Pembayaran',
        subtitle: 'Pemenang sedang menyelesaikan pembayaran',
        icon: Icons.access_time,
        iconColor: context.statusColors.warning,
        textColor: scheme.onSurface,
        backgroundColor: context.statusColors.warning.withValues(alpha: 0.12),
        fontWeight: FontWeight.w500,
      );
    }

    // PURGED: expired_bnr branch — never a backend state. Settlement failure
    // returns the auction to draft (TransitionToDraftOnSettlementFailure).

    // Ended auction - check for winner status
    // BOUNDARY NORMALIZATION: Status-based check only, not time-based
    if (auction.status == AuctionStatus.ended) {
      // User is the winner
      if (currentUserId != null && auction.isUserWinner(currentUserId!)) {
        final winningBid = auction.currentBid;
        return _TimerDisplay(
          label: 'Selamat! Anda Menang! 🎉',
          subtitle:
              'Menang di Rp ${formatGroupedAmount(winningBid.round())} - Lanjut ke pembayaran untuk amankan',
          icon: Icons.emoji_events,
          iconColor: context.statusColors.success,
          textColor: scheme.onSurface,
          backgroundColor: context.statusColors.success.withValues(alpha: 0.12),
          fontWeight: FontWeight.bold,
        );
      }

      // Auction was sold (has winner but not current user)
      if (auction.isSold) {
        return _TimerDisplay(
          label: 'Lelang Berakhir - Terjual',
          subtitle: 'Lelang ini telah terjual kepada pemenang',
          icon: Icons.check_circle,
          iconColor: scheme.onSurfaceVariant,
          textColor: scheme.onSurface,
          backgroundColor: scheme.surfaceContainerHighest,
          fontWeight: FontWeight.w500,
        );
      }

      // Auction expired without winner
      if (auction.isExpired) {
        return _TimerDisplay(
          label: 'Lelang Berakhir - Tidak Ada Pemenang',
          subtitle: 'Lelang ini berakhir tanpa bid yang memenuhi syarat',
          icon: Icons.info,
          iconColor: scheme.onSurfaceVariant,
          textColor: scheme.onSurface,
          backgroundColor: scheme.surfaceContainerHighest,
          fontWeight: FontWeight.w500,
        );
      }

      // Generic ended message
      return _TimerDisplay(
        label: 'Lelang Telah Berakhir',
        subtitle: '',
        icon: Icons.access_time,
        iconColor: scheme.onSurfaceVariant,
        textColor: scheme.onSurface,
        backgroundColor: scheme.surfaceContainerHighest,
        fontWeight: FontWeight.w500,
      );
    }

    // Default: show countdown
    return _TimerDisplay(
      label: 'Berakhir dalam:',
      subtitle: '',
      icon: Icons.access_time,
      iconColor: scheme.onSurfaceVariant,
      textColor: scheme.onSurface,
      backgroundColor: scheme.surface,
      fontWeight: FontWeight.normal,
    );
  }

  String _formatDuration(Duration duration) {
    if (duration.isNegative) return '00:00:00';
    final hours = duration.inHours;
    final minutes = duration.inMinutes.remainder(60);
    final seconds = duration.inSeconds.remainder(60);
    return '${hours.toString().padLeft(2, '0')}:'
        '${minutes.toString().padLeft(2, '0')}:'
        '${seconds.toString().padLeft(2, '0')}';
  }

  /// Countdown tone: status tones only where the remaining time IS the
  /// status (critical < 1h = error, urgent < 6h = warning). Plenty of time
  /// left is not a success event — it reads as the default data tone, so the
  /// countdown stops looking like a "berhasil" badge.
  Color _getTimeColor(
    BuildContext context,
    ColorScheme scheme,
    Duration duration,
  ) {
    if (duration.inHours < 1) return scheme.error;
    if (duration.inHours < 6) return context.statusColors.warning;
    return scheme.onSurface;
  }
}

class _TimerDisplay {
  final String label;
  final String subtitle;
  final IconData icon;
  final Color iconColor;
  final Color textColor;
  final Color backgroundColor;
  final FontWeight fontWeight;

  const _TimerDisplay({
    required this.label,
    this.subtitle = '',
    required this.icon,
    required this.iconColor,
    required this.textColor,
    required this.backgroundColor,
    required this.fontWeight,
  });
}
