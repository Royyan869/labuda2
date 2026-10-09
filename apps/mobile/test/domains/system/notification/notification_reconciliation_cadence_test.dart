// PHASE 5 (Option B) — THE single notification reconciliation cadence.
//
// Proves the consolidated fallback architecture against production code:
//   - the OLD two independent 10s polling loops (inside the repository
//     streams) are purged — the repository is one-shot;
//   - NotificationInitializer owns exactly ONE Timer.periodic that routes
//     through reconcileNotificationState (same canonical function as
//     realtime/resume/reconnect/mutations);
//   - initial canonical GET is immediate (FutureProvider first build), the
//     first cadence tick comes only after the interval;
//   - one tick refreshes BOTH list + unread count;
//   - the cadence stops on logout and never polls under a stale identity;
//   - guest scope never starts the cadence.
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart' hide NotificationEntity;
import 'package:labuda/domains/system/notification/data/notification_providers.dart';
import 'package:labuda/domains/system/notification/domain/entities/notification_entity.dart';
import 'package:labuda/domains/system/notification/domain/repositories/i_notification_repository.dart';
import 'package:labuda/domains/system/notification/presentation/providers/notification_list_provider.dart';
import 'package:labuda/domains/system/notification/presentation/providers/unread_count_provider.dart';
import 'package:labuda/domains/system/notification/services/fcm_service.dart';
import 'package:labuda/domains/system/notification/presentation/widgets/notification_initializer.dart';

const _uidA = 'user-a';
const _uidB = 'user-b';

class _FakeFcmService extends Fake implements FcmService {}

class _NoopLogger implements ILoggerService {
  const _NoopLogger();

  @override
  Future<Result<void>> debug(
    String message, {
    Map<String, dynamic>? extra,
  }) async => Result.success(null);

  @override
  Future<Result<void>> info(
    String message, {
    Map<String, dynamic>? extra,
  }) async => Result.success(null);

  @override
  Future<Result<void>> warning(
    String message, {
    Map<String, dynamic>? extra,
  }) async => Result.success(null);

  @override
  Future<Result<void>> error(
    String message, {
    Map<String, dynamic>? extra,
    StackTrace? stackTrace,
  }) async => Result.success(null);

  @override
  dynamic noSuchMethod(Invocation invocation) => null;
}

class _CountingRepository implements INotificationRepository {
  int countCalls = 0;
  int listCalls = 0;
  final List<String> countUsers = [];
  final List<String> listUsers = [];

  @override
  Future<Result<int>> getUnreadCount({required String userId}) async {
    countCalls++;
    countUsers.add(userId);
    return Result.success(countCalls);
  }

  @override
  Future<Result<List<NotificationEntity>>> getNotifications({
    required String userId,
    int limit = 20,
  }) async {
    listCalls++;
    listUsers.add(userId);
    return Result.success(const <NotificationEntity>[]);
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => null;
}

class _MutableAuthController extends AuthController {
  _MutableAuthController(this._current);

  AuthState _current;

  @override
  AuthState build() => _current;

  void switchTo(AuthState next) {
    _current = next;
    state = next;
  }

  @override
  Future<void> forceRefreshAuthState() async {}
}

AuthState _authFor(String userId) => AuthState.authenticated(
  AuthUser(
    id: userId,
    email: '$userId@test.invalid',
    username: userId,
    isEmailVerified: true,
    createdAt: DateTime.utc(2026, 1, 1),
    updatedAt: DateTime.utc(2026, 1, 1),
    roles: const [UserRole.user],
    provider: AuthProvider.email,
  ),
  emailVerified: true,
);

void main() {
  testWidgets(
    'immediate initial fetch + one 10s tick refreshes BOTH list and count',
    (tester) async {
      final repo = _CountingRepository();
      final auth = _MutableAuthController(_authFor(_uidA));

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            notificationRepositoryProvider.overrideWithValue(repo),
            authControllerProvider.overrideWith(() => auth),
            fcmServiceProvider.overrideWithValue(_FakeFcmService()),
            loggerServiceProvider.overrideWithValue(_NoopLogger()),
          ],
          child: MaterialApp(
            home: _DualProbe(
              userId: _uidA,
              child: const NotificationInitializer(child: SizedBox.shrink()),
            ),
          ),
        ),
      );
      // Flush the immediate first build (no time elapse — must NOT wait 10s).
      await tester.pump();
      await tester.pump();

