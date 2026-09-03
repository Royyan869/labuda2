import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:firebase_messaging/firebase_messaging.dart';

import '../providers/core_providers.dart';
import '../messaging/notification_service.dart';
import '../messaging/fcm_service_impl.dart';

// Observers
import 'screen_view_route_observer.dart';

// =============================================================================
// MESSAGING / NOTIFICATION
// =============================================================================

/// Provider for Firebase Messaging instance
final firebaseMessagingProvider = Provider<FirebaseMessaging>((ref) {
  return FirebaseMessaging.instance;
});

/// Provider for NotificationService (core interface)
final notificationServiceProvider = Provider<NotificationService>((ref) {
  final messaging = ref.watch(firebaseMessagingProvider);
  return FcmServiceImpl(messaging);
});

// =============================================================================
// OBSERVERS
// =============================================================================

/// Provider for screen view tracking through the canonical analytics sink.
final screenViewRouteObserverProvider = Provider<ScreenViewRouteObserver>((
  ref,
) {
  final analytics = ref.watch(coreAnalyticsRepositoryProvider);
  return ScreenViewRouteObserver(analytics);
});
