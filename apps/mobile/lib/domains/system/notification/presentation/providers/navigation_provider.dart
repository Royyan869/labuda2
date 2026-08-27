/// Notification Navigation Provider
///
/// Provider for NotificationNavigationService.
/// Uses core navigationHandlerProvider for navigation.
///
/// Size: < 50 lines (per GUIDELINES)
library;

// Dart
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/system/notification/services/notification_navigation_service.dart';

/// Provider for NotificationNavigationService
///
/// This is a feature-specific provider that creates a navigation service
/// using the core navigation handler. It is NOT a bridge provider - it's
/// simply providing a feature-specific service that uses core infrastructure.
final notificationNavigationServiceProvider =
    Provider<NotificationNavigationService>((ref) {
      final navigationHandler = ref.watch(navigationHandlerProvider);
      return NotificationNavigationService(navigationHandler);
    });
