import 'dart:io';

import 'package:dio/dio.dart';
import 'package:firebase_auth/firebase_auth.dart' hide AuthProvider;
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/identity/authentication/data/repositories/auth_core_repository.dart';
import 'package:labuda/domains/user/profile/data/datasources/user_api_datasource.dart';
import 'package:labuda/domains/user/profile/data/models/api/user_api_models.dart';
import 'package:labuda/domains/user/profile/data/services/user_sync_service.dart';

class _MockFirebaseUser extends Fake implements User {
  @override
  String get uid => 'firebase-user-1';
}

class _MockFirebaseAuth extends Fake implements FirebaseAuth {
  _MockFirebaseAuth({this.currentUserValue});

  final User? currentUserValue;

  @override
  Stream<User?> authStateChanges() => const Stream<User?>.empty();

  @override
  User? get currentUser => currentUserValue;
}

class _StaticUserApiDatasource extends UserApiDatasource {
  _StaticUserApiDatasource(this.currentUserResponse) : super(_MockApiClient());

  final UserApiResponse currentUserResponse;

  @override
  Future<Result<UserApiResponse>> getCurrentUser() async {
    return Result.success(currentUserResponse);
  }
}

class _MockApiClient implements ApiClient {
  @override
  Dio get dio => throw UnimplementedError();

