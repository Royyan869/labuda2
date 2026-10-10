/// Notification Initializer Widget
///
/// App-root seam for the notification domain:
/// - Auto-initializes FCM when user is authenticated (existing behavior).
/// - TASK_6A: keeps [notificationRealtimeSyncProvider] — THE
///   `notification.created` WebSocket consumer — alive for the whole
///   authenticated session. The consumer treats the event as an
///   invalidation/wake-up signal ONLY: canonical count/list are re-read from
///   the backend; `event.unread_count` is a point-in-time hint and is NEVER
///   applied as state.
/// - PHASE 5 (Option B): owns THE single notification reconciliation
///   cadence. One 10s timer — not one per projection — invalidates BOTH
///   canonical providers ([unreadCountProvider], [notificationListProvider])
///   via [reconcileNotificationState], the same function realtime events,
///   resume/reconnect, and mutation closures use. The old two independent
///   10s polling loops (one inside each repository stream) are gone:
///   providers are now one-shot FutureProviders, so a tick costs a request
///   only for projections actually being listened to.
///
/// Placement: mounted once in the app builder chain (app.dart), so the
/// provider instance (and its single WS subscription) is created exactly
/// once per session — no duplicate listeners regardless of route changes.
library;

// Dart
import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/system/notification/data/notification_providers.dart';
import 'package:hishumi/domains/system/notification/presentation/providers/notification_list_provider.dart';
import 'package:hishumi/domains/system/notification/presentation/providers/notification_realtime_sync_provider.dart';
import 'package:hishumi/domains/system/notification/presentation/providers/unread_count_provider.dart';

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

  /// PHASE 5 — THE single notification reconciliation cadence.
  ///
  /// Exactly one periodic mechanism for the whole notification domain
  /// (replaces the two independent 10s repository polling loops). Each tick
  /// routes through [reconcileNotificationState] — the SAME canonical
  /// function realtime events, resume/reconnect, and mutation closures use —
  /// so every trigger funnels into one convergence path. The timer is owned
  /// by this widget (cancelled on logout and dispose); a tick invalidates
  /// unlistened providers for free (no request is issued until a projection
  /// is watched again).
  Timer? _reconciliationTimer;

  static const Duration _reconciliationInterval = Duration(seconds: 10);

  @override
  void dispose() {
    _reconciliationTimer?.cancel();
    _reconciliationTimer = null;
    super.dispose();
  }

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
    // TASK_6A: keep THE notification realtime consumer alive for the whole
    // authenticated session. Watching (not reading) means the provider is
    // rebuilt on auth-scope changes and disposed with this root widget —
    // exactly ONE WS subscription per session.
    ref.watch(notificationRealtimeSyncProvider);

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
      // PHASE 5 — reconciliation cadence follows the authenticated scope:
      // start on login (and app-start resume), stop the moment the session
      // ends so no orphan timer outlives the account.
      _syncReconciliationCadence(next);
    });

    // Initial sync: ref.listen above only fires on CHANGES, so an app start
    // that is already authenticated must start the cadence here. Failure is
    // swallowed — host/test environments without auth wiring still boot.
    try {
      _syncReconciliationCadence(ref.read(authControllerProvider));
    } catch (_) {}

    return widget.child;
  }

  /// Starts THE single reconciliation cadence for an authenticated scope;
  /// stops it for any other scope. Idempotent per state — repeated builds
  /// and auth no-op changes never stack a second timer.
  void _syncReconciliationCadence(AuthState state) {
    if (state is AuthStateAuthenticated) {
      _startReconciliationCadence();
    } else {
      _stopReconciliationCadence();
    }
  }

  void _startReconciliationCadence() {
    if (_reconciliationTimer != null) return;
    _reconciliationTimer = Timer.periodic(
      _reconciliationInterval,
      (_) => _tickNotificationReconciliation(),
    );
  }

  void _stopReconciliationCadence() {
    _reconciliationTimer?.cancel();
    _reconciliationTimer = null;
  }

  /// One cadence tick = one canonical reconciliation of BOTH list + count.
  /// Identity is read FRESH at tick time so an account switch can never
  /// poll under the old user. Fail-closed on guest/unauthenticated scopes
  /// and on host/test environments without auth wiring.
  void _tickNotificationReconciliation() {
    try {
      final authState = ref.read(authControllerProvider);
      if (authState is! AuthStateAuthenticated) return;
      reconcileNotificationState(
        invalidate: (provider) => ref.invalidate(provider),
        userId: authState.user.id,
      );
    } catch (_) {
      // Provider not wired (tests/host) — a cadence tick must never crash.
    }
  }
}
