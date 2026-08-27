/// Auction time display extension
///
/// Provides presentation helpers for time-related display logic.
/// This is PRESENTATION logic, not business logic.
/// Business state comes from backend decision contract.
library;

import 'auction.dart';

/// Time remaining data for display
class AuctionTimeRemaining {
  final String displayText;
  final AuctionUrgencyLevel urgencyLevel;

  const AuctionTimeRemaining({
    required this.displayText,
    required this.urgencyLevel,
  });
}

/// Color enum for urgency (pure domain, no Flutter dependency)
enum AuctionUrgencyLevel { critical, warning, normal, ended }

/// Extension on Auction for time display calculations
extension AuctionTimeExtension on Auction {
  /// Calculate time remaining for display purposes
  ///
  /// PRESENTATION ONLY - This helps UI decide what to show,
  /// but NOT what actions are allowed. Use decision.state for business logic.
  AuctionTimeRemaining getTimeRemaining() {
    final now = DateTime.now();
    final remaining = endTime.difference(now);

    if (remaining.isNegative) {
      return const AuctionTimeRemaining(
        displayText: 'Berakhir',
        urgencyLevel: AuctionUrgencyLevel.ended,
      );
    } else if (remaining.inHours < 1) {
      final minutes = remaining.inMinutes;
      return AuctionTimeRemaining(
        displayText: '⏳ $minutes menit lagi',
        urgencyLevel: AuctionUrgencyLevel.critical,
      );
    } else if (remaining.inHours < 24) {
      final hours = remaining.inHours;
      return AuctionTimeRemaining(
        displayText: '⏳ $hours jam lagi',
        urgencyLevel: AuctionUrgencyLevel.warning,
      );
    } else {
      final days = remaining.inDays;
      return AuctionTimeRemaining(
        displayText: '⏳ $days hari lagi',
        urgencyLevel: AuctionUrgencyLevel.normal,
      );
    }
  }

  /// Get urgency level for color mapping
  AuctionUrgencyLevel get urgencyLevel {
    final now = DateTime.now();
    final remaining = endTime.difference(now);

    if (remaining.isNegative) {
      return AuctionUrgencyLevel.ended;
    } else if (remaining.inHours < 1) {
      return AuctionUrgencyLevel.critical;
    } else if (remaining.inHours < 24) {
      return AuctionUrgencyLevel.warning;
    } else {
      return AuctionUrgencyLevel.normal;
    }
  }
}
