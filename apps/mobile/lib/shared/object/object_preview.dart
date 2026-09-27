/// Object Preview — transport snapshot
///
/// The display data a share reference carries on the wire at the moment it is
/// written (title, image, coarse availability flags). It is a SNAPSHOT: it can
/// be stale, so it is never business truth and never drives business decisions
/// (no resolver consults it; the client holds no live-fetch path for it).
/// The canonical, viewer-aware representation of a resource is the server's
/// `ResourceProjection` envelope.
library;

import 'package:equatable/equatable.dart';

/// Transport snapshot of a shared object's display identity
class ObjectPreview extends Equatable {
  /// Unique identifier
  final String id;

  /// Type of object (forSale, auction, content, profile)
  final String type;

  /// Display title
  final String title;

  /// Image URL (if available)
  final String? imageUrl;

  /// Snapshot-era price field. Kept for the transport shape only — it carries
  /// no money authority; money comes from the canonical projection envelope.
  final int? price;

  /// Snapshot-era status string (coarse; not a commerce vocabulary)
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
