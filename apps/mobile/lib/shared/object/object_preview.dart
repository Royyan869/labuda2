/// Object Preview
///
/// Live preview data for a domain object.
/// Contains the most up-to-date information fetched from the backend.
library;

import 'package:equatable/equatable.dart';

/// Live preview data for a domain object
class ObjectPreview extends Equatable {
  /// Unique identifier
  final String id;

  /// Type of object (forSale, auction, content, profile)
  final String type;

  /// Display title
  final String title;

  /// Image URL (if available)
  final String? imageUrl;

  /// Price (for forSales and auctions)
  final int? price;

  /// Current status
  final String status;

  /// Status flags (unified from SharePreview)
  final bool isAvailable;
  final bool isSold;
  final bool isClosed;
  final bool isDeleted;

  const ObjectPreview({
    required this.id,
    required this.type,
    required this.title,
    this.imageUrl,
    this.price,
    required this.status,
    this.isAvailable = true,
    this.isSold = false,
    this.isClosed = false,
    this.isDeleted = false,
  });

  /// Create from a forSale entity
  factory ObjectPreview.fromForSale(Map<String, dynamic> forSale) {
    final status = forSale['status'] as String? ?? 'unknown';
    return ObjectPreview(
      id: forSale['id'] as String,
      type: 'forSale',
      title: forSale['title'] as String? ?? '',
      imageUrl:
          forSale['media'] is List && (forSale['media'] as List).isNotEmpty
          ? (forSale['media'] as List).first['originalUrl'] as String?
          : null,
      price: forSale['price'] as int?,
      status: status,
      isAvailable: status == 'available',
      isSold: status == 'sold',
      isClosed: false,
      isDeleted: status == 'deleted',
    );
  }

  /// Create from an auction entity
  factory ObjectPreview.fromAuction(Map<String, dynamic> auction) {
    final status = auction['status'] as String? ?? 'unknown';
    return ObjectPreview(
      id: auction['id'] as String,
      type: 'auction',
      title: auction['title'] as String? ?? '',
      imageUrl:
          auction['media'] is List && (auction['media'] as List).isNotEmpty
          ? (auction['media'] as List).first['originalUrl'] as String?
          : null,
      price: auction['currentBid'] as int?,
      status: status,
      isAvailable: status == 'active' || status == 'scheduled',
      isSold: false,
      isClosed:
          status == 'ended' ||
          status == 'cancelled' ||
          status == 'waiting_settlement',
      isDeleted: status == 'deleted',
    );
  }

  @override
  List<Object?> get props => [
    id,
    type,
    title,
    imageUrl,
    price,
    status,
    isAvailable,
    isSold,
    isClosed,
    isDeleted,
  ];

  @override
  String toString() =>
      'ObjectPreview(id: $id, type: $type, title: $title, status: $status)';
}