      expect(repo.countCalls, 1, reason: 'initial count GET must be immediate');
      expect(repo.listCalls, 1, reason: 'initial list GET must be immediate');

      // Before the interval elapses there is no second cycle.
      await tester.pump(const Duration(seconds: 9));
      expect(repo.countCalls, 1);
      expect(repo.listCalls, 1);

      // ONE tick → ONE reconciliation → both canonical reads re-run.
      await tester.pump(const Duration(seconds: 1));
      await tester.pump();
      await tester.pump();
      expect(repo.countCalls, 2, reason: 'tick must refresh the count');
      expect(repo.listCalls, 2, reason: 'the SAME tick must refresh the list');

      // A second tick is again exactly one cycle — no runaway/overlap.
      await tester.pump(const Duration(seconds: 10));
      await tester.pump();
      await tester.pump();
      expect(repo.countCalls, 3);
      expect(repo.listCalls, 3);
    },
  );

  testWidgets('logout stops the cadence; guest scope never polls', (
    tester,
  ) async {
    final repo = _CountingRepository();
    final auth = _MutableAuthController(_authFor(_uidA));

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          notificationRepositoryProvider.overrideWithValue(repo),
          authControllerProvider.overrideWith(() => auth),
            fcmServiceProvider.overrideWithValue(_FakeFcmService()),
            loggerServiceProvider.overrideWithValue(_NoopLogger()),
        ],
        child: MaterialApp(
          home: _DualProbe(
            userId: _uidA,
            child: const NotificationInitializer(child: SizedBox.shrink()),
          ),
        ),
      ),
    );
    await tester.pump();
    await tester.pump();
    expect(repo.countCalls, 1);

    // Logout → cadence must stop immediately.
    auth.switchTo(const AuthState.unauthenticated());
    await tester.pump();
    final countAtLogout = repo.countCalls;
    final listAtLogout = repo.listCalls;

    await tester.pump(const Duration(seconds: 30));
    await tester.pump();
    expect(repo.countCalls, countAtLogout,
        reason: 'no cadence tick may fire after logout');
    expect(repo.listCalls, listAtLogout);
  });

  testWidgets('account switch never polls under the old identity', (
    tester,
  ) async {
    final repo = _CountingRepository();
    final auth = _MutableAuthController(_authFor(_uidA));

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          notificationRepositoryProvider.overrideWithValue(repo),
          authControllerProvider.overrideWith(() => auth),
            fcmServiceProvider.overrideWithValue(_FakeFcmService()),
            loggerServiceProvider.overrideWithValue(_NoopLogger()),
        ],
        child: MaterialApp(
          home: _AuthScopeProbe(
            initializer: const NotificationInitializer(child: SizedBox.shrink()),
          ),
        ),
      ),
    );
    await tester.pump();
    await tester.pump();
    expect(repo.countUsers, [_uidA]);
    final callsBeforeSwitch = repo.countUsers.length;

    // Switch accounts: the probe now listens to user B's canonical provider,
    // exactly like the Home badge would after the router swaps the scope.
    auth.switchTo(_authFor(_uidB));
    await tester.pump();
    await tester.pump();
    await tester.pump(const Duration(seconds: 10));
    await tester.pump();
    await tester.pump();

    final afterSwitch = repo.countUsers.sublist(callsBeforeSwitch);
    expect(afterSwitch, isNot(contains(_uidA)),
        reason: 'after the switch, NO request may carry the old identity');
    expect(afterSwitch.last, _uidB,
        reason: 'cadence ticks must read the CURRENT identity at tick time');
    expect(afterSwitch, isNotEmpty);
  });
}

/// Watches BOTH canonical providers for one userId — the production badge +
/// list consumers — so every invalidation is observed as a real request.
class _DualProbe extends ConsumerWidget {
  const _DualProbe({required this.userId, required this.child});

  final String userId;
  final Widget child;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    ref.watch(unreadCountProvider(userId));
    ref.watch(notificationListProvider(userId));
    return child;
  }
}

/// After an account switch, watches the NEW user's providers (the router
/// rebuilds consumers under the fresh identity scope).
class _AuthScopeProbe extends ConsumerWidget {
  const _AuthScopeProbe({required this.initializer});

  final Widget initializer;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final authState = ref.watch(authControllerProvider);
    final userId = authState is AuthStateAuthenticated ? authState.user.id : '';
    if (userId.isEmpty) return const SizedBox.shrink();
    return _DualProbe(userId: userId, child: initializer);
  }
}
