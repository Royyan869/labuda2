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
/// Industry standard: optimistic UI (0ms) + server reconcile + push.
/// Polling removed to prevent N x Timer thundering herd (see like_handler.go:180).
/// Real-time for other users via single WS/SSE subscription (future) or
/// explicit [refreshLikeStats] after mutations.
class LikeRepositoryImpl implements LikeRepository {
  final LikeApiDatasource _datasource;
  final ILoggerService? _logger;

  // Active streams keyed by target (no per-card Timer)
  final Map<String, StreamController<LikeStats>> _activeStreamControllers = {};

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
  // REAL-TIME STREAMS (OPTIMISTIC + ON-DEMAND)
  // ===========================================

  String _streamKey(String targetId, LikeTargetType targetType) =>
      'likeStats_${targetId}_$targetType';

  @override
  Stream<LikeStats> watchLikeStats({
    required String targetId,
    required LikeTargetType targetType,
    required String currentUserId,
  }) {
    final key = _streamKey(targetId, targetType);
    _cleanupExistingStream(key);

    final controller = StreamController<LikeStats>.broadcast(
      onCancel: () => _cleanupStream(key),
    );
    _activeStreamControllers[key] = controller;

    // Initial fetch only - no periodic Timer (industry: 0ms optimistic + reconcile)
    _fetchAndEmitLikeStats(targetId, targetType, currentUserId, controller);

    return controller.stream;
  }

  @override
  void pushOptimisticLikeStats(LikeStats stats) {
    final key = _streamKey(stats.targetId, stats.targetType);
    final controller = _activeStreamControllers[key];
    if (controller == null || controller.isClosed) return;
    controller.add(stats);
  }

  @override
  Future<void> refreshLikeStats({
    required String targetId,
    required LikeTargetType targetType,
    required String currentUserId,
  }) async {
    final key = _streamKey(targetId, targetType);
    final controller = _activeStreamControllers[key];
    if (controller == null || controller.isClosed) return;
    _fetchAndEmitLikeStats(
      targetId,
      targetType,
      currentUserId,
      controller,
    );
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
    _activeStreamControllers.remove(key);
  }

  void _cleanupExistingStream(String key) {
    _activeStreamControllers[key]?.close();
    _activeStreamControllers.remove(key);
  }

  /// Cleanup all active streams
  void dispose() {
    for (final controller in _activeStreamControllers.values) {
      controller.close();
    }
    _activeStreamControllers.clear();
  }
}
