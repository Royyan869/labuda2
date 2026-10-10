/// TASK_6A — THE `notification.created` WebSocket consumer.
///
/// Subscribes ONCE to the existing authenticated WebSocket session (the same
/// socket chat uses — no second connection, no notification-specific
/// transport). The backend already user-targets the event; identity for
/// reconciliation comes from the local auth session
/// ([currentUserIdProvider]) — never from the event payload.
///
/// EVENT SEMANTICS: `notification.created` is an invalidation / wake-up
/// signal ONLY. The payload's `unread_count` is a supplemental point-in-time
/// hint and is deliberately NEVER read: reconciliation always re-reads the
/// canonical providers ([unreadCountProvider] → GET /notifications/unread-count,
/// [notificationListProvider] → GET /notifications). No local count math, no
/// mark-read, no Chat mutation.
///
/// SINGLE LISTENER: kept alive by the app-root NotificationInitializer via
/// `ref.watch`. Riverpod deduplicates provider instances, so any number of
/// consumers of this provider still yields exactly ONE WS subscription and
/// exactly one logical reconciliation per event.
library;

import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/core/websocket/websocket_message.dart';
import 'package:hishumi/domains/system/notification/presentation/providers/notification_list_provider.dart';
import 'package:hishumi/shared/providers/auth_status_providers.dart';

/// Active-notification realtime reconciliation seam. Stateless by design —
/// it holds no count and creates no authority; it only invalidates the
/// canonical providers when the backend signals a committed notification.
final notificationRealtimeSyncProvider = Provider<void>((ref) {
  // Fail closed without an authenticated user scope.
  final userId = ref.watch(currentUserIdProvider);
  if (userId.isEmpty) return;

  // Host/test environments without WS wiring keep the app functional —
  // polling and resume remain the recovery paths there.
  final WebSocketService socket;
  try {
    socket = ref.watch(webSocketServiceProvider);
  } catch (_) {
    return;
  }

  // Contract (backend Tasks 5 + 4.2):
  //   notification.created — a notification row was committed;
  //   notification.updated — a committed mutation changed this user's
  //     notification state (mark-read / mark-all / delete / chat-room-read
  //     sync); one event per user-level state change.
  // Both carry unread_count only as a supplemental point-in-time hint — it
  // is deliberately NOT parsed: reconciliation always re-reads the canonical
  // providers.
  StreamSubscription<WebSocketMessage>? subscription;
  subscription = socket.messages.listen(
    (message) {
      final type = message.type;
      if (type != 'notification.created' && type != 'notification.updated') {
        return;
      }
      try {
        // Canonical reconciliation path (Phase 4.3) — same function the
        // lifecycle triggers (resume/reconnect) and mutation closures use.
        reconcileNotificationState(
          invalidate: (provider) => ref.invalidate(provider),
          userId: userId,
        );
      } catch (_) {
        // A reconciliation failure must never propagate into the WS stream —
        // the connection and later events stay intact; polling/resume
        // reconcile whatever this event missed.
      }
    },
    onError: (Object _, StackTrace _) {
      // Malformed frames surface here (WebSocketService keeps the stream
      // open). Deliberately non-fatal — later valid events still reconcile.
    },
  );
  ref.onDispose(() => unawaited(subscription?.cancel()));
});
