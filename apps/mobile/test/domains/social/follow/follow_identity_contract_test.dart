import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/api/api_client.dart';
import 'package:labuda/domains/social/follow/data/datasources/follow_api_datasource.dart';
import 'package:labuda/domains/social/follow/data/dto/follow_api_models.dart';

class _FakeApiClient extends ApiClient {
  _FakeApiClient(this.respond) : super(baseUrl: 'https://example.com');

  final Response<dynamic> Function(
    String path,
    Map<String, dynamic>? queryParameters,
  ) respond;

  String? lastPath;
  Map<String, dynamic>? lastQueryParameters;

  @override
  Future<Response<T>> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async {
    lastPath = path;
    lastQueryParameters = queryParameters;
    return respond(path, queryParameters) as Response<T>;
  }
}

Response<dynamic> _response(
  String path,
  Object? data, {
  int statusCode = 200,
}) {
  return Response<dynamic>(
    requestOptions: RequestOptions(path: path),
    statusCode: statusCode,
    data: data,
  );
}

Map<String, dynamic> _followersEnvelope(List<Map<String, dynamic>> followers, {
  int limit = 20,
}) {
  return {
    'followers': followers,
    'limit': limit,
  };
}

Map<String, dynamic> _followingEnvelope(List<Map<String, dynamic>> following, {
  int limit = 20,
}) {
  return {
    'following': following,
    'limit': limit,
  };
}

