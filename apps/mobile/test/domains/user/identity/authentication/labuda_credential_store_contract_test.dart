import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/common/result.dart';
import 'package:labuda/core/src/interfaces/services/i_local_storage_service.dart';

/// In-memory mock implementation of the credential store contract.
/// Tests the canonical interface contract without device dependencies.
class MockCredentialStore implements ILocalStorageService {
  final Map<String, String> _secure = {};

  @override
  Future<Result<void>> saveLabudaCredential(
    String accessToken,
    String refreshToken,
  ) async {
    if (accessToken.isEmpty || refreshToken.isEmpty) {
      return Result.error('Both access and refresh tokens are required');
    }
    _secure[StorageKeys.authToken] = accessToken;
    _secure[StorageKeys.refreshToken] = refreshToken;
    return Result.success(null);
  }

  @override
  Future<Result<String?>> readLabudaAccessToken() async {
    return Result.success(_secure[StorageKeys.authToken]);
  }

  @override
  Future<Result<String?>> readLabudaRefreshToken() async {
    return Result.success(_secure[StorageKeys.refreshToken]);
  }

  @override
  Future<Result<void>> clearLabudaCredential() async {
    _secure.remove(StorageKeys.authToken);
    _secure.remove(StorageKeys.refreshToken);
    return Result.success(null);
  }

  @override
  Future<Result<bool>> hasLabudaCredential() async {
    final access = _secure[StorageKeys.authToken];
    final refresh = _secure[StorageKeys.refreshToken];
    return Result.success(
      access != null && access.isNotEmpty && refresh != null && refresh.isNotEmpty,
    );
  }

  // Stub implementations for other ILocalStorageService methods
  @override
  Future<Result<void>> initialize() async => Result.success(null);
  @override
  Future<Result<void>> setString(String key, String value) async => Result.success(null);
  @override
  Future<Result<String?>> getString(String key) async => Result.success(null);
  @override
  Future<Result<void>> setInt(String key, int value) async => Result.success(null);
  @override
  Future<Result<int?>> getInt(String key) async => Result.success(null);
  @override
  Future<Result<void>> setBool(String key, bool value) async => Result.success(null);
  @override
  Future<Result<bool?>> getBool(String key) async => Result.success(null);
  @override
  Future<Result<void>> setDouble(String key, double value) async => Result.success(null);
  @override
  Future<Result<double?>> getDouble(String key) async => Result.success(null);
  @override
  Future<Result<void>> setStringList(String key, List<String> value) async => Result.success(null);
  @override
  Future<Result<List<String>?>> getStringList(String key) async => Result.success(null);
  @override
  Future<Result<void>> setObject(String key, Map<String, dynamic> value) async => Result.success(null);
  @override
  Future<Result<Map<String, dynamic>?>> getObject(String key) async => Result.success(null);
  @override
  Future<Result<void>> setSecureString(String key, String value) async {
    _secure[key] = value;
    return Result.success(null);
  }
  @override
  Future<Result<String?>> getSecureString(String key) async => Result.success(_secure[key]);
  @override
  Future<Result<void>> remove(String key) async => Result.success(null);
  @override
  Future<Result<void>> removeSecure(String key) async {
    _secure.remove(key);
    return Result.success(null);
  }
  @override
  Future<Result<void>> clear() async => Result.success(null);
  @override
  Future<Result<void>> clearSecure() async {
    _secure.clear();
    return Result.success(null);
  }
  @override
  Future<Result<bool>> containsKey(String key) async => Result.success(_secure.containsKey(key));
  @override
  Future<Result<Set<String>>> getKeys() async => Result.success(_secure.keys.toSet());
  @override
  Future<Result<void>> setAuthToken(String token) async => Result.success(null);
  @override
  Future<Result<String?>> getAuthToken() async => Result.success(null);
  Future<Result<void>> clearAuthToken() async => Result.success(null);
  @override
  Future<Result<void>> setRefreshToken(String token) async => Result.success(null);
  @override
  Future<Result<String?>> getRefreshToken() async => Result.success(null);
  Future<Result<void>> clearRefreshToken() async => Result.success(null);
  @override
  Future<Result<void>> setUserSession(Map<String, dynamic> session) async => Result.success(null);
  @override
  Future<Result<Map<String, dynamic>?>> getUserSession() async => Result.success(null);
  Future<Result<void>> clearUserSession() async => Result.success(null);

