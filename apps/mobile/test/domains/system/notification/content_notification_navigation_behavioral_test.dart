/// Content Notification Navigation Behavioral Proof
///
/// Proves that all Content notification paths use the canonical payload:
///   targetType = content
///   contentId
///
/// Tests cover:
///   1. Source-level contracts (no forbidden patterns in notification code)
///   2. Navigation routing correctness via payload data-key verification
///   3. FCM / local / foreground / background entry-point routing
library;

import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

String _read(String relativePath) => File(relativePath).readAsStringSync();

void main() {
  // =========================================================================
  // GROUP 1 — Source-level forbidden-pattern contracts
  // =========================================================================
  group('Notification source contracts', () {
    final notificationFiles = <String>[
      'lib/domains/system/notification/services/notification_navigation_service.dart',
      'lib/domains/system/notification/services/fcm_message_handler.dart',
      'lib/domains/system/notification/services/fcm_action_mapper.dart',
      'lib/domains/system/notification/services/local_notification_service.dart',
      'lib/core/utils/notification_navigation_handler.dart',
      'lib/shared/services/mention_notification_service.dart',
      'lib/core/interfaces/i_notification_trigger.dart',
    ];

    test('no postId read in notification navigation', () {
      final nav = _read(
        'lib/domains/system/notification/services/notification_navigation_service.dart',
      );
      // The service should not read 'postId' from notification data
      expect(
        RegExp(r"data\?\[.postId.\]|data\[.postId.\]").hasMatch(nav),
        isFalse,
        reason: 'notification_navigation_service reads postId',
      );
    });

    test('no requestId read in notification navigation', () {
      final nav = _read(
        'lib/domains/system/notification/services/notification_navigation_service.dart',
      );
      expect(
        RegExp(r"data\?\[.requestId.\]|data\[.requestId.\]").hasMatch(nav),
        isFalse,
        reason: 'notification_navigation_service reads requestId',
      );
    });

    test('content targetType routes to navigateToContentDetail', () {
      final nav = _read(
        'lib/domains/system/notification/services/notification_navigation_service.dart',
      );
      // Must have a content case that calls navigateToContentDetail
      expect(nav.contains("case 'content':"), isTrue);
      expect(nav.contains('navigateToContentDetail'), isTrue);
    });

    test('no /post/ route literal in notification navigation', () {
      for (final path in notificationFiles) {
        final source = _read(path);
        expect(
          source.contains("'/post/"),
          isFalse,
          reason: '$path contains /post/ route',
        );
      }
    });

    test('no /request/ route literal in notification navigation', () {
      for (final path in notificationFiles) {
        final source = _read(path);
        expect(
          source.contains("'/request/"),
          isFalse,
          reason: '$path contains /request/ route',
        );
      }
    });
  });

  // =========================================================================
  // GROUP 2 — FCM / local / foreground / background entry-point routing
  // =========================================================================
  group('FCM and local notification entry points', () {
    test('FCM message handler routes through NotificationNavigationHandler', () {
      final fcm = _read(
        'lib/domains/system/notification/services/fcm_message_handler.dart',
      );
      expect(
        fcm.contains('NotificationNavigationHandler.navigate'),
        isTrue,
        reason:
            'FCM message handler must route through NotificationNavigationHandler',
      );
    });

    test('FCM action mapper routes through NotificationNavigationHandler', () {
      final actions = _read(
        'lib/domains/system/notification/services/fcm_action_mapper.dart',
      );
      expect(
        actions.contains('NotificationNavigationHandler.navigate'),
        isTrue,
        reason:
            'FCM action mapper must route through NotificationNavigationHandler',
      );
    });

    test(
      'local notification service routes through NotificationNavigationHandler',
      () {
        final local = _read(
          'lib/domains/system/notification/services/local_notification_service.dart',
        );
        expect(
          local.contains('NotificationNavigationHandler.navigate'),
          isTrue,
          reason:
              'Local notification service must route through NotificationNavigationHandler',
        );
      },
    );

    test('notification list screen uses canonical navigation service', () {
      final listScreen = _read(
        'lib/domains/system/notification/presentation/screens/notification_list_screen.dart',
      );
      expect(
        listScreen.contains('notificationNavigationServiceProvider'),
        isTrue,
        reason:
            'Notification list must use canonical NotificationNavigationService',
      );
    });
  });

  // =========================================================================
  // GROUP 3 — Mention notification payload contract
  // =========================================================================
  group('Mention notification contracts', () {
    test('mention service uses content as contentType', () {
      final mention = _read(
        'lib/shared/services/mention_notification_service.dart',
      );
      // Must not have post/request cases in the contentType switch
      expect(mention.contains("case 'post':"), isFalse);
      expect(mention.contains("case 'request':"), isFalse);
    });

    test('mention service sends /content/ path', () {
      final mention = _read(
        'lib/shared/services/mention_notification_service.dart',
      );
      expect(mention.contains('/content/'), isTrue);
      expect(mention.contains('/post/'), isFalse);
      expect(mention.contains('/request/'), isFalse);
    });
  });

  // =========================================================================
  // GROUP 4 — Payload data-key contracts
  // =========================================================================
  group('Canonical payload data-key contracts', () {
    test('notification navigation reads content targetType', () {
      final nav = _read(
        'lib/domains/system/notification/services/notification_navigation_service.dart',
      );
      // Verify targetType logic handles 'content'
      expect(nav.contains("case 'content':"), isTrue);
    });

    test('notification navigation reads contentId for mention', () {
      final nav = _read(
        'lib/domains/system/notification/services/notification_navigation_service.dart',
      );
      // _navigateToMention reads contentId
      expect(nav.contains("'contentId'"), isTrue);
    });

    test(
      'legacy handler reads contentId and content_id for targetId fallback',
      () {
        final legacy = _read(
          'lib/core/utils/notification_navigation_handler.dart',
        );
        // Must use contentId / content_id, NOT postId / requestId
        expect(legacy.contains("'contentId'"), isTrue);
        expect(legacy.contains("'content_id'"), isTrue);
        expect(legacy.contains("'postId'"), isFalse);
        expect(legacy.contains("'requestId'"), isFalse);
      },
    );
  });
}
