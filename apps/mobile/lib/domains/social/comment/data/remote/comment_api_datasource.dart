import 'package:dio/dio.dart';
import 'package:labuda/core/api/api.dart';
import 'package:labuda/core/common/result.dart';
import 'package:labuda/domains/social/comment/data/dto/comment_dto.dart';
import 'package:uuid/uuid.dart';

/// Remote Data Source for Comment API operations
///
/// Handles HTTP calls to Go backend API.
/// All Firebase/HTTP calls isolated to this layer.
class CommentApiDatasource extends BaseApiRepository {
  static const String _contentBasePath = '/contents';

  CommentApiDatasource(super.apiClient, {super.logger});

  /// Create a commerce reference comment on a content
  /// POST /contents/{contentId}/comments/reference
  ///
  /// Requires Idempotency-Key header for safe retries.
  Future<Result<CommentDto>> createCommerceReferenceComment({
    required String contentId,
    required CreateCommerceReferenceCommentDto request,
    String? idempotencyKey,
  }) async {
    // Build headers with idempotency key if provided
    final headers = idempotencyKey != null
        ? {'Idempotency-Key': idempotencyKey}
        : null;

    return executeRequest(
      () => apiClient.post(
        '$_contentBasePath/$contentId/comments/reference',
        data: request.toJson(),
        options: headers != null ? Options(headers: headers) : null,
      ),
      parser: (data) => CommentDto.fromJson(data as Map<String, dynamic>),
    );
  }

/// Create a new normal comment
  /// POST /contents/{contentId}/comments
  ///
  /// [idempotencyKey] is supplied by the repository and is stable across any
  /// transport retry of the same logical creation attempt (C-IPC). When
  /// absent, a fresh key is generated (callers that retry must always pass
  /// the same key).
  Future<Result<CommentDto>> createComment(
    CreateCommentDto request, {
    String? idempotencyKey,
  }) async {
    final resolvedKey =
        idempotencyKey ?? const Uuid().v4();
    return executeRequest(
      () => apiClient.post(
        '$_contentBasePath/${request.targetId}/comments',
        data: request.toJson(),
        options: Options(headers: {'Idempotency-Key': resolvedKey}),
      ),
      parser: (data) => CommentDto.fromJson(data as Map<String, dynamic>),
    );
  }

  /// List comments for a content with cursor-based pagination
  /// GET /contents/{contentId}/comments
  Future<Result<ListCommentsDto>> listContentComments({
    required String contentId,
    int limit = 20,
    String? cursor,
  }) async {
    final queryParams = <String, dynamic>{'limit': limit};
    if (cursor != null) {
      queryParams['cursor'] = cursor;
    }

    return executeRequest(
      () => apiClient.get(
        '$_contentBasePath/$contentId/comments',
        queryParameters: queryParams,
      ),
      parser: (data) => ListCommentsDto.fromJson(data as Map<String, dynamic>),
    );
  }

  /// Delete a comment
  /// DELETE /comments/{commentId}
  Future<Result<void>> deleteComment(String commentId) async {
    return executeRequest(
      () => apiClient.delete('/comments/$commentId'),
      parser: (_) {},
    );
  }
}
