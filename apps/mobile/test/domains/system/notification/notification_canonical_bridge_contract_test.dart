import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

String _read(String relativePath) => File(relativePath).readAsStringSync();

void main() {
  const canonicalCall =
      'NotificationNavigationService.canonical()'
      '.handleNotificationPayload';

  test('notification taps route through the one destination decision', () {
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

    // Push, local-notification, and banner-action taps all resolve through the
    // SAME decision table as the in-app list.
    expect(local.contains(canonicalCall), isTrue);
    expect(fcmMessage.contains(canonicalCall), isTrue);
    expect(fcmActions.contains(canonicalCall), isTrue);
    expect(listScreen.contains('notificationNavigationServiceProvider'), isTrue);

    // No surface keeps a second destination decision.
    for (final source in [local, fcmMessage, fcmActions, listScreen]) {
      expect(source.contains('NotificationNavigationHandler'), isFalse);
    }
  });
}
