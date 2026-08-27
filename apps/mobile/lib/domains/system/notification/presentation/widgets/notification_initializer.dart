/// Notification Initializer Widget
///
/// Auto-initializes FCM when user is authenticated.
/// Place this widget in the widget tree (e.g., in app.dart or main_screen.dart)
///
/// R4.1 DI Standardization: Now uses Riverpod providers as canonical DI path.
/// Previously used `sl<T>()` for service access - migrated to `ref.watch<T>()`.
library;

// Dart
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/system/notification/data/notification_providers.dart';

// Flutter
import 'package:flutter/material.dart';

class NotificationInitializer extends ConsumerStatefulWidget {
  final Widget child;

  const NotificationInitializer({super.key, required this.child});

  @override
  ConsumerState<NotificationInitializer> createState() =>
      _NotificationInitializerState();
}

class _NotificationInitializerState
    extends ConsumerState<NotificationInitializer> {
  bool _initialized = false;
  String? _currentUserId;

  @override
  void initState() {
    super.initState();
    // Note: FCM initialization is handled entirely in ref.listen() in build()
    // This covers both initial state and state changes in one place
  }

  /// Cleanup FCM on logout
  ///
  /// Removes FCM token from Firestore to prevent notifications
  /// from being sent to wrong user after account switch.
  Future<void> _cleanupFcm() async {
    if (_currentUserId == null) {
      _initialized = false;
      return;
    }

    try {
      // R4.1: Use provider path instead of sl<FcmService>()
      final fcmService = ref.read(fcmServiceProvider);
      await fcmService.cleanup(userId: _currentUserId!);

      final logger = ref.read(loggerServiceProvider);
      await logger.info(
        'FCM cleanup completed on logout',
        extra: {'userId': _currentUserId},
      );
    } catch (e) {
      final logger = ref.read(loggerServiceProvider);
      await logger.error(
        'Failed to cleanup FCM on logout',
        extra: {'error': e.toString(), 'userId': _currentUserId},
      );
    } finally {
      _initialized = false;
      _currentUserId = null;
    }
  }

  Future<void> _initializeFcm(String userId) async {
    if (_initialized) return;

    try {
      // R4.1: Use provider path instead of sl<FcmService>()
      final fcmService = ref.read(fcmServiceProvider);

      // Initialize FCM (context is now fetched from AppRouter when needed)
      await fcmService.initialize(userId: userId);

      _initialized = true;

      // Log successful initialization
      final logger = ref.read(loggerServiceProvider);
      await logger.info('FCM initialized for user', extra: {'userId': userId});
    } catch (e) {
      // Log error but don't block the app
      final logger = ref.read(loggerServiceProvider);
      await logger.error(
        'Failed to initialize FCM',
        extra: {'error': e.toString(), 'userId': userId},
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    // Listen to auth state changes (including initial state)
    // This single listener handles both app startup and login/logout events
    ref.listen<AuthState>(authControllerProvider, (previous, next) {
      if (next is AuthStateAuthenticated && !_initialized) {
        // User is authenticated (either initial state or just logged in)
        _currentUserId = next.user.id;
        _initializeFcm(next.user.id);
      } else if (next is! AuthStateAuthenticated && _initialized) {
        // User logged out, cleanup FCM then reset
        _cleanupFcm();
      }
    });

    return widget.child;
  }
}
