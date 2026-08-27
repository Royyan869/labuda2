import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/common/result.dart';
import 'package:labuda/domains/user/profile/data/datasources/user_api_datasource.dart';
import 'package:labuda/domains/user/profile/data/models/api/user_api_models.dart';
import 'package:labuda/domains/user/profile/data/services/avatar_cache_service.dart';

/// Minimal fake [UserApiDatasource] that delegates [getUserById] to a
/// controllable callback and tracks call count.
class _FakeUserApiDatasource extends Fake implements UserApiDatasource {
  int getUserByIdCallCount = 0;
  Future<Result<UserApiResponse>> Function(String userId)? onGetUserById;

  @override
  Future<Result<UserApiResponse>> getUserById(String userId) async {
    getUserByIdCallCount++;
    if (onGetUserById != null) {
      return onGetUserById!(userId);
    }
    return Result.error('not configured');
  }
}

/// Helper: build a [UserApiResponse] with a controlled profile avatar URL.
UserApiResponse _makeResponse({required String id, String? avatarUrl}) {
  return UserApiResponse(
    id: id,
    email: '$id@test.com',
    accountStatus: 'active',
    roles: const ['user'],
    createdAt: DateTime(2025),
    updatedAt: DateTime(2025),
    profile: UserProfileApiResponse(
      id: id,
      username: 'test',
      avatarUrl: avatarUrl,
      preferredLang: 'id',
    ),
  );
}

AvatarCacheService _makeService(_FakeUserApiDatasource ds) {
  return AvatarCacheService(datasource: ds);
}

