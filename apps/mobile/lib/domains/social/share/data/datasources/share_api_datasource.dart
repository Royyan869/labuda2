// Share API Datasource
// HTTP operations for Share domain - Go Backend API

import 'package:labuda/core/api/api.dart';

/// Share API Datasource
///
/// Menangani HTTP operations ke backend API untuk share functionality.
/// Isolasi dari domain logic - hanya handle request/response.
class ShareApiDatasource {
  final ApiClient _apiClient;

  ShareApiDatasource(this._apiClient);

  // ==========================================================================
  // Share as Post Operations (Canonical Repost Flow)
  // ==========================================================================

  /// Create a share/repost via Content API.
  /// POST /api/v1/contents/{id}/repost
  ///
  /// SHARE CONTRACT V1: Single canonical path for ALL share types.
  /// - Content shares: target_type=content, target_id=content ID
  /// - Non-content shares (listing, auction, profile): target_type + target_id
  /// - Backend routes through CreateInternalShare() for all target types
  Future<Map<String, dynamic>> createRepost({
    required String originalContentId,
    required String authorId,
    String? caption,
    String? originalAuthorId,
    String? originalContentTitle,
    String? originalContentImageURL,
    String? targetType,
    String? targetId,
  }) async {
    final requestData = <String, dynamic>{
      'original_content_id': originalContentId,
      if (originalAuthorId != null && originalAuthorId.isNotEmpty)
        'original_author_id': originalAuthorId,
      'caption': ?caption,
      if (originalContentTitle != null && originalContentTitle.isNotEmpty)
        'original_content_title': originalContentTitle,
      'original_content_image_url': ?originalContentImageURL,
      if (targetType != null) 'target_type': targetType,
      if (targetId != null) 'target_id': targetId,
    };

    final response = await _apiClient.post(
      '/contents/$originalContentId/repost',
      data: requestData,
    );
    return response.data['data'] as Map<String, dynamic>;
  }
}
