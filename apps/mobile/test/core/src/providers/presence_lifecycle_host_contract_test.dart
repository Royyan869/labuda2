import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/core/websocket/websocket_message.dart';
import 'package:labuda/core/websocket/websocket_service.dart' as ws;
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/shared/providers/authenticated_account_provider.dart'
    show authenticatedUserProvider;

AuthUser? _currentFakeAuthUser;

class _NoopLogger implements ILoggerService {
  @override
  Future<Result<void>> debug(String message, {Map<String, dynamic>? extra}) =>
      Future.value(Result.success(null));
  @override
  Future<Result<void>> info(String message, {Map<String, dynamic>? extra}) =>
      Future.value(Result.success(null));
  @override
  Future<Result<void>> warning(String message, {Map<String, dynamic>? extra}) =>
      Future.value(Result.success(null));
  @override
  Future<Result<void>> error(
    String message, {
    Map<String, dynamic>? extra,
    StackTrace? stackTrace,
  }) => Future.value(Result.success(null));
  @override
  Future<Result<void>> fatal(
    String message, {
    Map<String, dynamic>? extra,
    StackTrace? stackTrace,
  }) => Future.value(Result.success(null));
  @override
  Future<Result<void>> logUserAction(
    String action, {
    String? userId,
    Map<String, dynamic>? parameters,
  }) => Future.value(Result.success(null));
  @override
  Future<Result<void>> logPerformance(
    String operation, {
    required Duration duration,
    Map<String, dynamic>? metrics,
  }) => Future.value(Result.success(null));
  @override
  Future<Result<void>> logSecurityEvent(
    String event, {
    String? userId,
    String? severity,
    Map<String, dynamic>? details,
  }) => Future.value(Result.success(null));
  @override
  Future<Result<void>> logApiCall(
    String endpoint, {
    required String method,
    required int statusCode,
    required Duration duration,
    Map<String, dynamic>? requestData,
    Map<String, dynamic>? responseData,
  }) => Future.value(Result.success(null));
  @override
  Future<Result<void>> setLogLevel(LogLevel level) =>
      Future.value(Result.success(null));
  @override
  Future<Result<void>> clearLogs() => Future.value(Result.success(null));
  @override
  Future<Result<List<LogEntry>>> getLogs({
    LogLevel? minLevel,
    DateTime? startDate,
    DateTime? endDate,
    int? limit,
  }) => Future.value(Result.success(const <LogEntry>[]));
  @override
  Future<void> debugSync(String userId) async {}
  @override
  Future<void> debugSyncSuccess(String userId) async {}
  @override
  Future<void> debugSyncFailed(String userId, String? errorMessage) async {}
  @override
  Future<void> debugCallingGetCurrentUser() async {}
  @override
  Future<void> debugGetCurrentUserSuccess(
    String userId,
    bool isEmailVerified,
  ) async {}
  @override
  Future<void> debugGetCurrentUserFailed(
    String userId,
    String? errorMessage,
  ) async {}
  @override
  Future<void> debugSyncException(
    String userId,
    String errorMessage,
    String stackTrace,
  ) async {}
  @override
  Future<void> debugRouterCheck(
    String userId,
    bool isEmailVerified,
    String location,
    bool isVerificationRoute,
  ) async {}
  @override
  Future<void> log(String message, {LogLevel level = LogLevel.debug}) async {}
}

class _LifecycleRecordingWebSocketService extends ws.WebSocketService {
  _LifecycleRecordingWebSocketService()
    : super(baseUrl: 'ws://example.invalid');

  final StreamController<WebSocketMessage> _messages =
      StreamController<WebSocketMessage>.broadcast(sync: true);
  final StreamController<ws.ConnectionState> _connection =
      StreamController<ws.ConnectionState>.broadcast(sync: true);
  final List<String> events = <String>[];

  @override
  Stream<WebSocketMessage> get messages => _messages.stream;

  @override
  Stream<ws.ConnectionState> get connectionState => _connection.stream;

  void emitConnected() => _connection.add(ws.ConnectionState.connected);
  void emitDisconnected() => _connection.add(ws.ConnectionState.disconnected);

  @override
  Future<void> send(WebSocketMessage message, {bool requireAck = false}) async {
    events.add(message.type);
  }

  @override
  Future<void> disconnect() async {
    events.add('disconnect');
  }
}

