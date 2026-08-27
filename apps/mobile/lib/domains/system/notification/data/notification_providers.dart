/// Notification Data Providers - Riverpod providers for notification data layer
///
/// This file provides all data dependencies for the notification feature using pure Riverpod.
/// Replaces the GetIt-based NotificationApiDI dependency injection.
///
/// MIGRATION STATUS: Migrated from notification_api_di.dart (GetIt) to Riverpod
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/system/notification/data/datasources/notification_api_datasource.dart';
import 'package:labuda/domains/system/notification/data/repositories/notification_repository_api.dart';
import 'package:labuda/domains/system/notification/domain/repositories/i_notification_repository.dart';

import 'package:labuda/domains/system/notification/services/fcm_service.dart';
import 'package:labuda/domains/system/notification/services/local_notification_service.dart';

// =============================================================================
// DATASOURCE PROVIDERS
// =============================================================================

/// Notification API Datasource Provider
final notificationApiDatasourceProvider = Provider<NotificationApiDatasource>((
  ref,
) {
  final apiClient = ref.watch(apiClientProvider);
  return NotificationApiDatasource(apiClient);
});

// =============================================================================
// REPOSITORY PROVIDERS
// =============================================================================

/// Notification Repository Provider
///
/// Provides the API implementation of INotificationRepository.
/// This replaces the GetIt-based NotificationApiDI.notificationRepository.
///
/// MIGRATION: Previously accessed via NotificationApiDI.notificationRepository or sl\<INotificationRepository\>()
final notificationRepositoryProvider = Provider<INotificationRepository>((ref) {
  final datasource = ref.watch(notificationApiDatasourceProvider);
  final apiClient = ref.watch(apiClientProvider);
  return NotificationRepositoryApi(datasource, apiClient);
});

// =============================================================================
// SERVICE BRIDGE PROVIDERS (R4.1 DI Standardization)
// =============================================================================
//
// These providers must be overridden in main.dart ProviderScope with instances
// constructed directly during bootstrap. GetIt bridge removed.
//
// **CANONICAL PATH:** Override in ProviderScope, consume via ref.watch/read.

/// FCM Service Provider — must be overridden in ProviderScope.
final fcmServiceProvider = Provider<FcmService>((ref) {
  throw UnimplementedError(
    'FcmService must be provided externally. '
    'Override fcmServiceProvider in main.dart ProviderScope.',
  );
});

/// Local Notification Service Provider — must be overridden in ProviderScope.
final localNotificationServiceProvider = Provider<LocalNotificationService>((
  ref,
) {
  throw UnimplementedError(
    'LocalNotificationService must be provided externally. '
    'Override localNotificationServiceProvider in main.dart ProviderScope.',
  );
});
