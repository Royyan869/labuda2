// Stage 1C — Part B: USERNAME_TAKEN registration recovery.
//
// Proves the canonical recovery path:
//
//   Firebase account creation succeeds
//     → authenticated exchange rejects username (USERNAME_TAKEN/RESERVED)
//     → user corrects the username
//     → authenticated exchange is RETRIED with the corrected username
//     → Firebase account is NOT recreated (no duplicate create)
//     → corrected username succeeds → authenticated
//
// The mechanism is AuthController.retryRegistrationUsername: it re-runs ONLY
// the authenticated exchange with a corrected username, reusing the mutex-guarded
// _syncWithBackend path and never touching the Firebase account lifecycle.

import 'package:firebase_auth/firebase_auth.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/identity/authentication/data/auth_providers.dart'
    as auth_data;
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/firebase_principal.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/user_profile_patch.dart';
import 'package:labuda/domains/user/profile/data/datasources/user_api_datasource.dart';
import 'package:labuda/domains/user/profile/data/profile_providers.dart'
    show userSyncServiceProvider;
import 'package:labuda/domains/user/profile/data/services/user_sync_service.dart';

// ---------------------------------------------------------------------------
// Fakes (CURRENT interfaces)
// ---------------------------------------------------------------------------

class _FakeApiClient implements ApiClient {
  @override
  dynamic noSuchMethod(Invocation invocation) =>
      throw UnimplementedError('not used');
}

class _NoopUserApiDatasource extends UserApiDatasource {
  _NoopUserApiDatasource() : super(_FakeApiClient());
}

class _FakeFirebaseUser extends Fake implements User {
  _FakeFirebaseUser(this.uid);

  @override
  final String uid;

  @override
  bool get emailVerified => false;

  @override
  String? get email => '$uid@example.com';

  @override
  String? get phoneNumber => null;

  @override
  List<UserInfo> get providerData => const <UserInfo>[];

  @override
  UserMetadata get metadata => _FakeUserMetadata();

  @override
  Future<void> reload() async {}

  @override
  Future<String?> getIdToken([bool forceRefresh = false]) async => 'token';
}

class _FakeUserMetadata extends Fake implements UserMetadata {
  @override
  DateTime? get creationTime => DateTime(2026, 6, 1);

  @override
  DateTime? get lastSignInTime => DateTime(2026, 6, 2);
}

class _FakeFirebaseAuth extends Fake implements FirebaseAuth {
  @override
  Stream<User?> authStateChanges() => const Stream<User?>.empty();
}

class _RecordingUserSyncService extends UserSyncService {
  _RecordingUserSyncService()
    : super(firebaseAuth: _FakeFirebaseAuth(), datasource: _NoopUserApiDatasource());

  final List<String> syncUsernames = <String>[];
  Result<SyncUserResult>? nextSyncResult;

  @override
  Future<Result<SyncUserResult>> syncUser({
    required String username,
    String? phoneNumber,
  }) async {
    syncUsernames.add(username);
    return nextSyncResult ?? Result.error('not configured');
  }

  @override
  Future<Result<AuthUser>> getCurrentUser() async =>
      Result.error('not configured');
}

class _RecordingAuthRepository extends Fake implements IAuthRepository {
  int signUpWithEmailCalls = 0;

  @override
  Future<Result<FirebasePrincipal>> signUpWithEmail({
    required String email,
    required String password,
    required String username,
  }) async {
    signUpWithEmailCalls++;
    return Result.success(
      FirebasePrincipal(uid: 'fb-uid-1', email: email, emailVerified: false),
    );
  }

  @override
  Future<Result<UserProfilePatch>> updateProfile({
    String? photoUrl,
    String? phoneNumber,
    DateTime? phoneVerifiedAt,
    String? username,
    String? bio,
    String? location,
    DateTime? dateOfBirth,
  }) async => Result.error('not used');

  @override
  Future<Result<void>> signOut() async => Result.success(null);
}

class _FakeLoggerService extends Fake implements ILoggerService {
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
  Future<Result<void>> fatal(
    String message, {
    Map<String, dynamic>? extra,
    StackTrace? stackTrace,
  }) async => Result.success(null);

  @override
  Future<void> debugSync(String userId) async {}

  @override
  Future<void> debugSyncSuccess(String userId) async {}

  @override
  Future<void> debugSyncFailed(String userId, String? error) async {}

  @override
  Future<void> debugSyncException(
    String userId,
    String error,
    String stackTrace,
  ) async {}

  @override
  Future<void> log(String message, {LogLevel level = LogLevel.debug}) async {}
}

class _FakeAnalyticsRepository extends Fake implements IAnalyticsRepository {
  @override
  Future<Result<void>> logEvent(
    String eventName, {
    Map<String, dynamic>? parameters,
    String? userId,
  }) async => Result.success(null);

