// Domain

// Dart
import 'package:firebase_messaging/firebase_messaging.dart';
import 'package:flutter_local_notifications/flutter_local_notifications.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/core/messaging/fcm_service_impl.dart';

// Data
import 'package:labuda/domains/system/notification/data/datasources/notification_remote_datasource.dart';

// Services
import 'package:labuda/domains/system/notification/services/fcm_service.dart';
import 'package:labuda/domains/system/notification/services/local_notification_service.dart';
import 'package:labuda/domains/system/notification/services/in_app_banner_service.dart';
import 'package:labuda/domains/system/notification/services/notification_trigger_impl.dart';

/// Notification Module - Dependency Injection
///
/// Registers FCM & local notification services ke GetIt service locator.
///
/// MIGRATION: Repository and use cases are now in Riverpod.
/// This file only handles FCM/local notification services.
///
/// FIRESTORE SUNSET (2025-02-20): Firestore removed from datasource.
///
/// Size: < 200 lines (per GUIDELINES)
class NotificationDI {
  /// Register all notification services
  /// Note: Repository and use cases now in Riverpod (notification_providers.dart)
  static void register() {
    _registerDatasource(); // Still needed for FCM service
    _registerServices();
    _registerTrigger();
    _registerCoreNotificationService();
  }

  /// Register datasource (needed for FCM service)
  static void _registerDatasource() {
    ServiceLocator.registerLazyService<NotificationRemoteDatasource>(
      () => NotificationRemoteDatasource(),
    );
  }

  /// Register services
  static void _registerServices() {
    // Local notification service
    ServiceLocator.registerLazyService<FlutterLocalNotificationsPlugin>(
      () => FlutterLocalNotificationsPlugin(),
    );

    ServiceLocator.registerLazyService<LocalNotificationService>(
      () => LocalNotificationService(
        plugin: sl<FlutterLocalNotificationsPlugin>(),
      ),
    );

    // In-app banner service
    ServiceLocator.registerLazyService<InAppBannerService>(
      () => InAppBannerService(),
    );

    // FCM service
    ServiceLocator.registerLazyService<FcmService>(
      () => FcmService(
        messaging: FirebaseMessaging.instance,
        datasource: sl<NotificationRemoteDatasource>(),
        localNotificationService: sl<LocalNotificationService>(),
        inAppBannerService: sl<InAppBannerService>(),
      ),
    );
  }

  /// Register notification trigger implementation
  ///
  /// **IMPORTANT:** Now uses NotificationService interface instead of
  /// direct Firebase SDK access. All Firebase operations are abstracted
  /// through FcmServiceImpl.
  static void _registerTrigger() {
    ServiceLocator.registerLazyService<INotificationTrigger>(
      () => NotificationTriggerImpl(
        notificationService: sl<NotificationService>(),
      ),
    );
  }

  /// Register core NotificationService (FcmServiceImpl)
  ///
  /// This is the ONLY place where Firebase SDK is accessed for
  /// notification trigger operations. All other files use the interface.
  static void _registerCoreNotificationService() {
    ServiceLocator.registerLazyService<NotificationService>(
      () => FcmServiceImpl.instance(),
    );
  }
}
