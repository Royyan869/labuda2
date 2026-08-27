// PASS 2B: SplashScreen must render a dedicated, actionable UI for
// AuthStateBackendUnavailable/AuthStateBackendFailure instead of the
// ordinary loading spinner, and the retry button must call the existing
// AuthController.retryBackendSync() path (not forceRefreshAuthState(),
// which is guarded to only work from Authenticated/RequiresProfileCompletion
// states and would silently no-op here).
//
// STAGE 3B: AuthStateBackendUnavailable is only rendered as the terminal
// "Server Tidak Bisa Dijangkau" screen once the automatic retry budget is
// exhausted (isBackendRetryPending == false). While a retry is still
// scheduled the splash keeps the ordinary pending/loading presentation.
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/preference/onboarding/presentation/screens/splash_screen.dart';

/// Fake controller that starts directly in the given degraded state and
/// records calls to retryBackendSync()/signOut() instead of touching
/// Firebase/network. [isBackendRetryPending] is configurable to exercise
/// the STAGE 3B retry-budget gating.
class _FakeDegradedAuthController extends AuthController {
  _FakeDegradedAuthController(
    this._initialState, {
    this.isBackendRetryPending = false,
  });

  final AuthState _initialState;

  @override
  final bool isBackendRetryPending;

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

  group('SplashScreen — backendUnavailable while auto-retry pending (STAGE 3B)', () {
    testWidgets(
      'does NOT render the terminal unavailable screen while a retry is '
      'still pending — keeps the loading presentation instead',
      (tester) async {
        final controller = _FakeDegradedAuthController(
          const AuthState.backendUnavailable('Backend down'),
          isBackendRetryPending: true,
        );

        await tester.pumpWidget(_wrap(controller));
        // Not pumpAndSettle(): the loading indicator never settles.
        await tester.pump(const Duration(seconds: 2));

        expect(find.text('Server Tidak Bisa Dijangkau'), findsNothing);
        expect(find.text('Coba Lagi'), findsNothing);
        expect(find.text('Memuat aplikasi...'), findsOneWidget);
      },
    );

    testWidgets(
      'renders the terminal unavailable screen once the retry budget is '
      'exhausted (no retry pending)',
      (tester) async {
        final controller = _FakeDegradedAuthController(
          const AuthState.backendUnavailable('Backend down'),
          isBackendRetryPending: false,
        );

        await tester.pumpWidget(_wrap(controller));
        await tester.pumpAndSettle();

        expect(find.text('Server Tidak Bisa Dijangkau'), findsOneWidget);
        expect(find.text('Coba Lagi'), findsOneWidget);
      },
    );
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
