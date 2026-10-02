// SessionLifecycleObserver — single owner of foreground/resume session work.
//
// Regression lock for the "screenshot → loading → splash → home" bug: a
// transient window-focus loss (`inactive` → `resumed`, e.g. the system
// screenshot preview or an overlay taking focus) is NOT a backgrounding and
// must not re-hydrate the session. Only a real background/foreground cycle
// (`paused`/`hidden` → `resumed`) may reconnect the socket and refresh.

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';

class _RecordingAuthController extends AuthController {
  int refreshCalls = 0;

  @override
  AuthState build() => AuthState.authenticated(
    AuthUser(
      id: 'user-1',
      email: 'test@example.com',
      username: 'testuser',
      isEmailVerified: true,
      createdAt: DateTime.utc(2026, 1, 1),
      updatedAt: DateTime.utc(2026, 1, 1),
      roles: const [UserRole.user],
      provider: AuthProvider.email,
    ),
    emailVerified: true,
  );

  @override
  Future<void> forceRefreshAuthState() async => refreshCalls++;
}

class _GuestAuthController extends AuthController {
  int refreshCalls = 0;

  @override
  AuthState build() => const AuthState.unauthenticated();

  @override
  Future<void> forceRefreshAuthState() async => refreshCalls++;
}

class _RecordingWebSocketService extends WebSocketService {
  _RecordingWebSocketService() : super(baseUrl: 'ws://example.invalid');

  int reconnectCalls = 0;

  @override
  void reconnectNow() => reconnectCalls++;
}

Future<SessionLifecycleObserverState> _pumpHarness(
  WidgetTester tester, {
  required AuthController controller,
  required _RecordingWebSocketService socket,
}) async {
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        authControllerProvider.overrideWith(() => controller),
        webSocketServiceProvider.overrideWithValue(socket),
      ],
      child: const MaterialApp(
        home: SessionLifecycleObserver(
          child: Scaffold(body: Text('body')),
        ),
      ),
    ),
  );
  await tester.pumpAndSettle();
  return tester.state<SessionLifecycleObserverState>(
    find.byType(SessionLifecycleObserver),
  );
}

void main() {
  testWidgets(
    'focus blip (inactive → resumed) does NOT reconnect or refresh the session',
    (tester) async {
      final controller = _RecordingAuthController();
      final socket = _RecordingWebSocketService();
      final state = await _pumpHarness(tester, controller: controller, socket: socket);

      state.handleLifecycle(AppLifecycleState.inactive);
      state.handleLifecycle(AppLifecycleState.resumed);

      expect(socket.reconnectCalls, 0);
      expect(controller.refreshCalls, 0);
    },
  );

  testWidgets(
    'real background → foreground reconnects the socket and refreshes once',
    (tester) async {
      final controller = _RecordingAuthController();
      final socket = _RecordingWebSocketService();
      final state = await _pumpHarness(tester, controller: controller, socket: socket);

      state.handleLifecycle(AppLifecycleState.paused);
      state.handleLifecycle(AppLifecycleState.resumed);

      expect(socket.reconnectCalls, 1);
      expect(controller.refreshCalls, 1);

      // A second resume without another backgrounding is a focus blip again.
      state.handleLifecycle(AppLifecycleState.inactive);
      state.handleLifecycle(AppLifecycleState.resumed);
      expect(socket.reconnectCalls, 1);
      expect(controller.refreshCalls, 1);
    },
  );

  testWidgets('hidden (Android onStop-equivalent) also counts as backgrounding', (
    tester,
  ) async {
    final controller = _RecordingAuthController();
    final socket = _RecordingWebSocketService();
    final state = await _pumpHarness(tester, controller: controller, socket: socket);

    state.handleLifecycle(AppLifecycleState.hidden);
    state.handleLifecycle(AppLifecycleState.resumed);

    expect(socket.reconnectCalls, 1);
    expect(controller.refreshCalls, 1);
  });

  testWidgets('a guest session is never re-hydrated on resume', (tester) async {
    final controller = _GuestAuthController();
    final socket = _RecordingWebSocketService();
    final state = await _pumpHarness(tester, controller: controller, socket: socket);

    state.handleLifecycle(AppLifecycleState.paused);
    state.handleLifecycle(AppLifecycleState.resumed);

    expect(socket.reconnectCalls, 0);
    expect(controller.refreshCalls, 0);
  });
}
