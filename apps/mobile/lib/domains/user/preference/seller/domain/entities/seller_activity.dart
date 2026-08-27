/// Seller Activity Entities
///
/// Pure Dart entities for seller activity - no Firebase/Flutter dependencies.
library;

import 'package:equatable/equatable.dart';

/// Activity Type Enum
enum ActivityType {
  newOrder,
  orderPaid,
  orderShipped,
  orderCompleted,
  orderCancelled,
  orderRefunded,
  orderDisputed,
  newBid,
  auctionEnded,
  collectionSold,
  newReview,
}

/// Recent Activity Item
class RecentActivityItem extends Equatable {
  final String id;
  final ActivityType type;
  final String title;
  final String subtitle;
  final DateTime timestamp;
  final String? imageUrl;
  final double? amount;
  final String? targetId;

  const RecentActivityItem({
    required this.id,
    required this.type,
    required this.title,
    required this.subtitle,
    required this.timestamp,
    this.imageUrl,
    this.amount,
    this.targetId,
  });

  /// Time ago display
  String get timeAgo {
    final now = DateTime.now();
    final diff = now.difference(timestamp);

    if (diff.inMinutes < 1) return 'Just now';
    if (diff.inMinutes < 60) return '${diff.inMinutes} minutes ago';
    if (diff.inHours < 24) return '${diff.inHours} hours ago';
    if (diff.inDays < 7) return '${diff.inDays} days ago';
    return '${timestamp.day}/${timestamp.month}/${timestamp.year}';
  }

  @override
  List<Object?> get props => [
    id,
    type,
    title,
    subtitle,
    timestamp,
    imageUrl,
    amount,
    targetId,
  ];
}

/// Parameters for activity history with filters
class ActivityHistoryParams {
  final String sellerId;
  final ActivityType? filterType;

  const ActivityHistoryParams({required this.sellerId, this.filterType});

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is ActivityHistoryParams &&
          runtimeType == other.runtimeType &&
          sellerId == other.sellerId &&
          filterType == other.filterType;

  @override
  int get hashCode => sellerId.hashCode ^ filterType.hashCode;
}
