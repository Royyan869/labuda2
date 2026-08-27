// Stage 1B — UserSyncService forwards the registration username to the
// authenticated exchange call.
//
// Proves the threading seam between the AuthController (which carries the
// signup username in _pendingSignupUsername) and the API datasource call:
//
//   syncUser(username: 'alice_reg')
//     → datasource.exchangeFirebaseSession(firebaseIdToken, username: 'alice_reg')
//     → POST /auth/firebase/exchange {firebase_id_token, username}
//
// This is the exact seam that previously dropped the username (the datasource
// was called without it), causing the backend to return requires_profile_completion
// so the user had to re-enter the username in Complete Profile.

import 'package:firebase_auth/firebase_auth.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/api/api_client.dart';
import 'package:labuda/core/common/result.dart';
import 'package:labuda/core/core.dart' show ILocalStorageService;
import 'package:labuda/domains/user/profile/data/datasources/user_api_datasource.dart';
import 'package:labuda/domains/user/profile/data/models/api/user_api_models.dart';
import 'package:labuda/domains/user/profile/data/services/user_sync_service.dart';

class _FakeApiClient implements ApiClient {
  @override
  dynamic noSuchMethod(Invocation invocation) =>
      throw UnimplementedError('not used');
}

/// Fake datasource that records the `username` forwarded to the exchange call
/// and stubs the exchange + getCurrentUser responses for the complete path.
class _RecordingUserApiDatasource extends UserApiDatasource {
  final List<String?> exchangeUsernames = <String?>[];
  final List<String> exchangeTokens = <String>[];

  _RecordingUserApiDatasource() : super(_FakeApiClient());

  @override
  Future<Result<FirebaseExchangeResponse>> exchangeFirebaseSession({
    required String firebaseIdToken,
    String? username,
  }) async {
    exchangeTokens.add(firebaseIdToken);
    exchangeUsernames.add(username);
    return Result.success(
      const FirebaseExchangeResponse(
        userId: 'user-1',
        accessToken: 'access-token',
        refreshToken: 'refresh-token',
        expiresAt: '2026-07-19T00:00:00Z',
        refreshExpiresAt: '2026-07-20T00:00:00Z',
        requiresProfileCompletion: false,
        created: false,
      ),
    );
  }

  @override
  Future<Result<UserApiResponse>> getCurrentUser() async {
    return Result.success(
      UserApiResponse(
        id: 'user-1',
        email: 'alice@test.com',
        username: 'alice_reg',
        accountStatus: 'active',
        roles: const ['user'],
        createdAt: DateTime(2026, 6, 1),
        updatedAt: DateTime(2026, 6, 2),
        profile: UserProfileApiResponse(
          id: 'user-1',
          username: 'alice_reg',
          preferredLang: 'id',
        ),
      ),
    );
  }
}

class _FakeFirebaseUser extends Fake implements User {
  @override
  String get uid => 'fb-uid-1';

  @override
  Future<String?> getIdToken([bool forceRefresh = false]) async => 'firebase-token';
}

class _FakeFirebaseAuth extends Fake implements FirebaseAuth {
  final _FakeFirebaseUser user;
  _FakeFirebaseAuth(this.user);

  @override
  User? get currentUser => user;
}

class _RecordingLocalStorage extends Fake implements ILocalStorageService {
  int setAuthTokenCalls = 0;
  int setRefreshTokenCalls = 0;

  @override
  Future<Result<void>> setAuthToken(String token) async {
    setAuthTokenCalls++;
    return Result.success(null);
  }

  @override
  Future<Result<void>> setRefreshToken(String token) async {
    setRefreshTokenCalls++;
    return Result.success(null);
  }
}

void main() {
  test('syncUser forwards the registration username to the exchange datasource',
      () async {
    final datasource = _RecordingUserApiDatasource();

    final service = UserSyncService(
      firebaseAuth: _FakeFirebaseAuth(_FakeFirebaseUser()),
      datasource: datasource,
      localStorage: _RecordingLocalStorage(),
    );

    final result = await service.syncUser(username: 'alice_reg');

    expect(result.isSuccess, isTrue);
    // The username must reach the exchange call (before this fix it was dropped).
    expect(datasource.exchangeUsernames, hasLength(1));
    expect(datasource.exchangeUsernames.single, equals('alice_reg'));
    expect(datasource.exchangeTokens.single, equals('firebase-token'));

    // Complete path: profileComplete true and username preserved in result.
    expect(result.data?.profileComplete, isTrue);
    expect(result.data?.username, equals('alice_reg'));
  });

  test('syncUser still threads an empty username for login/Google first-sync',
      () async {
    final datasource = _RecordingUserApiDatasource();
    final service = UserSyncService(
      firebaseAuth: _FakeFirebaseAuth(_FakeFirebaseUser()),
      datasource: datasource,
      localStorage: _RecordingLocalStorage(),
    );

    final result = await service.syncUser(username: '');

    expect(result.isSuccess, isTrue);
    expect(datasource.exchangeUsernames.single, isEmpty);
  });
}