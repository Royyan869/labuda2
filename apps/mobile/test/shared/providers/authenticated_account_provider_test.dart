import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/shared/providers/authenticated_account_provider.dart';

class _FakeAuthController extends AuthController {
  _FakeAuthController(this._state);

  AuthState _state;

  @override
  AuthState build() => _state;

  void setAuthState(AuthState state) {
    _state = state;
    this.state = state;
  }
}

AuthUser _user() {
  return AuthUser(
    id: 'user-1',
    createdAt: DateTime(2025),
    updatedAt: DateTime(2025),
    email: 'user@test.com',
    username: 'testuser',
    isEmailVerified: true,
    roles: const [UserRole.user],
    provider: AuthProvider.email,
  );
}

ProviderContainer _container(AuthController controller) {
  return ProviderContainer(
    overrides: [authControllerProvider.overrideWith(() => controller)],
  );
}

void main() {
  group('authenticatedUserProvider', () {
    test('returns AuthUser only for AuthStateAuthenticated', () {
      final user = _user();
      final container = _container(
        _FakeAuthController(AuthState.authenticated(user, emailVerified: true)),
      );

      addTearDown(container.dispose);

      expect(container.read(authenticatedUserProvider), same(user));
    });

    test('returns null for AuthStateInitial', () {
      final container = _container(_FakeAuthController(const AuthState.initial()));

      addTearDown(container.dispose);

      expect(container.read(authenticatedUserProvider), isNull);
    });

    test('returns null for AuthStateLoading', () {
      final container = _container(
        _FakeAuthController(const AuthState.loading()),
      );

      addTearDown(container.dispose);

      expect(container.read(authenticatedUserProvider), isNull);
    });

    test('returns null for AuthStateFirebaseAuthenticated', () {
      final container = _container(
        _FakeAuthController(const AuthState.firebaseAuthenticated('user-1')),
      );

      addTearDown(container.dispose);

      expect(container.read(authenticatedUserProvider), isNull);
    });

    test('returns null for AuthStateSyncingWithBackend', () {
      final container = _container(
        _FakeAuthController(const AuthState.syncingWithBackend('user-1')),
      );

      addTearDown(container.dispose);

      expect(container.read(authenticatedUserProvider), isNull);
    });

    test('returns null for AuthStateBackendFailure', () {
      final container = _container(
        _FakeAuthController(const AuthState.backendFailure('Backend down')),
      );

      addTearDown(container.dispose);

      expect(container.read(authenticatedUserProvider), isNull);
    });

    test('returns null for AuthStateBackendUnavailable', () {
      final container = _container(
        _FakeAuthController(
          const AuthState.backendUnavailable('Backend down'),
        ),
      );

      addTearDown(container.dispose);

      expect(container.read(authenticatedUserProvider), isNull);
    });

    test('returns null for AuthStateUnauthenticated', () {
      final container = _container(
        _FakeAuthController(const AuthState.unauthenticated()),
      );

      addTearDown(container.dispose);

      expect(container.read(authenticatedUserProvider), isNull);
    });

    test('returns null for AuthStateError', () {
      final container = _container(_FakeAuthController(const AuthState.error('oops')));

      addTearDown(container.dispose);

      expect(container.read(authenticatedUserProvider), isNull);
    });

    test('returns null for AuthStateRequiresProfileCompletion', () {
      final container = _container(
        _FakeAuthController(
          const AuthState.requiresProfileCompletion(
            userId: 'user-1',
            email: 'user@test.com',
          ),
        ),
      );

      addTearDown(container.dispose);

      expect(container.read(authenticatedUserProvider), isNull);
    });

    test('returns null for AuthStateAccountRestricted', () {
      final user = _user();
      final container = _container(
        _FakeAuthController(
          AuthState.accountRestricted(
            user,
            restrictionType: AccountStatus.active,
          ),
        ),
      );

      addTearDown(container.dispose);

      expect(container.read(authenticatedUserProvider), isNull);
    });
  });
}
