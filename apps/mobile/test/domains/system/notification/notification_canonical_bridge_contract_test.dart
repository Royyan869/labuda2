import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

String _read(String relativePath) => File(relativePath).readAsStringSync();

void main() {
  test(
    'notification taps route through NotificationNavigationHandler directly',
    () {
      final local = _read(
        'lib/domains/system/notification/services/local_notification_service.dart',
      );
      final fcmMessage = _read(
        'lib/domains/system/notification/services/fcm_message_handler.dart',
      );
      final fcmActions = _read(
        'lib/domains/system/notification/services/fcm_action_mapper.dart',
      );
      final listScreen = _read(
        'lib/domains/system/notification/presentation/screens/notification_list_screen.dart',
      );

      expect(local.contains('NotificationNavigationHandler.navigate'), isTrue);
      expect(
        fcmMessage.contains('NotificationNavigationHandler.navigate'),
        isTrue,
      );
      expect(
        fcmActions.contains('NotificationNavigationHandler.navigate'),
        isTrue,
      );
      expect(
        listScreen.contains('notificationNavigationServiceProvider'),
        isTrue,
      );
    },
  );
}
