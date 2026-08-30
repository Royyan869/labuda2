import 'dart:async';

import 'package:labuda/core/common/result.dart';
import 'package:labuda/core/src/interfaces/services/i_logger_service.dart';
import 'package:labuda/domains/social/like/domain/entities/like.dart';
import 'package:labuda/domains/social/like/domain/repositories/like_repository.dart';
import 'package:labuda/domains/social/like/data/mappers/like_mapper.dart';
import 'package:labuda/domains/social/like/data/remote/like_api_datasource.dart';

/// API-based implementation of LikeRepository
///
/// Handles Like operations through the Go backend API.
/// Uses polling for real-time streams until WebSocket support is added.
class LikeRepositoryImpl implements LikeRepository {
  final LikeApiDatasource _datasource;
  final ILoggerService? _logger;

  // Polling timers for real-time streams
  final Map<String, Timer> _activePollingTimers = {};
  final Map<String, StreamController> _activeStreamControllers = {};

  LikeRepositoryImpl(this._datasource, {ILoggerService? logger})
    : _logger = logger;

  // ===========================================
  // LIKE OPERATIONS
  // ===========================================

  @override
  Future<Result<bool>> toggleLike({
    required String targetId,
    required LikeTargetType targetType,
    required String userId,
  }) async {
    _logger?.info('Toggling like on $targetType: $targetId');

    final request = LikeMapper.buildToggleRequest(
      targetId: targetId,
      targetType: targetType,
    );

    // Pass datasource Result through verbatim so the error code
    // (e.g. EMAIL_VERIFICATION_REQUIRED) reaches the notifier and call sites.
    return _datasource.toggleLike(request);
  }

  @override
  Future<Result<LikeStats>> getLikeStats({
    required String targetId,
    required LikeTargetType targetType,
    required String currentUserId,
  }) async {
    _logger?.info('Getting like stats for $targetType: $targetId');

    final result = await _datasource.getLikeStats(
      targetId: targetId,
      targetType: LikeMapper.buildToggleRequest(
        targetId: targetId,
        targetType: targetType,
      ).targetType,
    );

    return result.fold(
      (error) => Result.error(error),
      (response) => Result.success(LikeMapper.toLikeStats(response)),
    );
  }

  // ===========================================
  // REAL-TIME STREAMS (POLLING-BASED)
  // ===========================================

  @override
  Stream<LikeStats> watchLikeStats({
    required String targetId,
    required LikeTargetType targetType,
    required String currentUserId,
  }) {
    final key = 'likeStats_${targetId}_$targetType';
    _cleanupExistingStream(key);

    final controller = StreamController<LikeStats>.broadcast(
      onCancel: () => _cleanupStream(key),
    );
    _activeStreamControllers[key] = controller;

    // Initial fetch
    _fetchAndEmitLikeStats(targetId, targetType, currentUserId, controller);

    // Poll every 10 seconds (frequent for likes)
    _activePollingTimers[key] = Timer.periodic(
      const Duration(seconds: 10),
      (_) => _fetchAndEmitLikeStats(
        targetId,
        targetType,
        currentUserId,
        controller,
      ),
    );

    return controller.stream;
  }

  // ===========================================
  // STREAM HELPERS
  // ===========================================

  void _fetchAndEmitLikeStats(
    String targetId,
    LikeTargetType targetType,
    String currentUserId,
    StreamController<LikeStats> controller,
  ) async {
    if (controller.isClosed) return;

    final result = await getLikeStats(
      targetId: targetId,
      targetType: targetType,
      currentUserId: currentUserId,
    );

    result.fold(
      (error) => _logger?.warning('Failed to fetch like stats: $error'),
      (stats) {
        if (!controller.isClosed) {
          controller.add(stats);
        }
      },
    );
  }

  void _cleanupStream(String key) {
    _activePollingTimers[key]?.cancel();
    _activePollingTimers.remove(key);
    _activeStreamControllers.remove(key);
  }

  void _cleanupExistingStream(String key) {
    _activePollingTimers[key]?.cancel();
    _activePollingTimers.remove(key);
    _activeStreamControllers[key]?.close();
    _activeStreamControllers.remove(key);
  }

  /// Cleanup all active streams and timers
  void dispose() {
    for (final timer in _activePollingTimers.values) {
      timer.cancel();
    }
    _activePollingTimers.clear();

    for (final controller in _activeStreamControllers.values) {
      controller.close();
    }
    _activeStreamControllers.clear();
  }
}
