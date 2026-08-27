import 'package:labuda/core/core.dart';
import 'package:labuda/domains/system/notification/services/fcm_service.dart';

/// Auth Cleanup Service - Handles cleanup operations on sign out
///
/// **OWNERSHIP:**
/// - FCM token cleanup on sign out
/// - Other cleanup side effects that need to happen before/after auth state changes
///
/// **NOT THIS SERVICE'S RESPONSIBILITY:**
/// - Firebase sign out (owned by AuthSignInService)
/// - State management (owned by AuthController)
/// - Backend sync (owned by AuthController._syncWithBackend)
///
/// **IMPORTANT: FCM cleanup must happen BEFORE Firebase Auth sign out**
/// to ensure we can delete the token from Firestore while user is authenticated.
/// This prevents notifications from going to wrong user after account switch.
class AuthCleanupService {
  final ILoggerService logger;
  final FcmService fcmService;

  AuthCleanupService({required this.logger, required this.fcmService});

  /// Cleanup FCM token for current user
  ///
  /// This should be called BEFORE Firebase Auth sign out
  /// while user is still authenticated.
  ///
  /// [userId] - The user ID to cleanup FCM token for
  Future<void> cleanupFcmToken(String userId) async {
    try {
      await fcmService.cleanup(userId: userId);
      await logger.info('FCM cleanup completed', extra: {'userId': userId});
    } catch (e) {
      // Log but don't block sign out
      await logger.warning(
        'FCM cleanup failed',
        extra: {'error': e.toString()},
      );
    }
  }

  /// Perform all cleanup operations before sign out
  ///
  /// This should be called before Firebase Auth sign out
  /// while user is still authenticated.
  Future<void> cleanupBeforeSignOut(String userId) async {
    await cleanupFcmToken(userId);
    // Add other cleanup operations here as needed
  }

  /// Perform all cleanup operations after sign out
  ///
  /// This is called after Firebase Auth sign out completes.
  /// User is no longer authenticated at this point.
  Future<void> cleanupAfterSignOut() async {
    // No-op currently, but available for future use
    // Examples: clear local caches, reset state, etc.
    await logger.debug('Post-signout cleanup completed');
  }
}
