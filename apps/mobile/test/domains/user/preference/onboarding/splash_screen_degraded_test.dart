// SplashScreen degraded UI: AuthStateBackendUnavailable /
// AuthStateBackendFailure are terminal degraded states. Recovery is
// explicit via Coba Lagi → retryBackendSync() (current Firebase
// identity → _syncWithBackend). There is no automatic timer retry.
// AuthStateBackendUnavailable always renders the terminal
// "Server Tidak Bisa Dijangkau" scaffold with Coba Lagi enabled.
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/preference/onboarding/presentation/screens/splash_screen.dart';

/// Fake controller that starts directly in the given degraded state and
/// records calls to retryBackendSync()/signOut() instead of touching
/// Firebase/network.
class _FakeDegradedAuthController extends AuthController {
  _FakeDegradedAuthController(this._initialState);

  final AuthState _initialState;

  int retryCallCount = 0;
  int signOutCallCount = 0;

  @override
  AuthState build() => _initialState;

  @override
  Future<void> retryBackendSync() async {
    retryCallCount++;
  }

  @override
  Future<void> signOut() async {
    signOutCallCount++;
  }
}

Widget _wrap(AuthController controller) {
  return ProviderScope(
    overrides: [authControllerProvider.overrideWith(() => controller)],
    child: const MaterialApp(home: SplashScreen()),
  );
}

void main() {
  group('SplashScreen — AuthStateBackendUnavailable', () {
    testWidgets('renders the dedicated unavailable UI, not a plain spinner', (
      tester,
    ) async {
      final controller = _FakeDegradedAuthController(
        const AuthState.backendUnavailable('Backend down'),
      );

      await tester.pumpWidget(_wrap(controller));
      await tester.pumpAndSettle();

      expect(find.text('Server Tidak Bisa Dijangkau'), findsOneWidget);
      expect(
        find.textContaining('Tidak bisa terhubung ke server Labuda'),
        findsOneWidget,
      );
      expect(find.text('Coba Lagi'), findsOneWidget);
      expect(find.text('Keluar'), findsOneWidget);
      // Must NOT be mislabeled as a generic connectivity/no-internet issue.
      expect(find.textContaining('No internet'), findsNothing);
      expect(find.textContaining('Tidak ada internet'), findsNothing);
    });

    testWidgets('tapping Coba Lagi calls retryBackendSync exactly once', (
      tester,
    ) async {
      final controller = _FakeDegradedAuthController(
        const AuthState.backendUnavailable('Backend down'),
      );

      await tester.pumpWidget(_wrap(controller));
      await tester.pumpAndSettle();

      expect(controller.retryCallCount, 0);
      await tester.tap(find.text('Coba Lagi'));
      await tester.pump();

      expect(controller.retryCallCount, 1);
      expect(controller.signOutCallCount, 0);
    });

    testWidgets('tapping Keluar calls signOut', (tester) async {
      final controller = _FakeDegradedAuthController(
        const AuthState.backendUnavailable('Backend down'),
      );

      await tester.pumpWidget(_wrap(controller));
      await tester.pumpAndSettle();

      await tester.tap(find.text('Keluar'));
      await tester.pump();

      expect(controller.signOutCallCount, 1);
      expect(controller.retryCallCount, 0);
    });
  });

  group('SplashScreen — AuthStateBackendFailure', () {
    testWidgets('renders the failure message and retry action', (tester) async {
      final controller = _FakeDegradedAuthController(
        const AuthState.backendFailure('Username already taken'),
      );

      await tester.pumpWidget(_wrap(controller));
      await tester.pumpAndSettle();

      expect(find.text('Gagal Memuat Data'), findsOneWidget);
      expect(find.text('Username already taken'), findsOneWidget);
      expect(find.text('Coba Lagi'), findsOneWidget);
    });

    testWidgets('tapping Coba Lagi calls retryBackendSync', (tester) async {
      final controller = _FakeDegradedAuthController(
        const AuthState.backendFailure('Username already taken'),
      );

      await tester.pumpWidget(_wrap(controller));
      await tester.pumpAndSettle();

      await tester.tap(find.text('Coba Lagi'));
      await tester.pump();

      expect(controller.retryCallCount, 1);
    });
  });

  group('SplashScreen — AuthStateBackendUnavailable still retryable', () {
    testWidgets('BackendUnavailable still renders Coba Lagi', (tester) async {
      final controller = _FakeDegradedAuthController(
        const AuthState.backendUnavailable('Backend down'),
      );

      await tester.pumpWidget(_wrap(controller));
      await tester.pumpAndSettle();

      expect(find.text('Server Tidak Bisa Dijangkau'), findsOneWidget);
      expect(find.text('Coba Lagi'), findsOneWidget);
      expect(find.text('Keluar'), findsOneWidget);
    });
  });

  group('SplashScreen — AuthStateBackendFailure still retryable', () {
    testWidgets('BackendFailure still renders Coba Lagi', (tester) async {
      final controller = _FakeDegradedAuthController(
        const AuthState.backendFailure('validation failed'),
      );

      await tester.pumpWidget(_wrap(controller));
      await tester.pumpAndSettle();

      expect(find.text('Gagal Memuat Data'), findsOneWidget);
      expect(find.text('Coba Lagi'), findsOneWidget);
    });
  });

  group(
    'SplashScreen — normal loading states unaffected (regression guard)',
    () {
      testWidgets('AuthStateInitial still shows the ordinary loading UI', (
        tester,
      ) async {
        final controller = _FakeDegradedAuthController(
          const AuthState.initial(),
        );

        await tester.pumpWidget(_wrap(controller));
        // Not pumpAndSettle(): the loading indicator's indeterminate
        // animation never settles. Advance past SplashScreen's own chained
        // Future.delayed animation-sequencing calls (logo -> text -> loading,
        // ~1.2s total) instead, so no timer is left pending when the test
        // tears down the widget tree.
        await tester.pump(const Duration(seconds: 2));

        expect(find.text('Memuat aplikasi...'), findsOneWidget);
        expect(find.text('Coba Lagi'), findsNothing);
        expect(find.text('Server Tidak Bisa Dijangkau'), findsNothing);
      });
    },
  );
}
