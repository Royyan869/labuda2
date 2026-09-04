/// Content Notification Navigation Behavioral Proof
///
/// Proves that all Content notification paths use the canonical backend wire contract:
///   notification.type = "content.mentioned"
///   notification.data = { "targetId": "<uuid>", "targetType": "content" }
///
/// Tests cover:
///   1. Source-level contracts (no forbidden patterns / old type aliases)
///   2. Navigation routing correctness via payload data-key verification
///   3. FCM / local / foreground / background entry-point routing
///   4. Canonical type = content.mentioned (not mention / new_mention)
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

    test('mention notification type is content.mentioned (not mention)', () {
      final trigger = _read(
        'lib/core/interfaces/i_notification_trigger.dart',
      );
      expect(trigger.contains("contentMentioned('content.mentioned')"), isTrue,
          reason: 'NotificationType must define content.mentioned as canonical');
      expect(trigger.contains("mention('mention')"), isFalse,
          reason: 'Old mention("mention") enum must be removed');
    });

    test('no old mention type string in notification navigation', () {
      final nav = _read(
        'lib/domains/system/notification/services/notification_navigation_service.dart',
      );
      expect(nav.contains('NotificationType.mention'), isFalse,
          reason: 'Old NotificationType.mention must not be referenced');
    });

    test('no old mention type string in FCM action mapper', () {
      final mapper = _read(
        'lib/domains/system/notification/services/fcm_action_mapper.dart',
      );
      expect(mapper.contains("case 'mention':"), isFalse,
          reason: 'Old mention string must not be a case in FCM mapper');
      expect(mapper.contains("case 'new_mention':"), isFalse,
          reason: 'new_mention alias must not exist');
    });

    test('no old mention type string in core navigation handler', () {
      final handler = _read(
        'lib/core/utils/notification_navigation_handler.dart',
      );
      expect(handler.contains("case 'mention':"), isFalse,
          reason: 'Old mention string must not be a case in core handler');
    });

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
    test('mention service uses content.mentioned as canonical type', () {
      final mention = _read(
        'lib/shared/services/mention_notification_service.dart',
      );
      expect(mention.contains('NotificationType.contentMentioned'), isTrue,
          reason: 'MentionNotificationService must use canonical content.mentioned type');
      expect(mention.contains('NotificationType.mention'), isFalse,
          reason: 'Old mention type must not be used');
    });

    test('mention service sends targetId/targetType data keys', () {
      final mention = _read(
        'lib/shared/services/mention_notification_service.dart',
      );
      expect(mention.contains("'targetId'"), isTrue,
          reason: 'Mention service must send targetId (backend canonical key)');
      expect(mention.contains("'targetType'"), isTrue,
          reason: 'Mention service must send targetType (backend canonical key)');
      expect(mention.contains("'contentId'"), isFalse,
          reason: 'Old contentId key must not be used');
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

    test('mention navigation reads targetId/targetType (not contentId)', () {
      final nav = _read(
        'lib/domains/system/notification/services/notification_navigation_service.dart',
      );
      // _navigateToMention reads targetId/targetType (backend canonical)
      expect(nav.contains("'targetId'"), isTrue);
      expect(nav.contains("'targetType'"), isTrue);
      // Verify _navigateToMention method uses targetId, not contentId
      final mentionMethod = nav.substring(
        nav.indexOf('void _navigateToMention('),
        nav.indexOf('\n  void ', nav.indexOf('void _navigateToMention(') + 1),
      );
      expect(mentionMethod.contains("'targetId'"), isTrue,
          reason: '_navigateToMention must read targetId');
      expect(mentionMethod.contains("'contentId'"), isFalse,
          reason: '_navigateToMention must not read contentId');
    });

    test(
      'core handler case content.mentioned reads targetId/targetType',
      () {
        final handler = _read(
          'lib/core/utils/notification_navigation_handler.dart',
        );
        // Must use targetId/targetType, NOT postId / requestId
        expect(handler.contains("'targetId'"), isTrue);
        expect(handler.contains("'targetType'"), isTrue);
        expect(handler.contains("'postId'"), isFalse);
        expect(handler.contains("'requestId'"), isFalse);
      },
    );
  });
}
