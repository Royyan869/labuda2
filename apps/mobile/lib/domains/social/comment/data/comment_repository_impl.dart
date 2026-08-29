import 'package:labuda/core/common/result.dart';
import 'package:labuda/core/src/interfaces/services/i_logger_service.dart';
import 'package:labuda/domains/social/comment/data/dto/comment_dto.dart';
import 'package:labuda/domains/social/comment/data/mappers/comment_mapper.dart';
import 'package:labuda/domains/social/comment/data/remote/comment_api_datasource.dart';
import 'package:labuda/domains/social/comment/domain/entities/comment.dart';
import 'package:labuda/domains/social/comment/domain/repositories/comment_repository.dart';
import 'package:uuid/uuid.dart';

/// API-based implementation of CommentRepository
///
/// All external API/Firebase calls isolated to this layer.
/// Consumes the canonical cursor endpoint GET /contents/:id/comments.
class CommentRepositoryImpl implements CommentRepository {
  final CommentApiDatasource _datasource;
  final ILoggerService? _logger;

  CommentRepositoryImpl(this._datasource, {ILoggerService? logger})
    : _logger = logger;

  @override
  Future<Result<CommentPage>> getComments({
    required String targetId,
    required CommentTargetType targetType,
    String? cursor,
    int limit = 20,
  }) async {
    _logger?.info(
      'Getting comments for $targetType: $targetId (cursor: ${cursor ?? 'null'})',
    );

    // C-CURSOR — canonical GET /contents/:id/comments (created_at ASC).
    final result = await _datasource.listContentComments(
      contentId: targetId,
      limit: limit,
      cursor: cursor,
    );

    return result.fold(
      (error) => Result.error(error),
      (response) => Result.success(
        CommentPage(
          comments: CommentMapper.toEntityList(response.comments),
          nextCursor: response.nextCursor,
        ),
      ),
    );
  }

  @override
  Future<Result<Comment>> createComment({
    required String targetId,
    required CommentTargetType targetType,
    required String content,
    String? parentId,
    List<String> mentionedUserIds = const [],
  }) async {
    _logger?.info('Creating comment on $targetType: $targetId');

    // C-IPC — one key per logical creation attempt, reused verbatim for any
    // transport retry of that attempt (never regenerated mid-call).
    final idempotencyKey = const Uuid().v4();

    final request = CreateCommentDto(
      targetId: targetId,
      targetType: targetType.name,
      content: content,
      parentId: parentId,
      mentionedUserIds: mentionedUserIds.isEmpty ? null : mentionedUserIds,
    );

    final result = await _datasource.createComment(
      request,
      idempotencyKey: idempotencyKey,
    );

    return result.fold(
      (error) => Result.error(error),
      (dto) => Result.success(CommentMapper.toEntity(dto)),
    );
  }

  @override
  Future<Result<Comment>> createCommerceReferenceComment({
    required String contentId,
    required String resourceType,
    required String resourceId,
    String? body,
  }) async {
    _logger?.info(
      'Creating commerce reference comment on content: $contentId, resourceType: $resourceType, resourceId: $resourceId',
    );

    // Generate idempotency key for safe retries
    final idempotencyKey = const Uuid().v4();

    final request = CreateCommerceReferenceCommentDto(
      resourceReference: ResourceReferenceRequest(
        resourceType: resourceType,
        resourceId: resourceId,
      ),
      body: body,
    );

    final result = await _datasource.createCommerceReferenceComment(
      contentId: contentId,
      request: request,
      idempotencyKey: idempotencyKey,
    );

    return result.fold(
      (error) => Result.error(error),
      (dto) => Result.success(CommentMapper.toEntity(dto)),
    );
  }

  @override
  Future<Result<bool>> deleteComment(String commentId) async {
    _logger?.info('Deleting comment: $commentId');

    final result = await _datasource.deleteComment(commentId);

    return result.fold(
      (error) => Result.error(error),
      (_) => Result.success(true),
    );
  }

  @override
  Future<Result<bool>> validateContent(String content) async {
    try {
      if (content.trim().isEmpty) {
        return Result.error('Content cannot be empty');
      }

      if (content.length > 2000) {
        return Result.error('Content is too long (max 2000 characters)');
      }

      return Result.success(true);
    } catch (e) {
      _logger?.error('Error validating content: $e');
      return Result.error('Failed to validate content: $e');
    }
  }
}
