import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:flutter_local_notifications/flutter_local_notifications.dart';
import 'package:labuda/core/utils/notification_navigation_handler.dart';

/// Local Notification Service
///
/// Handles local notification display on device.
/// Used for showing notifications when app is in foreground.
///
/// Size: < 200 lines (per GUIDELINES)
class LocalNotificationService {
  final FlutterLocalNotificationsPlugin plugin;

  // BuildContext for navigation
  BuildContext? _context;

  // Notification tap callback (optional, for backward compatibility)
  Function(String?)? onNotificationTap;

  LocalNotificationService({required this.plugin});

  /// Set BuildContext for navigation
  void setContext(BuildContext context) {
    _context = context;
  }

  /// Initialize local notifications
  Future<void> initialize() async {
    try {
      // Android settings
      const androidSettings = AndroidInitializationSettings(
        '@mipmap/ic_launcher',
      );

      // iOS settings
      const iosSettings = DarwinInitializationSettings(
        requestAlertPermission: true,
        requestBadgePermission: true,
        requestSoundPermission: true,
      );

      // Combined settings
      const settings = InitializationSettings(
        android: androidSettings,
        iOS: iosSettings,
      );

      // Initialize with tap handler (API v20+ all named params)
      await plugin.initialize(
        settings: settings,
        onDidReceiveNotificationResponse: _handleNotificationTap,
      );

      // Create Android notification channel
      await _createNotificationChannel();
    } catch (e) {
      // Silently handle initialization errors
    }
  }

  /// Create Android notification channel
  Future<void> _createNotificationChannel() async {
    const channel = AndroidNotificationChannel(
      'labuda_default_channel', // id
      'General Notifications', // name
      description: 'Notifications for general activity in Labuda',
      importance: Importance.high,
      playSound: true,
      enableVibration: true,
    );

    await plugin
        .resolvePlatformSpecificImplementation<
          AndroidFlutterLocalNotificationsPlugin
        >()
        ?.createNotificationChannel(channel);
  }

  /// Show notification
  Future<void> show({
    required String title,
    required String body,
    String? payload,
  }) async {
    try {
      // Android notification details
      const androidDetails = AndroidNotificationDetails(
        'labuda_default_channel', // channel id
        'Notifikasi Umum', // channel name
        channelDescription: 'Notifikasi untuk aktivitas umum di Labuda',
        importance: Importance.high,
        priority: Priority.high,
        playSound: true,
        enableVibration: true,
        styleInformation: BigTextStyleInformation(''),
      );

      // iOS notification details
      const iosDetails = DarwinNotificationDetails(
        presentAlert: true,
        presentBadge: true,
        presentSound: true,
      );

      // Combined notification details
      const details = NotificationDetails(
        android: androidDetails,
        iOS: iosDetails,
      );

      // Show notification with unique ID (API v20+ uses named params)
      await plugin.show(
        id: DateTime.now().millisecondsSinceEpoch.remainder(100000),
        title: title,
        body: body,
        notificationDetails: details,
        payload: payload,
      );
    } catch (e) {
      // Silently handle show errors
    }
  }

  /// Show notification with big picture (future feature)
  Future<void> showWithImage({
    required String title,
    required String body,
    required String imageUrl,
    String? payload,
  }) async {
    try {
      // Android notification with big picture
      final androidDetails = AndroidNotificationDetails(
        'labuda_default_channel',
        'Notifikasi Umum',
        channelDescription: 'Notifikasi untuk aktivitas umum di Labuda',
        importance: Importance.high,
        priority: Priority.high,
        playSound: true,
        enableVibration: true,
        styleInformation: BigPictureStyleInformation(
          FilePathAndroidBitmap(imageUrl),
          contentTitle: title,
          summaryText: body,
        ),
      );

      // iOS notification details
      const iosDetails = DarwinNotificationDetails(
        presentAlert: true,
        presentBadge: true,
        presentSound: true,
        attachments: [],
      );

      // Combined notification details
      final details = NotificationDetails(
        android: androidDetails,
        iOS: iosDetails,
      );

      // Show notification (API v20+ uses named params)
      await plugin.show(
        id: DateTime.now().millisecondsSinceEpoch.remainder(100000),
        title: title,
        body: body,
        notificationDetails: details,
        payload: payload,
      );
    } catch (e) {
      // Silently handle show with image errors
    }
  }

  /// Cancel notification by ID
  Future<void> cancel(int id) async {
    await plugin.cancel(id: id);
  }

  /// Cancel all notifications
  Future<void> cancelAll() async {
    await plugin.cancelAll();
  }

  /// Handle notification tap
  void _handleNotificationTap(NotificationResponse response) {
    // Call callback if set (for backward compatibility)
    if (onNotificationTap != null) {
      onNotificationTap!(response.payload);
    }

    // Parse payload and navigate
    if (response.payload != null && _context != null && _context!.mounted) {
      try {
        // Payload should be JSON string with 'type' and other data
        final data = jsonDecode(response.payload!) as Map<String, dynamic>;
        final type = data['type'] as String?;

        if (type != null) {
          NotificationNavigationHandler.navigate(
            context: _context!,
            type: type,
            data: data,
          );
        }
      } catch (e) {
        // Silently handle payload parse errors
      }
    }
  }
}
