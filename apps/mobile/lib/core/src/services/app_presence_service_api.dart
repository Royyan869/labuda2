import 'dart:async';
import 'package:labuda/core/core.dart';
import 'package:labuda/core/websocket/websocket_message.dart';

/// App-level presence service using Go backend API + WebSocket
///
/// Replaces Firestore-based AppPresenceService with API calls.
/// Uses REST API for presence updates and WebSocket for real-time status.
class AppPresenceServiceApi implements IPresenceService {
  final ApiClient _apiClient;
  final WebSocketService _webSocketService;
  final ILoggerService _logger;

  /// Stream controllers for real-time presence updates
  final Map<String, StreamController<bool>> _presenceStreamControllers = {};
  final StreamController<Map<String, bool>> _multiPresenceController =
      StreamController<Map<String, bool>>.broadcast();

  /// Cache for user presence status
  final Map<String, bool> _presenceCache = {};

  /// WebSocket event subscription
  StreamSubscription<WebSocketMessage>? _wsEventSubscription;

  AppPresenceServiceApi({
    required ApiClient apiClient,
    required WebSocketService webSocketService,
    required ILoggerService logger,
  }) : _apiClient = apiClient,
       _webSocketService = webSocketService,
       _logger = logger {
    _initializeWebSocketListener();
  }

  // ========================================
  // WebSocket Integration
  // ========================================

  void _initializeWebSocketListener() {
    // Listen to presence events from WebSocket
    _wsEventSubscription = _webSocketService.messages.listen(
      _handleWebSocketEvent,
      onError: (error) {
        _logger.error('Presence WebSocket error: $error');
      },
    );
  }

  void _handleWebSocketEvent(WebSocketMessage message) {
    if (message.type != MessageType.presence) return;

    try {
      final userId = message.data['user_id'] as String?;
      final status = message.data['status'] as String?;

      if (userId == null || status == null) return;

      final isOnline = status == 'online';

      // Update cache
      _presenceCache[userId] = isOnline;

      // Update individual stream
      final controller = _presenceStreamControllers[userId];
      if (controller != null && !controller.isClosed) {
        controller.add(isOnline);
      }

      // Update multi-presence stream
      if (!_multiPresenceController.isClosed) {
        _multiPresenceController.add(Map.from(_presenceCache));
      }

      _logger.debug('Presence updated: $userId = $isOnline');
    } catch (e, stackTrace) {
      _logger.error(
        'Error handling presence event: $e',
        stackTrace: stackTrace,
      );
    }
  }

  // ========================================
  // Presence Operations
  // ========================================

  @override
  Future<Result<bool>> updatePresence({
    required String userId,
    required bool isOnline,
  }) async {
    try {
      _logger.info('Updating presence: $userId, online: $isOnline');

      final response = await _apiClient.post(
        '/users/presence',
        data: {'is_online': isOnline},
      );

      if (response.statusCode == 200) {
        // Send presence update via WebSocket for real-time broadcast
        if (_webSocketService.isConnected) {
          _webSocketService.send(
            WebSocketMessage(
              type: MessageType.presence,
              from: userId,
              data: {'status': isOnline ? 'online' : 'offline'},
            ),
          );
        }

        // Update cache
        _presenceCache[userId] = isOnline;

        return Result.success(true);
      }

      return Result.error('Failed to update presence: ${response.statusCode}');
    } catch (e, stackTrace) {
      _logger.error('Error updating presence: $e', stackTrace: stackTrace);
      return Result.error('Failed to update presence: ${e.toString()}');
    }
  }

  @override
  Future<Result<bool>> startTracking(String userId) async {
    try {
      _logger.info('Starting presence tracking: $userId');

      final response = await _apiClient.post('/users/presence/start');

      if (response.statusCode == 200) {
        // Send online status via WebSocket
        if (_webSocketService.isConnected) {
          _webSocketService.send(
            WebSocketMessage(
              type: MessageType.presence,
              from: userId,
              data: {'status': 'online'},
            ),
          );
        }

        _presenceCache[userId] = true;
        return Result.success(true);
      }

      return Result.error('Failed to start tracking: ${response.statusCode}');
    } catch (e, stackTrace) {
      _logger.error('Error starting tracking: $e', stackTrace: stackTrace);
      return Result.error('Failed to start tracking: ${e.toString()}');
    }
  }

