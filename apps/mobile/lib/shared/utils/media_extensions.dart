/// Extension methods for MediaEntity list
/// Provides convenient accessors for working with media entities
library;

import 'package:labuda/domains/social/content/domain/entities/content.dart';

/// Extension on `List<MediaEntity>` for convenient URL extraction
extension MediaEntityListExtensions on List<MediaEntity> {
  /// Extract original URLs from all media entities
  List<String> get urls => map((e) => e.originalUrl).toList();

  /// Get the first URL from the media list
  /// Returns empty string if list is empty
  String get firstUrl => isNotEmpty ? first.originalUrl : '';

  /// Check if media list has any items
  bool get isNotEmptyUrls => isNotEmpty;

  /// Check if media list is empty
  bool get isEmptyUrls => isEmpty;

  /// Get thumbnail URLs (falls back to original URL if thumbnail not available)
  List<String> get thumbnailUrls =>
      map((e) => e.thumbnailUrl ?? e.originalUrl).toList();

  /// Get first thumbnail URL (falls back to original URL)
  String get firstThumbnailUrl =>
      isNotEmpty ? (first.thumbnailUrl ?? first.originalUrl) : '';
}
