import 'dart:async';

import 'package:flutter/widgets.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart' hide ConnectionState;
// Explicit alias: `ConnectionState` is ambiguous between the async enum
// (re-exported by flutter/widgets) and the WebSocket enum (re-exported by
// core.dart). The socket lifecycle contract uses the WebSocket one.
import 'package:labuda/core/websocket/websocket_service.dart' as websocket;
import 'package:labuda/domains/chat/chat/presentation/providers/chat_notifier.dart';
import 'package:labuda/domains/system/notification/presentation/providers/notification_list_provider.dart';

/// Session-scoped app-lifecycle observer.
///
/// Single owner of "what must happen when the app comes back to the
/// foreground": reconnect the realtime socket and re-read backend truth.
///
/// CHAT UNREAD RECONCILIATION (Task 4): this observer is also the single
/// lifecycle owner of the canonical chat unread state ([chatListProvider]):
///   - initial authenticated scope → initialize it once per user, so the
///     Home Chat badge has correct state without opening the Chat List;
///   - real background→foreground resume → refresh it from canonical REST,
///     because WS reconnect restores the realtime pipe but events missed
///     while backgrounded are gone.
/// Consumers (Home badge, Chat List screen) only read that one state.
///
/// MENGAPA DI ROOT, BUKAN DI SCREEN:
/// pekerjaan ini milik SESI, jadi harus berjalan di route manapun. Dulu
/// pengamatnya hidup di `MainScreen`, sehingga route yang dicapai lewat `go()`
/// (yang mengganti seluruh stack) diam-diam kehilangan hook resume — socket
/// yang mati saat background tidak pernah pulih dan sesi tidak pernah
/// di-refresh dari layar tersebut.
///
/// MENGAPA HANYA REAKTIF PADA SIKLUS BACKGROUND/FOREGROUND NYATA:
/// `inactive → resumed` BUKAN kembali dari background — itu hanya kehilangan
/// fokus window sesaat (overlay sistem, split-screen, IME, preview
/// screenshot). Dulu setiap focus-blip seperti itu ikut menjalankan refresh
/// sesi; cukup satu screenshot untuk memicu rangkaian loading → splash →
/// home. Hanya `paused`/`hidden` yang membuktikan app benar-benar meninggalkan
/// foreground.
class SessionLifecycleObserver extends ConsumerStatefulWidget {
  const SessionLifecycleObserver({super.key, required this.child});

  /// The app subtree this observer supervises.
  final Widget child;

  @override
  SessionLifecycleObserverState createState() =>
      SessionLifecycleObserverState();
}

