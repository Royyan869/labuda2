import 'dart:async';

import 'package:flutter/widgets.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';

/// Session-scoped app-lifecycle observer.
///
/// Single owner of "what must happen when the app comes back to the
/// foreground": reconnect the realtime socket and re-read backend truth.
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

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    super.dispose();
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

  void _onReturnedToForeground() {
    if (ref.read(authControllerProvider) is! AuthStateAuthenticated) return;

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
  }

  @override
  Widget build(BuildContext context) => widget.child;
}
