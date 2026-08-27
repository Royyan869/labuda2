import 'dart:async';
import 'package:firebase_messaging/firebase_messaging.dart';
import 'notification_service.dart';

/// Firebase Cloud Messaging Implementation
///
/// **Location:** lib/core/messaging/fcm_service_impl.dart
/// **Interface:** lib/core/messaging/notification_service.dart
///
/// **IMPORTANT:** Firebase SDK is ONLY allowed in this file.
/// All other files must use the NotificationService interface.
///
/// **FIRESTORE SUNSET (2025-02-20):**
/// - Firestore operations REMOVED
/// - Notification preferences now managed via Backend API
/// - User tokens now managed via Backend API
///
/// This implementation handles:
/// - FCM operations (tokens, topics, message streams)
class FcmServiceImpl implements NotificationService {
  final FirebaseMessaging _messaging;

  // Stream controllers
  StreamController<Map<String, dynamic>>? _onMessageController;
  StreamController<Map<String, dynamic>>? _onMessageOpenedController;

  // Subscription tracking
  StreamSubscription<RemoteMessage>? _onMessageSubscription;
  StreamSubscription<RemoteMessage>? _onMessageOpenedSubscription;

  FcmServiceImpl(this._messaging);

  /// Factory for default instance
  factory FcmServiceImpl.instance() {
    return FcmServiceImpl(FirebaseMessaging.instance);
  }

  @override
  Future<bool> requestPermission() async {
    final settings = await _messaging.requestPermission(
      alert: true,
      announcement: false,
      badge: true,
      carPlay: false,
      criticalAlert: false,
      provisional: false,
      sound: true,
    );
    return settings.authorizationStatus == AuthorizationStatus.authorized;
  }

  @override
  Future<String?> getToken() async {
    return await _messaging.getToken();
  }

  @override
  Future<void> subscribeToTopic(String topic) async {
    await _messaging.subscribeToTopic(topic);
  }

  @override
  Future<void> unsubscribeFromTopic(String topic) async {
    await _messaging.unsubscribeFromTopic(topic);
  }

  @override
  Stream<Map<String, dynamic>> get onMessage {
    _onMessageController ??= StreamController<Map<String, dynamic>>.broadcast();
    _onMessageSubscription ??= FirebaseMessaging.onMessage.listen((event) {
      _onMessageController?.add(_messageToMap(event));
    });
    return _onMessageController!.stream;
  }

  @override
  Stream<Map<String, dynamic>> get onMessageOpened {
    _onMessageOpenedController ??=
        StreamController<Map<String, dynamic>>.broadcast();
    _onMessageOpenedSubscription ??= FirebaseMessaging.onMessageOpenedApp
        .listen((event) {
          _onMessageOpenedController?.add(_messageToMap(event));
        });
    return _onMessageOpenedController!.stream;
  }

  @override
  Future<void> initialize({required String userId}) async {
    // Streams are lazily initialized when accessed via getters
    // This method is a hook for future initialization needs
  }

  @override
  Future<void> cleanup({required String userId}) async {
    await _messaging.deleteToken();
  }

  /// Convert RemoteMessage to Map
  Map<String, dynamic> _messageToMap(RemoteMessage message) {
    return {
      'data': message.data,
      'notification': message.notification != null
          ? {
              'title': message.notification?.title,
              'body': message.notification?.body,
            }
          : null,
    };
  }

  /// Dispose resources
  void dispose() {
    _onMessageSubscription?.cancel();
    _onMessageOpenedSubscription?.cancel();
    _onMessageController?.close();
    _onMessageOpenedController?.close();
  }

  // ==================== Data Operations (Backend API) ====================
  // Firestore sunset 2025-02-20 - all data operations now use Backend API

  @override
  Future<Map<String, dynamic>?> getPreferences({required String userId}) async {
    throw UnimplementedError(
      'Notification preferences moved to backend. Use Backend API.',
    );
  }

  @override
  Future<void> saveNotification(Map<String, dynamic> data) async {
    throw UnimplementedError(
      'Notification creation moved to backend. Use backend API.',
    );
  }

  @override
  Future<String?> getUserToken({required String userId}) async {
    throw UnimplementedError(
      'User token retrieval moved to backend. Use Backend API.',
    );
  }

  @override
  Future<void> updatePreferences({
    required String userId,
    required Map<String, dynamic> preferences,
  }) async {
    throw UnimplementedError(
      'Notification preferences moved to backend. Use Backend API.',
    );
  }
}