  @override
  Future<Response<T>> delete<T>(
    String path, {
    dynamic data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) {
    throw UnimplementedError();
  }

  @override
  ApiException extractException(DioException e) =>
      UnknownApiException(message: e.message ?? 'unknown');

  @override
  Future<Response<T>> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) {
    throw UnimplementedError();
  }

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
  }) {
    throw UnimplementedError();
  }

  @override
  Future<Response<T>> post<T>(
    String path, {
    dynamic data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) {
    throw UnimplementedError();
  }

  @override
  Future<Response<T>> put<T>(
    String path, {
    dynamic data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) {
    throw UnimplementedError();
  }

  @override
  Future<Response<T>> uploadFile<T>(
    String path, {
    required String filePath,
    required String fieldName,
    Map<String, dynamic>? additionalFields,
    Options? options,
    CancelToken? cancelToken,
    void Function(int, int)? onSendProgress,
  }) {
    throw UnimplementedError();
  }
}

AuthUser _baseUser({
  bool hasSellerProfile = true,
  bool hasMarketAuthority = false,
}) {
  return AuthUser(
    id: 'user-1',
    createdAt: DateTime.parse('2026-06-01T00:00:00Z'),
    updatedAt: DateTime.parse('2026-06-02T00:00:00Z'),
    email: 'seller@example.com',
    username: 'seller-one',
    isEmailVerified: true,
    roles: const [UserRole.user],
    provider: AuthProvider.email,
    hasSellerProfile: hasSellerProfile,
    sellerSubscriptionStatus: hasMarketAuthority ? 'active' : 'expired',
    hasMarketAuthority: hasMarketAuthority,
  );
}

UserApiResponse _usersMeResponse({
  required String id,
  required bool hasSellerProfile,
  required bool hasMarketAuthority,
}) {
  return UserApiResponse.fromJson({
    'id': id,
    'email': 'seller@example.com',
    'username': 'seller-one',
    'account_status': 'active',
    'roles': ['seller'],
    'has_seller_profile': hasSellerProfile,
    'seller_subscription_status': hasMarketAuthority ? 'active' : 'expired',
    'has_market_authority': hasMarketAuthority,
    'is_email_verified': true,
    'created_at': '2026-06-01T00:00:00Z',
    'updated_at': '2026-06-02T00:00:00Z',
    'profile': {
      'id': id,
      'username': 'seller-one',
      'bio': 'bio',
      'avatar_url': 'https://example.com/avatar.png',
      'followers_count': 1,
      'following_count': 2,
      'preferred_lang': 'en',
    },
  });
}

void main() {
  test('AuthStateAuthenticated stores AuthUser directly', () {
    final user = _baseUser();
    final state = AuthState.authenticated(user, emailVerified: true);

    expect(state, isA<AuthStateAuthenticated>());
    expect((state as AuthStateAuthenticated).user, same(user));
    expect(state.user.runtimeType, AuthUser);
  });

  test('AuthUser has no firebaseUid and deleted snapshot files stay absent', () {
    final user = _baseUser();

    expect(
      () => (user as dynamic).firebaseUid,
      throwsA(isA<NoSuchMethodError>()),
    );
    expect(
      File(
        'lib/domains/user/identity/authentication/data/models/auth_user_model.dart',
      ).existsSync(),
      isFalse,
    );
    expect(
      File(
        'lib/domains/user/identity/authentication/domain/entities/current_account_snapshot.dart',
      ).existsSync(),
      isFalse,
    );
  });

  test('Firebase exchange responses expose session fields only', () {
    final incomplete = FirebaseExchangeResponse.fromJson({
      'user_id': 'user-1',
      'access_token': 'restricted-token',
      'expires_at': '2026-06-14T00:00:00Z',
      'requires_profile_completion': true,
      'created': false,
      'email': 'seller@example.com',
    });

    final complete = FirebaseExchangeCompleteResponse.fromJson({
      'user_id': 'user-1',
      'access_token': 'access-token',
      'refresh_token': 'refresh-token',
      'expires_at': '2026-06-14T00:00:00Z',
      'refresh_expires_at': '2026-07-14T00:00:00Z',
      'requires_profile_completion': false,
      'created': true,
    });

    expect(
      () => (incomplete as dynamic).user,
      throwsA(isA<NoSuchMethodError>()),
    );
    expect(
      () => (complete as dynamic).account,
      throwsA(isA<NoSuchMethodError>()),
    );
    expect(incomplete.userId, 'user-1');
    expect(complete.userId, 'user-1');
    expect(complete.refreshToken, 'refresh-token');
  });

  test(
    'AuthCoreRepository does not synthesize accounts from Firebase principal',
    () async {
      final repo = AuthCoreRepository(
        firebaseAuth: _MockFirebaseAuth(currentUserValue: _MockFirebaseUser()),
      );

      final result = await repo.getCurrentUser();

      expect(result.isSuccess, isTrue);
      expect(result.data, isNull);
    },
  );

  test(
    '/users/me mapping produces AuthUser and seller flags stay independent',
    () async {
      final response = _usersMeResponse(
        id: 'user-1',
        hasSellerProfile: true,
        hasMarketAuthority: false,
      );
      final service = UserSyncService(
        firebaseAuth: _MockFirebaseAuth(currentUserValue: _MockFirebaseUser()),
        datasource: _StaticUserApiDatasource(response),
      );

      final result = await service.getCurrentUser();

      expect(result.isSuccess, isTrue);
      final user = result.data!;
      expect(user, isA<AuthUser>());
      expect(user.hasSellerProfile, isTrue);
      expect(user.hasMarketAuthority, isFalse);
      expect(user.hasCreatedSellerProfile, isTrue);
      expect(user.isSeller, isFalse);

      final marketOnly = user.copyWith(
        hasSellerProfile: false,
        hasMarketAuthority: true,
      );
      expect(marketOnly.hasCreatedSellerProfile, isFalse);
      expect(marketOnly.isSeller, isTrue);
    },
  );

  test('hasSellerProfile and hasMarketAuthority remain independent', () {
    final sellerOnly = _baseUser(
      hasSellerProfile: true,
      hasMarketAuthority: false,
    );
    final marketOnly = _baseUser(
      hasSellerProfile: false,
      hasMarketAuthority: true,
    );

    expect(sellerOnly.hasCreatedSellerProfile, isTrue);
    expect(sellerOnly.isSeller, isFalse);
    expect(marketOnly.hasCreatedSellerProfile, isFalse);
    expect(marketOnly.isSeller, isTrue);
  });
}