AuthUser _authUser(String id) {
  return AuthUser(
    id: id,
    createdAt: DateTime.utc(2026, 6, 1),
    updatedAt: DateTime.utc(2026, 6, 1),
    email: 'presence@example.com',
    username: 'presence-user',
    isEmailVerified: true,
    roles: const [UserRole.user],
    provider: ShonaAuthProvider.email,
    accountStatus: AccountStatus.active,
  );
}

Widget _hostedApp(Widget child, _LifecycleRecordingWebSocketService wsService) {
  return ProviderScope(
    overrides: [
      authenticatedUserProvider.overrideWith((ref) => _currentFakeAuthUser),
      webSocketServiceProvider.overrideWithValue(wsService),
      loggerServiceProvider.overrideWithValue(_NoopLogger()),
    ],
    child: PresenceLifecycleHost(child: child),
  );
}

void _setAuth(ProviderContainer container, AuthUser? user) {
  _currentFakeAuthUser = user;
  container.refresh(authenticatedUserProvider);
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  group('PresenceLifecycleHost coordinator', () {
    testWidgets('foreground/background/reconnect are single-shot', (
      tester,
    ) async {
      final wsService = _LifecycleRecordingWebSocketService();

      await tester.pumpWidget(_hostedApp(const SizedBox(), wsService));

      final container = ProviderScope.containerOf(
        tester.element(find.byType(PresenceLifecycleHost)),
      );
      _setAuth(container, _authUser('123e4567-e89b-12d3-a456-426614174200'));
      wsService.emitConnected();
      await tester.pump();
      wsService.events.clear();

      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
      expect(wsService.events, isEmpty);

      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
      expect(wsService.events, isEmpty);

      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.paused);
      expect(wsService.events, [MessageType.presencePause]);

      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.paused);
      expect(wsService.events, [MessageType.presencePause]);

      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
      expect(wsService.events, [MessageType.presencePause]);

      wsService.emitDisconnected();
      expect(wsService.events, [MessageType.presencePause]);

      wsService.emitConnected();
      expect(wsService.events, [
        MessageType.presencePause,
        MessageType.presenceResume,
      ]);

      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.paused);
      wsService.events.clear();

      wsService.emitDisconnected();
      wsService.emitConnected();
      expect(
        wsService.events,
        [MessageType.presencePause],
        reason: 'background reconnect may pause, but must not resume',
      );

      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
      expect(wsService.events, [
        MessageType.presencePause,
        MessageType.presenceResume,
      ]);
    });

    testWidgets('screen navigation does not emit presence commands', (
      tester,
    ) async {
      final wsService = _LifecycleRecordingWebSocketService();

      await tester.pumpWidget(
        _hostedApp(
          MaterialApp(
            home: Builder(
              builder: (context) {
                return Scaffold(
                  body: Center(
                    child: ElevatedButton(
                      onPressed: () {
                        Navigator.of(context).push(
                          MaterialPageRoute<void>(
                            builder: (_) => const Scaffold(
                              body: Center(child: Text('next')),
                            ),
                          ),
                        );
                      },
                      child: const Text('navigate'),
                    ),
                  ),
                );
              },
            ),
          ),
          wsService,
        ),
      );

      final container = ProviderScope.containerOf(
        tester.element(find.byType(PresenceLifecycleHost)),
      );
      _setAuth(container, _authUser('123e4567-e89b-12d3-a456-426614174201'));
      wsService.emitConnected();
      await tester.pump();
      wsService.events.clear();

      await tester.tap(find.text('navigate'));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 16));

      expect(wsService.events, isEmpty);
    });

    testWidgets('app rebuild does not register another lifecycle coordinator', (
      tester,
    ) async {
      final wsService = _LifecycleRecordingWebSocketService();

      await tester.pumpWidget(_hostedApp(const SizedBox(), wsService));

      final container = ProviderScope.containerOf(
        tester.element(find.byType(PresenceLifecycleHost)),
      );
      _setAuth(container, _authUser('123e4567-e89b-12d3-a456-426614174202'));
      wsService.emitConnected();
      await tester.pump();
      wsService.events.clear();

      await tester.pumpWidget(
        _hostedApp(const SizedBox(key: ValueKey('rebuild')), wsService),
      );
      wsService.emitConnected();
      await tester.pump();
      wsService.events.clear();

      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.paused);
      expect(wsService.events, [MessageType.presencePause]);
    });
  });
}