void main() {
  // Static cache maps persist across test cases — clear before every test.
  setUp(() {
    // Create a throw-away service to access the clear method on the static
    // maps shared by all AvatarCacheService instances.
    _makeService(_FakeUserApiDatasource()).clearAllCache();
  });

  group('AvatarCacheService cache hit', () {
    test('first lookup fetches from API', () async {
      final ds = _FakeUserApiDatasource();
      ds.onGetUserById = (_) async => Result.success(
        _makeResponse(id: 'u1', avatarUrl: 'https://img.example/u1.png'),
      );
      final svc = _makeService(ds);

      final url = await svc.getUserAvatarUrl('u1');

      expect(url, 'https://img.example/u1.png');
      expect(ds.getUserByIdCallCount, 1);
    });

    test('second lookup within TTL uses cached value, no API call', () async {
      final ds = _FakeUserApiDatasource();
      ds.onGetUserById = (_) async => Result.success(
        _makeResponse(id: 'u2', avatarUrl: 'https://img.example/u2.png'),
      );
      final svc = _makeService(ds);

      // First call — API hit
      await svc.getUserAvatarUrl('u2');
      expect(ds.getUserByIdCallCount, 1);

      // Second call — cached
      final url2 = await svc.getUserAvatarUrl('u2');
      expect(url2, 'https://img.example/u2.png');
      expect(ds.getUserByIdCallCount, 1); // still 1
    });
  });

  group('AvatarCacheService clearUserCache', () {
    test('clears only targeted user; other user remains cached', () async {
      final ds = _FakeUserApiDatasource();
      ds.onGetUserById = (id) async {
        if (id == 'a') {
          return Result.success(
            _makeResponse(id: 'a', avatarUrl: 'https://img.example/a.png'),
          );
        }
        if (id == 'b') {
          return Result.success(
            _makeResponse(id: 'b', avatarUrl: 'https://img.example/b.png'),
          );
        }
        return Result.error('unknown');
      };
      final svc = _makeService(ds);

      // Cache both
      await svc.getUserAvatarUrl('a');
      await svc.getUserAvatarUrl('b');
      expect(ds.getUserByIdCallCount, 2);

      // Clear user A only
      svc.clearUserCache('a');

      // A fetches again
      final urlA = await svc.getUserAvatarUrl('a');
      expect(urlA, 'https://img.example/a.png');
      expect(ds.getUserByIdCallCount, 3); // +1 for re-fetch

      // B still cached
      final urlB = await svc.getUserAvatarUrl('b');
      expect(urlB, 'https://img.example/b.png');
      expect(ds.getUserByIdCallCount, 3); // no additional call
    });

    test('clearing unknown user is a safe no-op', () {
      final ds = _FakeUserApiDatasource();
      final svc = _makeService(ds);

      // Must not throw
      svc.clearUserCache('nonexistent');
    });
  });

  group('AvatarCacheService clearAllCache', () {
    test('clears all cached entries; both users re-fetch', () async {
      final ds = _FakeUserApiDatasource();
      ds.onGetUserById = (id) async {
        return Result.success(
          _makeResponse(id: id, avatarUrl: 'https://img.example/$id.png'),
        );
      };
      final svc = _makeService(ds);

      // Cache two users
      await svc.getUserAvatarUrl('c');
      await svc.getUserAvatarUrl('d');
      expect(ds.getUserByIdCallCount, 2);

      // Clear all
      svc.clearAllCache();

      // Both re-fetch
      await svc.getUserAvatarUrl('c');
      await svc.getUserAvatarUrl('d');
      expect(ds.getUserByIdCallCount, 4);
    });
  });

  group('AvatarCacheService null avatar behavior', () {
    test(
      'successful null avatar response IS cached (avoids repeated API)',
      () async {
        final ds = _FakeUserApiDatasource();
        ds.onGetUserById = (_) async =>
            Result.success(_makeResponse(id: 'n1', avatarUrl: null));
        final svc = _makeService(ds);

        // First lookup — null cached
        final url1 = await svc.getUserAvatarUrl('n1');
        expect(url1, isNull);
        expect(ds.getUserByIdCallCount, 1);

        // Second lookup — cache hit, no API call
        final url2 = await svc.getUserAvatarUrl('n1');
        expect(url2, isNull);
        expect(ds.getUserByIdCallCount, 1);
      },
    );

    test('null avatar in cache IS invalidated by clearUserCache', () async {
      final ds = _FakeUserApiDatasource();
      var call = 0;
      ds.onGetUserById = (_) async {
        call++;
        return Result.success(
          _makeResponse(
            id: 'n2',
            avatarUrl: call == 1 ? null : 'https://img.example/new.png',
          ),
        );
      };
      final svc = _makeService(ds);

      // First: API returns null → cached
      final url1 = await svc.getUserAvatarUrl('n2');
      expect(url1, isNull);

      // Clear and re-fetch → API now returns non-null
      svc.clearUserCache('n2');
      final url2 = await svc.getUserAvatarUrl('n2');
      expect(url2, 'https://img.example/new.png');
    });
  });

  group('AvatarCacheService failed request', () {
    test('API failures are NOT cached; next lookup retries', () async {
      final ds = _FakeUserApiDatasource();
      var call = 0;
      ds.onGetUserById = (_) async {
        call++;
        if (call == 1) {
          return Result.error('network error');
        }
        return Result.success(
          _makeResponse(id: 'f1', avatarUrl: 'https://img.example/f1.png'),
        );
      };
      final svc = _makeService(ds);

      // First call — fails
      final url1 = await svc.getUserAvatarUrl('f1');
      expect(url1, isNull);
      expect(ds.getUserByIdCallCount, 1);

      // Second call — succeeds (failure was not cached)
      final url2 = await svc.getUserAvatarUrl('f1');
      expect(url2, 'https://img.example/f1.png');
      expect(ds.getUserByIdCallCount, 2);
    });

    test('exception during fetch is NOT cached; next lookup retries', () async {
      final ds = _FakeUserApiDatasource();
      var call = 0;
      ds.onGetUserById = (_) async {
        call++;
        if (call == 1) {
          throw Exception('timeout');
        }
        return Result.success(
          _makeResponse(id: 'f2', avatarUrl: 'https://img.example/f2.png'),
        );
      };
      final svc = _makeService(ds);

      // First call — exception
      final url1 = await svc.getUserAvatarUrl('f2');
      expect(url1, isNull);
      expect(ds.getUserByIdCallCount, 1);

      // Second call — succeeds
      final url2 = await svc.getUserAvatarUrl('f2');
      expect(url2, 'https://img.example/f2.png');
      expect(ds.getUserByIdCallCount, 2);
    });
  });
}
