import 'package:labuda/core/api/base_api_repository.dart';
import 'package:labuda/core/common/result.dart';
import 'package:labuda/domains/social/like/data/dto/like_api_models.dart';

/// API Datasource for Like operations
///
/// Provides HTTP operations for:
/// - Toggle likes on content, collections, auctions, etc.
/// - Query like statistics
/// - Get list of users who liked content
/// - Get user's like history
class LikeApiDatasource extends BaseApiRepository {
  static const String _basePath = '/likes';

  LikeApiDatasource(super.apiClient, {super.logger});

  // ===========================================
  // LIKE OPERATIONS
  // ===========================================

  /// Toggle like on a target
  ///
  /// Endpoint: POST /api/v1/likes/toggle
  /// Returns: {"liked": true/false}
  Future<Result<bool>> toggleLike(LikeToggleApiRequest request) async {
    logger?.info('Toggling like on ${request.targetType}: ${request.targetId}');

    return executeRequest(
      () => apiClient.post('$_basePath/toggle', data: request.toJson()),
      parser: (data) {
        if (data is Map<String, dynamic>) {
          return data['liked'] as bool? ?? false;
        }
        return false;
      },
    );
  }

  /// Get like statistics for a target
  ///
  /// Endpoint: GET /api/v1/likes/stats
  Future<Result<LikeStatsApiResponse>> getLikeStats({
    required String targetId,
    required String targetType,
  }) async {
    logger?.info('Getting like stats for $targetType: $targetId');

    return executeRequest(
      () => apiClient.get(
        '$_basePath/stats',
        queryParameters: {'target_id': targetId, 'target_type': targetType},
      ),
      parser: (data) =>
          LikeStatsApiResponse.fromJson(data as Map<String, dynamic>),
    );
  }
}
