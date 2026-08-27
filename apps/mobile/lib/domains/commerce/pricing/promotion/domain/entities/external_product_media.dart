import 'package:equatable/equatable.dart';

/// Media asset attached to an external product.
class ExternalProductMedia extends Equatable {
  final String id;
  final String externalProductId;
  final String mediaType; // 'image' or 'video'
  final String storageKey;
  final String url;
  final String? thumbnailUrl;
  final int sortOrder;
  final DateTime createdAt;

  const ExternalProductMedia({
    required this.id,
    required this.externalProductId,
    required this.mediaType,
    required this.storageKey,
    required this.url,
    this.thumbnailUrl,
    required this.sortOrder,
    required this.createdAt,
  });

  @override
  List<Object?> get props => [
    id,
    externalProductId,
    mediaType,
    storageKey,
    url,
    thumbnailUrl,
    sortOrder,
    createdAt,
  ];
}
