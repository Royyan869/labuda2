import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/profile/data/datasources/user_api_datasource.dart';
import 'package:labuda/domains/user/profile/data/repositories/profile_repository_api.dart';
import 'package:labuda/domains/user/profile/domain/entities/profile_entity.dart';
import 'package:labuda/domains/user/profile/data/models/api/user_api_models.dart';
import 'package:labuda/shared/services/logger_service.dart';

class _MockApiClient implements ApiClient {
  const _MockApiClient();

  @override
  Dio get dio => throw UnimplementedError();

  @override
  Future<Response<T>> delete<T>(
    String path, {
    dynamic data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async => throw UnimplementedError();

  @override
  ApiException extractException(DioException e) =>
      UnknownApiException(message: e.message ?? 'unknown');

  @override
  Future<Response<T>> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async => throw UnimplementedError();

  @override
  bool isNetworkError(DioException e) => false;

  @override
  bool isNotFound(DioException e) => false;

  @override
  bool isUnauthorized(DioException e) => false;

  @override
  bool isValidationError(DioException e) => false;

  @override
  Future<Response<T>> patch<T>(
    String path, {
    dynamic data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async => throw UnimplementedError();

  @override
  Future<Response<T>> post<T>(
    String path, {
    dynamic data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async => throw UnimplementedError();

  @override
  Future<Response<T>> put<T>(
    String path, {
    dynamic data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async => throw UnimplementedError();

  @override
  Future<Response<T>> uploadFile<T>(
    String path, {
    required String filePath,
    required String fieldName,
    Map<String, dynamic>? additionalFields,
    Options? options,
    CancelToken? cancelToken,
    void Function(int, int)? onSendProgress,
  }) async => throw UnimplementedError();
}

class _WatchProfileDatasource extends UserApiDatasource {
  _WatchProfileDatasource()
    : super(const _MockApiClient(), logger: LoggerService.instance);

  @override
  Future<Result<UserApiResponse>> getUserById(String userId) async {
    return Result.success(
      UserApiResponse.fromJson({
        'id': userId,
        'email': 'seller@example.com',
        'username': 'seller_user',
        'account_status': 'active',
        'roles': ['user'],
        'created_at': '2026-06-01T00:00:00.000Z',
        'updated_at': '2026-06-02T00:00:00.000Z',
        'profile': {
          'id': 'profile-$userId',
          'username': 'seller_user',
          'avatar_url': 'https://example.com/avatar.png',
          'cover_photo_url': 'images/profile-covers/$userId.jpg',
          'location': 'Bandung',
          'preferred_lang': 'id',
          'followers_count': 0,
          'following_count': 0,
        },
      }),
    );
  }
}

void main() {
  test('watchProfile emits one immediate snapshot and then completes', () async {
    final repository = ProfileRepositoryApi(
      datasource: _WatchProfileDatasource(),
      logger: LoggerService.instance,
    );

    final stopwatch = Stopwatch()..start();
    await expectLater(
      repository.watchProfile('user-1'),
      emitsInOrder(<Matcher>[
        predicate<ProfileEntity?>((value) {
          expect(value, isA<ProfileEntity>());
          expect(value?.coverPhotoUrl, 'images/profile-covers/user-1.jpg');
          return true;
        }),
        emitsDone,
      ]),
    );
    stopwatch.stop();

    expect(stopwatch.elapsedMilliseconds, lessThan(1000));
  });
}
