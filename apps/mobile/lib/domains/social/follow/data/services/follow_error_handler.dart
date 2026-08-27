import 'package:labuda/core/core.dart';

/// Error handler untuk follow operations
/// Extracted dari follow_repository_impl.dart untuk better organization
class FollowErrorHandler {
  final ILoggerService _logger;

  const FollowErrorHandler(this._logger);

  /// Handle follow operation errors
  Future<Result<T>> handleFollowError<T>(
    Exception e, {
    required String followerId,
    required String followingId,
    required String operation,
    Map<String, dynamic>? extra,
  }) async {
    final errorData = {
      'error': e.toString(),
      'followerId': followerId,
      'followingId': followingId,
      'operation': operation,
      ...?extra,
    };

    await _logger.error('$operation failed', extra: errorData);

    return Result.error('Failed to $operation: ${e.toString()}');
  }

  /// Handle block/mute operation errors
  Future<Result<T>> handleBlockMuteError<T>(
    Exception e, {
    required String userId,
    required String targetUserId,
    required String operation,
    Map<String, dynamic>? extra,
  }) async {
    final errorData = {
      'error': e.toString(),
      'userId': userId,
      'targetUserId': targetUserId,
      'operation': operation,
      ...?extra,
    };

    await _logger.error('$operation failed', extra: errorData);

    return Result.error('Failed to $operation: ${e.toString()}');
  }

  /// Handle data retrieval errors
  Future<Result<T>> handleDataError<T>(
    Exception e, {
    required String operation,
    Map<String, dynamic>? extra,
  }) async {
    await _logger.error(
      '$operation failed',
      extra: {'error': e.toString(), ...?extra},
    );

    return Result.error('Failed to $operation: ${e.toString()}');
  }

  /// Handle search operation errors
  Future<Result<T>> handleSearchError<T>(
    Exception e, {
    required String query,
    Map<String, dynamic>? extra,
  }) async {
    final errorData = {'error': e.toString(), 'query': query, ...?extra};

    await _logger.error('Search operation failed', extra: errorData);

    return Result.error('Failed to search: ${e.toString()}');
  }
}
