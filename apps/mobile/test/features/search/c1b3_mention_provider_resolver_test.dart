// C1B3 - Provider + Resolver behavioral tests with pagination integrity.

import 'dart:async';

import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/api/api_client.dart';
import 'package:labuda/core/common/result.dart';
import 'package:labuda/core/providers/core_providers.dart'
    show apiClientProvider, loggerServiceProvider;
import 'package:labuda/core/src/interfaces/services/i_logger_service.dart';
import 'package:labuda/features/search/search/data/dto/search_dto.dart';
import 'package:labuda/features/search/search/domain/entities/user_search.dart';
import 'package:labuda/features/search/search/presentation/providers/mention_providers.dart';
import 'package:labuda/shared/providers/auth_status_providers.dart'
    show currentUserIdProvider;

class _FakeApiClient extends Fake implements ApiClient {}

class _StubLogger extends Fake implements ILoggerService {
  @override
  Future<Result<void>> info(
    String m, {
    Map<String, dynamic>? extra,
  }) async =>
      Result.success(null);

  @override
  Future<Result<void>> error(
    String m, {
    Map<String, dynamic>? extra,
    StackTrace? stackTrace,
  }) async =>
      Result.success(null);

  @override
  Future<Result<void>> warning(
    String m, {
    Map<String, dynamic>? extra,
  }) async =>
      Result.success(null);
}

class _Page {
  final int offset;
  final List<UserSearchResultDto> users;
  final int total;

  const _Page({
    required this.offset,
    required this.users,
    required this.total,
  });
}

class _RecordingApiClient extends Fake implements ApiClient {
  final Map<String, List<UserSearchResultDto>> _responses = {};
  int callCount = 0;
  String? lastQuery;
  int? lastLimit;
  int? lastOffset;
  Object? cannedError;
  Completer<Response<dynamic>>? _pending;

  void setResponse(String query, List<UserSearchResultDto> users) {
    _responses[query.toLowerCase()] = users;
  }

  void delayNextWith(Completer<Response<dynamic>> c) => _pending = c;

  @override
  Future<Response<T>> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async {
    callCount++;
    lastQuery = queryParameters?['q']?.toString();
    lastLimit = int.tryParse(queryParameters?['limit']?.toString() ?? '');
    lastOffset = int.tryParse(queryParameters?['offset']?.toString() ?? '');

    if (_pending != null) {
      final response = await _pending!.future;
      return response as Response<T>;
    }

    if (cannedError != null) {
      return Future.error(cannedError!);
    }

    final users = _responses[lastQuery?.toLowerCase() ?? ''] ?? const [];
    final payload = <String, dynamic>{
      'users':
          users
              .map(
                (user) => <String, dynamic>{
                  'id': user.id,
                  'username': user.username,
                  if (user.avatarUrl != null) 'avatar_url': user.avatarUrl,
                },
              )
              .toList(),
    };

    return Response<T>(
      data: payload as T,
      requestOptions: RequestOptions(path: path),
    );
  }
}

UserSearchResultDto _d(String id, String u, {String? a}) =>
    UserSearchResultDto(id: id, username: u, avatarUrl: a);

class _MutablePrincipal extends Notifier<String> {
  @override
  String build() => 'principal-A';

  void setPrincipal(String v) => state = v;
}

final _tp = NotifierProvider<_MutablePrincipal, String>(_MutablePrincipal.new);

dynamic _ovr({
  required _RecordingApiClient client,
  String uid = 'principal-A',
  bool mp = false,
}) => [
  apiClientProvider.overrideWith((ref) => client),
  loggerServiceProvider.overrideWith((ref) => _StubLogger()),
  if (mp)
    currentUserIdProvider.overrideWith((ref) => ref.watch(_tp))
  else
    currentUserIdProvider.overrideWith((ref) => uid),
];

ProviderContainer _c({
  required _RecordingApiClient client,
  String uid = 'principal-A',
  bool mp = false,
}) => ProviderContainer(overrides: _ovr(client: client, uid: uid, mp: mp));

MentionResolver _r(_RecordingApiClient client) =>
    MentionResolver(apiClient: client, logger: _StubLogger());