  @override
  Future<Result<bool>> stopTracking(String userId) async {
    try {
      _logger.info('Stopping presence tracking: $userId');

      final response = await _apiClient.post('/users/presence/stop');

      if (response.statusCode == 200) {
        // Send offline status via WebSocket
        if (_webSocketService.isConnected) {
          _webSocketService.send(
            WebSocketMessage(
              type: MessageType.presence,
              from: userId,
              data: {'status': 'offline'},
            ),
          );
        }

        _presenceCache[userId] = false;
        return Result.success(true);
      }

      return Result.error('Failed to stop tracking: ${response.statusCode}');
    } catch (e, stackTrace) {
      _logger.error('Error stopping tracking: $e', stackTrace: stackTrace);
      return Result.error('Failed to stop tracking: ${e.toString()}');
    }
  }

  @override
  Future<Result<bool>> getUserOnlineStatus(String userId) async {
    try {
      // Check cache first
      if (_presenceCache.containsKey(userId)) {
        return Result.success(_presenceCache[userId]!);
      }

      _logger.info('Getting online status: $userId');

      final response = await _apiClient.get('/users/$userId/presence');

      if (response.statusCode == 200) {
        final data = response.data as Map<String, dynamic>;
        final isOnline = data['is_online'] as bool? ?? false;

        // Update cache
        _presenceCache[userId] = isOnline;

        return Result.success(isOnline);
      }

      return Result.success(false);
    } catch (e, stackTrace) {
      _logger.error('Error getting online status: $e', stackTrace: stackTrace);
      return Result.success(false); // Default to offline on error
    }
  }

  @override
  Future<Result<DateTime?>> getUserLastSeen(String userId) async {
    try {
      _logger.info('Getting last seen: $userId');

      final response = await _apiClient.get('/users/$userId/presence');

      if (response.statusCode == 200) {
        final data = response.data as Map<String, dynamic>;
        final lastSeenStr = data['last_seen_at'] as String?;

        if (lastSeenStr == null) return Result.success(null);

        final lastSeen = DateTime.parse(lastSeenStr);
        return Result.success(lastSeen);
      }

      return Result.success(null);
    } catch (e, stackTrace) {
      _logger.error('Error getting last seen: $e', stackTrace: stackTrace);
      return Result.success(null);
    }
  }

  @override
  Stream<bool> watchUserPresence(String userId) {
    // Create stream controller if not exists
    if (!_presenceStreamControllers.containsKey(userId)) {
      _presenceStreamControllers[userId] = StreamController<bool>.broadcast(
        onListen: () => _subscribeToUserPresence(userId),
        onCancel: () => _unsubscribeFromUserPresence(userId),
      );
    }

    // Fetch initial status
    getUserOnlineStatus(userId).then((result) {
      result.fold((_) => null, (isOnline) {
        final controller = _presenceStreamControllers[userId];
        if (controller != null && !controller.isClosed) {
          controller.add(isOnline);
        }
      });
    });

    return _presenceStreamControllers[userId]!.stream;
  }

  @override
  Stream<Map<String, bool>> watchUsersPresence(List<String> userIds) {
    // Fetch initial statuses
    _fetchMultiplePresence(userIds);

    // Return stream that gets updated by WebSocket events
    return _multiPresenceController.stream;
  }

  Future<void> _subscribeToUserPresence(String userId) async {
    // Presence updates come via WebSocket automatically
    // Just fetch initial status
    await getUserOnlineStatus(userId);
  }

  void _unsubscribeFromUserPresence(String userId) {
    // Cleanup controller if no more listeners
    final controller = _presenceStreamControllers[userId];
    if (controller != null && !controller.hasListener) {
      _presenceStreamControllers.remove(userId);
    }
  }

  Future<void> _fetchMultiplePresence(List<String> userIds) async {
    try {
      // Fetch presence for multiple users
      // Note: Backend should support batch query, otherwise fetch individually
      for (final userId in userIds) {
        final result = await getUserOnlineStatus(userId);
        result.fold(
          (_) => null,
          (isOnline) => _presenceCache[userId] = isOnline,
        );
      }

      if (!_multiPresenceController.isClosed) {
        _multiPresenceController.add(Map.from(_presenceCache));
      }
    } catch (e) {
      _logger.error('Error fetching multiple presence: $e');
    }
  }

  void dispose() {
    _wsEventSubscription?.cancel();
    for (final controller in _presenceStreamControllers.values) {
      controller.close();
    }
    _presenceStreamControllers.clear();
    _multiPresenceController.close();
    _presenceCache.clear();
  }
}
