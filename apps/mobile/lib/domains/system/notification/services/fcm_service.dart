/// FCM Service (REFACTORED)
///
/// Main orchestrator for Firebase Cloud Messaging.
/// Delegates specific responsibilities to specialized services.
///
/// REFACTORED: Extracted token management, message handling, and action mapping
/// to comply with GUIDELINES file size limits.
///
/// Responsibilities:
/// - Initialization coordination
/// - Lifecycle management
/// - Cleanup on logout
///
/// Size: < 200 lines (per GUIDELINES) ✅
library;

// Dart
import 'dart:async';
import 'fcm_message_handler.dart';
import 'fcm_token_manager.dart';
import 'in_app_banner_service.dart';
import 'local_notification_service.dart';
import 'package:firebase_messaging/firebase_messaging.dart';
import 'package:labuda/core/src/interfaces/services/i_logger_service.dart';
import 'package:labuda/domains/system/notification/data/datasources/notification_remote_datasource.dart';

class FcmService {
  final FirebaseMessaging messaging;
  final NotificationRemoteDatasource datasource;
  final LocalNotificationService localNotificationService;
  final InAppBannerService inAppBannerService;
  final ILoggerService? logger;

  // Specialized services
  late final FCMTokenManager _tokenManager;
  late final FCMMessageHandler _messageHandler;

  // State tracking
  bool _isInitializing = false;
  bool _isCleaningUp = false;
  String? _currentUserId;

  FcmService({
    required this.messaging,
    required this.datasource,
    required this.localNotificationService,
    required this.inAppBannerService,
    this.logger,
  }) {
    // Initialize specialized services. Logger plumbed through so the
    // token manager's structured-error / retry / mutex paths emit
    // visible telemetry instead of failing silently.
    _tokenManager = FCMTokenManager(
      messaging: messaging,
      datasource: datasource,
      logger: logger,
    );
    _messageHandler = FCMMessageHandler(
      messaging: messaging,
      inAppBannerService: inAppBannerService,
    );
  }

  /// Get current FCM token
  String? get fcmToken => _tokenManager.fcmToken;

  /// Initialize FCM for user
  ///
  /// Steps:
  /// 1. Request permissions
  /// 2. Initialize token
  /// 3. Setup message handlers
  Future<void> initialize({required String userId}) async {
    try {
      // Guard: Already initialized for this user
      if (_currentUserId == userId && fcmToken != null) {
        return;
      }

      // Guard: Prevent concurrent initialization
      if (_isInitializing) {
        return;
      }

      // Guard: Different user - cleanup first
      if (_currentUserId != null && _currentUserId != userId) {
        await cleanup(userId: _currentUserId!);
      }

      _isInitializing = true;

      // Step 1: Request permission
      final isAuthorized = await _tokenManager.requestPermission();
      if (!isAuthorized) {
        _isInitializing = false;
        return;
      }

      // Step 2: Initialize token
      await _tokenManager.initializeToken(userId);

      // Step 3: Setup message handlers
      await _messageHandler.setupListeners();

      // Mark initialization complete
      _currentUserId = userId;
      _isInitializing = false;
    } catch (e) {
      _isInitializing = false;
    }
  }

  /// Subscribe to topic
  Future<void> subscribeToTopic(String topic) async {
    try {
      await messaging.subscribeToTopic(topic);
    } catch (e) {
      // Silently fail on subscribe error
    }
  }

  /// Unsubscribe from topic
  Future<void> unsubscribeFromTopic(String topic) async {
    try {
      await messaging.unsubscribeFromTopic(topic);
    } catch (e) {
      // Silently fail on unsubscribe error
    }
  }

  /// Cleanup FCM on logout
  ///
  /// IMPORTANT: Call BEFORE signing out to ensure proper token removal
  /// from Firestore while user is still authenticated.
  ///
  /// Prevents cross-account notification leakage by:
  /// 1. Deleting FCM token from device
  /// 2. Removing token from Firestore user document
  /// 3. Canceling all listeners
  Future<void> cleanup({required String userId}) async {
    try {
      // Guard: Prevent concurrent cleanup
      if (_isCleaningUp) {
        return;
      }

      _isCleaningUp = true;

      // 1. Delete token (device + Firestore)
      await _tokenManager.deleteToken(userId);

      // 2. Dispose message handlers
      _messageHandler.dispose();
      _tokenManager.dispose();

      // 3. Clear state
      _currentUserId = null;
      _isCleaningUp = false;
    } catch (e) {
      _isCleaningUp = false;
      rethrow;
    }
  }

  /// Dispose - cleanup all resources
  void dispose() {
    _tokenManager.dispose();
    _messageHandler.dispose();
  }
}