/// State of [SessionLifecycleObserver]; public so tests can drive the
/// lifecycle seam directly.
class SessionLifecycleObserverState
    extends ConsumerState<SessionLifecycleObserver>
    with WidgetsBindingObserver {
  /// True once the app has actually left the foreground; consumed by the
  /// next `resumed`.
  bool _leftForeground = false;

  /// User whose canonical chat unread state this observer currently scopes.
  /// Empty while unauthenticated.
  String _chatSyncUserId = '';

  /// Phase 4.3: set when the socket is actually DOWN (disconnected /
  /// reconnecting emission). The next `connected` transition is then a
  /// RECONNECT (not the initial connect) and must reconcile notification
  /// state — events missed while down are gone forever. Initial
  /// connecting→connected never sets this flag, so bootstrap does not
  /// double-fetch notifications.
  bool _sawSocketDown = false;

  /// The ONE session-long subscription to the socket's connection lifecycle
  /// (the stream is create-once — Phase 4.1 — so this survives reconnects).
  StreamSubscription<websocket.ConnectionState>? _connectionStateSub;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);

    // PHASE 4.3 — RECONNECT TRIGGER for the canonical notification
    // reconciliation path. Owned here (session lifecycle owner), not by a
    // notification-specific listener: a reconnect restores the realtime
    // pipe but events missed while down are never replayed, so the
    // canonical state must be re-read the moment the socket is healthy
    // again — without waiting for a poll tick.
    try {
      _connectionStateSub =
          ref.read(webSocketServiceProvider).connectionState.listen((
            socketState,
          ) {
            switch (socketState) {
              case websocket.ConnectionState.disconnected:
              case websocket.ConnectionState.reconnecting:
                _sawSocketDown = true;
              case websocket.ConnectionState.connected:
                if (_sawSocketDown) {
                  _sawSocketDown = false;
                  _reconcileNotificationState();
                }
              case websocket.ConnectionState.connecting:
                break;
            }
          });
    } catch (_) {
      // Provider not wired (tests/host) — reconciliation stays available
      // via the resume trigger; never crash session boot.
    }
  }

  @override
  void dispose() {
    _connectionStateSub?.cancel();
    WidgetsBinding.instance.removeObserver(this);
    super.dispose();
  }

  /// THE canonical notification reconciliation trigger entry (Phase 4.3).
  /// All lifecycle conditions (resume, reconnect) funnel into
  /// [reconcileNotificationState] with the CURRENT authenticated user —
  /// never a stale identity. Fail-closed on empty/guest scopes.
  @visibleForTesting
  void reconcileNotificationStateForLifecycle() => _reconcileNotificationState();

  void _reconcileNotificationState() {
    try {
      final authState = ref.read(authControllerProvider);
      if (authState is! AuthStateAuthenticated) return;
      reconcileNotificationState(
        invalidate: (provider) => ref.invalidate(provider),
        userId: authState.user.id,
      );
    } catch (_) {
      // Provider not wired (tests/host) — resume must never crash.
    }
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) =>
      handleLifecycle(state);

  /// Same entry point the platform lifecycle callback uses, exposed so tests
  /// can drive it without synthesizing platform lifecycle messages.
  @visibleForTesting
  void handleLifecycle(AppLifecycleState state) {
    switch (state) {
      case AppLifecycleState.paused:
      case AppLifecycleState.hidden:
        _leftForeground = true;
        return;
      case AppLifecycleState.resumed:
        if (!_leftForeground) return;
        _leftForeground = false;
        _onReturnedToForeground();
        return;
      case AppLifecycleState.inactive:
      case AppLifecycleState.detached:
        return;
    }
  }

  @override
  Widget build(BuildContext context) {
    // AUTH-SCOPED CHAT UNREAD SYNC.
    //
    // The keep-alive listener is registered on EVERY build (build-scoped
    // `ref.listen` subscriptions are cleared when this widget rebuilds, e.g.
    // when the router rebuilds the builder chain). While an authenticated
    // scope holds the listener, the autoDispose chatListProvider survives
    // between the session boot load and the first consumer watch (the Home
    // badge) — without it the initialized state would be disposed before the
    // badge ever read it, collapsing the badge back to zero.
    // The per-user guard makes loadChats run exactly once per authenticated
    // scope; rebuilds of the same scope are no-ops.
    final authState = ref.watch(authControllerProvider);
    final userId = authState is AuthStateAuthenticated ? authState.user.id : '';

    if (userId.isNotEmpty) {
      try {
        // onError: in host/test environments without chat wiring the chain
        // below this provider fails (e.g. missing ApiClient). The keep-alive
        // listener must swallow that error instead of surfacing it as an
        // unhandled exception; the app's real wiring never errors here.
        ref.listen(
          chatListProvider,
          (_, _) {},
          onError: (Object _, StackTrace _) {},
        );
      } catch (_) {
        // Host/test environments without chat wiring — session boot must
        // never crash on a missing optional dependency graph.
      }
    }
    _syncChatUnreadScope(userId);

    return widget.child;
  }

  /// Initializes the canonical chat unread state for [userId], exactly once
  /// per authenticated scope. No-op when the scope has not changed.
  void _syncChatUnreadScope(String userId) {
    if (userId == _chatSyncUserId) return;
    _chatSyncUserId = userId;
    if (userId.isEmpty) return;

    // Defer OUT of the build phase: loadChats publishes provider state
    // synchronously (state.loading()), and a provider write during widget
    // build is forbidden by Riverpod's tree-consistency guard.
    Future(() {
      // The scope may have changed between scheduling and running (fast
      // logout/account switch) — never load for a stale user.
      if (_chatSyncUserId != userId) return;
      _loadChatUnread(userId);
    });
  }

  /// Canonical chat unread load/reload, hardened for host/test environments
  /// without chat wiring: both synchronous throws (provider in error state)
  /// and async failures (loadChats rethrows a failed repository read) are
  /// swallowed so session boot/resume never crashes. loadChats itself already
  /// folds repository errors into ChatListState — the catch here only guards
  /// the missing-dependency case.
  void _loadChatUnread(String userId, {bool isRefresh = false}) {
    try {
      unawaited(
        ref
            .read(chatListProvider.notifier)
            .loadChats(userId, isRefresh: isRefresh)
            .catchError((Object _) {}),
      );
    } catch (_) {
      // Provider not wired (tests/host).
    }
  }

  void _onReturnedToForeground() {
    final authState = ref.read(authControllerProvider);
    if (authState is! AuthStateAuthenticated) return;

    // REALTIME RESUME: a socket killed while backgrounded does not recover by
    // itself (suspended timers + finite reconnect budget) — the backend then
    // logs "No connections for user" and an open chat silently stops
    // updating. Reconnect first, then re-read backend truth.
    try {
      ref.read(webSocketServiceProvider).reconnectNow();
    } catch (_) {
      // Provider not wired (tests/host) — resume must never crash.
    }

    // SESSION TRUTH RESUME: forceRefreshAuthState() is THE single mid-session
    // refresh authority — in-place (never AuthState.loading, which the router
    // maps to a forced /splash), publishes only when the fresh user actually
    // differs from the cached one, and carries the mid-session
    // account-restriction gate.
    unawaited(
      ref.read(authControllerProvider.notifier).forceRefreshAuthState(),
    );

    // CHAT UNREAD RECONCILIATION: WS reconnect restores the realtime pipe,
    // but room events missed while backgrounded are gone forever. The
    // canonical chat unread state must be re-read from REST so the Home
    // badge reflects backend truth without the user opening the Chat List.
    if (_chatSyncUserId == authState.user.id) {
      _loadChatUnread(authState.user.id, isRefresh: true);
    }

    // PHASE 4.3 — RESUME TRIGGER for the canonical notification
    // reconciliation path. The business requirement is resume-independent of
    // the socket: coming back to the foreground re-reads canonical
    // notification state so the badge is correct without waiting for a poll
    // tick. When the socket is DOWN at resume, the reconnect-connected
    // trigger (initState listener) owns the reconcile instead — one
    // lifecycle coincidence produces exactly ONE reconciliation, never a
    // duplicate pair.
    var socketHealthy = false;
    try {
      socketHealthy = ref.read(webSocketServiceProvider).isConnected;
    } catch (_) {
      // Provider not wired (tests/host).
    }
    if (socketHealthy) {
      _reconcileNotificationState();
    }
  }
}