  // Restricted token uses isolated key
  @override
  Future<Result<void>> setRestrictedToken(String token) async {
    _secure[StorageKeys.restrictedToken] = token;
    return Result.success(null);
  }

  @override
  Future<Result<String?>> getRestrictedToken() async {
    return Result.success(_secure[StorageKeys.restrictedToken]);
  }

  @override
  Future<Result<void>> clearRestrictedToken() async {
    _secure.remove(StorageKeys.restrictedToken);
    return Result.success(null);
  }
}

void main() {
  late MockCredentialStore store;

  setUp(() {
    store = MockCredentialStore();
  });

  group('saveLabudaCredential', () {
    test('saves both tokens', () async {
      final result = await store.saveLabudaCredential('access-1', 'refresh-1');
      expect(result.isSuccess, true);

      final access = await store.readLabudaAccessToken();
      final refresh = await store.readLabudaRefreshToken();
      expect(access.data, 'access-1');
      expect(refresh.data, 'refresh-1');
    });

    test('rejects empty access token', () async {
      final result = await store.saveLabudaCredential('', 'refresh-1');
      expect(result.isError, true);
    });

    test('rejects empty refresh token', () async {
      final result = await store.saveLabudaCredential('access-1', '');
      expect(result.isError, true);
    });
  });

  group('readLabudaAccessToken', () {
    test('returns null when no credential saved', () async {
      final result = await store.readLabudaAccessToken();
      expect(result.data, isNull);
    });

    test('returns access token after save', () async {
      await store.saveLabudaCredential('my-access', 'my-refresh');
      final result = await store.readLabudaAccessToken();
      expect(result.data, 'my-access');
    });
  });

  group('readLabudaRefreshToken', () {
    test('returns null when no credential saved', () async {
      final result = await store.readLabudaRefreshToken();
      expect(result.data, isNull);
    });

    test('returns refresh token after save', () async {
      await store.saveLabudaCredential('my-access', 'my-refresh');
      final result = await store.readLabudaRefreshToken();
      expect(result.data, 'my-refresh');
    });
  });

  group('hasLabudaCredential', () {
    test('returns false when no credential saved', () async {
      final result = await store.hasLabudaCredential();
      expect(result.data, false);
    });

    test('returns true when both tokens present', () async {
      await store.saveLabudaCredential('access-1', 'refresh-1');
      final result = await store.hasLabudaCredential();
      expect(result.data, true);
    });

    test('returns false when only access token present', () async {
      await store.setSecureString(StorageKeys.authToken, 'only-access');
      final result = await store.hasLabudaCredential();
      expect(result.data, false);
    });

    test('returns false when only refresh token present', () async {
      await store.setSecureString(StorageKeys.refreshToken, 'only-refresh');
      final result = await store.hasLabudaCredential();
      expect(result.data, false);
    });
  });

  group('clearLabudaCredential', () {
    test('removes both tokens', () async {
      await store.saveLabudaCredential('access-1', 'refresh-1');
      await store.clearLabudaCredential();

      final access = await store.readLabudaAccessToken();
      final refresh = await store.readLabudaRefreshToken();
      expect(access.data, isNull);
      expect(refresh.data, isNull);

      final hasCred = await store.hasLabudaCredential();
      expect(hasCred.data, false);
    });

    test('is idempotent when no credential exists', () async {
      final result = await store.clearLabudaCredential();
      expect(result.isSuccess, true);
    });
  });

  group('separation from Firebase', () {
    test('Firebase token is not stored as Labuda access token', () async {
      // Simulate: Firebase SDK manages its own tokens separately
      // Labuda credentials are stored only through saveLabudaCredential
      await store.saveLabudaCredential('labuda-access', 'labuda-refresh');

      final access = await store.readLabudaAccessToken();
      expect(access.data, 'labuda-access');

      // Firebase token stored in its own key should not affect Labuda credential
      await store.setSecureString('firebase_token', 'firebase-id-token');
      final labudaAccess = await store.readLabudaAccessToken();
      expect(labudaAccess.data, 'labuda-access');
    });
  });

  group('restricted vs normal token isolation', () {
    test('normal credential survives restricted token write', () async {
      // Save normal Labuda credential
      await store.saveLabudaCredential('normal-access', 'normal-refresh');

      // Simulate exchange returning restricted completion token
      await store.setRestrictedToken('restricted-completion-token');

      // Normal credential must be intact
      final access = await store.readLabudaAccessToken();
      final refresh = await store.readLabudaRefreshToken();
      final restricted = await store.getRestrictedToken();
      expect(access.data, 'normal-access');
      expect(refresh.data, 'normal-refresh');
      expect(restricted.data, 'restricted-completion-token');
    });

    test('restricted token survives normal credential write', () async {
      // Simulate incomplete profile: restricted token stored
      await store.setRestrictedToken('restricted-completion-token');

      // Simulate complete profile: normal credential stored
      await store.saveLabudaCredential('normal-access', 'normal-refresh');

      // Restricted token must be intact
      final restricted = await store.getRestrictedToken();
      expect(restricted.data, 'restricted-completion-token');

      // Normal credential must also be intact
      final access = await store.readLabudaAccessToken();
      final refresh = await store.readLabudaRefreshToken();
      expect(access.data, 'normal-access');
      expect(refresh.data, 'normal-refresh');
    });

    test('clearLabudaCredential does not clear restricted token', () async {
      await store.saveLabudaCredential('normal-access', 'normal-refresh');
      await store.setRestrictedToken('restricted-completion-token');

      await store.clearLabudaCredential();

      // Normal credential cleared
      final access = await store.readLabudaAccessToken();
      final refresh = await store.readLabudaRefreshToken();
      expect(access.data, isNull);
      expect(refresh.data, isNull);

      // Restricted token must survive
      final restricted = await store.getRestrictedToken();
      expect(restricted.data, 'restricted-completion-token');
    });

    test('clearRestrictedToken does not clear normal credential', () async {
      await store.saveLabudaCredential('normal-access', 'normal-refresh');
      await store.setRestrictedToken('restricted-completion-token');

      await store.clearRestrictedToken();

      // Restricted token cleared
      final restricted = await store.getRestrictedToken();
      expect(restricted.data, isNull);

      // Normal credential must survive
      final access = await store.readLabudaAccessToken();
      final refresh = await store.readLabudaRefreshToken();
      expect(access.data, 'normal-access');
      expect(refresh.data, 'normal-refresh');
    });

    test('hasLabudaCredential ignores restricted token', () async {
      // Only restricted token saved — not a complete session
      await store.setRestrictedToken('restricted-completion-token');

      final hasCred = await store.hasLabudaCredential();
      expect(hasCred.data, false);
    });

    test('restricted and normal tokens use different keys', () async {
      // Write to restricted key
      await store.setRestrictedToken('restricted-val');

      // Write to normal access key
      await store.setSecureString(StorageKeys.authToken, 'normal-val');

      // They must not interfere
      final restricted = await store.getRestrictedToken();
      final normal = await store.readLabudaAccessToken();
      expect(restricted.data, 'restricted-val');
      expect(normal.data, 'normal-val');
    });
  });

  group('partial state', () {
    test('access only without refresh is not a complete credential', () async {
      await store.setSecureString(StorageKeys.authToken, 'only-access');
      final has = await store.hasLabudaCredential();
      expect(has.data, false);
    });

    test('refresh only without access is not a complete credential', () async {
      await store.setSecureString(StorageKeys.refreshToken, 'only-refresh');
      final has = await store.hasLabudaCredential();
      expect(has.data, false);
    });

    test('restricted token is not treated as normal access', () async {
      await store.setRestrictedToken('restricted-val');
      await store.setSecureString(StorageKeys.refreshToken, 'some-refresh');

      // readLabudaAccessToken reads from auth_token key, NOT restricted_token
      final access = await store.readLabudaAccessToken();
      expect(access.data, isNull);

      // hasLabudaCredential should be false (access key empty)
      final has = await store.hasLabudaCredential();
      expect(has.data, false);
    });
  });
}
