import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/api/api_client.dart';
import 'package:labuda/domains/user/profile/data/datasources/user_api_datasource.dart';
import 'package:labuda/domains/user/profile/data/services/blocked_users_service.dart';
import 'package:labuda/domains/user/profile/data/services/user_lookup_service.dart';
import 'package:labuda/features/search/search/domain/entities/user_search.dart';

class _FakeApiClient implements ApiClient {
  _FakeApiClient(this._blockedIds);

  final List<String> _blockedIds;

  @override
  Future<Response<T>> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async {
    return Response<T>(
      requestOptions: RequestOptions(path: path),
      data:
          <String, dynamic>{
                'data': <String, dynamic>{'blocked': _blockedIds},
              }
              as T,
    );
  }

  @override
  noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _FakeUserLookupService extends UserLookupService {
  _FakeUserLookupService()
    : super(datasource: UserApiDatasource(_FakeApiClient(const [])));

  @override
  Future<List<UserSearch>> getUsersByIds(List<String> userIds) async {
    return userIds
        .map(
          (id) =>
              UserSearch(userId: id, username: id == 'u1' ? 'alice' : 'bob'),
        )
        .toList();
  }
}

void main() {
  test('BlockedUsersService hydrates @username from lookup', () async {
    final service = BlockedUsersService(
      _FakeApiClient(const ['u1', 'u2']),
      userLookupService: _FakeUserLookupService(),
    );

    final blockedUsers = await service.getBlockedUsers();

    expect(blockedUsers, hasLength(2));
    expect(blockedUsers[0].username, 'alice');
    expect(blockedUsers[1].username, 'bob');
  });
}
