// Share API Datasource
// HTTP operations for Share domain - Go Backend API
//
// HONESTY CLEANUP v1:
// - Share tracking endpoints (/social/shares, etc.) removed - backend does not implement them
// - Only canonical repost flow is preserved (createRepost)
// - Native share operations (WhatsApp, Instagram, etc.) are handled by NativeShareService

import 'package:labuda/core/api/api.dart';
import '../../domain/entities/share_target.dart';

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

  /// Create a repost via Content API
  /// POST /api/v1/contents/{id}/repost
  ///
  /// SHARE CONTRACT V1: Creates NEW Content with ShareReference
  /// - Original author attribution is preserved via originalAuthorId
  /// - ShareReference contains canonical reference to original content
  /// - Source object is NOT mutated
  Future<Map<String, dynamic>> createRepost({
    required String originalContentId,
    required String authorId,
    String? caption,
    required String originalAuthorId,
    required String originalContentTitle,
    String? originalContentImageURL,
  }) async {
    final requestData = {
      'original_content_id': originalContentId,
      'original_author_id': originalAuthorId,
      if (caption != null) 'caption': caption,
      'original_content_title': originalContentTitle,
      if (originalContentImageURL != null)
        'original_content_image_url': originalContentImageURL,
    };

    final response = await _apiClient.post(
      '/contents/$originalContentId/repost',
      data: requestData,
    );
    return response.data['data'] as Map<String, dynamic>;
  }

  /// Create a share-reference post via Content API.
  /// POST /api/v1/contents
  ///
  /// Uses the canonical CreateContent contract with share_reference for
  /// non-content share targets.
  @Deprecated('Use createRepost for content shares instead')
  Future<Map<String, dynamic>> createShareReferencePost({
    required String authorId,
    String? authorUsername,
    String? authorAvatarUrl,
    required String content,
    required ShareTarget target,
    List<String> mediaUrls = const [],
  }) async {
    final requestData = <String, dynamic>{
      'caption': content,
      'type': 'post',
      'visibility': 'public',
      'allow_comments': true,
      'share_reference': _buildShareReferencePayload(target),
      if (mediaUrls.isNotEmpty) 'media': mediaUrls,
    };

    final response = await _apiClient.post('/contents', data: requestData);
    return response.data['data'] as Map<String, dynamic>;
  }

  Map<String, dynamic> _buildShareReferencePayload(ShareTarget target) {
    return {
      'targetType': target.type.name,
      'targetId': target.id,
      'preview': {
        'title': target.title,
        if (target.imageUrl != null) 'imageUrl': target.imageUrl,
        'isAvailable': _metadataBool(target, 'isAvailable', true),
        'isSold': _metadataBool(target, 'isSold', false),
        'isClosed': _metadataBool(target, 'isClosed', false),
        'isDeleted': _metadataBool(target, 'isDeleted', false),
      },
    };
  }

  bool _metadataBool(ShareTarget target, String key, bool fallback) {
    final value = target.metadata[key];
    if (value is bool) return value;
    return fallback;
  }
}