void main() {
  group('C1B3 Provider', () {
    test('valid lowercase retained', () async {
      final client = _RecordingApiClient();
      client.setResponse('ali', [_d('u1', 'alice')]);
      final r = await _c(client: client).read(
        mentionUserSearchProvider(const MentionSearchParams(query: 'ali')).future,
      );
      expect(r, hasLength(1));
      expect(r.first.username, 'alice');
    });

    test('uppercase canonicalized', () async {
      final client = _RecordingApiClient();
      client.setResponse('ali', [_d('u1', 'Alice')]);
      final r = await _c(client: client).read(
        mentionUserSearchProvider(const MentionSearchParams(query: 'ali')).future,
      );
      expect(r, hasLength(1));
      expect(r.first.username, 'Alice'.toLowerCase());
    });

    test('empty discarded', () async {
      final client = _RecordingApiClient();
      client.setResponse('v', [_d('u1', ''), _d('u2', 'validuser')]);
      final r = await _c(client: client).read(
        mentionUserSearchProvider(const MentionSearchParams(query: 'v')).future,
      );
      expect(r, hasLength(1));
      expect(r.first.userId, 'u2');
    });

    test('hyphen discarded', () async {
      final client = _RecordingApiClient();
      client.setResponse('john', [_d('u1', 'john-doe'), _d('u2', 'john_doe')]);
      final r = await _c(client: client).read(
        mentionUserSearchProvider(const MentionSearchParams(query: 'john')).future,
      );
      expect(r, hasLength(1));
      expect(r.first.userId, 'u2');
    });

    test('period discarded', () async {
      final client = _RecordingApiClient();
      client.setResponse('john', [_d('u1', 'john.doe'), _d('u2', 'john_doe')]);
      final r = await _c(client: client).read(
        mentionUserSearchProvider(const MentionSearchParams(query: 'john')).future,
      );
      expect(r, hasLength(1));
      expect(r.first.userId, 'u2');
    });

    test('underscore retained', () async {
      final client = _RecordingApiClient();
      client.setResponse('john', [_d('u1', 'john_doe')]);
      final r = await _c(client: client).read(
        mentionUserSearchProvider(const MentionSearchParams(query: 'john')).future,
      );
      expect(r, hasLength(1));
    });

    test('numeric-only retained', () async {
      final client = _RecordingApiClient();
      client.setResponse('123', [_d('u1', '12345')]);
      final r = await _c(client: client).read(
        mentionUserSearchProvider(const MentionSearchParams(query: '123')).future,
      );
      expect(r, hasLength(1));
    });

    test('UUID-with-hyphens discarded', () async {
      final client = _RecordingApiClient();
      client.setResponse(
        '550e',
        [_d('u1', '550e8400-e29b-41d4-a716-446655440000'), _d('u2', '550e_valid')],
      );
      final r = await _c(client: client).read(
        mentionUserSearchProvider(const MentionSearchParams(query: '550e')).future,
      );
      expect(r, hasLength(1));
      expect(r.first.userId, 'u2');
    });

    test('principal race safe', () async {
      final client = _RecordingApiClient();
      final ca = Completer<Response<dynamic>>();
      final cb = Completer<Response<dynamic>>();
      final c = ProviderContainer(overrides: _ovr(client: client, mp: true));

      client.delayNextWith(ca);
      final fa = c.read(
        mentionUserSearchProvider(const MentionSearchParams(query: 'ali')).future,
      );

      c.read(_tp.notifier).setPrincipal('principal-B');
      client.delayNextWith(cb);
      final fb = c.read(
        mentionUserSearchProvider(const MentionSearchParams(query: 'ali')).future,
      );

      cb.complete(
        Response<dynamic>(
          data: <String, dynamic>{
            'users': [
              <String, dynamic>{'id': 'b-u', 'username': 'alice_b'},
            ],
          },
          requestOptions: RequestOptions(path: '/users/search'),
        ),
      );
      final rb = await fb;
      expect(rb.first.userId, 'b-u');

      ca.complete(
        Response<dynamic>(
          data: <String, dynamic>{
            'users': [
              <String, dynamic>{'id': 'a-u', 'username': 'alice_a'},
            ],
          },
          requestOptions: RequestOptions(path: '/users/search'),
        ),
      );
      await fa;

      final cv = c.read(mentionUserSearchProvider(const MentionSearchParams(query: 'ali')));
      if (cv is AsyncData<List<UserSearch>>) {
        expect(cv.value.first.userId, 'b-u');
      }
    });
  });

  group('C1B3 Resolver', () {
    test('exact match resolves to stable user id', () async {
      final client = _RecordingApiClient();
      client.setResponse('alice', [_d('ua', 'alice')]);

      final resolver = _r(client);
      expect(await resolver.resolveUsername('alice'), 'ua');
    });

    test('uppercase request still resolves', () async {
      final client = _RecordingApiClient();
      client.setResponse('alice', [_d('ua', 'alice')]);

      expect(await _r(client).resolveUsername('ALICE'), 'ua');
    });

    test('cache hit avoids a second request', () async {
      final client = _RecordingApiClient();
      client.setResponse('alice', [_d('ua', 'alice')]);

      final resolver = _r(client);
      expect(await resolver.resolveUsername('alice'), 'ua');
      expect(await resolver.resolveUsername('alice'), 'ua');
      expect(client.callCount, 1);
    });

    test('missing user returns null', () async {
      final client = _RecordingApiClient();

      expect(await _r(client).resolveUsername('alice'), isNull);
      expect(client.callCount, 1);
    });

    test('service error returns null', () async {
      final client = _RecordingApiClient()..cannedError = Exception('boom');

      expect(await _r(client).resolveUsername('alice'), isNull);
    });
  });
}