void main() {
  group('FollowListUserCardDto fail-closed contract', () {
    test('parses active card with username and avatar', () {
      final dto = FollowListUserCardDto.fromJson({
        'id': 'user-1',
        'username': 'alice',
        'avatar_url': 'https://cdn.example.com/a.jpg',
        'followers_count': 11,
        'following_count': 7,
        'lifecycle': 'active',
      });

      expect(dto.id, 'user-1');
      expect(dto.username, 'alice');
      expect(dto.avatarUrl, 'https://cdn.example.com/a.jpg');
      expect(dto.followersCount, 11);
      expect(dto.followingCount, 7);
      expect(dto.lifecycle, 'active');
    });

    test('parses active card with empty username', () {
      final dto = FollowListUserCardDto.fromJson({
        'id': 'user-2',
        'username': '',
        'avatar_url': null,
        'followers_count': 0,
        'following_count': 0,
        'lifecycle': 'active',
      });

      expect(dto.username, '');
      expect(dto.avatarUrl, isNull);
      expect(dto.lifecycle, 'active');
    });

    test('parses unavailable card with redacted identity', () {
      final dto = FollowListUserCardDto.fromJson({
        'id': 'user-3',
        'username': '',
        'avatar_url': null,
        'followers_count': 0,
        'following_count': 0,
        'lifecycle': 'unavailable',
      });

      expect(dto.username, '');
      expect(dto.avatarUrl, isNull);
      expect(dto.lifecycle, 'unavailable');
    });

    test('parses removed card with redacted identity', () {
      final dto = FollowListUserCardDto.fromJson({
        'id': 'user-4',
        'username': '',
        'avatar_url': null,
        'followers_count': 0,
        'following_count': 0,
        'lifecycle': 'removed',
      });

      expect(dto.username, '');
      expect(dto.avatarUrl, isNull);
      expect(dto.lifecycle, 'removed');
    });

    // --- Permissive lifecycle: production accepts null/missing/unknown ---

    test('accepts missing lifecycle (pre-S5 compat, defaults to null)', () {
      final dto = FollowListUserCardDto.fromJson({
        'id': 'user-5',
        'username': 'alice',
        'avatar_url': null,
        'followers_count': 0,
        'following_count': 0,
      });

      expect(dto.id, 'user-5');
      expect(dto.lifecycle, isNull);
    });

    test('accepts null lifecycle (pre-S5 compat)', () {
      final dto = FollowListUserCardDto.fromJson({
        'id': 'user-6',
        'username': 'alice',
        'avatar_url': null,
        'followers_count': 0,
        'following_count': 0,
        'lifecycle': null,
      });

      expect(dto.lifecycle, isNull);
    });

    test('rejects non-string lifecycle with TypeError', () {
      expect(
        () => FollowListUserCardDto.fromJson({
          'id': 'user-7',
          'username': 'alice',
          'avatar_url': null,
          'followers_count': 0,
          'following_count': 0,
          'lifecycle': 123,
        }),
        throwsA(isA<TypeError>()),
      );
    });

    test('accepts unknown lifecycle (production accepts any string)', () {
      final dto = FollowListUserCardDto.fromJson({
        'id': 'user-8',
        'username': 'alice',
        'avatar_url': null,
        'followers_count': 0,
        'following_count': 0,
        'lifecycle': 'mystery',
      });

      expect(dto.lifecycle, 'mystery');
    });

    // --- Type enforcement: null/missing id throws TypeError ---

    test('rejects missing id with TypeError', () {
      expect(
        () => FollowListUserCardDto.fromJson({
          'username': 'alice',
          'avatar_url': null,
          'followers_count': 0,
          'following_count': 0,
          'lifecycle': 'active',
        }),
        throwsA(isA<TypeError>()),
      );
    });

    test('accepts empty id (no length validation)', () {
      final dto = FollowListUserCardDto.fromJson({
        'id': '',
        'username': 'alice',
        'avatar_url': null,
        'followers_count': 0,
        'following_count': 0,
        'lifecycle': 'active',
      });

      expect(dto.id, '');
    });

    // --- Permissive username: production defaults null to '' ---

    test('accepts missing username (defaults to empty string)', () {
      final dto = FollowListUserCardDto.fromJson({
        'id': 'user-9',
        'avatar_url': null,
        'followers_count': 0,
        'following_count': 0,
        'lifecycle': 'active',
      });

      expect(dto.username, '');
    });

    test('rejects non-string username with TypeError', () {
      expect(
        () => FollowListUserCardDto.fromJson({
          'id': 'user-10',
          'username': 123,
          'avatar_url': null,
          'followers_count': 0,
          'following_count': 0,
          'lifecycle': 'active',
        }),
        throwsA(isA<TypeError>()),
      );
    });

    test('rejects wrong avatar type with TypeError', () {
      expect(
        () => FollowListUserCardDto.fromJson({
          'id': 'user-11',
          'username': 'alice',
          'avatar_url': 123,
          'followers_count': 0,
          'following_count': 0,
          'lifecycle': 'active',
        }),
        throwsA(isA<TypeError>()),
      );
    });

    test('rejects malformed counts with TypeError', () {
      expect(
        () => FollowListUserCardDto.fromJson({
          'id': 'user-12',
          'username': 'alice',
          'avatar_url': null,
          'followers_count': 'many',
          'following_count': 'few',
          'lifecycle': 'active',
        }),
        throwsA(isA<TypeError>()),
      );
    });

    test('followers wrapper parses active and redacted items', () {
      final dto = FollowListResponseDto.fromFollowersJson({
        'followers': [
          {
            'id': 'user-13',
            'username': 'alice',
            'avatar_url': 'https://cdn.example.com/a.jpg',
            'followers_count': 1,
            'following_count': 2,
            'lifecycle': 'active',
          },
          {
            'id': 'user-14',
            'username': '',
            'avatar_url': null,
            'followers_count': 0,
            'following_count': 0,
            'lifecycle': 'unavailable',
          },
        ],
        'limit': 20,
      });

      expect(dto.items, hasLength(2));
      expect(dto.items[0].username, 'alice');
      expect(dto.items[0].lifecycle, 'active');
      expect(dto.items[1].username, '');
      expect(dto.items[1].lifecycle, 'unavailable');
    });

    test('following wrapper parses active and redacted items', () {
      final dto = FollowListResponseDto.fromFollowingJson({
        'following': [
          {
            'id': 'user-15',
            'username': 'bob',
            'avatar_url': null,
            'followers_count': 3,
            'following_count': 4,
            'lifecycle': 'active',
          },
          {
            'id': 'user-16',
            'username': '',
            'avatar_url': null,
            'followers_count': 0,
            'following_count': 0,
            'lifecycle': 'removed',
          },
        ],
        'limit': 20,
      });

      expect(dto.items, hasLength(2));
      expect(dto.items[0].username, 'bob');
      expect(dto.items[0].lifecycle, 'active');
      expect(dto.items[1].username, '');
      expect(dto.items[1].lifecycle, 'removed');
    });
  });

  group('FollowApiDatasource envelope parsing', () {
    test('getFollowers parses canonical envelope and preserves empty username', () async {
      final client = _FakeApiClient((path, queryParameters) {
        return _response(
          path,
          {
            'success': true,
            'data': _followersEnvelope([
              {
                'id': 'user-20',
                'username': '',
                'avatar_url': null,
                'followers_count': 9,
                'following_count': 4,
                'lifecycle': 'active',
              },
            ], limit: 7),
          },
        );
      });
      final datasource = FollowApiDatasource(client);

      final result = await datasource.getFollowers(
        'user-20',
        limit: 7,
        cursor: '2026-07-23T10:00:00Z',
      );

      expect(client.lastPath, '/users/user-20/followers');
      expect(client.lastQueryParameters, {
        'limit': 7,
        'cursor': '2026-07-23T10:00:00Z',
      });
      expect(result.isSuccess, isTrue);
      expect(result.data, isNotNull);
      expect(result.data!.items, hasLength(1));
      expect(result.data!.items.first.id, 'user-20');
      expect(result.data!.items.first.username, '');
      expect(result.data!.items.first.avatarUrl, isNull);
      expect(result.data!.items.first.lifecycle, 'active');
    });

    test('getFollowing parses canonical envelope and preserves redaction', () async {
      final client = _FakeApiClient((path, queryParameters) {
        return _response(
          path,
          {
            'success': true,
            'data': _followingEnvelope([
              {
                'id': 'user-21',
                'username': '',
                'avatar_url': null,
                'followers_count': 0,
                'following_count': 0,
                'lifecycle': 'removed',
              },
            ], limit: 9),
          },
        );
      });
      final datasource = FollowApiDatasource(client);

      final result = await datasource.getFollowing(
        'user-21',
        limit: 9,
      );

      expect(client.lastPath, '/users/user-21/following');
      expect(client.lastQueryParameters, {'limit': 9});
      expect(result.isSuccess, isTrue);
      expect(result.data!.items.first.username, '');
      expect(result.data!.items.first.avatarUrl, isNull);
      expect(result.data!.items.first.lifecycle, 'removed');
    });

    test('backend error envelope returns Result.error', () async {
      final client = _FakeApiClient((path, queryParameters) {
        return _response(
          path,
          {
            'success': false,
            'error': {
              'code': 'FOLLOW_ERROR',
              'message': 'boom',
            },
          },
          statusCode: 400,
        );
      });
      final datasource = FollowApiDatasource(client);

      final result = await datasource.getFollowers('user-30');

      expect(result.isFailure, isTrue);
      expect(result.error, 'boom');
      expect(result.errorCode, 'FOLLOW_ERROR');
      expect(result.statusCode, 400);
    });

    test('missing lifecycle succeeds (pre-S5 compat, defaults to null)', () async {
      final client = _FakeApiClient((path, queryParameters) {
        return _response(
          path,
          {
            'success': true,
            'data': {
              'followers': [
                {
                  'id': 'user-40',
                  'username': 'alice',
                  'avatar_url': null,
                  'followers_count': 0,
                  'following_count': 0,
                },
              ],
              'limit': 20,
            },
          },
        );
      });
      final datasource = FollowApiDatasource(client);

      final result = await datasource.getFollowers('user-40');

      expect(result.isSuccess, isTrue);
      expect(result.data!.items.first.lifecycle, isNull);
    });

    test('null lifecycle succeeds (pre-S5 compat)', () async {
      final client = _FakeApiClient((path, queryParameters) {
        return _response(
          path,
          {
            'success': true,
            'data': {
              'following': [
                {
                  'id': 'user-41',
                  'username': 'alice',
                  'avatar_url': null,
                  'followers_count': 0,
                  'following_count': 0,
                  'lifecycle': null,
                },
              ],
              'limit': 20,
            },
          },
        );
      });
      final datasource = FollowApiDatasource(client);

      final result = await datasource.getFollowing('user-41');

      expect(result.isSuccess, isTrue);
      expect(result.data!.items.first.lifecycle, isNull);
    });

    test('malformed item fails with parse error', () async {
      final client = _FakeApiClient((path, queryParameters) {
        return _response(
          path,
          {
            'success': true,
            'data': {
              'followers': [
                {
                  'id': 'user-42',
                  'username': 'alice',
                  'avatar_url': null,
                  'followers_count': 'many',
                  'following_count': 0,
                  'lifecycle': 'active',
                },
              ],
              'limit': 20,
            },
          },
        );
      });
      final datasource = FollowApiDatasource(client);

      final result = await datasource.getFollowers('user-42');

      expect(result.isFailure, isTrue);
      expect(result.error, 'Invalid response data format');
    });

    test('unknown lifecycle succeeds (production accepts any string)', () async {
      final client = _FakeApiClient((path, queryParameters) {
        return _response(
          path,
          {
            'success': true,
            'data': {
              'followers': [
                {
                  'id': 'user-43',
                  'username': 'alice',
                  'avatar_url': null,
                  'followers_count': 0,
                  'following_count': 0,
                  'lifecycle': 'unknown',
                },
              ],
              'limit': 20,
            },
          },
        );
      });
      final datasource = FollowApiDatasource(client);

      final result = await datasource.getFollowers('user-43');

      expect(result.isSuccess, isTrue);
      expect(result.data!.items.first.lifecycle, 'unknown');
    });
  });
}
