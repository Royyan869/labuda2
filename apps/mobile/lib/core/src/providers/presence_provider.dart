import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/providers/core_providers.dart';
import 'package:labuda/core/src/presence/presence.dart';
import 'package:labuda/core/src/presence/presence_api_datasource.dart';
import 'package:labuda/core/websocket/websocket_message.dart';

/// Canonical mobile Presence state.
/// One authority: Map from userId to Presence with version ordering.
class PresenceNotifier extends Notifier<Map<String, Presence>> {
  PresenceApiDatasource? _datasource;
  StreamSubscription? _wsSub;

  @override
  Map<String, Presence> build() {
    final apiClient = ref.watch(apiClientProvider);
    final logger = ref.watch(loggerServiceProvider);
    final ws = ref.watch(webSocketServiceProvider);

    _datasource = PresenceApiDatasource(apiClient, logger: logger);

    _wsSub = ws.messages.listen(
      _handleWsMessage,
      onError: (_) {},
    );
    ref.onDispose(() {
      _wsSub?.cancel();
    });
    return {};
  }

  /// Fetch initial snapshot for target userIds via canonical REST.
  /// Version-aware: only replaces if incoming version > current.
  Future<void> fetchInitial(List<String> userIds) async {
    if (userIds.isEmpty) return;
    final ds = _datasource;
    if (ds == null) return;
    final result = await ds.getPresences(userIds);
    result.fold(
      (err) => null,
      (list) {
        final next = Map<String, Presence>.from(state);
        for (final p in list) {
          final cur = next[p.userId];
          if (cur == null || p.version > cur.version) {
            next[p.userId] = p;
          }
        }
        state = next;
      },
    );
  }

  /// Fetch single user (convenience).
  Future<void> fetchSingle(String userId) => fetchInitial([userId]);

  void _handleWsMessage(WebSocketMessage msg) {
    try {
      if (msg.type != 'presence.changed') return;
      final data = msg.data;
      // Canonical data: user_id, is_online, last_seen_at, version
      // Reject legacy raw event that used `state` key
      if (data.containsKey('state')) return;
      if (!data.containsKey('user_id') ||
          !data.containsKey('is_online') ||
          !data.containsKey('version')) {
        return;
      }
      final presence = Presence.fromJson({
        'user_id': data['user_id'],
        'is_online': data['is_online'],
        'last_seen_at': data['last_seen_at'],
        'version': data['version'],
      });
      final cur = state[presence.userId];
      if (cur != null && presence.version <= cur.version) return; // older or duplicate
      state = {...state, presence.userId: presence};
    } catch (_) {
      // malformed event → ignore, do not mutate to false
    }
  }

  /// Direct apply for tests (bypasses WS).
  void applyPresence(Presence presence) {
    final cur = state[presence.userId];
    if (cur != null && presence.version <= cur.version) return;
    state = {...state, presence.userId: presence};
  }

  void clear() {
    state = {};
  }
}

/// Canonical presence map provider – single state authority.
final presenceProvider =
    NotifierProvider<PresenceNotifier, Map<String, Presence>>(
        PresenceNotifier.new);

/// Family to watch single user's presence (null if not loaded).
final userPresenceProvider = Provider.family<Presence?, String>((ref, userId) {
  final map = ref.watch(presenceProvider);
  return map[userId];
});

/// Family to watch isOnline for a user (false if not loaded).
final isUserOnlineProvider = Provider.family<bool, String>((ref, userId) {
  final p = ref.watch(userPresenceProvider(userId));
  return p?.isOnline ?? false;
});

/// Family to watch lastSeen for a user.
final userLastSeenProvider = Provider.family<DateTime?, String>((ref, userId) {
  final p = ref.watch(userPresenceProvider(userId));
  return p?.lastSeenAt;
});

/// Legacy alias for migration: old `userOnlineStatusProvider` now delegates to canonical.
/// Will be removed after consumers migrate.
final userOnlineStatusProvider = StreamProvider.family<bool, String>((ref, userId) {
  final presence = ref.watch(userPresenceProvider(userId));
  return Stream.value(presence?.isOnline ?? false);
});

/// Helper to trigger initial fetch for a set of users.
/// Usage: ref.read(presenceProvider.notifier).fetchInitial(['id1','id2'])