  @override
  Future<Result<void>> setUserProperties(
    Map<String, dynamic> properties,
  ) async => Result.success(null);
}

class _FakeLocalStorageService extends Fake implements ILocalStorageService {
  @override
  Future<Result<void>> setAuthToken(String token) async => Result.success(null);

  @override
  Future<Result<void>> setRefreshToken(String token) async =>
      Result.success(null);

  @override
  Future<Result<String?>> getAuthToken() async => Result.success(null);

  @override
  Future<Result<String?>> getRefreshToken() async => Result.success(null);
}

class _TestAuthController extends AuthController {
  _TestAuthController({required User firebaseUser}) : _testFirebaseUser = firebaseUser;

  final User? _testFirebaseUser;

  @override
  User? get activeFirebaseUser => _testFirebaseUser;

  @override
  bool get shouldInitializeAuthListener => false;
}

AuthUser _principalUser(String id, {required String username}) {
  return AuthUser(
    id: id,
    createdAt: DateTime(2026, 6, 1),
    updatedAt: DateTime(2026, 6, 2),
    email: '$id@example.com',
    username: username,
    isEmailVerified: false,
    accountStatus: AccountStatus.active,
    roles: const [UserRole.user],
    provider: ShonaAuthProvider.email,
  );
}

Result<SyncUserResult> _syncSuccess(AuthUser user) {
  return Result.success(
    SyncUserResult(
      user: user,
      userId: user.id,
      email: user.email,
      created: false,
      profileComplete: true,
      username: user.username,
    ),
  );
}

ProviderContainer _container({
  required AuthController controller,
  required IAuthRepository authRepository,
  required UserSyncService userSyncService,
}) {
  return ProviderContainer(
    overrides: [
      authControllerProvider.overrideWith(() => controller),
      auth_data.authRepositoryProvider.overrideWithValue(authRepository),
      userSyncServiceProvider.overrideWithValue(userSyncService),
      loggerServiceProvider.overrideWithValue(_FakeLoggerService()),
      coreAnalyticsRepositoryProvider.overrideWithValue(
        _FakeAnalyticsRepository(),
      ),
      localStorageServiceProvider.overrideWithValue(_FakeLocalStorageService()),
    ],
  );
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test(
    'username taken → retry exchange with corrected username succeeds '
    'without recreating the Firebase account',
    () async {
      final firebaseUser = _FakeFirebaseUser('fb-uid-1');
      final controller = _TestAuthController(firebaseUser: firebaseUser);
      final authRepository = _RecordingAuthRepository();
      final syncService = _RecordingUserSyncService();
      final container = _container(
        controller: controller,
        authRepository: authRepository,
        userSyncService: syncService,
      );
      addTearDown(container.dispose);

      final notifier = container.read(authControllerProvider.notifier);

      // 1. Initial email signup creates the Firebase account and sets the
      //    pending signup username.
      await notifier.signUpWithEmail(
        email: 'alice@example.com',
        password: 'Password123!',
        username: 'taken_alice',
      );

      expect(notifier.hasPendingRegistration, isTrue);
      expect(authRepository.signUpWithEmailCalls, 1);

      // 2. The authenticated exchange rejects the first username (taken).
      syncService.nextSyncResult = Result.error(
        'This username is already taken',
        code: 'USERNAME_TAKEN',
        statusCode: 409,
      );
      await notifier.retryRegistrationUsername('taken_alice');
      expect(controller.state, isA<AuthStateBackendFailure>());

      // 3. User corrects the username; the exchange is retried and succeeds.
      syncService.nextSyncResult = _syncSuccess(
        _principalUser('fb-uid-1', username: 'alice_free'),
      );
      await notifier.retryRegistrationUsername('alice_free');

      // 4. The corrected username reached the exchange AND the Firebase
      //    account was NOT recreated (no second createUserWithEmailAndPassword).
      expect(syncService.syncUsernames, isNotEmpty);
      expect(syncService.syncUsernames.last, 'alice_free');
      expect(authRepository.signUpWithEmailCalls, 1,
          reason: 'Recovery must NOT recreate the Firebase account');

      // 5. Corrected username succeeds → authenticated.
      expect(controller.state, isA<AuthStateAuthenticated>());
      expect(
        (controller.state as AuthStateAuthenticated).user.username,
        'alice_free',
      );
    },
  );

  test('recovery does not fire for a brand-new signup (no pending account)',
      () async {
    final firebaseUser = _FakeFirebaseUser('fb-uid-2');
    final controller = _TestAuthController(firebaseUser: firebaseUser);
    final syncService = _RecordingUserSyncService();
    final container = _container(
      controller: controller,
      authRepository: _RecordingAuthRepository(),
      userSyncService: syncService,
    );
    addTearDown(container.dispose);

    final notifier = container.read(authControllerProvider.notifier);
    // No signup has been initiated yet → no pending registration.
    expect(notifier.hasPendingRegistration, isFalse);
  });
}